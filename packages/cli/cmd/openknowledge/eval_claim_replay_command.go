package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type evalClaimReplayOptions struct {
	dataset         string
	target          string
	format          string
	out             string
	spec            string
	maxStale        int
	maxHallucinated int
	maxUnverified   int
}

func runEvalClaimReplay(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, evalClaimReplayHelpText())
		return 0
	}
	options, err := parseEvalClaimReplayOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	loaded, err := knowledgeeval.LoadClaimReplayDataset(options.dataset)
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
	provider := gitClaimReplayProvider{root: root, spec: resolvedSpec}
	report, err := knowledgeeval.RunClaimReplay(context.Background(), loaded, provider)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printEvalClaimReplayReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if report.Summary.Stale > options.maxStale || report.Summary.Hallucinated > options.maxHallucinated || report.Summary.Unverified > options.maxUnverified {
		return 1
	}
	return 0
}

type gitClaimReplayProvider struct {
	root string
	spec string
}

func (provider gitClaimReplayProvider) ClaimsAt(ctx context.Context, checkpoint knowledgeeval.ClaimReplayCheckpoint) ([]knowledgeeval.ObservedClaim, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, err := knowledgeeval.CreateGitClaimReplaySnapshot(provider.root, checkpoint.Revision)
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	index, err := claimops.BuildIndex(snapshot.Root, provider.spec, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	observed := make([]knowledgeeval.ObservedClaim, 0, len(index.Occurrences))
	for _, occurrence := range index.Occurrences {
		switch occurrence.Claim.Status {
		case "rejected", "superseded", "archived":
			continue
		}
		object, normalizeErr := okf.NormalizeClaimObject(occurrence.Claim.Object)
		if normalizeErr != nil {
			object = fmt.Sprint(occurrence.Claim.Object.Value)
		}
		observed = append(observed, knowledgeeval.ObservedClaim{
			ClaimID: occurrence.Claim.ID,
			Statement: strings.TrimSpace(strings.Join([]string{
				occurrence.Claim.Subject, occurrence.Claim.Predicate, object,
			}, " ")),
			Document: occurrence.Path,
		})
	}
	return observed, nil
}

func parseEvalClaimReplayOptions(args []string) (evalClaimReplayOptions, error) {
	options := evalClaimReplayOptions{target: ".", format: "text", spec: "latest"}
	var operands []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--json":
			options.format = "json"
		case argument == "--format" || argument == "--out" || argument == "--spec" || argument == "--max-stale" || argument == "--max-hallucinated" || argument == "--max-unverified":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			index = next
			if err := setEvalClaimReplayOption(&options, argument, value); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--format=") || strings.HasPrefix(argument, "--out=") || strings.HasPrefix(argument, "--spec=") || strings.HasPrefix(argument, "--max-stale=") || strings.HasPrefix(argument, "--max-hallucinated=") || strings.HasPrefix(argument, "--max-unverified="):
			parts := strings.SplitN(argument, "=", 2)
			if err := setEvalClaimReplayOption(&options, parts[0], parts[1]); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "-"):
			return options, fmt.Errorf("unknown claim replay option: %s", argument)
		default:
			operands = append(operands, argument)
		}
	}
	if len(operands) == 0 {
		return options, fmt.Errorf("eval claims requires a dataset path")
	}
	if len(operands) > 2 {
		return options, fmt.Errorf("eval claims accepts one dataset and at most one knowledge base")
	}
	options.dataset = operands[0]
	if len(operands) == 2 {
		options.target = operands[1]
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != "text" && options.format != "json" && options.format != "markdown" {
		return options, fmt.Errorf("unsupported claim replay format: %s", options.format)
	}
	if options.out != "" && options.format == "text" {
		return options, fmt.Errorf("--out requires --format json or markdown")
	}
	return options, nil
}

func setEvalClaimReplayOption(options *evalClaimReplayOptions, name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s requires a value", name)
	}
	switch name {
	case "--format":
		options.format = value
	case "--out":
		options.out = value
	case "--spec":
		options.spec = value
	case "--max-stale", "--max-hallucinated", "--max-unverified":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return fmt.Errorf("%s must be a non-negative integer", name)
		}
		if name == "--max-stale" {
			options.maxStale = parsed
		} else if name == "--max-hallucinated" {
			options.maxHallucinated = parsed
		} else {
			options.maxUnverified = parsed
		}
	}
	return nil
}

func printEvalClaimReplayReport(report knowledgeeval.ClaimReplayReport, options evalClaimReplayOptions) error {
	var content []byte
	var err error
	switch options.format {
	case "json":
		content, err = json.MarshalIndent(report, "", "  ")
		if err == nil {
			content = append(content, '\n')
		}
	case "markdown":
		content = []byte(renderEvalClaimReplayMarkdown(report))
	default:
		fmt.Printf("Claim replay: %d supported, %d stale, %d hallucinated, %d unverified\n", report.Summary.Supported, report.Summary.Stale, report.Summary.Hallucinated, report.Summary.Unverified)
		for _, checkpoint := range report.Checkpoints {
			fmt.Printf("  %s (%s): %d supported, %d stale, %d hallucinated, %d unverified\n", checkpoint.ID, checkpoint.Revision, checkpoint.Summary.Supported, checkpoint.Summary.Stale, checkpoint.Summary.Hallucinated, checkpoint.Summary.Unverified)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if options.out != "" {
		return writeOutputFileAtomically(options.out, content)
	}
	_, err = os.Stdout.Write(content)
	return err
}

func renderEvalClaimReplayMarkdown(report knowledgeeval.ClaimReplayReport) string {
	var output strings.Builder
	output.WriteString("# Claim replay eval\n\n")
	fmt.Fprintf(&output, "Supported: **%d** · Stale: **%d** · Hallucinated: **%d** · Unverified: **%d**\n\n", report.Summary.Supported, report.Summary.Stale, report.Summary.Hallucinated, report.Summary.Unverified)
	for _, checkpoint := range report.Checkpoints {
		fmt.Fprintf(&output, "## %s\n\nRevision: `%s`\n\n", checkpoint.ID, strings.ReplaceAll(checkpoint.Revision, "`", "'"))
		output.WriteString("| Claim | Classification | Document |\n| --- | --- | --- |\n")
		for _, claim := range checkpoint.Claims {
			fmt.Fprintf(&output, "| `%s` | %s | `%s` |\n", strings.ReplaceAll(claim.ClaimID, "|", "\\|"), claim.Classification, strings.ReplaceAll(claim.Document, "|", "\\|"))
		}
		output.WriteByte('\n')
	}
	return output.String()
}

func evalClaimReplayHelpText() string {
	return `openknowledge eval claims

Replay typed claims across immutable Git checkpoints.

Usage:
  openknowledge eval claims <dataset> [key-or-path]
  openknowledge eval claims <dataset> [key-or-path] --format json|markdown
  openknowledge eval claims <dataset> [key-or-path] --max-stale 0 --max-hallucinated 0

Flags:
  --format              Output format: text, json, or markdown. Defaults to text.
  --json                Alias for --format json.
  --out                 Atomically write a JSON or Markdown report.
  --spec                OKF spec version. Defaults to latest.
  --max-stale           Maximum allowed stale classifications. Defaults to 0.
  --max-hallucinated    Maximum allowed hallucinated classifications. Defaults to 0.
  --max-unverified      Maximum allowed unverified classifications. Defaults to 0.
`
}
