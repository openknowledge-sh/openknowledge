package okf

import (
	"sort"
	"strings"
)

const DefaultContextBudget = 2400

func BuildContextIndex(root string) (ContextIndex, error) {
	return BuildContextIndexWithVersion(root, LatestSpecVersion)
}

func BuildContextIndexWithVersion(root string, version string) (ContextIndex, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return ContextIndex{}, err
	}

	return contextIndexFromAST(validation, ast), nil
}

func contextIndexFromAST(validation Result, ast ASTBundle) ContextIndex {
	issues := append([]Issue{}, validation.Errors...)
	issues = append(issues, validation.Warnings...)
	var sections []ContextSection
	for _, document := range ast.Documents {
		if document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		metadata := document.Metadata
		if document.FrontmatterDiagnostic != nil {
			metadata = ASTDocumentMetadata{}
		}
		entry := listEntryFromASTSummary(SummarizeASTDocument(document, metadata))
		sections = append(sections, splitContextSections(entry, document.Frontmatter.Values, document.Body, document.Links, document.Frontmatter.BodyLine)...)
	}

	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Path != sections[j].Path {
			return sections[i].Path < sections[j].Path
		}
		return sections[i].LineStart < sections[j].LineStart
	})
	return ContextIndex{Root: validation.Root, Sections: sections, Issues: issues}
}

func ResolveContext(root string, options ContextOptions) (ContextResult, error) {
	return ResolveContextWithVersion(root, LatestSpecVersion, options)
}

func ResolveContextWithVersion(root string, version string, options ContextOptions) (ContextResult, error) {
	index, err := BuildContextIndexWithVersion(root, version)
	if err != nil {
		return ContextResult{}, err
	}
	return index.Resolve(options)
}

func (index ContextIndex) Resolve(options ContextOptions) (ContextResult, error) {
	query := strings.TrimSpace(options.Query)
	budget := options.Budget
	if budget <= 0 {
		budget = DefaultContextBudget
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}

	result := ContextResult{
		Root:   index.Root,
		Query:  query,
		Budget: budget,
		Issues: index.Issues,
	}
	if query == "" {
		return result, nil
	}

	candidates := scoreContextSections(index.Sections, query)
	selected := packContextCandidates(candidates, budget, limit, false)
	selected = appendContextNeighbors(selected, index.Sections, budget, limit)
	for _, match := range selected {
		result.EstimatedTokens += match.EstimatedTokens
	}
	result.Results = selected
	return result, nil
}
