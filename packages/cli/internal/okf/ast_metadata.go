package okf

func astDocumentMetadataFromValues(values map[string]string) ASTDocumentMetadata {
	return ASTDocumentMetadata{
		Type:        values["type"],
		Title:       values["title"],
		Description: values["description"],
		Resource:    values["resource"],
		Tags:        parseFlowStringList(values["tags"]),
		UseWhen:     parseFlowStringList(values["use_when"]),
		Bundle:      bundleMetadataFromFrontmatter(values),
	}
}
