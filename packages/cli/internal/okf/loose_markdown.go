package okf

import "sort"

// BuildLooseMarkdownIndex builds retrieval projections from ordinary Markdown.
// It deliberately skips OKF validation so local discovery and search can work
// before setup. Publication and managed-bundle operations remain strict.
func BuildLooseMarkdownIndex(root string) (ContextIndex, error) {
	discovery, err := DiscoverMarkdown(root, MarkdownDiscoveryOptions{})
	if err != nil {
		return ContextIndex{}, err
	}
	documents := make([]ASTDocument, 0, len(discovery.Documents))
	for _, discovered := range discovery.Documents {
		document := parseASTDocumentLinks(discovery.Root, parseASTDocumentContent(ASTDocument{
			Absolute: discovered.absolute,
			Rel:      discovered.Path,
			ID:       trimMarkdownExtension(discovered.Path),
			Kind:     "concept",
		}, discovered.content))
		documents = append(documents, document)
	}
	sort.Slice(documents, func(i, j int) bool { return documents[i].Rel < documents[j].Rel })
	bundle := ASTBundle{SchemaVersion: MachineSchemaVersion, Root: discovery.Root, SpecVersion: LatestSpecVersion, Documents: documents}
	index := ContextIndexFromAST(Result{SchemaVersion: MachineSchemaVersion, Root: discovery.Root, SpecVersion: LatestSpecVersion}, bundle)
	index.Status = "unmanaged"
	return index, nil
}

func SearchLooseMarkdown(root string, options SearchOptions) (SearchResultSet, error) {
	index, err := BuildLooseMarkdownIndex(root)
	if err != nil {
		return SearchResultSet{}, err
	}
	return index.Search(options), nil
}

func ResolveLooseMarkdownContext(root string, options ContextOptions) (ContextResult, error) {
	index, err := BuildLooseMarkdownIndex(root)
	if err != nil {
		return ContextResult{}, err
	}
	return index.Resolve(options)
}
