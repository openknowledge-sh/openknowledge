package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

type auditOptions struct {
	target             string
	spec               string
	format             string
	out                string
	usage              []string
	baseline           string
	updateBaseline     bool
	minimumOccurrences int
	highUseThreshold   int
	failOn             string
}

func runAudit(args []string) int {
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
	if err := printAuditReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if auditFails(report, options.failOn) {
		return 1
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
		case argument == "--format" || argument == "--out" || argument == "--usage" || argument == "--baseline" || argument == "--spec" || argument == "--min-occurrences" || argument == "--high-use-threshold" || argument == "--fail-on":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			index = next
			if err := setAuditOption(&options, argument, value); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--format=") || strings.HasPrefix(argument, "--out=") || strings.HasPrefix(argument, "--usage=") || strings.HasPrefix(argument, "--baseline=") || strings.HasPrefix(argument, "--spec=") || strings.HasPrefix(argument, "--min-occurrences=") || strings.HasPrefix(argument, "--high-use-threshold=") || strings.HasPrefix(argument, "--fail-on="):
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
	if options.format != "text" && options.format != "json" {
		return options, fmt.Errorf("--format must be text or json")
	}
	if options.out != "" && options.format != "json" {
		return options, fmt.Errorf("--out requires --format json")
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

Options:
  --usage <path>              Add private runtime usage events. Repeatable.
  --baseline <file>           Compare content-bound source identities.
  --update-baseline           Write the current source identities after audit.
  --min-occurrences <n>       Recurring unanswered-query threshold (default 2).
  --high-use-threshold <n>    Used-unverified threshold (default 5).
  --fail-on <severity>        none, low, medium, or high (default none).
  --spec <version>            OKF version (default latest).
  --format text|json          Output format (default text).
  --json                      Alias for --format json.
  --out <file>                Write JSON output atomically.
  -h, --help                  Show this help.
`
}
