package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type setupSkillOptions struct {
	scope       string
	project     string
	harnesses   []string
	interactive bool
}

func runSetupSkill(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupSkillHelpText())
		return 0
	}
	options, err := parseSetupSkillArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.interactive || (options.scope == "" && setupInputIsTerminal()) {
		options, err = runSetupSkillWizard(options)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	if err := validateSetupSkillOptions(options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	return applySetupSkill(options)
}

func parseSetupSkillArgs(args []string) (setupSkillOptions, error) {
	options := setupSkillOptions{}
	fs := flag.NewFlagSet("setup skill", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	fs.StringVar(&options.scope, "scope", "", "skill scope")
	fs.StringVar(&options.project, "project", "", "project knowledge base")
	fs.BoolVar(&options.interactive, "interactive", false, "run the terminal wizard")
	var harnesses stringListFlag
	fs.Var(&harnesses, "harness", "agent harness")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return options, err
	}
	if fs.NArg() != 0 {
		return options, fmt.Errorf("setup skill does not accept a positional knowledge base; use --project <target>")
	}
	options.scope = strings.ToLower(strings.TrimSpace(options.scope))
	options.project = strings.TrimSpace(options.project)
	seen := map[string]bool{}
	for _, harness := range harnesses {
		harness = strings.ToLower(strings.TrimSpace(harness))
		if _, err := agents.HarnessForRuntime(harness); err != nil {
			return options, err
		}
		if !seen[harness] {
			seen[harness] = true
			options.harnesses = append(options.harnesses, harness)
		}
	}
	sort.Strings(options.harnesses)
	return options, nil
}

func validateSetupSkillOptions(options setupSkillOptions) error {
	switch options.scope {
	case setupSkillGlobal:
		if options.project != "" {
			return fmt.Errorf("--project requires --scope project or --scope both")
		}
	case setupSkillProject, setupSkillBoth:
		if options.project == "" {
			return fmt.Errorf("setup skill requires --project for project or both scope")
		}
	default:
		return fmt.Errorf("setup skill requires --scope global, project, or both")
	}
	if len(options.harnesses) == 0 {
		return fmt.Errorf("setup skill requires at least one --harness")
	}
	return nil
}

func runSetupSkillWizard(options setupSkillOptions) (setupSkillOptions, error) {
	reader := bufio.NewReader(setupInput)
	if options.scope == "" {
		choice, err := setupChoice(reader, "Where should Open Knowledge install the skill?", []string{
			"Personal — available across all projects",
			"Project — shared in one repository",
			"Both — personal and project installations",
		}, 0)
		if err != nil {
			return options, err
		}
		options.scope = []string{setupSkillGlobal, setupSkillProject, setupSkillBoth}[choice]
	}
	if options.scope != setupSkillGlobal && options.project == "" {
		project, err := setupSkillProjectTarget(reader)
		if err != nil {
			return options, err
		}
		options.project = project
	}
	if len(options.harnesses) == 0 {
		available := detectSetupRuntimes(context.Background())
		harnesses, err := setupHarnesses(reader, available, available)
		if err != nil {
			return options, err
		}
		options.harnesses = harnesses
	}

	fmt.Fprintln(os.Stdout, "\nOpen Knowledge skill plan")
	fmt.Fprintf(os.Stdout, "  Scope:      %s\n", options.scope)
	if options.project != "" {
		fmt.Fprintf(os.Stdout, "  Project:    %s\n", options.project)
	}
	fmt.Fprintf(os.Stdout, "  Harnesses:  %s\n", setupHarnessLabel(options.harnesses))
	confirmed, err := setupConfirm(reader, "Install skill?", true)
	if err != nil {
		return options, err
	}
	if !confirmed {
		return options, fmt.Errorf("skill installation cancelled")
	}
	return options, nil
}

func setupSkillProjectTarget(reader *bufio.Reader) (string, error) {
	entries, err := okf.RegistryEntries()
	if err != nil {
		return "", err
	}
	var compatible []okf.RegistryEntry
	var labels []string
	for _, entry := range entries {
		if entry.Managed {
			continue
		}
		if _, err := integration.RepositoryRoot(entry.Path); err != nil {
			continue
		}
		compatible = append(compatible, entry)
		labels = append(labels, fmt.Sprintf("%s — %s", entry.Name, entry.Path))
	}
	labels = append(labels, "Enter another knowledge base path")
	choice, err := setupChoice(reader, "Which project should receive the skill?", labels, 0)
	if err != nil {
		return "", err
	}
	if choice < len(compatible) {
		return compatible[choice].Name, nil
	}
	return setupLine(reader, "Knowledge base path", "Wiki")
}

func applySetupSkill(options setupSkillOptions) int {
	projectTarget := ""
	if options.scope == setupSkillProject || options.scope == setupSkillBoth {
		resolved, err := okf.ResolveKnowledgeRoot(options.project)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		projectTarget, err = filepath.Abs(resolved)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if info, err := os.Stat(projectTarget); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		} else if !info.IsDir() {
			fmt.Fprintf(stderrOutput(), "knowledge base is not a directory: %s\n", projectTarget)
			return 1
		}
		if _, err := integration.RepositoryRoot(projectTarget); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if code := runValidate([]string{projectTarget}); code != 0 {
			return code
		}
	}

	if options.scope == setupSkillGlobal || options.scope == setupSkillBoth {
		result, err := integration.ReconcileGlobal("", options.harnesses)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Global skill: %s\n", path)
		}
	}
	if options.scope == setupSkillProject || options.scope == setupSkillBoth {
		if _, connected, err := okf.ResolveRegistryTarget(projectTarget); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		} else if !connected {
			if code := runConnect([]string{projectTarget, "--access", "write"}, "openknowledge connect"); code != 0 {
				return code
			}
		}
		result, err := integration.ReconcileProject(projectTarget, integration.ProjectOptions{
			Harnesses:     options.harnesses,
			ProjectSkills: true,
		})
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Project integration: %s\n", filepath.Join(result.Root, filepath.FromSlash(path)))
		}
	}

	fmt.Fprintln(os.Stdout, "\nOpen Knowledge skill installation is complete.")
	fmt.Fprintf(os.Stdout, "  Scope:      %s\n", options.scope)
	if projectTarget != "" {
		fmt.Fprintf(os.Stdout, "  Project:    %s\n", projectTarget)
	}
	fmt.Fprintf(os.Stdout, "  Harnesses:  %s\n", setupHarnessLabel(options.harnesses))
	return 0
}

func setupSkillHelpText() string {
	return `openknowledge setup skill

Install the Open Knowledge skill without creating a knowledge base.

Usage:
  openknowledge setup skill
  openknowledge setup skill --interactive
  openknowledge setup skill --scope global --harness <name>
  openknowledge setup skill --scope project --project <target> --harness <name>
  openknowledge setup skill --scope both --project <target> --harness <name>

With terminal input and no scope, the command starts an interactive installer.
The installer detects available agent harnesses. Repeat --harness to install
the skill for more than one harness. Global scope does not require a project.

Flags:
  --scope        Skill scope: global, project, or both.
  --project      Connected registry key or local knowledge base path.
  --harness      Agent harness. Repeat for more than one harness.
  --interactive  Run the terminal installer.
`
}
