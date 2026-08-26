package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type evalRetrievalOptions struct {
	dataset        string
	target         string
	format         string
	out            string
	spec           string
	embeddingURL   string
	embeddingModel string
	embeddingCache string
}

func runEvalRetrieval(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, evalRetrievalHelpText())
		return 0
	}
	options, err := parseEvalRetrievalOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	loaded, err := knowledgeeval.LoadRetrievalDataset(options.dataset)
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
	systems := []knowledgeeval.RetrievalSystemOptions{{Name: "local-hash"}}
	provider, cachePath, err := queryEmbeddingProvider(context.Background(), root, queryCommandOptions{
		embeddingURL: options.embeddingURL, embeddingModel: options.embeddingModel, embeddingCache: options.embeddingCache,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if provider != nil {
		model := provider.Model()
		systems = append(systems, knowledgeeval.RetrievalSystemOptions{Name: model.Provider + "/" + model.ID, Embedding: provider, EmbeddingCache: cachePath})
	}
	report, err := knowledgeeval.RunRetrieval(context.Background(), root, resolvedSpec, loaded, systems)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printRetrievalReport(report, options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if report.Summary.Status == "fail" {
		return 1
	}
	return 0
}

func parseEvalRetrievalOptions(args []string) (evalRetrievalOptions, error) {
	model := strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_MODEL"))
	if model == "" {
		model = okf.DefaultHTTPEmbeddingModel
	}
	options := evalRetrievalOptions{
		target: ".", format: "text", spec: "latest",
		embeddingURL: strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_URL")), embeddingModel: model,
		embeddingCache: strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_CACHE")),
	}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name, inlineValue, inline := strings.Cut(arg, "=")
		next := func() (string, error) {
			if inline {
				if strings.TrimSpace(inlineValue) == "" {
					return "", fmt.Errorf("%s requires a value", name)
				}
				return inlineValue, nil
			}
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return "", fmt.Errorf("%s requires a value", name)
			}
			return args[index], nil
		}
		switch name {
		case "--json":
			if inline {
				return evalRetrievalOptions{}, errorsFlagTakesNoValue(name)
			}
			options.format = "json"
		case "--format", "--out", "--spec", "--embedding-url", "--embedding-model", "--embedding-cache":
			value, err := next()
			if err != nil {
				return evalRetrievalOptions{}, err
			}
			switch name {
			case "--format":
				options.format = strings.ToLower(strings.TrimSpace(value))
			case "--out":
				options.out = strings.TrimSpace(value)
			case "--spec":
				options.spec = strings.TrimSpace(value)
			case "--embedding-url":
				options.embeddingURL = strings.TrimSpace(value)
			case "--embedding-model":
				options.embeddingModel = strings.TrimSpace(value)
			case "--embedding-cache":
				options.embeddingCache = strings.TrimSpace(value)
			}
		case "--help", "-h":
			return evalRetrievalOptions{}, errorsFlagTakesNoValue(name)
		default:
			if strings.HasPrefix(arg, "-") {
				return evalRetrievalOptions{}, fmt.Errorf("unknown flag: %s", arg)
			}
			operands = append(operands, arg)
		}
	}
	if len(operands) == 0 {
		return evalRetrievalOptions{}, fmt.Errorf("eval retrieval requires a dataset path")
	}
	if len(operands) > 2 {
		return evalRetrievalOptions{}, fmt.Errorf("eval retrieval accepts one dataset and at most one knowledge base")
	}
	options.dataset = operands[0]
	if len(operands) == 2 {
		options.target = operands[1]
	}
	if options.format != "text" && options.format != "json" {
		return evalRetrievalOptions{}, fmt.Errorf("unsupported retrieval eval format: %s", options.format)
	}
	if options.out != "" && options.format != "json" {
		return evalRetrievalOptions{}, fmt.Errorf("--out requires --format json")
	}
	if options.spec == "" {
		return evalRetrievalOptions{}, fmt.Errorf("--spec requires a value")
	}
	if options.embeddingURL != "" && options.embeddingModel == "" {
		return evalRetrievalOptions{}, fmt.Errorf("--embedding-url requires --embedding-model or OPENKNOWLEDGE_EMBEDDING_MODEL")
	}
	if options.embeddingURL == "" && options.embeddingCache != "" {
		return evalRetrievalOptions{}, fmt.Errorf("--embedding-cache requires --embedding-url or OPENKNOWLEDGE_EMBEDDING_URL")
	}
	return options, nil
}

func errorsFlagTakesNoValue(name string) error {
	return fmt.Errorf("%s does not accept a value", name)
}

func printRetrievalReport(report knowledgeeval.RetrievalReport, options evalRetrievalOptions) error {
	if options.format == "json" {
		content, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		if options.out == "" {
			_, err = os.Stdout.Write(content)
			return err
		}
		if err := writeOutputFileAtomically(options.out, content); err != nil {
			return err
		}
		terminal.success("Wrote retrieval eval report")
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return nil
	}
	terminal.title("Open Knowledge Retrieval Eval", "ranked retrieval quality")
	fmt.Printf("%s %s (%d queries)\n", terminal.muted("dataset"), report.Dataset.ID, report.Systems[0].Section.Queries)
	fmt.Printf("%s %s\n\n", terminal.muted("target"), terminal.path(report.Target.Root))
	for _, system := range report.Systems {
		fmt.Printf("  %s  index %.2f ms / queries %.2f ms\n", system.Name, system.IndexDurationMS, system.QueryDurationMS)
		fmt.Printf("    section  MRR %.3f%s\n", system.Section.MRR, retrievalCutoffText(system.Section.Cutoffs))
		fmt.Printf("    document MRR %.3f%s\n", system.Document.MRR, retrievalCutoffText(system.Document.Cutoffs))
		for _, category := range system.Categories {
			fmt.Printf("      %-16s section %.3f / document %.3f\n", category.Category, category.Section.MRR, category.Document.MRR)
		}
		fmt.Println()
	}
	if len(report.Gates) > 0 {
		fmt.Println("  quality gates")
		for _, gate := range report.Gates {
			actual := "not run"
			if gate.Actual != nil {
				actual = fmt.Sprintf("%.3f", *gate.Actual)
			}
			metric := strings.ToUpper(gate.Metric)
			if gate.Cutoff > 0 {
				metric = fmt.Sprintf("%s@%d", metric, gate.Cutoff)
			}
			fmt.Printf("    %-7s %-28s %s %s %.3f, got %s\n", gate.Status, gate.ID, gate.Level, metric, gate.Minimum, actual)
		}
		fmt.Printf("\n%s %s: %d passed, %d failed, %d skipped\n", terminal.status(report.Summary.Status), strings.ToUpper(report.Summary.Status), report.Summary.Passed, report.Summary.Failed, report.Summary.Skipped)
	}
	return nil
}

func retrievalCutoffText(cutoffs []knowledgeeval.RetrievalCutoffMetric) string {
	var values []string
	for _, cutoff := range cutoffs {
		values = append(values, fmt.Sprintf("R@%d %.3f nDCG@%d %.3f", cutoff.K, cutoff.Recall, cutoff.K, cutoff.NDCG))
	}
	if len(values) == 0 {
		return ""
	}
	return " / " + strings.Join(values, " / ")
}

func evalRetrievalHelpText() string {
	return `openknowledge eval retrieval

Measure ranked retrieval quality against graded relevance judgments.

Usage:
  openknowledge eval retrieval <dataset> [key-or-path]
  openknowledge eval retrieval <dataset> [key-or-path] --format json
  openknowledge eval retrieval <dataset> [key-or-path] --embedding-url <url>

Arguments:
  dataset      YAML dataset with type openknowledge.retrieval-eval and version 1.
  key-or-path  Registry key or bundle directory. Defaults to the current directory.

Flags:
  --format           Output format: text or json. Defaults to text.
  --json             Alias for --format json.
  --out              Atomically write a JSON report to a file.
  --spec             OKF spec version. Defaults to latest.
  --embedding-url    OpenAI-compatible base URL or embedding endpoint.
  --embedding-model  Provider model ID. Defaults to embeddinggemma.
  --embedding-cache  Persistent vector cache path. Defaults to the user cache.

The report compares the local hash route with the configured embedding provider.
It reports section and document MRR, Recall@k, and nDCG@k.
Declared quality gates fail the command when a measured value is below its minimum.

Gate systems:
  all              Test every measured system.
  local-hash       Test the deterministic local baseline.
  embedding        Test each configured embedding provider.
  embedding-delta  Test embedding uplift from the local baseline.

Exit codes:
  0  Every measured quality gate passed. Unavailable embedding gates can skip.
  1  A measured quality gate failed, or evaluation could not run.
  2  Command usage or dataset validation failed.
`
}
