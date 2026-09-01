package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type publishPlan struct {
	Root          string
	SpecVersion   string
	Targets       []string
	ViewerOut     string
	RuntimeConfig string
	RuntimeID     string
	Files         map[string]int
}

func runPublish(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, publishHelpText())
		return 0
	}
	args = lifecycleFlagsAfterPath(args)
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	target := flags.String("target", "", "publication target: viewer or mcp (default: configured outputs)")
	planOnly := flags.Bool("plan", false, "print the publication plan without writing")
	out := flags.String("out", "", "viewer output directory")
	runtimeConfig := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration for MCP publication")
	runtimeID := flags.String("id", "", "runtime knowledge-base ID for MCP publication")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "publish accepts at most one knowledge base path")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	resolved, err := resolveWhereTarget(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	plan, err := buildPublishPlan(resolved, *target, *out, *runtimeConfig, *runtimeID)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	printPublishPlan(plan)
	if *planOnly {
		return 0
	}
	for _, publicationTarget := range plan.Targets {
		switch publicationTarget {
		case okf.ReleaseOutputViewer:
			if code := runExportHTML([]string{plan.Root, "--out", plan.ViewerOut}); code != 0 {
				return code
			}
		case okf.ReleaseOutputMCP:
			runtimeArgs := []string{"--config", plan.RuntimeConfig}
			if plan.RuntimeID != "" {
				runtimeArgs = append(runtimeArgs, "--id", plan.RuntimeID)
			}
			if code := runRuntimeBuild(runtimeArgs); code != 0 {
				return code
			}
		}
	}
	return 0
}

func buildPublishPlan(root, requestedTarget, out, runtimeConfig, runtimeID string) (publishPlan, error) {
	if !setupExistingBundle(root) {
		return publishPlan{}, fmt.Errorf("publication requires a managed OKF knowledge base; run okn setup %q first", root)
	}
	report, err := buildKnowledgeCheckReport(root)
	if err != nil {
		return publishPlan{}, err
	}
	if report.Overall == checkBlocked || report.Overall == checkUnmanaged {
		return publishPlan{}, fmt.Errorf("publication is blocked by okn check (%s)", report.Overall)
	}
	publicationReady := false
	for _, layer := range report.Layers {
		if layer.Name == "Publication" {
			if layer.Status != checkReady {
				return publishPlan{}, fmt.Errorf("publication is not ready: %s", layer.Message)
			}
			publicationReady = true
			break
		}
	}
	if !publicationReady {
		return publishPlan{}, fmt.Errorf("publication readiness was not checked")
	}
	config, err := okf.LoadProjectConfig(root)
	if err != nil {
		return publishPlan{}, err
	}
	targets := append([]string(nil), config.Release.Outputs...)
	if requested := strings.ToLower(strings.TrimSpace(requestedTarget)); requested != "" {
		if requested != okf.ReleaseOutputViewer && requested != okf.ReleaseOutputMCP {
			return publishPlan{}, fmt.Errorf("unsupported publication target: %s", requestedTarget)
		}
		targets = []string{requested}
	}
	if len(targets) == 0 {
		return publishPlan{}, fmt.Errorf("publication is disabled; configure release.outputs in %s", okf.ValidationConfigFile)
	}
	version := report.SpecVersion
	plan := publishPlan{Root: root, SpecVersion: version, Targets: targets, RuntimeConfig: runtimeConfig, RuntimeID: strings.TrimSpace(runtimeID), Files: map[string]int{}}
	if strings.TrimSpace(out) == "" {
		plan.ViewerOut = filepath.Join(root, ".openknowledge", "publish", "viewer")
	} else {
		absoluteOut, err := filepath.Abs(out)
		if err != nil {
			return publishPlan{}, err
		}
		plan.ViewerOut = absoluteOut
	}
	for _, target := range targets {
		set, err := okf.BuildPublicationSetForTargetWithVersion(root, version, okf.PublicationTarget(target))
		if err != nil {
			return publishPlan{}, err
		}
		plan.Files[target] = len(set.Markdown) + len(set.Assets)
		if target == okf.ReleaseOutputMCP {
			resolvedID, err := resolvePublishRuntimeTarget(plan)
			if err != nil {
				return publishPlan{}, err
			}
			plan.RuntimeID = resolvedID
		}
	}
	return plan, nil
}

func resolvePublishRuntimeTarget(plan publishPlan) (string, error) {
	config, err := okruntime.LoadConfig(plan.RuntimeConfig)
	if err != nil {
		return "", fmt.Errorf("MCP publication requires a runtime config: %w", err)
	}
	selected, err := selectRuntimeKnowledgeBases(config, plan.RuntimeID)
	if err != nil {
		return "", err
	}
	for _, knowledge := range selected {
		if samePublishPath(knowledge.Path, plan.Root) && knowledge.HasOutput(okf.ReleaseOutputMCP) {
			return knowledge.ID, nil
		}
	}
	return "", fmt.Errorf("runtime config does not select %s with MCP output", plan.Root)
}

func samePublishPath(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func printPublishPlan(plan publishPlan) {
	fmt.Fprintln(os.Stdout, "Publication plan")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "Knowledge base:", plan.Root)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "OKF version:", plan.SpecVersion)
	for _, target := range plan.Targets {
		destination := plan.ViewerOut
		if target == okf.ReleaseOutputMCP {
			destination = plan.RuntimeConfig
		}
		fmt.Fprintf(os.Stdout, "  %-8s %d files -> %s\n", strings.ToUpper(target), plan.Files[target], destination)
	}
}

func publishHelpText() string {
	return `openknowledge publish

Build configured publication artifacts from a checked, managed knowledge base.

Usage:
  openknowledge publish [path] --plan
  openknowledge publish [path] --target viewer [--out <folder>]
  openknowledge publish [path] --target mcp [--config runtime.toml] [--id <name>]

Without --target, every output configured in release.outputs is published.
Unmanaged or blocked knowledge is never published.
`
}
