package okf

import (
	"fmt"
	"sort"
	"strings"
)

func BuildGraph(root string) (Graph, error) {
	return BuildGraphWithVersion(root, LatestSpecVersion)
}

func BuildGraphWithVersion(root string, version string) (Graph, error) {
	return BuildGraphWithType(root, version, GraphTypeSource)
}

func BuildGraphWithType(root string, version string, graphType string) (Graph, error) {
	graphType = strings.TrimSpace(strings.ToLower(graphType))
	if graphType == "" {
		graphType = GraphTypeSource
	}
	switch graphType {
	case GraphTypeSource:
		return buildSourceGraphWithVersion(root, version)
	case GraphTypeSearch:
		return buildSearchGraphWithVersion(root, version)
	default:
		return Graph{}, fmt.Errorf("unsupported graph type %q; use source or search", graphType)
	}
}

func buildSourceGraphWithVersion(root string, version string) (Graph, error) {
	bundle, err := ParseBundleWithVersion(root, version)
	if err != nil {
		return Graph{}, err
	}
	return GraphFromBundle(bundle), nil
}

func GraphFromBundle(bundle Bundle) Graph {
	// Source graphs are the original authored-file view: one node per bundle
	// file and one edge per existing local Markdown link.
	nodes := make([]GraphNode, 0, len(bundle.Files))
	paths := make(map[string]BundleFile, len(bundle.Files))
	for _, file := range bundle.Files {
		paths[file.Path] = file
		nodes = append(nodes, GraphNode{
			ID:          file.ID,
			Path:        file.Path,
			Kind:        file.Kind,
			Reserved:    file.Reserved,
			Type:        file.Type,
			Title:       file.Title,
			Description: file.Description,
			Resource:    file.Resource,
			Issues:      file.Issues,
		})
	}

	seenEdges := map[string]bool{}
	edges := []GraphEdge{}
	for _, file := range bundle.Files {
		for _, link := range file.Links {
			if link.Kind != "local" || link.TargetPath == "" || link.TargetPath == file.Path {
				continue
			}
			target, ok := paths[link.TargetPath]
			if !ok || !link.Exists {
				continue
			}
			key := file.Path + "\x00" + link.TargetPath
			if seenEdges[key] {
				continue
			}
			seenEdges[key] = true
			edges = append(edges, GraphEdge{
				Source:       file.Path,
				Target:       link.TargetPath,
				SourceID:     file.ID,
				TargetID:     target.ID,
				Label:        link.Label,
				Href:         link.Href,
				Line:         link.Line,
				LinkTargetID: link.TargetID,
			})
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source == edges[j].Source {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Source < edges[j].Source
	})

	return Graph{
		Root:        bundle.Root,
		SpecVersion: bundle.SpecVersion,
		Type:        GraphTypeSource,
		Nodes:       nodes,
		Edges:       edges,
		Issues:      bundle.Issues,
	}
}

func buildSearchGraphWithVersion(root string, version string) (Graph, error) {
	source, err := buildSourceGraphWithVersion(root, version)
	if err != nil {
		return Graph{}, err
	}
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return Graph{}, err
	}
	index := ContextIndexFromAST(validation, ast)

	graph := source
	graph.Type = GraphTypeSearch
	graph.Issues = index.Issues

	// Search graphs reuse source nodes, then layer derivative chunk nodes and
	// typed retrieval edges on top. The source Markdown remains unchanged.
	fileIDByPath := map[string]string{}
	for _, node := range source.Nodes {
		if node.Kind != "chunk" {
			fileIDByPath[node.Path] = node.ID
		}
	}

	firstSectionByPath := map[string]ContextSection{}
	sectionsByPath := map[string][]ContextSection{}
	for _, section := range index.Sections {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:          section.ID,
			Path:        section.Path,
			Kind:        "chunk",
			Type:        section.Type,
			Title:       section.Title,
			Description: section.Description,
			Heading:     section.Heading,
			HeadingPath: append([]string{}, section.HeadingPath...),
			LineStart:   section.LineStart,
			LineEnd:     section.LineEnd,
		})
		if existing, ok := firstSectionByPath[section.Path]; !ok || section.LineStart < existing.LineStart {
			firstSectionByPath[section.Path] = section
		}
		sectionsByPath[section.Path] = append(sectionsByPath[section.Path], section)
		sourceID := fileIDByPath[section.Path]
		if sourceID == "" {
			sourceID = section.Path
		}
		graph.Edges = append(graph.Edges, GraphEdge{
			Kind:     "contains",
			Source:   section.Path,
			Target:   section.ID,
			SourceID: sourceID,
			TargetID: section.ID,
		})
	}

	// Reading-order edges let downstream tools walk adjacent chunks without
	// inferring order from line numbers or IDs.
	for _, sections := range sectionsByPath {
		sort.SliceStable(sections, func(i, j int) bool {
			return sections[i].LineStart < sections[j].LineStart
		})
		for index := 0; index+1 < len(sections); index++ {
			graph.Edges = append(graph.Edges, GraphEdge{
				Kind:     "next",
				Source:   sections[index].ID,
				Target:   sections[index+1].ID,
				SourceID: sections[index].ID,
				TargetID: sections[index+1].ID,
			})
		}
	}

	// Chunk-level link edges connect the section containing a link to the first
	// content-bearing chunk in the target file.
	seenChunkLinks := map[string]bool{}
	for _, section := range index.Sections {
		for _, link := range section.Links {
			if link.Kind != "local" || link.TargetPath == "" || !link.Exists || link.TargetPath == section.Path {
				continue
			}
			target, ok := firstSectionByPath[link.TargetPath]
			if !ok {
				continue
			}
			key := section.ID + "\x00" + target.ID + "\x00" + link.Label
			if seenChunkLinks[key] {
				continue
			}
			seenChunkLinks[key] = true
			graph.Edges = append(graph.Edges, GraphEdge{
				Kind:         "local-link",
				Source:       section.ID,
				Target:       target.ID,
				SourceID:     section.ID,
				TargetID:     target.ID,
				Label:        link.Label,
				Href:         link.Href,
				Line:         link.Line,
				LinkTargetID: link.TargetID,
			})
		}
	}

	sort.SliceStable(graph.Nodes, func(i, j int) bool {
		if graph.Nodes[i].Kind != graph.Nodes[j].Kind {
			return graph.Nodes[i].Kind < graph.Nodes[j].Kind
		}
		if graph.Nodes[i].Path != graph.Nodes[j].Path {
			return graph.Nodes[i].Path < graph.Nodes[j].Path
		}
		if graph.Nodes[i].LineStart != graph.Nodes[j].LineStart {
			return graph.Nodes[i].LineStart < graph.Nodes[j].LineStart
		}
		return graph.Nodes[i].ID < graph.Nodes[j].ID
	})
	sort.SliceStable(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].Kind != graph.Edges[j].Kind {
			return graph.Edges[i].Kind < graph.Edges[j].Kind
		}
		if graph.Edges[i].Source != graph.Edges[j].Source {
			return graph.Edges[i].Source < graph.Edges[j].Source
		}
		return graph.Edges[i].Target < graph.Edges[j].Target
	})
	return graph, nil
}
