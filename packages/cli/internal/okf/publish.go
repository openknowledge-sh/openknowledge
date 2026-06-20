package okf

import "strings"

func ShouldPublish(file BundleFile) bool {
	return shouldPublishFrontmatterValues(file.Frontmatter)
}

func shouldPublishASTDocument(document astDocument) bool {
	return shouldPublishFrontmatterValues(document.Frontmatter.Values)
}

func shouldPublishFrontmatterValues(values map[string]string) bool {
	return strings.TrimSpace(strings.ToLower(values["okf_publish"])) != "false"
}
