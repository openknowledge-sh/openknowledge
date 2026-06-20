package okf

func astFrontmatterFromParse(meta frontmatter) astFrontmatter {
	values := frontmatterValues(meta)

	keys := make(map[string]struct{}, len(meta.keys))
	for key := range meta.keys {
		keys[key] = struct{}{}
	}

	warnings := make([]astFrontmatterWarning, 0, len(meta.warnings))
	for _, warning := range meta.warnings {
		warnings = append(warnings, astFrontmatterWarning{
			Line:    warning.line,
			Message: warning.message,
		})
	}

	return astFrontmatter{
		Has:      meta.has,
		Values:   values,
		Keys:     keys,
		Warnings: warnings,
		BodyLine: meta.bodyLine,
	}
}

func frontmatterValues(meta frontmatter) map[string]string {
	if !meta.has || len(meta.values) == 0 {
		return nil
	}

	values := make(map[string]string, len(meta.values))
	for key, value := range meta.values {
		values[key] = value
	}
	return values
}
