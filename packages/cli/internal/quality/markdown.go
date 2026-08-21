package quality

import (
	"fmt"
	"strings"
)

func RenderMarkdown(report Report) string {
	var output strings.Builder
	output.WriteString("# Open Knowledge quality report\n\n")
	fmt.Fprintf(&output, "- Evaluated: `%s`\n", report.EvaluatedAt)
	fmt.Fprintf(&output, "- Bundle: `%s` (OKF %s)\n", report.Bundle.Path, report.Bundle.Spec)
	fmt.Fprintf(&output, "- Inputs: %d usage, %d feedback, %d eval reports, %d comparisons, %d audits, %d interventions\n\n", report.Inputs.UsageEvents, report.Inputs.FeedbackEvents, report.Inputs.EvalReports, report.Inputs.Comparisons, report.Inputs.AuditReports, report.Inputs.InterventionEvents)
	output.WriteString("## Metrics\n\n")
	output.WriteString("| Metric | Status | Value | Evidence |\n| --- | --- | ---: | --- |\n")
	for _, metric := range report.Metrics {
		value := "—"
		if metric.Value != nil {
			value = formatMetricValue(*metric.Value, metric.Unit)
			if metric.Change != nil {
				value += fmt.Sprintf(" (%+.2f pp)", *metric.Change)
			}
		}
		evidence := metric.Note
		if metric.Numerator != nil && metric.Denominator != nil {
			evidence = fmt.Sprintf("%d/%d. %s", *metric.Numerator, *metric.Denominator, evidence)
		}
		fmt.Fprintf(&output, "| `%s` | %s | %s | %s |\n", metric.ID, metric.Status, value, escapeTable(evidence))
	}
	output.WriteString("\n## Priorities\n\n")
	priorityCount := 0
	for _, concept := range report.Concepts {
		if concept.Priority == "none" {
			continue
		}
		priorityCount++
		fmt.Fprintf(&output, "- **%s** `%s` — %s", strings.ToUpper(concept.Priority), concept.Path, strings.Join(concept.RiskReasons, ", "))
		if concept.Uses > 0 {
			fmt.Fprintf(&output, "; %d evidence selections", concept.Uses)
		}
		if concept.NegativeFeedback > 0 {
			fmt.Fprintf(&output, "; %d negative feedback", concept.NegativeFeedback)
		}
		output.WriteByte('\n')
	}
	if priorityCount == 0 {
		output.WriteString("No concrete priorities in the supplied observation window.\n")
	}
	if len(report.Changes) > 0 {
		output.WriteString("\n## Knowledge changes\n\n")
		for _, change := range report.Changes {
			fmt.Fprintf(&output, "- `%s`: %d improved, %d regressed; answer accuracy %.2f%% → %.2f%% (%+.2f pp)\n", change.Dataset, change.Improved, change.Regressed, change.BaseAccuracy, change.ProposedAccuracy, change.AccuracyChange)
		}
	}
	return output.String()
}

func formatMetricValue(value float64, unit string) string {
	switch unit {
	case "percent":
		return fmt.Sprintf("%.2f%%", value)
	case "percentage-points":
		return fmt.Sprintf("%.2f pp", value)
	case "count":
		return fmt.Sprintf("%.0f", value)
	default:
		return fmt.Sprintf("%.2f %s", value, unit)
	}
}

func escapeTable(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", " ")
}
