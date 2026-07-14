package okf

import "strings"

func ShouldPublish(file BundleFile) bool {
	switch value := file.Frontmatter["okf_publish"].(type) {
	case bool:
		return value
	case string:
		return strings.TrimSpace(strings.ToLower(value)) != "false"
	default:
		return true
	}
}

func shouldPublishASTDocument(document ASTDocument) bool {
	if value, exists := document.Frontmatter.Data["okf_publish"]; exists {
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return strings.TrimSpace(strings.ToLower(typed)) != "false"
		default:
			return true
		}
	}
	return shouldPublishFrontmatterValues(document.Frontmatter.Values)
}

func shouldPublishFrontmatterValues(values map[string]string) bool {
	return strings.TrimSpace(strings.ToLower(values["okf_publish"])) != "false"
}
