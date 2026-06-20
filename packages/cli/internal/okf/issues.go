package okf

func issuesFromResult(result Result) []Issue {
	issues := append([]Issue{}, result.Errors...)
	return append(issues, result.Warnings...)
}

func groupIssuesByPath(issues []Issue) map[string][]Issue {
	grouped := make(map[string][]Issue)
	for _, issue := range issues {
		grouped[issue.Path] = append(grouped[issue.Path], issue)
	}
	return grouped
}
