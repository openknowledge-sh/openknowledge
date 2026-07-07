package okf

func parseAndValidateASTBundle(root string, version string) (Result, ASTBundle, error) {
	return parseAndValidateASTBundleWithOptions(root, version, ValidationOptions{})
}

func parseAndValidateASTBundleWithOptions(root string, version string, options ValidationOptions) (Result, ASTBundle, error) {
	bundle, err := ParseASTWithVersion(root, version)
	if err != nil {
		return Result{}, ASTBundle{}, err
	}
	result, err := ValidateASTWithOptions(bundle, options)
	return result, bundle, err
}

func ValidateAST(bundle ASTBundle) Result {
	result, _ := ValidateASTWithOptions(bundle, ValidationOptions{})
	return result
}

func ValidateASTWithOptions(bundle ASTBundle, options ValidationOptions) (Result, error) {
	result := Result{Root: bundle.Root, SpecVersion: bundle.SpecVersion}
	for _, document := range bundle.Documents {
		result.Files++
		validateDocument(bundle.Root, document, &result)
	}
	result.Errors = append(result.Errors, ValidateRuleCatalog(bundle)...)

	sortIssues(result.Errors)
	sortIssues(result.Warnings)
	if err := applyValidationOptions(&result, options); err != nil {
		return Result{}, err
	}
	result.Checks = buildChecks(result)
	result.Summary = buildValidationSummary(result)
	result.Errors = nonNilIssues(result.Errors)
	result.Warnings = nonNilIssues(result.Warnings)
	result.Issues = nonNilIssues(issuesFromResult(result))
	return result, nil
}

func validateDocument(root string, document ASTDocument, result *Result) {
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
		validateMarkdownDiagnostics(rel, document.Markdown, result)
	}
	validateDocumentLinks(root, document, result)
}
