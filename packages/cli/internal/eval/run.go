package eval

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type Report struct {
	SchemaVersion string          `json:"schemaVersion"`
	Dataset       DatasetIdentity `json:"dataset"`
	Target        TargetIdentity  `json:"target"`
	Summary       Summary         `json:"summary"`
	Cases         []CaseResult    `json:"cases"`
}

type DatasetIdentity struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	ID      string `json:"id"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

type TargetIdentity struct {
	Root     string                `json:"root"`
	Revision okf.RetrievalRevision `json:"revision"`
}

type Summary struct {
	Status       string `json:"status"`
	Total        int    `json:"total"`
	Passed       int    `json:"passed"`
	Failed       int    `json:"failed"`
	Checks       int    `json:"checks"`
	PassedChecks int    `json:"passedChecks"`
}

type CaseResult struct {
	ID          string        `json:"id"`
	Question    string        `json:"question"`
	Status      string        `json:"status"`
	Context     ContextResult `json:"context"`
	Metrics     Metrics       `json:"metrics"`
	Checks      []Check       `json:"checks"`
	Answer      *AnswerResult `json:"answer,omitempty"`
	answerInput []AnswerSource
}

type ContextResult struct {
	Budget          int      `json:"budget"`
	Limit           int      `json:"limit"`
	NoExpand        bool     `json:"noExpand"`
	EstimatedTokens int      `json:"estimatedTokens"`
	Sources         []Source `json:"sources"`
}

type Source struct {
	ID            string  `json:"id"`
	Path          string  `json:"path"`
	Title         string  `json:"title"`
	Heading       string  `json:"heading"`
	Locator       string  `json:"locator"`
	ContentSHA256 string  `json:"contentSha256"`
	Relation      string  `json:"relation"`
	Score         float64 `json:"score"`
}

type Metrics struct {
	ExpectedSources int      `json:"expectedSources"`
	MatchedSources  int      `json:"matchedSources"`
	SourceRecall    float64  `json:"sourceRecall"`
	Checks          int      `json:"checks"`
	PassedChecks    int      `json:"passedChecks"`
	Citations       int      `json:"citations,omitempty"`
	ValidCitations  int      `json:"validCitations,omitempty"`
	Groundedness    *float64 `json:"groundedness,omitempty"`
}

type Check struct {
	Kind     string `json:"kind"`
	Expected string `json:"expected"`
	Actual   string `json:"actual,omitempty"`
	Passed   bool   `json:"passed"`
}

func Run(root string, specVersion string, loaded LoadedDataset) (Report, error) {
	if DatasetRequiresAnswerRunner(loaded.Dataset) {
		return Report{}, fmt.Errorf("eval dataset %s contains answer expectations; use an answer runner", loaded.Dataset.ID)
	}
	return runRetrieval(root, specVersion, loaded)
}

func runRetrieval(root string, specVersion string, loaded LoadedDataset) (Report, error) {
	index, err := okf.BuildContextIndexWithVersion(root, specVersion)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		SchemaVersion: okf.MachineSchemaVersion,
		Dataset: DatasetIdentity{
			Type: DatasetType, Version: DatasetVersion, ID: loaded.Dataset.ID,
			Path: loaded.Path, SHA256: loaded.SHA256,
		},
		Target:  TargetIdentity{Root: index.Root, Revision: index.Revision},
		Summary: Summary{Status: "pass", Total: len(loaded.Dataset.Cases)},
		Cases:   make([]CaseResult, 0, len(loaded.Dataset.Cases)),
	}
	for _, evalCase := range loaded.Dataset.Cases {
		settings := resolveContextSettings(loaded.Dataset.Defaults, evalCase.Context)
		context, err := index.Resolve(okf.ContextOptions{
			Query: evalCase.Question, Budget: settings.Budget, Limit: settings.Limit, NoExpand: contextNoExpand(settings),
		})
		if err != nil {
			return Report{}, fmt.Errorf("eval case %s: %w", evalCase.ID, err)
		}
		result := evaluateCase(evalCase, settings, context)
		report.Cases = append(report.Cases, result)
		report.Summary.Checks += result.Metrics.Checks
		report.Summary.PassedChecks += result.Metrics.PassedChecks
		if result.Status == "pass" {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
			report.Summary.Status = "fail"
		}
	}
	return report, nil
}

func resolveContextSettings(defaults ContextSettings, override ContextSettings) ContextSettings {
	settings := defaults
	if override.Budget > 0 {
		settings.Budget = override.Budget
	}
	if override.Limit > 0 {
		settings.Limit = override.Limit
	}
	if override.NoExpand != nil {
		settings.NoExpand = override.NoExpand
	}
	if settings.Budget == 0 {
		settings.Budget = okf.DefaultContextBudget
	}
	if settings.Limit == 0 {
		settings.Limit = 12
	}
	return settings
}

func contextNoExpand(settings ContextSettings) bool {
	return settings.NoExpand != nil && *settings.NoExpand
}

func evaluateCase(evalCase Case, settings ContextSettings, context okf.ContextResult) CaseResult {
	result := CaseResult{
		ID: evalCase.ID, Question: strings.TrimSpace(evalCase.Question), Status: "pass",
		Context: ContextResult{
			Budget: settings.Budget, Limit: settings.Limit, NoExpand: contextNoExpand(settings),
			EstimatedTokens: context.EstimatedTokens, Sources: make([]Source, 0, len(context.Sources)),
		},
		Checks: []Check{},
	}
	var evidence strings.Builder
	for _, source := range context.Sources {
		result.Context.Sources = append(result.Context.Sources, Source{
			ID: source.ID, Path: filepath.ToSlash(source.Path), Title: source.Title, Heading: source.Heading,
			Locator: source.Locator, ContentSHA256: source.ContentSHA256, Relation: source.Relation, Score: source.Score,
		})
		result.answerInput = append(result.answerInput, AnswerSource{
			ID: source.ID, Path: filepath.ToSlash(source.Path), Title: source.Title, Heading: source.Heading,
			Locator: source.Locator, ContentSHA256: source.ContentSHA256, Relation: source.Relation, Markdown: source.Markdown,
		})
		evidence.WriteString(source.Markdown)
		evidence.WriteByte('\n')
	}
	for _, expected := range evalCase.Expect.Sources {
		normalized, _ := normalizeExpectedSource(expected)
		matched := findExpectedSource(normalized, result.Context.Sources)
		result.Checks = append(result.Checks, Check{Kind: "source", Expected: normalized, Actual: matched, Passed: matched != ""})
		if matched != "" {
			result.Metrics.MatchedSources++
		}
	}
	result.Metrics.ExpectedSources = len(evalCase.Expect.Sources)
	result.Metrics.SourceRecall = 1
	if result.Metrics.ExpectedSources > 0 {
		result.Metrics.SourceRecall = float64(result.Metrics.MatchedSources) / float64(result.Metrics.ExpectedSources)
	}
	normalizedEvidence := strings.ToLower(evidence.String())
	for _, expected := range evalCase.Expect.EvidenceContains {
		passed := strings.Contains(normalizedEvidence, strings.ToLower(strings.TrimSpace(expected)))
		result.Checks = append(result.Checks, Check{Kind: "evidence_contains", Expected: strings.TrimSpace(expected), Passed: passed})
	}
	for _, expected := range evalCase.Expect.EvidenceExcludes {
		passed := !strings.Contains(normalizedEvidence, strings.ToLower(strings.TrimSpace(expected)))
		result.Checks = append(result.Checks, Check{Kind: "evidence_excludes", Expected: strings.TrimSpace(expected), Passed: passed})
	}
	if evalCase.Expect.MinSources > 0 {
		actual := len(result.Context.Sources)
		result.Checks = append(result.Checks, Check{
			Kind: "min_sources", Expected: fmt.Sprintf("%d", evalCase.Expect.MinSources),
			Actual: fmt.Sprintf("%d", actual), Passed: actual >= evalCase.Expect.MinSources,
		})
	}
	result.Metrics.Checks = len(result.Checks)
	for _, check := range result.Checks {
		if check.Passed {
			result.Metrics.PassedChecks++
		} else {
			result.Status = "fail"
		}
	}
	return result
}

func DatasetRequiresAnswerRunner(dataset Dataset) bool {
	for _, evalCase := range dataset.Cases {
		expect := evalCase.Expect
		if len(expect.AnswerContains) > 0 || len(expect.AnswerExcludes) > 0 || len(expect.CitationSources) > 0 ||
			expect.MinCitations > 0 || expect.MinGroundedness != nil {
			return true
		}
	}
	return false
}

func findExpectedSource(expected string, sources []Source) string {
	for _, source := range sources {
		path := filepath.ToSlash(source.Path)
		if expected == path || expected == source.ID {
			return source.ID
		}
		if strings.Contains(expected, "#") && expected == path+sourceFragment(source.ID) {
			return source.ID
		}
	}
	return ""
}

func sourceFragment(id string) string {
	if index := strings.IndexByte(id, '#'); index >= 0 {
		return id[index:]
	}
	return ""
}
