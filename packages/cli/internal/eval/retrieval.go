package eval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	"go.yaml.in/yaml/v3"
)

const (
	RetrievalDatasetType    = "openknowledge.retrieval-eval"
	RetrievalDatasetVersion = 1
	RetrievalReportVersion  = "1"
)

type RetrievalDataset struct {
	Type    string              `json:"type" yaml:"type"`
	Version int                 `json:"version" yaml:"version"`
	ID      string              `json:"id" yaml:"id"`
	Cutoffs []int               `json:"cutoffs" yaml:"cutoffs"`
	Gates   []RetrievalGate     `json:"gates,omitempty" yaml:"gates,omitempty"`
	Cases   []RetrievalEvalCase `json:"cases" yaml:"cases"`
}

type RetrievalGate struct {
	ID      string  `json:"id" yaml:"id"`
	System  string  `json:"system" yaml:"system"`
	Level   string  `json:"level" yaml:"level"`
	Metric  string  `json:"metric" yaml:"metric"`
	Cutoff  int     `json:"cutoff,omitempty" yaml:"cutoff,omitempty"`
	Minimum float64 `json:"minimum" yaml:"minimum"`
}

type RetrievalEvalCase struct {
	ID        string              `json:"id" yaml:"id"`
	Category  string              `json:"category" yaml:"category"`
	Query     string              `json:"query" yaml:"query"`
	Judgments []RetrievalJudgment `json:"judgments" yaml:"judgments"`
}

type RetrievalJudgment struct {
	Path      string `json:"path" yaml:"path"`
	Heading   string `json:"heading,omitempty" yaml:"heading,omitempty"`
	Relevance int    `json:"relevance" yaml:"relevance"`
}

type LoadedRetrievalDataset struct {
	Dataset RetrievalDataset
	Path    string
	SHA256  string
}

type RetrievalSystemOptions struct {
	Name           string
	Embedding      okf.EmbeddingProvider
	EmbeddingCache string
}

type RetrievalReport struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Dataset       RetrievalDatasetIdentity `json:"dataset"`
	Target        RetrievalTarget          `json:"target"`
	Systems       []RetrievalSystemReport  `json:"systems"`
	Gates         []RetrievalGateResult    `json:"gates"`
	Summary       RetrievalGateSummary     `json:"summary"`
}

type RetrievalGateResult struct {
	ID      string   `json:"id"`
	System  string   `json:"system"`
	Level   string   `json:"level"`
	Metric  string   `json:"metric"`
	Cutoff  int      `json:"cutoff,omitempty"`
	Minimum float64  `json:"minimum"`
	Actual  *float64 `json:"actual,omitempty"`
	Status  string   `json:"status"`
	Reason  string   `json:"reason,omitempty"`
}

type RetrievalGateSummary struct {
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
}

type RetrievalDatasetIdentity struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	ID      string `json:"id"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

type RetrievalTarget struct {
	Root     string                `json:"root"`
	Revision okf.RetrievalRevision `json:"revision"`
}

type RetrievalSystemReport struct {
	Name            string                    `json:"name"`
	Model           okf.EmbeddingModel        `json:"model"`
	IndexDurationMS float64                   `json:"indexDurationMs"`
	QueryDurationMS float64                   `json:"queryDurationMs"`
	Section         RetrievalMetricSummary    `json:"section"`
	Document        RetrievalMetricSummary    `json:"document"`
	Categories      []RetrievalCategoryReport `json:"categories"`
	Cases           []RetrievalCaseReport     `json:"cases"`
}

type RetrievalCategoryReport struct {
	Category string                 `json:"category"`
	Section  RetrievalMetricSummary `json:"section"`
	Document RetrievalMetricSummary `json:"document"`
}

type RetrievalMetricSummary struct {
	Queries int                     `json:"queries"`
	MRR     float64                 `json:"mrr"`
	Cutoffs []RetrievalCutoffMetric `json:"cutoffs"`
}

type RetrievalCutoffMetric struct {
	K      int     `json:"k"`
	Recall float64 `json:"recall"`
	NDCG   float64 `json:"ndcg"`
}

type RetrievalCaseReport struct {
	ID              string                  `json:"id"`
	Category        string                  `json:"category"`
	Query           string                  `json:"query"`
	Section         RetrievalQueryMetrics   `json:"section"`
	Document        RetrievalQueryMetrics   `json:"document"`
	Results         []RetrievalRankedResult `json:"results"`
	QueryDurationMS float64                 `json:"queryDurationMs"`
}

type RetrievalQueryMetrics struct {
	ReciprocalRank float64                 `json:"reciprocalRank"`
	Cutoffs        []RetrievalCutoffMetric `json:"cutoffs"`
}

type RetrievalRankedResult struct {
	Rank      int     `json:"rank"`
	Path      string  `json:"path"`
	Heading   string  `json:"heading,omitempty"`
	Score     float64 `json:"score"`
	Relevance int     `json:"relevance"`
}

func LoadRetrievalDataset(path string) (LoadedRetrievalDataset, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return LoadedRetrievalDataset{}, err
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return LoadedRetrievalDataset{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var dataset RetrievalDataset
	if err := decoder.Decode(&dataset); err != nil {
		return LoadedRetrievalDataset{}, fmt.Errorf("retrieval eval dataset YAML is invalid: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return LoadedRetrievalDataset{}, errors.New("retrieval eval dataset YAML must contain one document")
		}
		return LoadedRetrievalDataset{}, fmt.Errorf("retrieval eval dataset YAML is invalid: %w", err)
	}
	if err := ValidateRetrievalDataset(dataset); err != nil {
		return LoadedRetrievalDataset{}, err
	}
	digest := sha256.Sum256(content)
	return LoadedRetrievalDataset{Dataset: dataset, Path: absolute, SHA256: hex.EncodeToString(digest[:])}, nil
}

func ValidateRetrievalDataset(dataset RetrievalDataset) error {
	if dataset.Type != RetrievalDatasetType {
		return fmt.Errorf("retrieval eval dataset type must be %q", RetrievalDatasetType)
	}
	if dataset.Version != RetrievalDatasetVersion {
		return fmt.Errorf("retrieval eval dataset version must be %d", RetrievalDatasetVersion)
	}
	if !datasetIDPattern.MatchString(dataset.ID) {
		return errors.New("retrieval eval dataset id is invalid")
	}
	if len(dataset.Cutoffs) == 0 || len(dataset.Cutoffs) > 10 {
		return errors.New("retrieval eval cutoffs must contain between 1 and 10 values")
	}
	previous := 0
	for _, cutoff := range dataset.Cutoffs {
		if cutoff < 1 || cutoff > 100 || cutoff <= previous {
			return errors.New("retrieval eval cutoffs must be unique ascending values between 1 and 100")
		}
		previous = cutoff
	}
	gateIDs := map[string]bool{}
	cutoffSet := map[int]bool{}
	for _, cutoff := range dataset.Cutoffs {
		cutoffSet[cutoff] = true
	}
	for index, gate := range dataset.Gates {
		prefix := fmt.Sprintf("retrieval eval gate %d", index)
		if !datasetIDPattern.MatchString(gate.ID) || gateIDs[gate.ID] {
			return fmt.Errorf("%s has an invalid or duplicate id", prefix)
		}
		gateIDs[gate.ID] = true
		if gate.System != "all" && gate.System != "local-hash" && gate.System != "embedding" && gate.System != "embedding-delta" {
			return fmt.Errorf("%s system must be all, local-hash, embedding, or embedding-delta", prefix)
		}
		if gate.Level != "section" && gate.Level != "document" {
			return fmt.Errorf("%s level must be section or document", prefix)
		}
		if gate.Metric != "mrr" && gate.Metric != "recall" && gate.Metric != "ndcg" {
			return fmt.Errorf("%s metric must be mrr, recall, or ndcg", prefix)
		}
		if gate.Minimum < 0 || gate.Minimum > 1 {
			return fmt.Errorf("%s minimum must be between 0 and 1", prefix)
		}
		if gate.Metric == "mrr" && gate.Cutoff != 0 {
			return fmt.Errorf("%s cutoff is not valid for mrr", prefix)
		}
		if gate.Metric != "mrr" && !cutoffSet[gate.Cutoff] {
			return fmt.Errorf("%s cutoff must occur in dataset cutoffs", prefix)
		}
	}
	if len(dataset.Cases) == 0 {
		return errors.New("retrieval eval dataset requires at least one case")
	}
	caseIDs := map[string]bool{}
	for index, evalCase := range dataset.Cases {
		prefix := fmt.Sprintf("retrieval eval case %d", index)
		if !datasetIDPattern.MatchString(evalCase.ID) || caseIDs[evalCase.ID] {
			return fmt.Errorf("%s has an invalid or duplicate id", prefix)
		}
		caseIDs[evalCase.ID] = true
		if !datasetIDPattern.MatchString(evalCase.Category) {
			return fmt.Errorf("%s has an invalid category", prefix)
		}
		if strings.TrimSpace(evalCase.Query) == "" || len(evalCase.Query) > 4096 {
			return fmt.Errorf("%s query must contain between 1 and 4096 bytes", prefix)
		}
		if len(evalCase.Judgments) == 0 {
			return fmt.Errorf("%s requires at least one judgment", prefix)
		}
		seen := map[string]bool{}
		for _, judgment := range evalCase.Judgments {
			judgment.Path = filepath.ToSlash(strings.TrimSpace(judgment.Path))
			key := judgment.Path + "\x00" + normalizeRetrievalText(judgment.Heading)
			if judgment.Path == "" || filepath.IsAbs(judgment.Path) || strings.HasPrefix(judgment.Path, "../") {
				return fmt.Errorf("%s has an invalid judgment path", prefix)
			}
			if judgment.Relevance < 1 || judgment.Relevance > 3 {
				return fmt.Errorf("%s judgment relevance must be between 1 and 3", prefix)
			}
			if seen[key] {
				return fmt.Errorf("%s has a duplicate judgment", prefix)
			}
			seen[key] = true
		}
	}
	return nil
}

func RunRetrieval(ctx context.Context, root, version string, loaded LoadedRetrievalDataset, systems []RetrievalSystemOptions) (RetrievalReport, error) {
	if len(systems) == 0 {
		systems = []RetrievalSystemOptions{{Name: "local-hash"}}
	}
	report := RetrievalReport{
		SchemaVersion: RetrievalReportVersion,
		Dataset:       RetrievalDatasetIdentity{Type: loaded.Dataset.Type, Version: loaded.Dataset.Version, ID: loaded.Dataset.ID, Path: loaded.Path, SHA256: loaded.SHA256},
		Systems:       []RetrievalSystemReport{},
		Gates:         []RetrievalGateResult{},
		Summary:       RetrievalGateSummary{Status: "pass"},
	}
	seenSystems := map[string]bool{}
	for _, system := range systems {
		name := strings.TrimSpace(system.Name)
		if name == "" {
			return RetrievalReport{}, errors.New("retrieval system name is required")
		}
		if seenSystems[name] {
			return RetrievalReport{}, fmt.Errorf("duplicate retrieval system name: %s", name)
		}
		seenSystems[name] = true
		started := time.Now()
		snapshot, err := okf.BuildHybridSnapshotWithVersion(root, version, okf.HybridQueryOptions{Embedding: system.Embedding, EmbeddingCache: system.EmbeddingCache})
		if err != nil {
			return RetrievalReport{}, fmt.Errorf("build %s retrieval index: %w", name, err)
		}
		indexDuration := time.Since(started)
		model := okf.HashedEmbeddingProvider{}.Model()
		if system.Embedding != nil {
			model = system.Embedding.Model()
		}
		systemReport := RetrievalSystemReport{Name: name, Model: model, IndexDurationMS: retrievalMilliseconds(indexDuration), Categories: []RetrievalCategoryReport{}, Cases: []RetrievalCaseReport{}}
		maxK := loaded.Dataset.Cutoffs[len(loaded.Dataset.Cutoffs)-1]
		var queryDuration time.Duration
		for _, evalCase := range loaded.Dataset.Cases {
			queryStarted := time.Now()
			result, err := snapshot.Query(ctx, okf.HybridQuery{Text: evalCase.Query, Limit: maxK})
			elapsed := time.Since(queryStarted)
			queryDuration += elapsed
			if err != nil {
				return RetrievalReport{}, fmt.Errorf("run %s case %s: %w", name, evalCase.ID, err)
			}
			caseReport := scoreRetrievalCase(evalCase, loaded.Dataset.Cutoffs, result.Results)
			caseReport.QueryDurationMS = retrievalMilliseconds(elapsed)
			systemReport.Cases = append(systemReport.Cases, caseReport)
			if report.Target.Root == "" {
				report.Target = RetrievalTarget{Root: result.Root, Revision: result.Revision}
			}
		}
		systemReport.QueryDurationMS = retrievalMilliseconds(queryDuration)
		systemReport.Section, systemReport.Document = summarizeRetrievalCases(systemReport.Cases, loaded.Dataset.Cutoffs)
		categories := map[string][]RetrievalCaseReport{}
		for _, evalCase := range systemReport.Cases {
			categories[evalCase.Category] = append(categories[evalCase.Category], evalCase)
		}
		categoryNames := make([]string, 0, len(categories))
		for category := range categories {
			categoryNames = append(categoryNames, category)
		}
		sort.Strings(categoryNames)
		for _, category := range categoryNames {
			section, document := summarizeRetrievalCases(categories[category], loaded.Dataset.Cutoffs)
			systemReport.Categories = append(systemReport.Categories, RetrievalCategoryReport{Category: category, Section: section, Document: document})
		}
		report.Systems = append(report.Systems, systemReport)
	}
	report.Gates, report.Summary = evaluateRetrievalGates(loaded.Dataset.Gates, report.Systems)
	return report, nil
}

func evaluateRetrievalGates(gates []RetrievalGate, systems []RetrievalSystemReport) ([]RetrievalGateResult, RetrievalGateSummary) {
	results := []RetrievalGateResult{}
	summary := RetrievalGateSummary{Status: "pass"}
	var baseline *RetrievalSystemReport
	var embeddings []RetrievalSystemReport
	for index := range systems {
		if systems[index].Name == "local-hash" {
			baseline = &systems[index]
		} else {
			embeddings = append(embeddings, systems[index])
		}
	}
	for _, gate := range gates {
		var selected []RetrievalSystemReport
		switch gate.System {
		case "all":
			selected = systems
		case "local-hash":
			if baseline != nil {
				selected = []RetrievalSystemReport{*baseline}
			}
		case "embedding", "embedding-delta":
			selected = embeddings
		}
		if len(selected) == 0 || (gate.System == "embedding-delta" && baseline == nil) {
			reason := "matching retrieval system is not configured"
			if gate.System == "embedding-delta" && baseline == nil {
				reason = "local-hash baseline is not configured"
			}
			results = append(results, RetrievalGateResult{ID: gate.ID, System: gate.System, Level: gate.Level, Metric: gate.Metric, Cutoff: gate.Cutoff, Minimum: gate.Minimum, Status: "skipped", Reason: reason})
			continue
		}
		for _, system := range selected {
			actual := retrievalGateMetric(system, gate)
			if gate.System == "embedding-delta" {
				actual -= retrievalGateMetric(*baseline, gate)
			}
			actual = roundRetrievalMetric(actual)
			status := "pass"
			if actual+1e-9 < gate.Minimum {
				status = "fail"
			}
			actualCopy := actual
			results = append(results, RetrievalGateResult{ID: gate.ID, System: system.Name, Level: gate.Level, Metric: gate.Metric, Cutoff: gate.Cutoff, Minimum: gate.Minimum, Actual: &actualCopy, Status: status})
		}
	}
	summary.Total = len(results)
	for _, result := range results {
		switch result.Status {
		case "pass":
			summary.Passed++
		case "fail":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}
	if summary.Failed > 0 {
		summary.Status = "fail"
	}
	return results, summary
}

func retrievalGateMetric(system RetrievalSystemReport, gate RetrievalGate) float64 {
	metrics := system.Section
	if gate.Level == "document" {
		metrics = system.Document
	}
	if gate.Metric == "mrr" {
		return metrics.MRR
	}
	for _, cutoff := range metrics.Cutoffs {
		if cutoff.K == gate.Cutoff {
			if gate.Metric == "recall" {
				return cutoff.Recall
			}
			return cutoff.NDCG
		}
	}
	return 0
}

func scoreRetrievalCase(evalCase RetrievalEvalCase, cutoffs []int, results []okf.HybridResult) RetrievalCaseReport {
	sectionJudgments := map[string]int{}
	pathJudgments := map[string]int{}
	documentJudgments := map[string]int{}
	for _, judgment := range evalCase.Judgments {
		path := filepath.ToSlash(strings.TrimSpace(judgment.Path))
		if strings.TrimSpace(judgment.Heading) == "" {
			pathJudgments[path] = judgment.Relevance
			sectionJudgments[retrievalSectionKey(path, "*")] = judgment.Relevance
		} else {
			sectionJudgments[retrievalSectionKey(path, judgment.Heading)] = judgment.Relevance
		}
		if judgment.Relevance > documentJudgments[path] {
			documentJudgments[path] = judgment.Relevance
		}
	}
	sectionKeys := []string{}
	documentKeys := []string{}
	ranked := []RetrievalRankedResult{}
	seenDocuments := map[string]bool{}
	for _, result := range results {
		if result.Text == nil {
			continue
		}
		path := filepath.ToSlash(result.Text.Path)
		sectionKey := retrievalSectionKey(path, result.Text.Heading)
		relevance := sectionJudgments[sectionKey]
		if relevance == 0 && pathJudgments[path] > 0 {
			sectionKey = retrievalSectionKey(path, "*")
			relevance = pathJudgments[path]
		}
		sectionKeys = append(sectionKeys, sectionKey)
		if !seenDocuments[path] {
			documentKeys = append(documentKeys, path)
			seenDocuments[path] = true
		}
		ranked = append(ranked, RetrievalRankedResult{Rank: len(ranked) + 1, Path: path, Heading: result.Text.Heading, Score: result.Score, Relevance: relevance})
	}
	return RetrievalCaseReport{
		ID: evalCase.ID, Category: evalCase.Category, Query: evalCase.Query,
		Section:  scoreRetrievalRanking(sectionKeys, sectionJudgments, cutoffs),
		Document: scoreRetrievalRanking(documentKeys, documentJudgments, cutoffs), Results: ranked,
	}
}

func scoreRetrievalRanking(ranked []string, judgments map[string]int, cutoffs []int) RetrievalQueryMetrics {
	metrics := RetrievalQueryMetrics{Cutoffs: make([]RetrievalCutoffMetric, 0, len(cutoffs))}
	uniqueRanked := make([]string, 0, len(ranked))
	seen := map[string]bool{}
	for _, key := range ranked {
		if !seen[key] {
			uniqueRanked = append(uniqueRanked, key)
			seen[key] = true
		}
	}
	ranked = uniqueRanked
	for rank, key := range ranked {
		if judgments[key] > 0 {
			metrics.ReciprocalRank = 1 / float64(rank+1)
			break
		}
	}
	for _, cutoff := range cutoffs {
		retrievedRelevant := 0
		dcg := 0.0
		for rank := 0; rank < len(ranked) && rank < cutoff; rank++ {
			grade := judgments[ranked[rank]]
			if grade > 0 {
				retrievedRelevant++
				dcg += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(rank)+2)
			}
		}
		grades := make([]int, 0, len(judgments))
		for _, grade := range judgments {
			if grade > 0 {
				grades = append(grades, grade)
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(grades)))
		idcg := 0.0
		for rank := 0; rank < len(grades) && rank < cutoff; rank++ {
			idcg += (math.Pow(2, float64(grades[rank])) - 1) / math.Log2(float64(rank)+2)
		}
		recall := 0.0
		if len(grades) > 0 {
			recall = float64(retrievedRelevant) / float64(len(grades))
		}
		ndcg := 0.0
		if idcg > 0 {
			ndcg = dcg / idcg
		}
		metrics.Cutoffs = append(metrics.Cutoffs, RetrievalCutoffMetric{K: cutoff, Recall: roundRetrievalMetric(recall), NDCG: roundRetrievalMetric(ndcg)})
	}
	metrics.ReciprocalRank = roundRetrievalMetric(metrics.ReciprocalRank)
	return metrics
}

func summarizeRetrievalCases(cases []RetrievalCaseReport, cutoffs []int) (RetrievalMetricSummary, RetrievalMetricSummary) {
	section := RetrievalMetricSummary{Queries: len(cases), Cutoffs: make([]RetrievalCutoffMetric, len(cutoffs))}
	document := RetrievalMetricSummary{Queries: len(cases), Cutoffs: make([]RetrievalCutoffMetric, len(cutoffs))}
	for index, cutoff := range cutoffs {
		section.Cutoffs[index].K = cutoff
		document.Cutoffs[index].K = cutoff
	}
	if len(cases) == 0 {
		return section, document
	}
	for _, evalCase := range cases {
		section.MRR += evalCase.Section.ReciprocalRank
		document.MRR += evalCase.Document.ReciprocalRank
		for index := range cutoffs {
			section.Cutoffs[index].Recall += evalCase.Section.Cutoffs[index].Recall
			section.Cutoffs[index].NDCG += evalCase.Section.Cutoffs[index].NDCG
			document.Cutoffs[index].Recall += evalCase.Document.Cutoffs[index].Recall
			document.Cutoffs[index].NDCG += evalCase.Document.Cutoffs[index].NDCG
		}
	}
	denominator := float64(len(cases))
	section.MRR = roundRetrievalMetric(section.MRR / denominator)
	document.MRR = roundRetrievalMetric(document.MRR / denominator)
	for index := range cutoffs {
		section.Cutoffs[index].Recall = roundRetrievalMetric(section.Cutoffs[index].Recall / denominator)
		section.Cutoffs[index].NDCG = roundRetrievalMetric(section.Cutoffs[index].NDCG / denominator)
		document.Cutoffs[index].Recall = roundRetrievalMetric(document.Cutoffs[index].Recall / denominator)
		document.Cutoffs[index].NDCG = roundRetrievalMetric(document.Cutoffs[index].NDCG / denominator)
	}
	return section, document
}

func retrievalSectionKey(path, heading string) string {
	return filepath.ToSlash(strings.TrimSpace(path)) + "\x00" + normalizeRetrievalText(heading)
}

func normalizeRetrievalText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func retrievalMilliseconds(duration time.Duration) float64 {
	return math.Round(float64(duration.Microseconds())/10) / 100
}

func roundRetrievalMetric(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
