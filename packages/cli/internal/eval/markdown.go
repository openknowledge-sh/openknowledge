package eval

import (
	"fmt"
	"strings"
)

func RenderMarkdown(report Report) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# Open Knowledge eval: %s\n\n", markdownInline(report.Dataset.ID))
	fmt.Fprintf(&output, "**Status:** %s · **Passed:** %d/%d · **Checks:** %d/%d\n\n", strings.ToUpper(report.Summary.Status), report.Summary.Passed, report.Summary.Total, report.Summary.PassedChecks, report.Summary.Checks)
	for _, result := range report.Cases {
		fmt.Fprintf(&output, "## %s — %s\n\n%s\n\n", markdownInline(result.ID), strings.ToUpper(result.Status), result.Question)
		renderCaseMarkdown(&output, result)
	}
	return output.String()
}

func RenderComparisonMarkdown(report ComparisonReport) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# Open Knowledge eval comparison: %s\n\n", markdownInline(report.Dataset.ID))
	fmt.Fprintf(&output, "**Status:** %s · **Gate:** %s · **Improved:** %d · **Regressed:** %d · **Proposed failures:** %d\n\n", strings.ToUpper(report.Summary.Status), report.Summary.Gate, report.Summary.Improved, report.Summary.Regressed, report.Summary.ProposedFailed)
	renderImpactMarkdown(&output, report.Impact)
	fmt.Fprintf(&output, "| Case | Classification | Base | Proposed |\n| --- | --- | --- | --- |\n")
	for _, result := range report.Cases {
		fmt.Fprintf(&output, "| %s | %s | %s | %s |\n", markdownTable(result.ID), result.Classification, result.Base.Status, result.Proposed.Status)
	}
	output.WriteByte('\n')
	for _, result := range report.Cases {
		fmt.Fprintf(&output, "## %s — %s\n\n%s\n\n", markdownInline(result.ID), result.Classification, result.Question)
		if result.Base.Answer != nil || result.Proposed.Answer != nil {
			output.WriteString("### Answer change\n\n")
			renderAnswerMarkdown(&output, "Base", result.Base.Answer)
			renderAnswerMarkdown(&output, "Proposed", result.Proposed.Answer)
		}
		output.WriteString("### Proposed checks\n\n")
		renderChecksMarkdown(&output, result.Proposed)
	}
	return output.String()
}

func renderImpactMarkdown(output *strings.Builder, impact ImpactSummary) {
	output.WriteString("## Change impact\n\n")
	fmt.Fprintf(output, "Changed paths: **%d** · Affected questions: **%d** · Affected agents: **%d** · Uncovered paths: **%d**\n\n", len(impact.ChangedPaths), len(impact.AffectedQuestions), len(impact.AffectedAgents), len(impact.UncoveredPaths))
	if len(impact.AffectedAgents) > 0 {
		fmt.Fprintf(output, "Affected agents: %s\n\n", strings.Join(impact.AffectedAgents, ", "))
	}
	if len(impact.AffectedQuestions) > 0 {
		output.WriteString("| Question | Agents | Reasons | Changed paths |\n| --- | --- | --- | --- |\n")
		for _, question := range impact.AffectedQuestions {
			fmt.Fprintf(output, "| %s | %s | %s | %s |\n", markdownTable(question.ID), markdownTable(strings.Join(question.Agents, ", ")), markdownTable(strings.Join(question.Reasons, ", ")), markdownTable(strings.Join(question.Paths, ", ")))
		}
		output.WriteByte('\n')
	}
	if len(impact.UncoveredPaths) > 0 {
		fmt.Fprintf(output, "Uncovered changed paths: %s\n\n", strings.Join(impact.UncoveredPaths, ", "))
	}
}

func renderCaseMarkdown(output *strings.Builder, result CaseResult) {
	if result.Answer != nil {
		renderAnswerMarkdown(output, "Answer", result.Answer)
	}
	output.WriteString("### Checks\n\n")
	renderChecksMarkdown(output, result)
}

func renderAnswerMarkdown(output *strings.Builder, label string, answer *AnswerResult) {
	fmt.Fprintf(output, "#### %s\n\n", label)
	if answer == nil {
		output.WriteString("_No answer runner output._\n\n")
		return
	}
	for _, line := range strings.Split(answer.Text, "\n") {
		fmt.Fprintf(output, "> %s\n", line)
	}
	fmt.Fprintf(output, "\nGroundedness: **%.1f%%** (%d/%d claims) · Valid citations: **%d/%d**\n\n", answer.Groundedness*100, answer.GroundedClaims, answer.ClaimCount, answer.ValidCitations, answer.CitationCount)
	if len(answer.CitedSources) > 0 {
		fmt.Fprintf(output, "Cited sources: %s\n\n", strings.Join(answer.CitedSources, ", "))
	}
}

func renderChecksMarkdown(output *strings.Builder, result CaseResult) {
	failed := 0
	for _, check := range result.Checks {
		if !check.Passed {
			failed++
			fmt.Fprintf(output, "- ❌ `%s`: expected %s", check.Kind, markdownInline(check.Expected))
			if check.Actual != "" {
				fmt.Fprintf(output, ", got %s", markdownInline(check.Actual))
			}
			output.WriteByte('\n')
		}
	}
	if failed == 0 {
		output.WriteString("- ✅ All checks passed.\n")
	}
	output.WriteByte('\n')
}

func markdownInline(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "`", "\\`"), "\n", " ")
}

func markdownTable(value string) string {
	return strings.ReplaceAll(markdownInline(value), "|", "\\|")
}
