package main

import (
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

const (
	setupSkillGlobal  = "global"
	setupSkillProject = "project"
	setupSkillBoth    = "both"
	setupSkillNone    = "none"
)

type setupCompleteOptions struct {
	wiki      string
	skill     string
	harnesses []string
	observe   bool
}

func runSetupComplete(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupCompleteHelpText())
		return 0
	}
	options, err := parseSetupCompleteArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	wiki, err := filepath.Abs(options.wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if info, err := os.Stat(wiki); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	} else if !info.IsDir() {
		fmt.Fprintf(stderrOutput(), "knowledge base is not a directory: %s\n", wiki)
		return 1
	}
	if _, err := os.Stat(filepath.Join(wiki, "SETUP.MD")); err == nil {
		fmt.Fprintln(stderrOutput(), "setup is not complete: remove SETUP.MD after applying its instructions")
		return 1
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if code := runValidate([]string{wiki}); code != 0 {
		return code
	}
	installProjectSkill := options.skill == setupSkillProject || options.skill == setupSkillBoth
	if installProjectSkill || options.observe {
		if _, err := integration.RepositoryRoot(wiki); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	projectStatus, existingProject, err := setupProjectStatus(wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	entry, connected, err := okf.ResolveRegistryTarget(wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !connected {
		if code := runConnect([]string{wiki, "--access", "write"}, "openknowledge connect"); code != 0 {
			return code
		}
		entry, connected, err = okf.ResolveRegistryTarget(wiki)
		if err != nil || !connected {
			if err == nil {
				err = fmt.Errorf("connection was not recorded")
			}
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}

	if options.skill == setupSkillGlobal || options.skill == setupSkillBoth {
		result, err := integration.ReconcileGlobal("", options.harnesses)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Global skill: %s\n", path)
		}
	}
	projectHarnesses := append([]string(nil), options.harnesses...)
	if existingProject && len(projectHarnesses) == 0 {
		projectHarnesses = append(projectHarnesses, projectStatus.Harnesses...)
	}
	if installProjectSkill || options.observe || existingProject {
		result, err := integration.ReconcileProject(wiki, integration.ProjectOptions{
			Harnesses:     projectHarnesses,
			Observe:       options.observe,
			ProjectSkills: installProjectSkill,
		})
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Project integration: %s\n", filepath.Join(result.Root, filepath.FromSlash(path)))
		}
	}
	fmt.Fprintln(os.Stdout, "\nOpen Knowledge setup is complete.")
	fmt.Fprintf(os.Stdout, "  Knowledge base: %s\n", wiki)
	fmt.Fprintf(os.Stdout, "  Connection:     %s (%s)\n", entry.Name, registryEntryAccess(entry))
	fmt.Fprintf(os.Stdout, "  Skills:         %s\n", options.skill)
	fmt.Fprintf(os.Stdout, "  Harnesses:      %s\n", setupHarnessLabel(options.harnesses))
	fmt.Fprintf(os.Stdout, "  Observation:    %s\n", enabledLabel(options.observe))
	return 0
}

func parseSetupCompleteArgs(args []string) (setupCompleteOptions, error) {
	options := setupCompleteOptions{}
	fs := flag.NewFlagSet("setup complete", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	fs.StringVar(&options.skill, "skill", "", "skill scope")
	var harnesses stringListFlag
	fs.Var(&harnesses, "harness", "agent harness")
	observe := fs.String("observe", "off", "session observation: on or off")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return options, err
	}
	if fs.NArg() != 1 {
		return options, fmt.Errorf("usage: openknowledge setup complete <wiki> --skill <global|project|both|none>")
	}
	options.wiki = fs.Arg(0)
	options.skill = strings.ToLower(strings.TrimSpace(options.skill))
	switch options.skill {
	case setupSkillGlobal, setupSkillProject, setupSkillBoth, setupSkillNone:
	default:
		return options, fmt.Errorf("--skill requires global, project, both, or none")
	}
	options.observe, _ = parseSetupOnOff(*observe)
	if _, ok := parseSetupOnOff(*observe); !ok {
		return options, fmt.Errorf("--observe requires on or off")
	}
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
	needsHarness := options.skill != setupSkillNone || options.observe
	if needsHarness && len(options.harnesses) == 0 {
		return options, fmt.Errorf("setup complete requires --harness when installing skills or enabling observation")
	}
	if !needsHarness && len(options.harnesses) > 0 {
		return options, fmt.Errorf("--harness requires a skill installation or --observe on")
	}
	return options, nil
}

func parseSetupOnOff(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on":
		return true, true
	case "off":
		return false, true
	default:
		return false, false
	}
}

func runSetupStatus(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupStatusHelpText())
		return 0
	}
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderrOutput(), "setup status accepts at most one knowledge base path")
		return 2
	}
	wiki := "Wiki"
	if len(args) == 1 {
		wiki = args[0]
	} else if root, config, err := integration.FindRepository("."); err == nil {
		wiki = filepath.Join(root, filepath.FromSlash(config.KnowledgeBase))
	}
	wikiAbs, err := filepath.Abs(wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	healthy := true
	validation := "missing"
	if result, err := okf.Validate(wikiAbs); err == nil {
		switch {
		case len(result.Errors) > 0:
			validation = "invalid"
			healthy = false
		case len(result.Warnings) > 0:
			validation = "warnings"
		default:
			validation = "healthy"
		}
	} else {
		healthy = false
	}
	entry, connected, err := okf.ResolveRegistryTarget(wikiAbs)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	connection := "not connected"
	if connected {
		connection = fmt.Sprintf("%s (%s)", entry.Name, registryEntryAccess(entry))
	} else {
		healthy = false
	}
	fmt.Fprintf(os.Stdout, "Knowledge base: %s\n", wikiAbs)
	fmt.Fprintf(os.Stdout, "Validation:     %s\n", validation)
	fmt.Fprintf(os.Stdout, "Connection:     %s\n", connection)
	if status, exists, err := setupProjectStatus(wikiAbs); err != nil {
		fmt.Fprintf(os.Stdout, "Project setup:  invalid (%v)\n", err)
		fmt.Fprintln(os.Stdout, "Observation:    unknown")
		healthy = false
	} else if exists {
		projectSkills := "none"
		if status.ProjectSkills {
			projectSkills = strings.Join(status.Harnesses, ", ")
		}
		fmt.Fprintf(os.Stdout, "Project skills: %s\n", projectSkills)
		fmt.Fprintf(os.Stdout, "Observation:    %s\n", enabledLabel(status.Observe))
		for _, file := range status.Files {
			fmt.Fprintf(os.Stdout, "  %-10s %s\n", file.State, file.Path)
			if file.State != "managed" {
				healthy = false
			}
		}
	} else {
		fmt.Fprintln(os.Stdout, "Project skills: none")
		fmt.Fprintln(os.Stdout, "Observation:    disabled")
	}
	allHarnesses := uniqueSortedStrings(agents.SupportedAgentRuntimes())
	global, err := integration.GlobalStatus("", allHarnesses)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	for _, file := range global.Files {
		fmt.Fprintf(os.Stdout, "Global skill:   %-10s %s\n", file.State, file.Path)
	}
	if !healthy {
		return 1
	}
	return 0
}

func runSetupRepair(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupRepairHelpText())
		return 0
	}
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderrOutput(), "setup repair accepts at most one knowledge base or repository path")
		return 2
	}
	start := "."
	if len(args) == 1 {
		start = args[0]
	}
	repaired := false
	harnesses := []string{}
	status, exists, err := setupProjectStatus(start)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if exists {
		harnesses = append(harnesses, status.Harnesses...)
		result, err := integration.RepairProject(start)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Repaired project integration: %s\n", filepath.Join(result.Root, filepath.FromSlash(path)))
		}
		repaired = true
	}
	allHarnesses := uniqueSortedStrings(agents.SupportedAgentRuntimes())
	globalStatus, err := integration.GlobalStatus("", allHarnesses)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	for index, file := range globalStatus.Files {
		if file.State != "missing" {
			harnesses = append(harnesses, allHarnesses[index])
		}
	}
	harnesses = uniqueSortedStrings(harnesses)
	if len(harnesses) > 0 {
		result, err := integration.RepairGlobal("", harnesses)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		for _, path := range result.Files {
			fmt.Fprintf(os.Stdout, "✓ Repaired global skill: %s\n", path)
		}
		repaired = true
	}
	if !repaired {
		fmt.Fprintln(stderrOutput(), "no Open Knowledge setup found to repair")
		return 1
	}
	return 0
}

func setupProjectStatus(start string) (integration.StatusResult, bool, error) {
	status, err := integration.Status(start)
	if err == nil {
		return status, true, nil
	}
	if strings.Contains(err.Error(), "no project setup found") {
		return integration.StatusResult{}, false, nil
	}
	return integration.StatusResult{}, false, err
}

func runSetupObserve(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupObserveHelpText())
		return 0
	}
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge setup observe <on|off> [repository]")
		return 2
	}
	enabled, ok := parseSetupOnOff(args[0])
	if !ok {
		fmt.Fprintln(stderrOutput(), "setup observe requires on or off")
		return 2
	}
	start := "."
	if len(args) == 2 {
		start = args[1]
	}
	result, err := integration.SetObservation(start, enabled)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Knowledge-gap observation is %s in %s.\n", enabledLabel(enabled), result.Root)
	return 0
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func setupCompleteHelpText() string {
	return `openknowledge setup complete

Validate and activate a completed knowledge base setup.

Usage:
  openknowledge setup complete <wiki> --skill <global|project|both|none> [--harness <name>] [--observe on|off]

Repeat --harness to install for multiple agent environments. Observation is
off by default. SETUP.MD must be removed before this command can succeed.
`
}

func setupStatusHelpText() string {
	return "Usage: openknowledge setup status [wiki]\n"
}

func setupRepairHelpText() string {
	return "Usage: openknowledge setup repair [wiki-or-repository]\n"
}

func setupObserveHelpText() string {
	return "Usage: openknowledge setup observe <on|off> [repository]\n"
}
