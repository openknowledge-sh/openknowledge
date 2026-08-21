package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	AnswerProtocolVersion = "1"
	defaultAnswerTimeout  = 2 * time.Minute
	maxAnswerOutputBytes  = 8 << 20
	maxAnswerErrorBytes   = 256 << 10
)

type AnswerRunner struct {
	Context     context.Context
	Command     string
	Args        []string
	Directory   string
	Environment []string
	Timeout     time.Duration
}

type AnswerRequest struct {
	SchemaVersion string              `json:"schemaVersion"`
	Dataset       AnswerDataset       `json:"dataset"`
	Target        TargetIdentity      `json:"target"`
	Cases         []AnswerRequestCase `json:"cases"`
}

type AnswerDataset struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type AnswerRequestCase struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Sources  []AnswerSource `json:"sources"`
}

type AnswerSource struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Title         string `json:"title"`
	Heading       string `json:"heading"`
	Locator       string `json:"locator"`
	ContentSHA256 string `json:"contentSha256"`
	Relation      string `json:"relation"`
	Markdown      string `json:"markdown"`
}

type AnswerResponse struct {
	SchemaVersion string         `json:"schemaVersion"`
	Answers       []RunnerAnswer `json:"answers"`
}

type RunnerAnswer struct {
	CaseID string        `json:"caseId"`
	Answer string        `json:"answer"`
	Claims []AnswerClaim `json:"claims"`
}

type AnswerClaim struct {
	Text      string   `json:"text"`
	Citations []string `json:"citations"`
}

type AnswerResult struct {
	Text           string        `json:"text"`
	Claims         []ClaimResult `json:"claims"`
	CitedSources   []string      `json:"citedSources"`
	CitationCount  int           `json:"citationCount"`
	ValidCitations int           `json:"validCitations"`
	ClaimCount     int           `json:"claimCount"`
	GroundedClaims int           `json:"groundedClaims"`
	Groundedness   float64       `json:"groundedness"`
}

type ClaimResult struct {
	Text      string           `json:"text"`
	Citations []CitationResult `json:"citations"`
	Grounded  bool             `json:"grounded"`
}

type CitationResult struct {
	Locator string `json:"locator"`
	Path    string `json:"path,omitempty"`
	Valid   bool   `json:"valid"`
}

func RunWithAnswers(root string, specVersion string, loaded LoadedDataset, runner AnswerRunner) (Report, error) {
	report, err := runRetrieval(root, specVersion, loaded)
	if err != nil {
		return Report{}, err
	}
	if err := attachAnswers(&report, loaded.Dataset, runner); err != nil {
		return Report{}, err
	}
	return report, nil
}

func attachAnswers(report *Report, dataset Dataset, runner AnswerRunner) error {
	request := AnswerRequest{
		SchemaVersion: AnswerProtocolVersion,
		Dataset:       AnswerDataset{ID: report.Dataset.ID, SHA256: report.Dataset.SHA256},
		Target:        report.Target,
		Cases:         make([]AnswerRequestCase, 0, len(report.Cases)),
	}
	for _, result := range report.Cases {
		request.Cases = append(request.Cases, AnswerRequestCase{ID: result.ID, Question: result.Question, Sources: result.answerInput})
	}
	response, err := executeAnswerRunner(runner, request)
	if err != nil {
		return err
	}
	answers, err := validateAnswerResponse(response, request)
	if err != nil {
		return err
	}
	for index := range report.Cases {
		answer := answers[report.Cases[index].ID]
		applyAnswer(&report.Cases[index], dataset.Cases[index].Expect, answer)
	}
	recalculateReportSummary(report)
	return nil
}

func executeAnswerRunner(runner AnswerRunner, request AnswerRequest) (AnswerResponse, error) {
	commandName := strings.TrimSpace(runner.Command)
	if commandName == "" || strings.HasPrefix(commandName, "-") || strings.ContainsAny(commandName, "\x00\r\n") {
		return AnswerResponse{}, fmt.Errorf("answer command is invalid")
	}
	timeout := runner.Timeout
	if timeout == 0 {
		timeout = defaultAnswerTimeout
	}
	if timeout < 0 || timeout > time.Hour {
		return AnswerResponse{}, fmt.Errorf("answer timeout must be positive and at most 1h")
	}
	content, err := json.Marshal(request)
	if err != nil {
		return AnswerResponse{}, err
	}
	parent := runner.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, commandName, runner.Args...)
	command.Dir = runner.Directory
	if runner.Environment != nil {
		command.Env = runner.Environment
	}
	command.Stdin = bytes.NewReader(append(content, '\n'))
	stdout := &boundedRunnerBuffer{limit: maxAnswerOutputBytes}
	stderr := &boundedRunnerBuffer{limit: maxAnswerErrorBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return AnswerResponse{}, fmt.Errorf("answer command timed out after %s", timeout)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return AnswerResponse{}, fmt.Errorf("answer command failed: %s", message)
	}
	if stdout.overflow {
		return AnswerResponse{}, fmt.Errorf("answer command output exceeds %d bytes", maxAnswerOutputBytes)
	}
	var response AnswerResponse
	if err := okf.DecodeStrictJSON(stdout.Bytes(), &response); err != nil {
		return AnswerResponse{}, fmt.Errorf("answer command returned invalid protocol JSON: %w", err)
	}
	return response, nil
}

type boundedRunnerBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *boundedRunnerBuffer) Write(content []byte) (int, error) {
	written := len(content)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if remaining > len(content) {
			remaining = len(content)
		}
		_, _ = buffer.buffer.Write(content[:remaining])
	}
	if remaining < len(content) {
		buffer.overflow = true
	}
	return written, nil
}

func (buffer *boundedRunnerBuffer) Bytes() []byte  { return buffer.buffer.Bytes() }
func (buffer *boundedRunnerBuffer) String() string { return buffer.buffer.String() }

func validateAnswerResponse(response AnswerResponse, request AnswerRequest) (map[string]RunnerAnswer, error) {
	if response.SchemaVersion != AnswerProtocolVersion {
		return nil, fmt.Errorf("answer response schemaVersion must be %s", AnswerProtocolVersion)
	}
	expected := make(map[string]bool, len(request.Cases))
	for _, evalCase := range request.Cases {
		expected[evalCase.ID] = true
	}
	answers := make(map[string]RunnerAnswer, len(response.Answers))
	for index, answer := range response.Answers {
		if !expected[answer.CaseID] {
			return nil, fmt.Errorf("answer response contains unknown case %q", answer.CaseID)
		}
		if _, exists := answers[answer.CaseID]; exists {
			return nil, fmt.Errorf("answer response repeats case %q", answer.CaseID)
		}
		for claimIndex, claim := range answer.Claims {
			if strings.TrimSpace(claim.Text) == "" {
				return nil, fmt.Errorf("answer response case %s claim %d has empty text", answer.CaseID, claimIndex)
			}
			seen := map[string]bool{}
			for _, citation := range claim.Citations {
				if strings.TrimSpace(citation) == "" || seen[citation] {
					return nil, fmt.Errorf("answer response case %s claim %d has an empty or repeated citation", answer.CaseID, claimIndex)
				}
				seen[citation] = true
			}
		}
		answers[answer.CaseID] = response.Answers[index]
	}
	for caseID := range expected {
		if _, ok := answers[caseID]; !ok {
			return nil, fmt.Errorf("answer response is missing case %q", caseID)
		}
	}
	return answers, nil
}

func applyAnswer(result *CaseResult, expect Expectations, answer RunnerAnswer) {
	byLocator := make(map[string]AnswerSource, len(result.answerInput))
	for _, source := range result.answerInput {
		byLocator[source.Locator] = source
	}
	answerResult := &AnswerResult{Text: answer.Answer, Claims: []ClaimResult{}, CitedSources: []string{}}
	cited := map[string]bool{}
	for _, claim := range answer.Claims {
		claimResult := ClaimResult{Text: claim.Text, Citations: []CitationResult{}}
		for _, locator := range claim.Citations {
			answerResult.CitationCount++
			citation := CitationResult{Locator: locator}
			if source, ok := byLocator[locator]; ok {
				citation.Valid = true
				citation.Path = source.Path
				answerResult.ValidCitations++
				claimResult.Grounded = true
				cited[source.Path] = true
				cited[source.ID] = true
				cited[source.Path+sourceFragment(source.ID)] = true
			}
			claimResult.Citations = append(claimResult.Citations, citation)
		}
		if claimResult.Grounded {
			answerResult.GroundedClaims++
		}
		answerResult.Claims = append(answerResult.Claims, claimResult)
	}
	answerResult.ClaimCount = len(answerResult.Claims)
	if answerResult.ClaimCount > 0 {
		answerResult.Groundedness = float64(answerResult.GroundedClaims) / float64(answerResult.ClaimCount)
	}
	for _, source := range result.answerInput {
		if cited[source.Path] {
			answerResult.CitedSources = append(answerResult.CitedSources, source.Path)
		}
	}
	sort.Strings(answerResult.CitedSources)
	result.Answer = answerResult
	result.Metrics.Citations = answerResult.CitationCount
	result.Metrics.ValidCitations = answerResult.ValidCitations
	groundedness := answerResult.Groundedness
	result.Metrics.Groundedness = &groundedness

	normalizedAnswer := strings.ToLower(answer.Answer)
	for _, expected := range expect.AnswerContains {
		passed := strings.Contains(normalizedAnswer, strings.ToLower(strings.TrimSpace(expected)))
		result.Checks = append(result.Checks, Check{Kind: "answer_contains", Expected: strings.TrimSpace(expected), Passed: passed})
	}
	for _, expected := range expect.AnswerExcludes {
		passed := !strings.Contains(normalizedAnswer, strings.ToLower(strings.TrimSpace(expected)))
		result.Checks = append(result.Checks, Check{Kind: "answer_excludes", Expected: strings.TrimSpace(expected), Passed: passed})
	}
	for _, expected := range expect.CitationSources {
		normalized, _ := normalizeExpectedSource(expected)
		passed := cited[normalized]
		result.Checks = append(result.Checks, Check{Kind: "citation_source", Expected: normalized, Passed: passed})
	}
	if expect.MinCitations > 0 {
		result.Checks = append(result.Checks, Check{Kind: "min_citations", Expected: fmt.Sprintf("%d", expect.MinCitations), Actual: fmt.Sprintf("%d", answerResult.ValidCitations), Passed: answerResult.ValidCitations >= expect.MinCitations})
	}
	if expect.MinGroundedness != nil {
		result.Checks = append(result.Checks, Check{Kind: "min_groundedness", Expected: fmt.Sprintf("%.4f", *expect.MinGroundedness), Actual: fmt.Sprintf("%.4f", answerResult.Groundedness), Passed: answerResult.Groundedness >= *expect.MinGroundedness})
	}
	recalculateCase(result)
}

func recalculateCase(result *CaseResult) {
	result.Status = "pass"
	result.Metrics.Checks = len(result.Checks)
	result.Metrics.PassedChecks = 0
	for _, check := range result.Checks {
		if check.Passed {
			result.Metrics.PassedChecks++
		} else {
			result.Status = "fail"
		}
	}
}

func recalculateReportSummary(report *Report) {
	report.Summary = Summary{Status: "pass", Total: len(report.Cases)}
	for _, result := range report.Cases {
		report.Summary.Checks += result.Metrics.Checks
		report.Summary.PassedChecks += result.Metrics.PassedChecks
		if result.Status == "pass" {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
			report.Summary.Status = "fail"
		}
	}
}
