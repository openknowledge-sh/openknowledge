package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type runtimeRelease struct {
	Generation    string   `json:"generation"`
	Commit        string   `json:"commit"`
	Spec          string   `json:"spec"`
	ContentDigest string   `json:"contentDigest"`
	Checks        []string `json:"checks"`
	Files         int      `json:"files"`
	Active        bool     `json:"active"`
}

type runtimeReleasesResult struct {
	SchemaVersion      string           `json:"schemaVersion"`
	KnowledgeBase      string           `json:"knowledgeBase"`
	ActiveGeneration   string           `json:"activeGeneration,omitempty"`
	PreviousGeneration string           `json:"previousGeneration,omitempty"`
	Releases           []runtimeRelease `json:"releases"`
}

type runtimeReleaseActionResult struct {
	SchemaVersion      string `json:"schemaVersion"`
	Action             string `json:"action"`
	KnowledgeBase      string `json:"knowledgeBase"`
	PreviousGeneration string `json:"previousGeneration,omitempty"`
	Generation         string `json:"generation"`
	ContentDigest      string `json:"contentDigest"`
	Address            string `json:"address,omitempty"`
}

func runRuntimeReleases(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeReleasesHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime releases", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "knowledge base ID")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "runtime releases accepts no positional arguments")
		return 2
	}
	_, knowledge, store, err := runtimeReleaseStore(*configPath, *knowledgeID)
	if err != nil {
		return printAgentCommandError(err)
	}
	stored, err := store.Releases(knowledge.ID)
	if err != nil {
		return printAgentCommandError(err)
	}
	result := runtimeReleasesResult{SchemaVersion: okf.MachineSchemaVersion, KnowledgeBase: knowledge.ID, Releases: []runtimeRelease{}}
	if active, _, activeErr := store.Active(knowledge.ID); activeErr == nil {
		result.ActiveGeneration = active.Generation
		result.PreviousGeneration = active.PreviousGeneration
	} else if !os.IsNotExist(activeErr) {
		return printAgentCommandError(activeErr)
	}
	for _, release := range stored {
		result.Releases = append(result.Releases, runtimeRelease{
			Generation: release.Name, Commit: release.Manifest.Commit, Spec: release.Manifest.Spec,
			ContentDigest: release.Manifest.ContentDigest, Checks: nonNilStrings(release.Manifest.Checks), Files: len(release.Manifest.Files), Active: release.Active,
		})
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runRuntimePin(args []string) int {
	return runRuntimeReleaseChange("pin", args)
}

func runRuntimeRollback(args []string) int {
	return runRuntimeReleaseChange("rollback", args)
}

func runRuntimeReleaseChange(action string, args []string) int {
	if hasHelpFlag(args) {
		if action == "pin" {
			fmt.Fprint(os.Stdout, runtimePinHelpText())
		} else {
			fmt.Fprint(os.Stdout, runtimeRollbackHelpText())
		}
		return 0
	}
	flags := flag.NewFlagSet("runtime "+action, flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "knowledge base ID")
	generation := flags.String("generation", "", "stored generation name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || (action == "pin" && *generation == "") {
		fmt.Fprintf(stderrOutput(), "runtime %s requires --generation and accepts no positional arguments\n", action)
		return 2
	}
	config, knowledge, store, err := runtimeReleaseStore(*configPath, *knowledgeID)
	if err != nil {
		return printAgentCommandError(err)
	}
	previous := ""
	if active, _, activeErr := store.Active(knowledge.ID); activeErr == nil {
		previous = active.Generation
	} else if !os.IsNotExist(activeErr) {
		return printAgentCommandError(activeErr)
	}
	target := *generation
	implicitRollback := action == "rollback" && *generation == ""
	var pointer okruntime.ActivePointer
	if implicitRollback {
		pointer, _, err = store.Rollback(knowledge.ID, "")
	} else {
		manifest, _, generationErr := store.Generation(knowledge.ID, target)
		if generationErr != nil {
			return printAgentCommandError(generationErr)
		}
		if !equalStringLists(manifest.Checks, config.GitHub.RequiredChecks) {
			return printAgentCommandError(fmt.Errorf("generation does not carry the configured required checks: %v", config.GitHub.RequiredChecks))
		}
		if action == "pin" {
			pointer, _, err = store.Pin(knowledge.ID, target)
		} else {
			pointer, _, err = store.Rollback(knowledge.ID, target)
		}
	}
	if err != nil {
		return printAgentCommandError(err)
	}
	result := runtimeReleaseActionResult{
		SchemaVersion: okf.MachineSchemaVersion, Action: action, KnowledgeBase: knowledge.ID,
		PreviousGeneration: previous, Generation: pointer.Generation, ContentDigest: pointer.ContentDigest,
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runRuntimePreview(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimePreviewHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime preview", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	knowledgeID := flags.String("id", "", "knowledge base ID")
	generation := flags.String("generation", "", "stored generation name")
	address := flags.String("address", "127.0.0.1:8081", "preview listen address")
	check := flags.Bool("check", false, "validate the preview generation and print its descriptor")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *generation == "" {
		fmt.Fprintln(stderrOutput(), "runtime preview requires --generation and accepts no positional arguments")
		return 2
	}
	config, knowledge, store, err := runtimeReleaseStore(*configPath, *knowledgeID)
	if err != nil {
		return printAgentCommandError(err)
	}
	manifest, root, err := store.Generation(knowledge.ID, *generation)
	if err != nil {
		return printAgentCommandError(err)
	}
	pointer := okruntime.ActivePointer{
		Type: okruntime.ActivePointerType, Version: okruntime.GenerationManifestVersion, KnowledgeBaseID: knowledge.ID,
		Generation: *generation, ContentDigest: manifest.ContentDigest,
	}
	config.KnowledgeBases = []okruntime.KnowledgeBaseConfig{knowledge}
	config.Serve.Address = *address
	config.Serve.UsageEvents.Enabled = false
	config.Serve.UsageEvents.CaptureQueries = false
	manager := newRuntimeSnapshotManager(config)
	snapshot, err := manager.loadSnapshot(knowledge, pointer, root)
	if err != nil {
		return printAgentCommandError(err)
	}
	result := runtimeReleaseActionResult{
		SchemaVersion: okf.MachineSchemaVersion, Action: "preview", KnowledgeBase: knowledge.ID,
		Generation: *generation, ContentDigest: manifest.ContentDigest, Address: *address,
	}
	if *check {
		if err := printJSON(result); err != nil {
			return printAgentCommandError(err)
		}
		return 0
	}
	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		return printAgentCommandError(err)
	}
	handler.preview = true
	handler.snapshots.mu.Lock()
	handler.snapshots.active = map[string]runtimeGenerationSnapshot{knowledge.ID: snapshot}
	handler.snapshots.knowledge = []okruntime.KnowledgeBaseConfig{knowledge}
	handler.snapshots.mu.Unlock()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtimeInfof("runtime preview serving %s generation %s on %s\n", knowledge.ID, *generation, *address)
	if err := serveRuntimeHTTP(ctx, handler, *address, config.Serve.RequestTimeout); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func runtimeReleaseStore(configPath string, knowledgeID string) (okruntime.Config, okruntime.KnowledgeBaseConfig, okruntime.FilesystemStore, error) {
	config, err := okruntime.LoadConfig(configPath)
	if err != nil {
		return okruntime.Config{}, okruntime.KnowledgeBaseConfig{}, okruntime.FilesystemStore{}, err
	}
	if config.ArtifactStore.Type != "filesystem" {
		return okruntime.Config{}, okruntime.KnowledgeBaseConfig{}, okruntime.FilesystemStore{}, fmt.Errorf("runtime release control requires a filesystem artifact store")
	}
	selected, err := selectRuntimeKnowledgeBases(config, knowledgeID)
	if err != nil {
		return okruntime.Config{}, okruntime.KnowledgeBaseConfig{}, okruntime.FilesystemStore{}, err
	}
	if len(selected) != 1 {
		return okruntime.Config{}, okruntime.KnowledgeBaseConfig{}, okruntime.FilesystemStore{}, fmt.Errorf("--id is required when multiple knowledge bases are published")
	}
	return config, selected[0], okruntime.FilesystemStore{Root: config.ArtifactStore.Path}, nil
}

func runtimeReleasesHelpText() string {
	return "openknowledge automation runtime releases --config runtime.toml [--id <id>]\n\nList verified immutable generations and the production pin as JSON.\n"
}

func runtimePinHelpText() string {
	return "openknowledge automation runtime pin --config runtime.toml [--id <id>] --generation <name>\n\nAtomically pin production to a verified stored generation.\n"
}

func runtimeRollbackHelpText() string {
	return "openknowledge automation runtime rollback --config runtime.toml [--id <id>] [--generation <name>]\n\nAtomically return production to the previous pin, or to an explicit verified generation.\n"
}

func runtimePreviewHelpText() string {
	return "openknowledge automation runtime preview --config runtime.toml [--id <id>] --generation <name> [--address 127.0.0.1:8081] [--check]\n\nServe one verified stored generation without changing the production pin. Preview usage is not written to production usage logs.\n"
}
