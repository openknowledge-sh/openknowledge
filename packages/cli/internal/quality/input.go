package quality

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func ReadEval(path string) (*knowledgeeval.Report, *knowledgeeval.ComparisonReport, error) {
	content, err := okf.ReadFileAtMost(path, 32<<20)
	if err != nil {
		return nil, nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, nil, err
	}
	if _, comparison := fields["base"]; comparison {
		var report knowledgeeval.ComparisonReport
		if err := okf.DecodeStrictJSON(content, &report); err != nil {
			return nil, nil, err
		}
		if err := validateComparison(report); err != nil {
			return nil, nil, err
		}
		return nil, &report, nil
	}
	var report knowledgeeval.Report
	if err := okf.DecodeStrictJSON(content, &report); err != nil {
		return nil, nil, err
	}
	if err := validateEvalReport(report); err != nil {
		return nil, nil, err
	}
	return &report, nil, nil
}

func validateEvalReport(report knowledgeeval.Report) error {
	if report.SchemaVersion != okf.MachineSchemaVersion || report.Dataset.ID == "" || report.Dataset.Type == "" || report.Dataset.Version < 1 || len(report.Dataset.SHA256) != 64 || report.Cases == nil {
		return fmt.Errorf("unsupported eval report contract")
	}
	if report.Summary.Total != len(report.Cases) || report.Summary.Passed+report.Summary.Failed != report.Summary.Total || report.Summary.PassedChecks > report.Summary.Checks {
		return fmt.Errorf("eval report summary is inconsistent")
	}
	seen := map[string]bool{}
	passed, checks, passedChecks := 0, 0, 0
	for _, result := range report.Cases {
		if result.ID == "" || seen[result.ID] || result.Question == "" || result.Agents == nil || result.Context.Sources == nil || result.Checks == nil || result.Status != "pass" && result.Status != "fail" {
			return fmt.Errorf("eval report contains an invalid case")
		}
		seen[result.ID] = true
		if result.Status == "pass" {
			passed++
		}
		for _, source := range result.Context.Sources {
			if !safeRelativePath(source.Path) || source.ID == "" || source.Locator == "" {
				return fmt.Errorf("eval report contains an invalid source")
			}
		}
		checks += len(result.Checks)
		for _, check := range result.Checks {
			if check.Kind == "" || check.Expected == "" {
				return fmt.Errorf("eval report contains an invalid check")
			}
			if check.Passed {
				passedChecks++
			}
		}
	}
	if passed != report.Summary.Passed || checks != report.Summary.Checks || passedChecks != report.Summary.PassedChecks {
		return fmt.Errorf("eval report case totals are inconsistent")
	}
	return nil
}

func validateComparison(report knowledgeeval.ComparisonReport) error {
	if report.SchemaVersion != okf.MachineSchemaVersion || report.Dataset.ID == "" || report.Cases == nil || report.Impact.ChangedPaths == nil || report.Impact.AffectedAgents == nil || report.Impact.AffectedQuestions == nil || report.Impact.UncoveredPaths == nil {
		return fmt.Errorf("unsupported eval comparison contract")
	}
	if report.Summary.Total != len(report.Cases) || report.Summary.Improved+report.Summary.Regressed+report.Summary.UnchangedPassed+report.Summary.UnchangedFailed != report.Summary.Total || report.Summary.ProposedPassed+report.Summary.ProposedFailed != report.Summary.Total {
		return fmt.Errorf("eval comparison summary is inconsistent")
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, result := range report.Cases {
		if result.ID == "" || seen[result.ID] || result.Base.ID != result.ID || result.Proposed.ID != result.ID {
			return fmt.Errorf("eval comparison contains an invalid case")
		}
		seen[result.ID] = true
		counts[result.Classification]++
		if result.Classification != "improved" && result.Classification != "regressed" && result.Classification != "unchanged_pass" && result.Classification != "unchanged_fail" {
			return fmt.Errorf("eval comparison classification is invalid")
		}
	}
	if counts["improved"] != report.Summary.Improved || counts["regressed"] != report.Summary.Regressed || counts["unchanged_pass"] != report.Summary.UnchangedPassed || counts["unchanged_fail"] != report.Summary.UnchangedFailed {
		return fmt.Errorf("eval comparison classifications are inconsistent")
	}
	return nil
}

func safeRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	clean := filepath.ToSlash(filepath.Clean(path))
	return path != "" && !filepath.IsAbs(path) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
