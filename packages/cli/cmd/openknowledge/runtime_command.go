package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type runtimePlan struct {
	SchemaVersion  string                          `json:"schemaVersion"`
	Config         string                          `json:"config"`
	StateDir       string                          `json:"stateDir"`
	ArtifactStore  okruntime.ArtifactStoreConfig   `json:"artifactStore"`
	Serve          okruntime.ServeConfig           `json:"serve"`
	Worker         okruntime.WorkerConfig          `json:"worker"`
	GitHub         okruntime.GitHubConfig          `json:"github"`
	KnowledgeBases []okruntime.KnowledgeBaseConfig `json:"knowledgeBases"`
}

type runtimeBuildResult struct {
	SchemaVersion string                   `json:"schemaVersion"`
	KnowledgeBase string                   `json:"knowledgeBase"`
	Generation    string                   `json:"generation"`
	Commit        string                   `json:"commit"`
	ContentDigest string                   `json:"contentDigest"`
	Output        string                   `json:"output"`
	Published     *okruntime.ActivePointer `json:"published,omitempty"`
}

func runRuntime(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, runtimeHelpText())
		return 0
	}
	switch args[0] {
	case "plan":
		return runRuntimePlan(args[1:])
	case "build":
		return runRuntimeBuild(args[1:])
	case "serve":
		return runRuntimeServe(args[1:])
	case "worker":
		return runRuntimeWorker(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown runtime subcommand: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, runtimeHelpText())
		return 2
	}
}

func runRuntimePlan(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimePlanHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime plan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "runtime plan accepts no positional arguments")
		return 2
	}
	config, err := okruntime.LoadConfig(*configPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	plan := runtimePlan{
		SchemaVersion:  okf.MachineSchemaVersion,
		Config:         config.Path,
		StateDir:       config.Runtime.StateDir,
		ArtifactStore:  config.ArtifactStore,
		Serve:          config.Serve,
		Worker:         config.Worker,
		GitHub:         config.GitHub,
		KnowledgeBases: config.KnowledgeBases,
	}
	if err := printJSON(plan); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runRuntimeBuild(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeBuildHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "build only this knowledge base")
	commit := flags.String("commit", "", "source commit identity")
	out := flags.String("out", "", "generation output directory (single knowledge base only)")
	noPublish := flags.Bool("no-publish", false, "build without promoting to the artifact store")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "runtime build accepts no positional arguments")
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
	if *out != "" && len(selected) != 1 {
		fmt.Fprintln(os.Stderr, "--out requires exactly one selected knowledge base")
		return 2
	}
	resolvedCommit := strings.TrimSpace(*commit)
	if resolvedCommit == "" {
		resolvedCommit, err = runtimeGitOutput(config.Worker.Repo, "rev-parse", "HEAD")
		if err != nil {
			return printAgentCommandError(fmt.Errorf("resolve source commit (or pass --commit): %w", err))
		}
	}
	results := make([]runtimeBuildResult, 0, len(selected))
	for _, knowledge := range selected {
		output := *out
		if output == "" {
			output = filepath.Join(config.Runtime.StateDir, "builds", knowledge.ID)
		}
		result, err := buildRuntimeKnowledgeGeneration(config, knowledge, resolvedCommit, output, !*noPublish)
		if err != nil {
			return printAgentCommandError(fmt.Errorf("build knowledge base %s: %w", knowledge.ID, err))
		}
		results = append(results, result)
	}
	if len(results) == 1 {
		if err := printJSON(results[0]); err != nil {
			return printAgentCommandError(err)
		}
	} else if err := printJSON(map[string]any{"schemaVersion": okf.MachineSchemaVersion, "generations": results}); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func buildRuntimeKnowledgeGeneration(config okruntime.Config, knowledge okruntime.KnowledgeBaseConfig, commit string, out string, publish bool) (runtimeBuildResult, error) {
	if !knowledge.Publish {
		return runtimeBuildResult{}, fmt.Errorf("knowledge base is configured with publish = false")
	}
	if publish && config.ArtifactStore.Type != "filesystem" {
		return runtimeBuildResult{}, fmt.Errorf("runtime build can promote only to a filesystem artifact store")
	}
	absoluteOut, err := okf.WriteDirectoryAtomically(out, func(staging string) error {
		public := filepath.Join(staging, "public")
		if _, err := writeViewerHTMLWithVersion(knowledge.Path, public, knowledge.Spec); err != nil {
			return err
		}
		source := filepath.Join(staging, "source")
		archive := filepath.Join(public, filepath.FromSlash(okf.BundleArchiveRelPath))
		if err := okf.ExtractBundleArchive(archive, source); err != nil {
			return err
		}
		for _, projection := range []struct {
			name   string
			target okf.PublicationTarget
		}{
			{name: "search", target: okf.PublicationTargetSearch},
			{name: "mcp", target: okf.PublicationTargetMCP},
		} {
			projectionArchive := filepath.Join(staging, "."+projection.name+".tar.gz")
			if _, err := okf.WritePublishedTargetBundleTarGzipWithVersion(knowledge.Path, projectionArchive, knowledge.Spec, []string{out, staging}, projection.target); err != nil {
				return err
			}
			if err := okf.ExtractBundleArchive(projectionArchive, filepath.Join(staging, projection.name)); err != nil {
				return err
			}
			if err := os.Remove(projectionArchive); err != nil {
				return err
			}
		}
		_, err := okruntime.WriteGenerationManifest(staging, knowledge.ID, commit, knowledge.Spec)
		return err
	})
	if err != nil {
		return runtimeBuildResult{}, err
	}
	manifest, err := okruntime.LoadAndValidateGeneration(absoluteOut)
	if err != nil {
		return runtimeBuildResult{}, err
	}
	result := runtimeBuildResult{
		SchemaVersion: okf.MachineSchemaVersion,
		KnowledgeBase: knowledge.ID,
		Generation:    okruntime.GenerationName(manifest),
		Commit:        manifest.Commit,
		ContentDigest: manifest.ContentDigest,
		Output:        absoluteOut,
	}
	if publish {
		store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
		pointer, _, err := store.Publish(absoluteOut)
		if err != nil {
			return runtimeBuildResult{}, err
		}
		result.Published = &pointer
	}
	return result, nil
}

func selectRuntimeKnowledgeBases(config okruntime.Config, id string) ([]okruntime.KnowledgeBaseConfig, error) {
	var selected []okruntime.KnowledgeBaseConfig
	for _, knowledge := range config.KnowledgeBases {
		if !knowledge.Publish || (id != "" && knowledge.ID != id) {
			continue
		}
		selected = append(selected, knowledge)
	}
	if len(selected) == 0 {
		if id != "" {
			return nil, fmt.Errorf("published knowledge base not found: %s", id)
		}
		return nil, fmt.Errorf("runtime has no published knowledge bases")
	}
	return selected, nil
}

func runtimeGitOutput(repo string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func runtimeHelpText() string {
	return `openknowledge runtime

Run isolated public serving and private maintenance roles.

Usage:
  openknowledge runtime plan --config runtime.toml
  openknowledge runtime build --config runtime.toml [--id <id>] [--commit <sha>]
  openknowledge runtime serve --config runtime.toml
  openknowledge runtime worker --role publisher --config runtime.toml
  openknowledge runtime worker --role agents --config runtime.toml

The public serve role reads only verified immutable generations from the
artifact store. Private publisher and agent roles use isolated state and no
inbound ports; GitHub and model credentials are never mounted into one role.
`
}

func runtimePlanHelpText() string {
	return "openknowledge runtime plan --config runtime.toml\n\nValidate strict runtime configuration and print its normalized execution plan as JSON.\n"
}

func runtimeBuildHelpText() string {
	return `openknowledge runtime build --config runtime.toml

Build deterministic filtered public generations and atomically promote them to
the configured filesystem artifact store. Use --no-publish for local inspection.
`
}
