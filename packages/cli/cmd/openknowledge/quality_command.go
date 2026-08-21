package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgefeedback "github.com/openknowledge-sh/openknowledge/packages/cli/internal/feedback"
	knowledgequality "github.com/openknowledge-sh/openknowledge/packages/cli/internal/quality"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

type qualityReportOptions struct {
	target   string
	spec     string
	format   string
	out      string
	usage    []string
	feedback []string
	evals    []string
	audits   []string
}

func runQuality(args []string) int {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Fprint(os.Stdout, qualityHelpText())
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	if args[0] != "report" {
		fmt.Fprintf(stderrOutput(), "unknown quality subcommand: %s\n", args[0])
		return 2
	}
	if hasHelpFlag(args[1:]) {
		fmt.Fprint(os.Stdout, qualityReportHelpText())
		return 0
	}
	options, err := parseQualityReportOptions(args[1:])
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	usageEvents, err := readQualityUsage(options.usage)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	feedbackEvents, err := readQualityFeedback(options.feedback)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	build := knowledgequality.Options{Root: root, Spec: options.spec, Usage: usageEvents, Feedback: feedbackEvents}
	for _, path := range options.evals {
		report, comparison, err := knowledgequality.ReadEval(path)
		if err != nil {
			fmt.Fprintf(stderrOutput(), "%s: %v\n", path, err)
			return 1
		}
		if report != nil {
			build.Evals = append(build.Evals, *report)
		} else {
			build.Comparisons = append(build.Comparisons, *comparison)
		}
	}
	for _, path := range options.audits {
		report, err := knowledgeaudit.ReadReport(path)
		if err != nil {
			fmt.Fprintf(stderrOutput(), "%s: %v\n", path, err)
			return 1
		}
		build.Audits = append(build.Audits, report)
	}
	report, err := knowledgequality.Build(build)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printQualityReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return 0
}

func parseQualityReportOptions(args []string) (qualityReportOptions, error) {
	options := qualityReportOptions{target: ".", spec: "latest", format: "text"}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			options.format = "json"
		case arg == "--format" || arg == "--out" || arg == "--spec" || arg == "--usage" || arg == "--feedback" || arg == "--eval" || arg == "--audit":
			value, next, err := nextFlagValue(args, index, arg)
			if err != nil {
				return qualityReportOptions{}, err
			}
			index = next
			setQualityOption(&options, arg, value)
		case strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "--out=") || strings.HasPrefix(arg, "--spec=") || strings.HasPrefix(arg, "--usage=") || strings.HasPrefix(arg, "--feedback=") || strings.HasPrefix(arg, "--eval=") || strings.HasPrefix(arg, "--audit="):
			name, value, _ := strings.Cut(arg, "=")
			setQualityOption(&options, name, value)
		case strings.HasPrefix(arg, "-"):
			return qualityReportOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			operands = append(operands, arg)
		}
	}
	if len(operands) > 1 {
		return qualityReportOptions{}, fmt.Errorf("quality report accepts at most one knowledge base")
	}
	if len(operands) == 1 {
		options.target = operands[0]
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != "text" && options.format != "json" && options.format != "markdown" {
		return qualityReportOptions{}, fmt.Errorf("unsupported quality report format: %s", options.format)
	}
	if options.out != "" && options.format == "text" {
		return qualityReportOptions{}, fmt.Errorf("--out requires --format json or markdown")
	}
	if strings.TrimSpace(options.spec) == "" {
		return qualityReportOptions{}, fmt.Errorf("--spec requires a value")
	}
	return options, nil
}

func setQualityOption(options *qualityReportOptions, name, value string) {
	value = strings.TrimSpace(value)
	switch name {
	case "--format":
		options.format = value
	case "--out":
		options.out = value
	case "--spec":
		options.spec = value
	case "--usage":
		options.usage = append(options.usage, value)
	case "--feedback":
		options.feedback = append(options.feedback, value)
	case "--eval":
		options.evals = append(options.evals, value)
	case "--audit":
		options.audits = append(options.audits, value)
	}
}

func readQualityUsage(paths []string) ([]knowledgeusage.Event, error) {
	if len(paths) == 0 {
		return []knowledgeusage.Event{}, nil
	}
	return knowledgeusage.Read(paths)
}

func readQualityFeedback(paths []string) ([]knowledgefeedback.Event, error) {
	if len(paths) == 0 {
		return []knowledgefeedback.Event{}, nil
	}
	return knowledgefeedback.Read(paths)
}

func printQualityReport(report knowledgequality.Report, options qualityReportOptions) error {
	if options.format == "markdown" {
		return writeQualityOutput(options.out, []byte(knowledgequality.RenderMarkdown(report)))
	}
	if options.format == "json" {
		content, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		return writeQualityOutput(options.out, append(content, '\n'))
	}
	terminal.title("Open Knowledge Quality", "usage-grounded knowledge outcomes")
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(report.Bundle.Path))
	fmt.Printf("%s %d usage / %d feedback / %d eval / %d comparison / %d audit\n\n", terminal.muted("inputs"), report.Inputs.UsageEvents, report.Inputs.FeedbackEvents, report.Inputs.EvalReports, report.Inputs.Comparisons, report.Inputs.AuditReports)
	for _, metric := range report.Metrics {
		value := "unavailable"
		if metric.Value != nil {
			value = fmt.Sprintf("%.2f %s", *metric.Value, metric.Unit)
		}
		fmt.Printf("  %-48s %s\n", metric.ID, value)
	}
	fmt.Println()
	shown := 0
	for _, concept := range report.Concepts {
		if concept.Priority == "none" || shown == 10 {
			continue
		}
		fmt.Printf("  %-6s %s — %s\n", strings.ToUpper(concept.Priority), concept.Path, strings.Join(concept.RiskReasons, ", "))
		shown++
	}
	if shown == 0 {
		fmt.Println("  No concrete priorities in the supplied observation window.")
	}
	return nil
}

func writeQualityOutput(path string, content []byte) error {
	if path == "" {
		fmt.Print(string(content))
		return nil
	}
	if err := writeOutputFileAtomically(path, content); err != nil {
		return err
	}
	terminal.success("Wrote quality report")
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(path))
	return nil
}

func qualityHelpText() string {
	return `openknowledge quality <command>

Measure knowledge outcomes from current metadata, runtime use, feedback, evals, and audits.

Commands:
  report   Build a deterministic quality and priority report.

Run openknowledge quality report --help for input contracts and examples.
`
}

func qualityReportHelpText() string {
	return `openknowledge quality report [knowledge-base] [flags]

Build a report without a global quality score. Every metric exposes its evidence,
and metrics that lack the required event history are marked unavailable.

Flags:
  --usage <path>       Usage JSONL file or directory. Repeatable.
  --feedback <path>    Feedback JSONL file or directory. Repeatable.
  --eval <path>        Eval report or comparison JSON. Repeatable.
  --audit <path>       Audit report JSON for the current bundle. Repeatable.
  --spec <version>     OKF version (default latest).
  --format <format>    text, json, or markdown (default text).
  --json               Alias for --format json.
  --out <path>         Write JSON or Markdown atomically.
`
}
