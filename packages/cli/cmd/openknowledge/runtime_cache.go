package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type runtimeCacheEntry struct {
	KnowledgeBase string `json:"knowledgeBase"`
	Generation    string `json:"generation"`
	Target        string `json:"target"`
	State         string `json:"state"`
	IndexSHA256   string `json:"indexSha256,omitempty"`
	Sections      int    `json:"sections,omitempty"`
	Path          string `json:"path"`
	Error         string `json:"error,omitempty"`
}

type runtimeCacheRemoval struct {
	KnowledgeBase string `json:"knowledgeBase"`
	Generation    string `json:"generation"`
}

type runtimeCacheResult struct {
	SchemaVersion string                `json:"schemaVersion"`
	Action        string                `json:"action"`
	Applied       *bool                 `json:"applied,omitempty"`
	Entries       []runtimeCacheEntry   `json:"entries"`
	Removed       []runtimeCacheRemoval `json:"removed"`
}

func runRuntimeCache(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, runtimeCacheHelpText())
		return 0
	}
	switch args[0] {
	case "status":
		return runRuntimeCacheInspect("status", args[1:])
	case "rebuild":
		return runRuntimeCacheInspect("rebuild", args[1:])
	case "prune":
		return runRuntimeCachePrune(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown runtime cache subcommand: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), runtimeCacheHelpText())
		return 2
	}
}

func runRuntimeCacheInspect(action string, args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeCacheHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime cache "+action, flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "knowledge base ID")
	generation := flags.String("generation", "", "stored generation name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderrOutput(), "runtime cache %s accepts no positional arguments\n", action)
		return 2
	}
	config, err := okruntime.LoadConfig(*configPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	selected, err := selectRuntimeKnowledgeBases(config, *knowledgeID)
	if err != nil {
		return printAgentCommandError(err)
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	cache := okruntime.IndexCache{Root: filepath.Join(config.Runtime.StateDir, "indexes")}
	result := runtimeCacheResult{SchemaVersion: okf.MachineSchemaVersion, Action: action, Entries: []runtimeCacheEntry{}, Removed: []runtimeCacheRemoval{}}
	for _, knowledge := range selected {
		releases, err := store.Releases(knowledge.ID)
		if err != nil {
			return printAgentCommandError(err)
		}
		for _, release := range releases {
			if *generation != "" && release.Name != *generation {
				continue
			}
			for _, target := range runtimeIndexTargets(knowledge) {
				projection := runtimeProjectionRoot(release.Root, target)
				path, _ := cache.Path(knowledge.ID, release.Name, target)
				entry := runtimeCacheEntry{KnowledgeBase: knowledge.ID, Generation: release.Name, Target: target, Path: path}
				if action == "rebuild" {
					index, buildErr := okf.BuildContextIndexWithVersion(projection, release.Manifest.Spec)
					if buildErr != nil {
						return printAgentCommandError(fmt.Errorf("rebuild %s %s %s index: %w", knowledge.ID, release.Name, target, buildErr))
					}
					if _, storeErr := cache.Store(knowledge.ID, release.Name, release.Manifest.ContentDigest, target, index); storeErr != nil {
						return printAgentCommandError(storeErr)
					}
					entry.State, entry.IndexSHA256, entry.Sections = "rebuilt", index.Revision.IndexSHA256, len(index.Sections)
				} else {
					index, loadErr := cache.Load(knowledge.ID, release.Name, release.Manifest.ContentDigest, release.Manifest.Spec, target, projection)
					switch {
					case loadErr == nil:
						entry.State, entry.IndexSHA256, entry.Sections = "ready", index.Revision.IndexSHA256, len(index.Sections)
					case os.IsNotExist(loadErr):
						entry.State = "missing"
					default:
						entry.State, entry.Error = "invalid", loadErr.Error()
					}
				}
				result.Entries = append(result.Entries, entry)
			}
		}
	}
	if *generation != "" && len(result.Entries) == 0 {
		return printAgentCommandError(fmt.Errorf("stored generation not found: %s", *generation))
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runRuntimeCachePrune(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeCacheHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime cache prune", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "knowledge base ID")
	apply := flags.Bool("apply", false, "remove stale cache generations")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "runtime cache prune accepts no positional arguments")
		return 2
	}
	config, err := okruntime.LoadConfig(*configPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	selected, err := selectRuntimeKnowledgeBases(config, *knowledgeID)
	if err != nil {
		return printAgentCommandError(err)
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	cache := okruntime.IndexCache{Root: filepath.Join(config.Runtime.StateDir, "indexes")}
	applied := *apply
	result := runtimeCacheResult{SchemaVersion: okf.MachineSchemaVersion, Action: "prune", Applied: &applied, Entries: []runtimeCacheEntry{}, Removed: []runtimeCacheRemoval{}}
	for _, knowledge := range selected {
		releases, err := store.Releases(knowledge.ID)
		if err != nil {
			return printAgentCommandError(err)
		}
		keep := make(map[string]bool, len(releases))
		for _, release := range releases {
			keep[release.Name] = true
		}
		removed, err := cache.Prune(knowledge.ID, keep, *apply)
		if err != nil {
			return printAgentCommandError(err)
		}
		for _, generation := range removed {
			result.Removed = append(result.Removed, runtimeCacheRemoval{KnowledgeBase: knowledge.ID, Generation: generation})
		}
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runtimeIndexTargets(knowledge okruntime.KnowledgeBaseConfig) []string {
	targets := []string{}
	if knowledge.HasOutput(okf.ReleaseOutputViewer) {
		targets = append(targets, okruntime.IndexTargetSearch)
	}
	if knowledge.HasOutput(okf.ReleaseOutputMCP) {
		targets = append(targets, okruntime.IndexTargetMCP)
	}
	return targets
}

func runtimeCacheHelpText() string {
	return `openknowledge automation runtime cache <status|rebuild|prune> --config runtime.toml

Inspect persistent generation-bound search and MCP indexes, rebuild selected
indexes, or preview stale cache removal. Prune is dry-run unless --apply is set.
`
}
