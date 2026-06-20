package okf

func ParseBundle(root string) (Bundle, error) {
	return ParseBundleWithVersion(root, LatestSpecVersion)
}

func ParseBundleWithVersion(root string, version string) (Bundle, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return Bundle{}, err
	}

	return bundleFromAST(ast, issuesFromResult(validation))
}

func bundleFromAST(ast ASTBundle, issues []Issue) (Bundle, error) {
	files, err := bundleFilesFromAST(ast, issues)
	if err != nil {
		return Bundle{}, err
	}

	return Bundle{
		Root:        ast.Root,
		SpecVersion: ast.SpecVersion,
		Files:       files,
		Issues:      issues,
	}, nil
}

func bundleFilesFromAST(bundle ASTBundle, issues []Issue) ([]BundleFile, error) {
	issuesByPath := groupIssuesByPath(issues)
	files := make([]BundleFile, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadDiagnostic != nil {
			return nil, document.ReadDiagnostic
		}
		files = append(files, bundleFile(document, issuesByPath[document.Rel]))
	}
	return files, nil
}

func bundleFile(document ASTDocument, issues []Issue) BundleFile {
	summary := summarizeASTDocument(document, document.Metadata)

	return BundleFile{
		ID:          summary.ID,
		Path:        summary.Path,
		Kind:        summary.Kind,
		Reserved:    summary.Reserved,
		Type:        summary.Type,
		Title:       summary.Title,
		Description: summary.Description,
		Resource:    summary.Resource,
		Frontmatter: document.Frontmatter.Values,
		Body:        document.Body,
		Links:       document.Links,
		Issues:      issues,
	}
}
