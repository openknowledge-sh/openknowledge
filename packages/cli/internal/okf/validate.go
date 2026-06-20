package okf

func Validate(root string) (Result, error) {
	return ValidateWithVersion(root, LatestSpecVersion)
}

func ValidateWithVersion(root string, version string) (Result, error) {
	result, _, err := parseAndValidateASTBundle(root, version)
	return result, err
}

func parseAndValidateASTBundle(root string, version string) (Result, astBundle, error) {
	bundle, err := parseBundleAST(root, version)
	if err != nil {
		return Result{}, astBundle{}, err
	}
	return validateASTBundle(bundle), bundle, nil
}

func validateASTBundle(bundle astBundle) Result {
	result := Result{Root: bundle.Root, SpecVersion: bundle.SpecVersion}
	for _, document := range bundle.Documents {
		result.Files++
		validateDocument(bundle.Root, document, &result)
	}

	sortIssues(result.Errors)
	sortIssues(result.Warnings)
	result.Checks = buildChecks(result)
	return result
}

func validateDocument(root string, document astDocument, result *Result) {
	rel := document.Rel

	switch document.Kind {
	case "index":
		result.Indexes++
	case "log":
		result.Logs++
	default:
		result.Concepts++
	}

	if document.ReadDiagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Rule: "bundle-read", Message: document.ReadDiagnostic.Message})
		return
	}
	if document.UTF8Diagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: document.UTF8Diagnostic.Line, Rule: "utf-8", Message: document.UTF8Diagnostic.Message})
		return
	}

	if document.FrontmatterDiagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: document.FrontmatterDiagnostic.Line, Rule: "frontmatter", Message: document.FrontmatterDiagnostic.Message})
	}
	validateFrontmatterFormatting(rel, document.Frontmatter, result)

	if document.FrontmatterDiagnostic == nil {
		switch document.Kind {
		case "index":
			validateIndex(rel, document.Frontmatter, result)
		case "log":
			validateLog(rel, document.Frontmatter, document.Content, result)
		default:
			validateConcept(rel, document.Frontmatter, result)
		}
		validateMarkdownSyntax(rel, document.Body, document.Frontmatter.BodyLine, result)
	}
	validateDocumentLinks(root, document, result)
}
