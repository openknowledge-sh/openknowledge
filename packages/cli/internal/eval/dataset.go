package eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	DatasetType    = "openknowledge.eval"
	DatasetVersion = 1
)

var datasetIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Dataset struct {
	Type     string          `json:"type" yaml:"type"`
	Version  int             `json:"version" yaml:"version"`
	ID       string          `json:"id" yaml:"id"`
	Defaults ContextSettings `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Cases    []Case          `json:"cases" yaml:"cases"`
}

type ContextSettings struct {
	Budget   int   `json:"budget,omitempty" yaml:"budget,omitempty"`
	Limit    int   `json:"limit,omitempty" yaml:"limit,omitempty"`
	NoExpand *bool `json:"no_expand,omitempty" yaml:"no_expand,omitempty"`
}

type Case struct {
	ID       string          `json:"id" yaml:"id"`
	Question string          `json:"question" yaml:"question"`
	Agents   []string        `json:"agents,omitempty" yaml:"agents,omitempty"`
	Context  ContextSettings `json:"context,omitempty" yaml:"context,omitempty"`
	Expect   Expectations    `json:"expect" yaml:"expect"`
}

type Expectations struct {
	Sources                   []string `json:"sources,omitempty" yaml:"sources,omitempty"`
	EvidenceContains          []string `json:"evidence_contains,omitempty" yaml:"evidence_contains,omitempty"`
	EvidenceExcludes          []string `json:"evidence_excludes,omitempty" yaml:"evidence_excludes,omitempty"`
	AnswerContains            []string `json:"answer_contains,omitempty" yaml:"answer_contains,omitempty"`
	AnswerExcludes            []string `json:"answer_excludes,omitempty" yaml:"answer_excludes,omitempty"`
	CitationSources           []string `json:"citation_sources,omitempty" yaml:"citation_sources,omitempty"`
	MinSources                int      `json:"min_sources,omitempty" yaml:"min_sources,omitempty"`
	MinCitations              int      `json:"min_citations,omitempty" yaml:"min_citations,omitempty"`
	MinGroundedness           *float64 `json:"min_groundedness,omitempty" yaml:"min_groundedness,omitempty"`
	AnswerDecision            string   `json:"answer_decision,omitempty" yaml:"answer_decision,omitempty"`
	RequireConflictDisclosure *bool    `json:"require_conflict_disclosure,omitempty" yaml:"require_conflict_disclosure,omitempty"`
	MinEntailedCitations      int      `json:"min_entailed_citations,omitempty" yaml:"min_entailed_citations,omitempty"`
	MinimumTrust              string   `json:"minimum_trust,omitempty" yaml:"minimum_trust,omitempty"`
	AllowStale                *bool    `json:"allow_stale,omitempty" yaml:"allow_stale,omitempty"`
	AllowedStatuses           []string `json:"allowed_statuses,omitempty" yaml:"allowed_statuses,omitempty"`
	RequireSources            *bool    `json:"require_sources,omitempty" yaml:"require_sources,omitempty"`
}

type LoadedDataset struct {
	Dataset Dataset
	Path    string
	SHA256  string
}

type ValidationIssue struct {
	Field   string
	Message string
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (err ValidationError) Error() string {
	if len(err.Issues) == 0 {
		return "eval dataset is invalid"
	}
	return fmt.Sprintf("eval dataset is invalid: %s: %s", err.Issues[0].Field, err.Issues[0].Message)
}

func LoadDataset(path string) (LoadedDataset, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return LoadedDataset{}, err
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return LoadedDataset{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var dataset Dataset
	if err := decoder.Decode(&dataset); err != nil {
		return LoadedDataset{}, fmt.Errorf("eval dataset YAML is invalid: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return LoadedDataset{}, fmt.Errorf("eval dataset YAML must contain one document")
		}
		return LoadedDataset{}, fmt.Errorf("eval dataset YAML is invalid: %w", err)
	}
	if err := ValidateDataset(dataset); err != nil {
		return LoadedDataset{}, err
	}
	digest := sha256.Sum256(content)
	return LoadedDataset{Dataset: dataset, Path: absolute, SHA256: hex.EncodeToString(digest[:])}, nil
}

func WriteNewDataset(path string, dataset Dataset) error {
	if err := ValidateDataset(dataset); err != nil {
		return err
	}
	content, err := yaml.Marshal(dataset)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(path)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	return nil
}

func ValidateDataset(dataset Dataset) error {
	var issues []ValidationIssue
	add := func(field string, message string) {
		issues = append(issues, ValidationIssue{Field: field, Message: message})
	}
	if dataset.Type != DatasetType {
		add("type", fmt.Sprintf("must be %q", DatasetType))
	}
	if dataset.Version != DatasetVersion {
		add("version", fmt.Sprintf("must be %d", DatasetVersion))
	}
	if !datasetIDPattern.MatchString(dataset.ID) {
		add("id", "must start with a letter or number and contain at most 128 letters, numbers, dots, underscores, or hyphens")
	}
	validateContextSettings("defaults", dataset.Defaults, &issues)
	if len(dataset.Cases) == 0 {
		add("cases", "must contain at least one case")
	}
	seen := map[string]bool{}
	for index, evalCase := range dataset.Cases {
		prefix := fmt.Sprintf("cases[%d]", index)
		if !datasetIDPattern.MatchString(evalCase.ID) {
			add(prefix+".id", "must start with a letter or number and contain at most 128 letters, numbers, dots, underscores, or hyphens")
		} else if seen[evalCase.ID] {
			add(prefix+".id", "must be unique")
		}
		seen[evalCase.ID] = true
		question := strings.TrimSpace(evalCase.Question)
		if question == "" {
			add(prefix+".question", "is required")
		} else if len(question) > 4096 {
			add(prefix+".question", "must be at most 4096 bytes")
		}
		validateContextSettings(prefix+".context", evalCase.Context, &issues)
		validateAgents(prefix+".agents", evalCase.Agents, &issues)
		validateExpectations(prefix+".expect", evalCase.Expect, &issues)
	}
	if len(issues) > 0 {
		return ValidationError{Issues: issues}
	}
	return nil
}

func validateAgents(field string, agents []string, issues *[]ValidationIssue) {
	if len(agents) > 20 {
		*issues = append(*issues, ValidationIssue{Field: field, Message: "must contain at most 20 agent IDs"})
	}
	seen := map[string]bool{}
	for index, agent := range agents {
		if !datasetIDPattern.MatchString(agent) {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s[%d]", field, index), Message: "must be a bounded agent ID"})
		}
		if seen[agent] {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s[%d]", field, index), Message: "must be unique"})
		}
		seen[agent] = true
	}
}

func validateContextSettings(field string, settings ContextSettings, issues *[]ValidationIssue) {
	if settings.Budget < 0 || settings.Budget > 32000 {
		*issues = append(*issues, ValidationIssue{Field: field + ".budget", Message: "must be between 1 and 32000 when set"})
	}
	if settings.Limit < 0 || settings.Limit > 50 {
		*issues = append(*issues, ValidationIssue{Field: field + ".limit", Message: "must be between 1 and 50 when set"})
	}
}

func validateExpectations(field string, expect Expectations, issues *[]ValidationIssue) {
	if len(expect.Sources) == 0 && len(expect.EvidenceContains) == 0 && len(expect.EvidenceExcludes) == 0 &&
		len(expect.AnswerContains) == 0 && len(expect.AnswerExcludes) == 0 && len(expect.CitationSources) == 0 &&
		expect.MinSources == 0 && expect.MinCitations == 0 && expect.MinGroundedness == nil && expect.MinimumTrust == "" &&
		expect.AllowStale == nil && len(expect.AllowedStatuses) == 0 && expect.RequireSources == nil {
		if expect.AnswerDecision == "" && expect.RequireConflictDisclosure == nil && expect.MinEntailedCitations == 0 {
			*issues = append(*issues, ValidationIssue{Field: field, Message: "must define at least one expectation"})
		}
	}
	if expect.MinSources < 0 || expect.MinSources > 50 {
		*issues = append(*issues, ValidationIssue{Field: field + ".min_sources", Message: "must be between 1 and 50 when set"})
	}
	if expect.MinCitations < 0 || expect.MinCitations > 50 {
		*issues = append(*issues, ValidationIssue{Field: field + ".min_citations", Message: "must be between 1 and 50 when set"})
	}
	if expect.MinGroundedness != nil && (*expect.MinGroundedness < 0 || *expect.MinGroundedness > 1) {
		*issues = append(*issues, ValidationIssue{Field: field + ".min_groundedness", Message: "must be between 0 and 1"})
	}
	if expect.AnswerDecision != "" && expect.AnswerDecision != "answer" && expect.AnswerDecision != "abstain" {
		*issues = append(*issues, ValidationIssue{Field: field + ".answer_decision", Message: "must be answer or abstain"})
	}
	if expect.MinEntailedCitations < 0 || expect.MinEntailedCitations > 50 {
		*issues = append(*issues, ValidationIssue{Field: field + ".min_entailed_citations", Message: "must be between 1 and 50 when set"})
	}
	if expect.MinimumTrust != "" && expect.MinimumTrust != "unverified" && expect.MinimumTrust != "machine-confirmed" && expect.MinimumTrust != "human-reviewed" {
		*issues = append(*issues, ValidationIssue{Field: field + ".minimum_trust", Message: "must be unverified, machine-confirmed, or human-reviewed"})
	}
	seenStatuses := map[string]bool{}
	for index, status := range expect.AllowedStatuses {
		if status != "draft" && status != "stable" && status != "deprecated" {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.allowed_statuses[%d]", field, index), Message: "must be draft, stable, or deprecated"})
		}
		if seenStatuses[status] {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.allowed_statuses[%d]", field, index), Message: "must be unique"})
		}
		seenStatuses[status] = true
	}
	seenSources := map[string]bool{}
	for index, source := range expect.Sources {
		normalized, err := normalizeExpectedSource(source)
		if err != nil {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.sources[%d]", field, index), Message: err.Error()})
			continue
		}
		if seenSources[normalized] {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.sources[%d]", field, index), Message: "must be unique"})
		}
		seenSources[normalized] = true
	}
	validateExpectedText(field+".evidence_contains", expect.EvidenceContains, issues)
	validateExpectedText(field+".evidence_excludes", expect.EvidenceExcludes, issues)
	validateExpectedText(field+".answer_contains", expect.AnswerContains, issues)
	validateExpectedText(field+".answer_excludes", expect.AnswerExcludes, issues)
	seenCitationSources := map[string]bool{}
	for index, source := range expect.CitationSources {
		normalized, err := normalizeExpectedSource(source)
		if err != nil {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.citation_sources[%d]", field, index), Message: err.Error()})
			continue
		}
		if seenCitationSources[normalized] {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s.citation_sources[%d]", field, index), Message: "must be unique"})
		}
		seenCitationSources[normalized] = true
	}
}

func validateExpectedText(field string, values []string, issues *[]ValidationIssue) {
	seen := map[string]bool{}
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s[%d]", field, index), Message: "must not be empty"})
		} else if len(value) > 4096 {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s[%d]", field, index), Message: "must be at most 4096 bytes"})
		}
		key := strings.ToLower(value)
		if seen[key] {
			*issues = append(*issues, ValidationIssue{Field: fmt.Sprintf("%s[%d]", field, index), Message: "must be unique"})
		}
		seen[key] = true
	}
}

func normalizeExpectedSource(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("must not be empty")
	}
	pathPart := value
	fragment := ""
	if index := strings.IndexByte(value, '#'); index >= 0 {
		pathPart = value[:index]
		fragment = value[index:]
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(pathPart)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("must be a bundle-relative source path")
	}
	return clean + fragment, nil
}
