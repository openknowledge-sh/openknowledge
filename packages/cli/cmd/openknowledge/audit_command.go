package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/insights"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

type auditOptions struct {
	target             string
	spec               string
	format             string
	out                string
	markdownOut        string
	usage              []string
	baseline           string
	updateBaseline     bool
	observeRemote      bool
	minimumOccurrences int
	highUseThreshold   int
	failOn             string
}

func runAudit(args []string) int {
	if len(args) > 0 && args[0] == "propose" {
		return runAuditPropose(args[1:])
	}
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, auditHelpText())
		return 0
	}
	options, err := parseAuditOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	var events []knowledgeusage.Event
	if len(options.usage) > 0 {
		events, err = knowledgeusage.Read(options.usage)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	var baseline *knowledgeaudit.SourceBaseline
	if options.baseline != "" {
		loaded, loadErr := knowledgeaudit.ReadBaseline(options.baseline)
		if loadErr == nil {
			baseline = &loaded
		} else if !options.updateBaseline || !os.IsNotExist(loadErr) {
			fmt.Fprintln(stderrOutput(), loadErr)
			return 1
		}
	}
	report, currentBaseline, err := knowledgeaudit.Run(knowledgeaudit.Options{
		Root: root, Spec: options.spec, Usage: events, Baseline: baseline,
		ObserveRemote:      options.observeRemote,
		MinimumOccurrences: options.minimumOccurrences, HighUseThreshold: options.highUseThreshold,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if options.updateBaseline {
		content, encodeErr := knowledgeaudit.EncodeBaseline(currentBaseline)
		if encodeErr != nil {
			fmt.Fprintln(stderrOutput(), encodeErr)
			return 1
		}
		if err := writeOutputFileAtomically(options.baseline, content); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	if options.markdownOut != "" {
		if err := writeOutputFileAtomically(options.markdownOut, []byte(knowledgeaudit.RenderMarkdown(report))); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	if err := printAuditReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if auditFails(report, options.failOn) {
		return 1
	}
	return 0
}

func runAuditPropose(args []string) int {
	flags := flag.NewFlagSet("audit propose", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	reportPath := flags.String("report", ".openknowledge/reports/audit.json", "audit report")
	knowledgePath := flags.String("path", "", "knowledge base path")
	if err := parseInterspersedFlags(flags, args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge audit propose <finding-id> [--report <audit.json>] [--path <knowledge>]")
		return 2
	}
	report, err := knowledgeaudit.ReadReport(*reportPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	repo, config, err := integration.FindRepository(".")
	if err != nil {
		return printAgentCommandError(err)
	}
	root := filepath.Join(repo, filepath.FromSlash(config.KnowledgeBase))
	if strings.TrimSpace(*knowledgePath) != "" {
		root, err = okf.ResolveKnowledgeRoot(*knowledgePath)
		if err != nil {
			return printAgentCommandError(err)
		}
	}
	path, created, err := insights.CreateAuditFinding(repo, root, report, flags.Arg(0))
	if err != nil {
		return printAgentCommandError(err)
	}
	state := "preserved"
	if created {
		state = "created"
	}
	if err := printJSON(map[string]any{
		"schemaVersion": okf.MachineSchemaVersion, "findingId": flags.Arg(0),
		"proposal": path, "state": state, "next": "okn automation insights run " + path,
	}); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func parseAuditOptions(args []string) (auditOptions, error) {
	options := auditOptions{target: ".", spec: "latest", format: "text", minimumOccurrences: 2, highUseThreshold: 5, failOn: "none"}
	var operands []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--json":
			options.format = "json"
		case argument == "--update-baseline":
			options.updateBaseline = true
		case argument == "--observe-remote":
			options.observeRemote = true
		case argument == "--format" || argument == "--out" || argument == "--markdown-out" || argument == "--usage" || argument == "--baseline" || argument == "--spec" || argument == "--min-occurrences" || argument == "--high-use-threshold" || argument == "--fail-on":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			index = next
			if err := setAuditOption(&options, argument, value); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--format=") || strings.HasPrefix(argument, "--out=") || strings.HasPrefix(argument, "--markdown-out=") || strings.HasPrefix(argument, "--usage=") || strings.HasPrefix(argument, "--baseline=") || strings.HasPrefix(argument, "--spec=") || strings.HasPrefix(argument, "--min-occurrences=") || strings.HasPrefix(argument, "--high-use-threshold=") || strings.HasPrefix(argument, "--fail-on="):
			parts := strings.SplitN(argument, "=", 2)
			if err := setAuditOption(&options, parts[0], parts[1]); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "-"):
			return options, fmt.Errorf("unknown audit option: %s", argument)
		default:
			operands = append(operands, argument)
		}
	}
	if len(operands) > 1 {
		return options, fmt.Errorf("audit accepts at most one knowledge base path")
	}
	if len(operands) == 1 {
		options.target = operands[0]
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != "text" && options.format != "json" && options.format != "markdown" {
		return options, fmt.Errorf("--format must be text, json, or markdown")
	}
	if options.out != "" && options.format == "text" {
		return options, fmt.Errorf("--out requires --format json or markdown")
	}
	if options.markdownOut != "" && options.out != "" {
		markdownPath, _ := filepath.Abs(options.markdownOut)
		outputPath, _ := filepath.Abs(options.out)
		if markdownPath == outputPath {
			return options, fmt.Errorf("--markdown-out and --out must use different files")
		}
	}
	if options.updateBaseline && strings.TrimSpace(options.baseline) == "" {
		return options, fmt.Errorf("--update-baseline requires --baseline")
	}
	options.failOn = strings.ToLower(strings.TrimSpace(options.failOn))
	if options.failOn != "none" && options.failOn != "low" && options.failOn != "medium" && options.failOn != "high" {
		return options, fmt.Errorf("--fail-on must be none, low, medium, or high")
	}
	return options, nil
}

func setAuditOption(options *auditOptions, name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s requires a value", name)
	}
	switch name {
	case "--format":
		options.format = value
	case "--out":
		options.out = value
	case "--markdown-out":
		options.markdownOut = value
	case "--usage":
		options.usage = append(options.usage, value)
	case "--baseline":
		options.baseline = value
	case "--spec":
		options.spec = value
	case "--fail-on":
		options.failOn = value
	case "--min-occurrences", "--high-use-threshold":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return fmt.Errorf("%s must be a positive integer", name)
		}
		if name == "--min-occurrences" {
			options.minimumOccurrences = parsed
		} else {
			options.highUseThreshold = parsed
		}
	}
	return nil
}

func printAuditReport(report knowledgeaudit.Report, options auditOptions) error {
	if options.format == "markdown" {
		content := []byte(knowledgeaudit.RenderMarkdown(report))
		if options.out != "" {
			return writeOutputFileAtomically(options.out, content)
		}
		_, err := os.Stdout.Write(content)
		return err
	}
	if options.format == "json" {
		content, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		if options.out != "" {
			return writeOutputFileAtomically(options.out, content)
		}
		_, err = os.Stdout.Write(content)
		return err
	}
	terminal.title("Open Knowledge Audit", "concrete evidence-backed findings")
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(report.Bundle.Path))
	fmt.Printf("%s %s / %s\n", terminal.muted("revision"), report.Bundle.Spec, report.Bundle.SHA256[:12])
	fmt.Printf("%s %d high, %d medium, %d low\n\n", terminal.muted("findings"), report.Summary.High, report.Summary.Medium, report.Summary.Low)
	if len(report.Findings) == 0 {
		terminal.success("No audit findings")
	} else {
		for _, finding := range report.Findings {
			fmt.Printf("  %-6s %-24s %s\n", strings.ToUpper(finding.Severity), finding.Category, finding.Title)
			fmt.Printf("         impact: %s\n", finding.Impact)
			fmt.Printf("         targets: %s\n", strings.Join(finding.Targets, ", "))
			for _, evidence := range finding.Evidence {
				location := evidence.Path
				if location != "" {
					location += ":"
				}
				fmt.Printf("         evidence: %s%s=%s\n", location, evidence.Field, evidence.Value)
			}
		}
	}
	if options.updateBaseline {
		fmt.Printf("\n%s %s\n", terminal.muted("baseline"), terminal.path(options.baseline))
	}
	return nil
}

func auditFails(report knowledgeaudit.Report, threshold string) bool {
	rank := map[string]int{"none": 4, "high": 3, "medium": 2, "low": 1}[threshold]
	for _, finding := range report.Findings {
		if map[string]int{"high": 3, "medium": 2, "low": 1}[finding.Severity] >= rank {
			return true
		}
	}
	return false
}

func auditHelpText() string {
	return `openknowledge audit

Find concrete knowledge risks and print evidence for each finding.

Usage:
  openknowledge audit [path]
  openknowledge audit [path] --usage <file-or-dir>
  openknowledge audit [path] --baseline <file> [--update-baseline]
  openknowledge audit [path] --format json [--out <file>]
  openknowledge audit propose <finding-id> [--report <audit.json>]

Options:
  --usage <path>              Add private runtime usage events. Repeatable.
  --baseline <file>           Compare content-bound source identities.
  --update-baseline           Write the current source identities after audit.
  --observe-remote            Run configured metadata or fetch observations.
  --min-occurrences <n>       Recurring unanswered-query threshold (default 2).
  --high-use-threshold <n>    Used-unverified threshold (default 5).
  --fail-on <severity>        none, low, medium, or high (default none).
  --spec <version>            OKF version (default latest).
  --format text|json|markdown Output format (default text).
  --json                      Alias for --format json.
  --out <file>                Write JSON or Markdown output atomically.
  --markdown-out <file>       Also write Markdown from the same audit.
  -h, --help                  Show this help.
`
}
