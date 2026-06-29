package okf

import "sort"

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

func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Path < issues[j].Path
	})
}
