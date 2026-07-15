package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
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

	return ContextIndexFromAST(validation, ast), nil
}

func ContextIndexFromAST(validation Result, ast ASTBundle) ContextIndex {
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
		sections = append(sections, splitContextSectionsFromASTDocument(entry, document)...)
	}
	revision := RetrievalRevision{SpecVersion: validation.SpecVersion, IndexSHA256: retrievalIndexSHA256(ast)}
	for index := range sections {
		sections[index].Locator = retrievalLocator(revision.IndexSHA256, sections[index].Path, sections[index].ContentSHA256)
	}

	// The context index predates the search command and is still the shared
	// source of section chunks for context packing, search, and search graphs.
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Path != sections[j].Path {
			return sections[i].Path < sections[j].Path
		}
		return sections[i].LineStart < sections[j].LineStart
	})
	return ContextIndex{Root: validation.Root, Revision: revision, Sections: sections, Issues: issues}
}

func retrievalIndexSHA256(ast ASTBundle) string {
	documents := append([]ASTDocument(nil), ast.Documents...)
	sort.Slice(documents, func(i, j int) bool { return documents[i].Rel < documents[j].Rel })
	hash := sha256.New()
	for _, document := range documents {
		if document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		writeContentHashRecord(hash, 'f', filepath.ToSlash(document.Rel), int64(len(document.Content)))
		_, _ = hash.Write([]byte(document.Content))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func retrievalLocator(indexSHA256 string, path string, contentSHA256 string) string {
	return "okf+sha256://" + indexSHA256 + "/" + url.PathEscape(filepath.ToSlash(path)) + "#" + contentSHA256
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
		SchemaVersion: MachineSchemaVersion,
		Root:          index.Root,
		Revision:      index.Revision,
		Query:         query,
		Budget:        budget,
		Limit:         limit,
		Sources:       []ContextSource{},
		Issues:        index.Issues,
	}
	if query == "" {
		return result, nil
	}

	searchOptions := SearchOptions{Query: query, Limit: limit, Fuzzy: true}
	direct := index.rankKnowledgeSearch(searchOptions)
	seedCount := minInt(limit, len(direct))
	var neighbors []SearchResult
	if !options.NoExpand && seedCount > 0 {
		neighbors = knowledgeSearchGraphNeighbors(direct[:seedCount], direct, index.Sections)
	}
	result.Sources = packContextSources(index.Sections, direct, neighbors, budget, limit)
	for _, source := range result.Sources {
		result.EstimatedTokens += source.EstimatedTokens
	}
	return result, nil
}
