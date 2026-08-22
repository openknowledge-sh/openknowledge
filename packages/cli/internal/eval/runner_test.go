package eval

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunWithAnswersScoresAnswerCitationsAndGroundedness(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nRead [Rollback](rollback.md).\n")
	writeTestFile(t, root, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nRestore the previous release.\n")
	minimum := 1.0
	loaded := LoadedDataset{
		Path: "/evals/deploy.yaml", SHA256: strings.Repeat("a", 64),
		Dataset: Dataset{Type: DatasetType, Version: DatasetVersion, ID: "deploy", Cases: []Case{{
			ID: "rollback", Question: "How do we restore a release?",
			Expect: Expectations{AnswerContains: []string{"restore"}, CitationSources: []string{"rollback.md"}, MinCitations: 1, MinGroundedness: &minimum},
		}}},
	}
	report, err := RunWithAnswers(root, "0.2", loaded, AnswerRunner{
		Command: os.Args[0], Args: []string{"-test.run=TestAnswerRunnerHelperProcess", "--", "valid"}, Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := report.Cases[0]
	if result.Status != "pass" || result.Answer == nil || result.Answer.Groundedness != 1 || result.Answer.ValidCitations != 1 {
		t.Fatalf("unexpected answer result: %#v", result)
	}
	if report.Summary.Status != "pass" || report.Summary.Checks != 4 || report.Summary.PassedChecks != 4 {
		t.Fatalf("unexpected report summary: %#v", report.Summary)
	}
}

func TestAnswerRunnerRejectsUnknownCasesAndInvalidCitations(t *testing.T) {
	request := AnswerRequest{SchemaVersion: AnswerProtocolVersion, Cases: []AnswerRequestCase{{ID: "known"}}}
	if _, err := validateAnswerResponse(AnswerResponse{SchemaVersion: AnswerProtocolVersion, Answers: []RunnerAnswer{{CaseID: "other"}}}, request); err == nil {
		t.Fatal("expected unknown answer case to fail")
	}
	result := CaseResult{Status: "pass", Checks: []Check{}, answerInput: []AnswerSource{{ID: "doc#section", Path: "doc.md", Locator: "okf+sha256://" + strings.Repeat("a", 64) + "/doc.md#section"}}}
	minimum := 1.0
	applyAnswer(&result, Expectations{MinCitations: 1, MinGroundedness: &minimum}, RunnerAnswer{
		CaseID: "known", Answer: "Unsupported", Claims: []AnswerClaim{{Text: "Unsupported", Citations: []string{"okf+sha256://" + strings.Repeat("b", 64) + "/doc.md#section"}}},
	})
	if result.Status != "fail" || result.Answer == nil || result.Answer.ValidCitations != 0 || result.Answer.Groundedness != 0 {
		t.Fatalf("invalid citation must fail checks: %#v", result)
	}
}

func TestAnswerEvaluationScoresAbstentionConflictsAndEntailmentAttestations(t *testing.T) {
	request := AnswerRequest{SchemaVersion: AnswerProtocolVersion, Cases: []AnswerRequestCase{{ID: "unknown"}}}
	abstention := AnswerResponse{SchemaVersion: AnswerProtocolVersion, Answers: []RunnerAnswer{{CaseID: "unknown", Decision: "abstain", Answer: "The corpus does not contain enough evidence.", Claims: []AnswerClaim{}, RefusalReasons: []string{"no_relevant_evidence"}}}}
	answers, err := validateAnswerResponse(abstention, request)
	if err != nil || answers["unknown"].Decision != "abstain" {
		t.Fatalf("valid abstention failed: %#v %v", answers, err)
	}
	requireConflict := true
	abstentionResult := CaseResult{Status: "pass", Checks: []Check{}, answerInput: []AnswerSource{}}
	applyAnswer(&abstentionResult, Expectations{AnswerDecision: "abstain", RequireConflictDisclosure: &requireConflict}, RunnerAnswer{
		CaseID: "unknown", Decision: "abstain", Answer: "Evidence conflicts and cannot support one answer.", Claims: []AnswerClaim{}, RefusalReasons: []string{"conflicting_evidence"}, Conflicts: []string{"okn:claim/one contradicts okn:claim/two"},
	})
	if abstentionResult.Status != "pass" || abstentionResult.Answer == nil || abstentionResult.Answer.Decision != "abstain" || len(abstentionResult.Answer.Conflicts) != 1 {
		t.Fatalf("abstention checks failed: %#v", abstentionResult)
	}

	locator := "okf+sha256://" + strings.Repeat("a", 64) + "/doc.md#section"
	answerResult := CaseResult{Status: "pass", Checks: []Check{}, answerInput: []AnswerSource{{ID: "doc#section", Path: "doc.md", Locator: locator}}}
	applyAnswer(&answerResult, Expectations{AnswerDecision: "answer", MinEntailedCitations: 1}, RunnerAnswer{
		CaseID: "known", Decision: "answer", Answer: "The policy is active.", Claims: []AnswerClaim{{Text: "The policy is active.", Citations: []string{locator}}},
		Entailment: []CitationEntailmentAttestation{{Locator: locator, Status: "entailed", Method: "human-review", Reason: "The source states the claim directly."}},
	})
	if answerResult.Status != "pass" || answerResult.Answer.EntailedCitations != 1 || answerResult.Answer.Claims[0].Citations[0].Entailment != "entailed" {
		t.Fatalf("entailment checks failed: %#v", answerResult)
	}
}

func TestAnswerRunnerHelperProcess(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) || os.Args[separator+1] != "valid" {
		return
	}
	var request AnswerRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(3)
	}
	response := AnswerResponse{SchemaVersion: AnswerProtocolVersion, Answers: make([]RunnerAnswer, 0, len(request.Cases))}
	for _, evalCase := range request.Cases {
		if len(evalCase.Sources) == 0 {
			os.Exit(4)
		}
		locator := evalCase.Sources[0].Locator
		for _, source := range evalCase.Sources {
			if source.Path == "rollback.md" {
				locator = source.Locator
				break
			}
		}
		response.Answers = append(response.Answers, RunnerAnswer{
			CaseID: evalCase.ID, Answer: "Restore the previous release.",
			Claims: []AnswerClaim{{Text: "Restore the previous release.", Citations: []string{locator}}},
		})
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		os.Exit(5)
	}
	os.Exit(0)
}
