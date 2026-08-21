package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type evalRunOptions struct {
	dataset       string
	target        string
	format        string
	out           string
	spec          string
	base          string
	gate          string
	answerCommand string
	answerArgs    []string
	answerTimeout time.Duration
}

func runEval(args []string) int {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Fprint(os.Stdout, evalHelpText())
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	if args[0] != "run" {
		fmt.Fprintf(stderrOutput(), "unknown eval subcommand: %s\n", args[0])
		return 2
	}
	if hasHelpFlag(args[1:]) {
		fmt.Fprint(os.Stdout, evalRunHelpText())
		return 0
	}
	options, err := parseEvalRunOptions(args[1:])
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	loaded, err := knowledgeeval.LoadDataset(options.dataset)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	resolvedSpec, ok := okf.ResolveSpecVersion(options.spec)
	if !ok {
		fmt.Fprintf(stderrOutput(), "unsupported OKF spec version: %s\n", strings.TrimSpace(options.spec))
		return 2
	}
	if options.base != "" {
		var report knowledgeeval.ComparisonReport
		if options.answerCommand != "" {
			report, err = knowledgeeval.CompareWithAnswers(root, resolvedSpec, loaded, options.base, options.gate, options.answerRunner())
		} else {
			report, err = knowledgeeval.Compare(root, resolvedSpec, loaded, options.base, options.gate)
		}
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if err := printEvalComparison(report, options); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if report.Summary.Status == "fail" {
			return 1
		}
		return 0
	}
	var report knowledgeeval.Report
	if options.answerCommand != "" {
		report, err = knowledgeeval.RunWithAnswers(root, resolvedSpec, loaded, options.answerRunner())
	} else {
		report, err = knowledgeeval.Run(root, resolvedSpec, loaded)
	}
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printEvalReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if report.Summary.Status == "fail" {
		return 1
	}
	return 0
}

func (options evalRunOptions) answerRunner() knowledgeeval.AnswerRunner {
	return knowledgeeval.AnswerRunner{Command: options.answerCommand, Args: options.answerArgs, Timeout: options.answerTimeout}
}

func parseEvalRunOptions(args []string) (evalRunOptions, error) {
	options := evalRunOptions{target: ".", format: "text", spec: "latest", gate: knowledgeeval.GateAll}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			options.format = "json"
		case arg == "--format" || arg == "--out" || arg == "--spec" || arg == "--base" || arg == "--gate" ||
			arg == "--answer-command" || arg == "--answer-arg" || arg == "--answer-timeout":
			value, next, err := nextFlagValue(args, index, arg)
			if err != nil {
				return evalRunOptions{}, err
			}
			index = next
			switch arg {
			case "--format":
				options.format = value
			case "--out":
				options.out = value
			case "--spec":
				options.spec = value
			case "--base":
				options.base = value
			case "--gate":
				options.gate = value
			case "--answer-command":
				options.answerCommand = value
			case "--answer-arg":
				options.answerArgs = append(options.answerArgs, value)
			case "--answer-timeout":
				options.answerTimeout, err = time.ParseDuration(value)
				if err != nil {
					return evalRunOptions{}, fmt.Errorf("invalid --answer-timeout: %w", err)
				}
			}
		case strings.HasPrefix(arg, "--format="):
			options.format = strings.TrimPrefix(arg, "--format=")
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
		case strings.HasPrefix(arg, "--base="):
			options.base = strings.TrimPrefix(arg, "--base=")
		case strings.HasPrefix(arg, "--gate="):
			options.gate = strings.TrimPrefix(arg, "--gate=")
		case strings.HasPrefix(arg, "--answer-command="):
			options.answerCommand = strings.TrimPrefix(arg, "--answer-command=")
		case strings.HasPrefix(arg, "--answer-arg="):
			options.answerArgs = append(options.answerArgs, strings.TrimPrefix(arg, "--answer-arg="))
		case strings.HasPrefix(arg, "--answer-timeout="):
			parsedTimeout, parseErr := time.ParseDuration(strings.TrimPrefix(arg, "--answer-timeout="))
			if parseErr != nil {
				return evalRunOptions{}, fmt.Errorf("invalid --answer-timeout: %w", parseErr)
			}
			options.answerTimeout = parsedTimeout
		case strings.HasPrefix(arg, "-"):
			return evalRunOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			operands = append(operands, arg)
		}
	}
	if len(operands) == 0 {
		return evalRunOptions{}, fmt.Errorf("eval run requires a dataset path")
	}
	if len(operands) > 2 {
		return evalRunOptions{}, fmt.Errorf("eval run accepts one dataset and at most one knowledge base")
	}
	options.dataset = operands[0]
	if len(operands) == 2 {
		options.target = operands[1]
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != "text" && options.format != "json" && options.format != "markdown" {
		return evalRunOptions{}, fmt.Errorf("unsupported eval format: %s", options.format)
	}
	if strings.TrimSpace(options.out) != "" && options.format == "text" {
		return evalRunOptions{}, fmt.Errorf("--out requires --format json or markdown")
	}
	if strings.TrimSpace(options.spec) == "" {
		return evalRunOptions{}, fmt.Errorf("--spec requires a value")
	}
	options.base = strings.TrimSpace(options.base)
	options.gate = strings.ToLower(strings.TrimSpace(options.gate))
	if options.gate != knowledgeeval.GateAll && options.gate != knowledgeeval.GateRegressions {
		return evalRunOptions{}, fmt.Errorf("--gate must be all or regressions")
	}
	if options.base == "" && options.gate != knowledgeeval.GateAll {
		return evalRunOptions{}, fmt.Errorf("--gate regressions requires --base")
	}
	options.answerCommand = strings.TrimSpace(options.answerCommand)
	if options.answerCommand == "" && (len(options.answerArgs) > 0 || options.answerTimeout != 0) {
		return evalRunOptions{}, fmt.Errorf("--answer-arg and --answer-timeout require --answer-command")
	}
	if options.answerTimeout < 0 || options.answerTimeout > time.Hour {
		return evalRunOptions{}, fmt.Errorf("--answer-timeout must be positive and at most 1h")
	}
	return options, nil
}

func printEvalReport(report knowledgeeval.Report, options evalRunOptions) error {
	if options.format == "markdown" {
		return writeEvalOutput(options.out, []byte(knowledgeeval.RenderMarkdown(report)), "eval report")
	}
	if options.format == "json" {
		content, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		if options.out == "" {
			fmt.Print(string(content))
			return nil
		}
		if err := writeOutputFileAtomically(options.out, content); err != nil {
			return err
		}
		terminal.success("Wrote eval report")
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return nil
	}
	terminal.title("Open Knowledge Eval", "deterministic retrieval evidence")
	fmt.Printf("%s %s\n", terminal.muted("dataset"), report.Dataset.ID)
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(report.Target.Root))
	revision := report.Target.Revision
	indexDigest := revision.IndexSHA256
	if len(indexDigest) > 12 {
		indexDigest = indexDigest[:12]
	}
	fmt.Printf("%s OKF %s / %s\n\n", terminal.muted("revision"), revision.SpecVersion, indexDigest)
	for _, result := range report.Cases {
		fmt.Printf("  %-4s %s\n", terminal.status(result.Status), result.ID)
		for _, check := range result.Checks {
			if !check.Passed {
				fmt.Printf("       %s expected %s", check.Kind, check.Expected)
				if check.Actual != "" {
					fmt.Printf(", got %s", check.Actual)
				}
				fmt.Println()
			}
		}
	}
	status := strings.ToUpper(report.Summary.Status)
	fmt.Printf("\n%s %s: %d passed, %d failed, %d/%d checks passed\n",
		terminal.status(report.Summary.Status), status, report.Summary.Passed, report.Summary.Failed,
		report.Summary.PassedChecks, report.Summary.Checks)
	return nil
}

func printEvalComparison(report knowledgeeval.ComparisonReport, options evalRunOptions) error {
	if options.format == "markdown" {
		return writeEvalOutput(options.out, []byte(knowledgeeval.RenderComparisonMarkdown(report)), "eval comparison")
	}
	if options.format == "json" {
		content, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		if options.out == "" {
			fmt.Print(string(content))
			return nil
		}
		if err := writeOutputFileAtomically(options.out, content); err != nil {
			return err
		}
		terminal.success("Wrote eval comparison")
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return nil
	}
	terminal.title("Open Knowledge Eval Comparison", "base versus proposed retrieval evidence")
	fmt.Printf("%s %s\n", terminal.muted("dataset"), report.Dataset.ID)
	fmt.Printf("%s %s (%s)\n", terminal.muted("base"), report.Base.Ref, shortDigest(report.Base.Commit))
	fmt.Printf("%s %s\n", terminal.muted("proposed"), terminal.path(report.Proposed.Root))
	fmt.Printf("%s %s\n\n", terminal.muted("gate"), report.Summary.Gate)
	fmt.Printf("%s %d changed paths / %d questions / %d agents / %d uncovered\n\n", terminal.muted("impact"), len(report.Impact.ChangedPaths), len(report.Impact.AffectedQuestions), len(report.Impact.AffectedAgents), len(report.Impact.UncoveredPaths))
	for _, result := range report.Cases {
		fmt.Printf("  %-15s %s\n", result.Classification, result.ID)
	}
	fmt.Printf("\n%s %s: %d improved, %d regressed, %d proposed failures\n",
		terminal.status(report.Summary.Status), strings.ToUpper(report.Summary.Status), report.Summary.Improved,
		report.Summary.Regressed, report.Summary.ProposedFailed)
	return nil
}

func writeEvalOutput(path string, content []byte, label string) error {
	if path == "" {
		fmt.Print(string(content))
		return nil
	}
	if err := writeOutputFileAtomically(path, content); err != nil {
		return err
	}
	terminal.success("Wrote " + label)
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(path))
	return nil
}

func shortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func evalHelpText() string {
	return `openknowledge eval

Run versioned knowledge eval datasets.

Usage:
  openknowledge eval run <dataset> [key-or-path]
  openknowledge eval run <dataset> [key-or-path] --format json|markdown
  openknowledge eval run <dataset> [key-or-path] --format json|markdown --out <file>
  openknowledge eval run <dataset> [key-or-path] --base <git-ref>
  openknowledge eval --help

Commands:
  run  Retrieve evidence for every question and test declared expectations.

The dataset is strict YAML with type openknowledge.eval and version 1.
`
}

func evalRunHelpText() string {
	return `openknowledge eval run

Evaluate deterministic retrieval evidence against a versioned dataset.

Usage:
  openknowledge eval run <dataset> [key-or-path]
  openknowledge eval run <dataset> [key-or-path] --format json
  openknowledge eval run <dataset> [key-or-path] --format json --out <file>
  openknowledge eval run <dataset> [key-or-path] --spec <version>
  openknowledge eval run <dataset> [key-or-path] --base <git-ref> --gate all|regressions
  openknowledge eval run <dataset> [key-or-path] --answer-command <executable> [--answer-arg <value>...]

Arguments:
  dataset      YAML eval dataset with type openknowledge.eval and version 1.
  key-or-path  Registry key or bundle directory. Defaults to the current directory.

Flags:
  --format     Output format: text, json, or markdown. Defaults to text.
  --json       Alias for --format json.
  --out        Atomically write a JSON or Markdown report to a file.
  --spec       OKF spec version. Defaults to latest.
  --base       Compare the working knowledge base with this Git commit or ref.
  --gate       Fail on all proposed failures or only regressions. Defaults to all.
  --answer-command  Executable implementing the versioned JSON answer protocol.
  --answer-arg      Argument passed directly to the answer executable. Repeatable.
  --answer-timeout  Runner timeout as a Go duration. Defaults to 2m; maximum 1h.

Exit codes:
  0  Every expectation passed.
  1  One or more expectations failed, or evaluation could not run.
  2  Command usage or dataset validation failed.
`
}
