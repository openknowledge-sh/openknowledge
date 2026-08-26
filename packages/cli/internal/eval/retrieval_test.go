package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func writeRetrievalEvalFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type retrievalFixtureProvider struct{}

func (retrievalFixtureProvider) Model() okf.EmbeddingModel {
	return okf.EmbeddingModel{Provider: "fixture", ID: "retrieval", Revision: "1", Dimensions: 2, Metric: okf.EmbeddingMetricCosine}
}

func (retrievalFixtureProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for index, value := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lower := strings.ToLower(value)
		if strings.Contains(lower, "capacity") || strings.Contains(lower, "factory") || strings.Contains(lower, "výrob") {
			vectors[index] = []float32{1, 0}
		} else {
			vectors[index] = []float32{0, 1}
		}
	}
	return vectors, nil
}

func TestRetrievalMetricsScoreSectionAndDocumentSeparately(t *testing.T) {
	evalCase := RetrievalEvalCase{ID: "capacity", Category: "semantic", Query: "factory throughput", Judgments: []RetrievalJudgment{{Path: "calls/northstar.md", Heading: "Capacity", Relevance: 3}}}
	results := []okf.HybridResult{
		{Score: 2, Text: &okf.SearchResult{Path: "calls/northstar.md", Heading: "Outlook"}},
		{Score: 1, Text: &okf.SearchResult{Path: "calls/northstar.md", Heading: "Capacity"}},
	}
	report := scoreRetrievalCase(evalCase, []int{1, 2}, results)
	if report.Section.ReciprocalRank != 0.5 || report.Document.ReciprocalRank != 1 {
		t.Fatalf("unexpected section/document rank: %#v", report)
	}
	if report.Section.Cutoffs[0].Recall != 0 || report.Document.Cutoffs[0].Recall != 1 {
		t.Fatalf("unexpected section/document recall: %#v", report)
	}
}

func TestRetrievalPathJudgmentMatchesFirstSectionOnce(t *testing.T) {
	evalCase := RetrievalEvalCase{ID: "call", Category: "document", Query: "northstar", Judgments: []RetrievalJudgment{{Path: "calls/northstar.md", Relevance: 3}}}
	results := []okf.HybridResult{
		{Score: 2, Text: &okf.SearchResult{Path: "calls/northstar.md", Heading: "Outlook"}},
		{Score: 1, Text: &okf.SearchResult{Path: "calls/northstar.md", Heading: "Capacity"}},
	}
	report := scoreRetrievalCase(evalCase, []int{1, 2}, results)
	if report.Section.ReciprocalRank != 1 || report.Section.Cutoffs[1].Recall != 1 || report.Results[0].Relevance != 3 {
		t.Fatalf("path judgment did not match the first section once: %#v", report)
	}
}

func TestRunRetrievalProducesCategoryAndAggregateMetrics(t *testing.T) {
	root := t.TempDir()
	writeRetrievalEvalFile(t, root, "index.md", "---\nokf_version: \"0.2\"\ntype: Index\n---\n\n# Calls\n")
	writeRetrievalEvalFile(t, root, "northstar.md", "---\ntype: Earnings Call\ntitle: Northstar\n---\n\n# Northstar\n\n## Capacity\n\nThe new factory increases production throughput.\n")
	writeRetrievalEvalFile(t, root, "helio.md", "---\ntype: Earnings Call\ntitle: Helio\n---\n\n# Helio\n\n## Demand\n\nCustomer renewals remain strong.\n")
	datasetPath := filepath.Join(t.TempDir(), "retrieval.yaml")
	writeRetrievalEvalFile(t, filepath.Dir(datasetPath), filepath.Base(datasetPath), `type: openknowledge.retrieval-eval
version: 1
id: earnings
cutoffs: [1, 2]
cases:
  - id: capacity
    category: semantic
    query: factory capacity
    judgments:
      - path: northstar.md
        heading: Capacity
        relevance: 3
`)
	loaded, err := LoadRetrievalDataset(datasetPath)
	if err != nil {
		t.Fatal(err)
	}
	report, err := RunRetrieval(context.Background(), root, "0.2", loaded, []RetrievalSystemOptions{{Name: "fixture", Embedding: retrievalFixtureProvider{}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != "1" || len(report.Systems) != 1 || report.Systems[0].Section.MRR != 1 || len(report.Systems[0].Categories) != 1 {
		t.Fatalf("unexpected retrieval report: %#v", report)
	}
	validateRetrievalReportSchema(t, report)
}

func TestRetrievalDatasetRejectsUnknownFieldsAndInvalidJudgments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")
	writeRetrievalEvalFile(t, filepath.Dir(path), filepath.Base(path), "type: openknowledge.retrieval-eval\nversion: 1\nid: bad\ncutoffs: [1]\nunknown: true\ncases: []\n")
	if _, err := LoadRetrievalDataset(path); err == nil {
		t.Fatal("expected unknown field to fail")
	}
	dataset := RetrievalDataset{Type: RetrievalDatasetType, Version: 1, ID: "bad", Cutoffs: []int{3, 1}, Cases: []RetrievalEvalCase{{ID: "q", Category: "semantic", Query: "query", Judgments: []RetrievalJudgment{{Path: "doc.md", Relevance: 4}}}}}
	if err := ValidateRetrievalDataset(dataset); err == nil {
		t.Fatal("expected invalid cutoffs and relevance to fail")
	}
	dataset = RetrievalDataset{Type: RetrievalDatasetType, Version: 1, ID: "bad-gate", Cutoffs: []int{1}, Gates: []RetrievalGate{{ID: "gate", System: "embedding-delta", Level: "document", Metric: "recall", Cutoff: 3, Minimum: 0.5}}, Cases: []RetrievalEvalCase{{ID: "q", Category: "semantic", Query: "query", Judgments: []RetrievalJudgment{{Path: "doc.md", Relevance: 3}}}}}
	if err := ValidateRetrievalDataset(dataset); err == nil {
		t.Fatal("expected gate cutoff outside dataset cutoffs to fail")
	}
}

func TestRetrievalGatesEvaluateAbsoluteDeltaAndSkippedSystems(t *testing.T) {
	metric := func(mrr, recall float64) RetrievalMetricSummary {
		return RetrievalMetricSummary{MRR: mrr, Cutoffs: []RetrievalCutoffMetric{{K: 3, Recall: recall, NDCG: recall}}}
	}
	systems := []RetrievalSystemReport{
		{Name: "local-hash", Section: metric(0.5, 0.7), Document: metric(0.6, 0.75)},
		{Name: "http/model", Section: metric(0.9, 1), Document: metric(0.95, 1)},
	}
	gates := []RetrievalGate{
		{ID: "coverage", System: "all", Level: "document", Metric: "recall", Cutoff: 3, Minimum: 0.75},
		{ID: "uplift", System: "embedding-delta", Level: "section", Metric: "mrr", Minimum: 0.3},
		{ID: "too-high", System: "local-hash", Level: "section", Metric: "mrr", Minimum: 0.8},
	}
	results, summary := evaluateRetrievalGates(gates, systems)
	if summary.Status != "fail" || summary.Passed != 3 || summary.Failed != 1 || len(results) != 4 {
		t.Fatalf("unexpected gate results: results=%#v summary=%#v", results, summary)
	}
	if results[2].Actual == nil || *results[2].Actual != 0.4 || results[2].Status != "pass" {
		t.Fatalf("unexpected delta gate: %#v", results[2])
	}
	results, summary = evaluateRetrievalGates([]RetrievalGate{{ID: "uplift", System: "embedding-delta", Level: "section", Metric: "mrr", Minimum: 0.3}}, systems[:1])
	if summary.Status != "pass" || summary.Skipped != 1 || results[0].Status != "skipped" {
		t.Fatalf("missing embedding must skip delta gate: results=%#v summary=%#v", results, summary)
	}
}

func TestVersionedEarningsRetrievalBaseline(t *testing.T) {
	fixtureRoot := filepath.Join("testdata", "earnings-retrieval-v1")
	loaded, err := LoadRetrievalDataset(filepath.Join(fixtureRoot, "retrieval.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := RunRetrieval(context.Background(), filepath.Join(fixtureRoot, "knowledge"), "0.2", loaded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Systems) != 1 || report.Systems[0].Section.Queries != 12 {
		t.Fatalf("unexpected versioned retrieval fixture: %#v", report.Systems)
	}
	system := report.Systems[0]
	if system.Section.MRR != 0.694444 || system.Document.MRR != 0.694444 || system.Document.Cutoffs[1].Recall != 0.833333 {
		t.Fatalf("local hash relevance baseline changed: section MRR=%.6f document MRR=%.6f document R@3=%.6f", system.Section.MRR, system.Document.MRR, system.Document.Cutoffs[1].Recall)
	}
	if report.Summary.Status != "pass" || report.Summary.Passed != 2 || report.Summary.Skipped != 4 {
		t.Fatalf("unexpected deterministic fixture gates: %#v", report.Summary)
	}
}

func validateRetrievalReportSchema(t *testing.T, report RetrievalReport) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, name := range []string{"common.schema.json", "retrieval-eval-report.schema.json"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "schemas", "v1", name))
		if err != nil {
			t.Fatal(err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource("https://openknowledge.sh/schemas/cli/v1/"+name, document); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := compiler.Compile("https://openknowledge.sh/schemas/cli/v1/retrieval-eval-report.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("retrieval report does not satisfy its published schema: %v", err)
	}
}
