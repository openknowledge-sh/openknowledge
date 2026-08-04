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
		var signals *OKFV02Signals
		if bundle.SpecVersion == "0.2" && !file.Reserved {
			signals = DeriveOKFV02Signals(file.Frontmatter)
		}
		nodes = append(nodes, GraphNode{
			ID:          file.ID,
			Path:        file.Path,
			Kind:        file.Kind,
			Reserved:    file.Reserved,
			Type:        file.Type,
			Title:       file.Title,
			Description: file.Description,
			Resource:    file.Resource,
			OKF02:       signals,
			Issues:      file.Issues,
		})
	}

	edges := []GraphEdge{}
	for _, file := range bundle.Files {
		for _, link := range file.Links {
			if link.Kind != "local" || link.TargetPath == "" || !link.Exists {
				continue
			}
			target, ok := graphTargetFile(paths, link.TargetPath)
			if !ok || target.Path == file.Path {
				continue
			}
			edges = append(edges, GraphEdge{
				Source:       file.Path,
				Target:       target.Path,
				SourceID:     file.ID,
				TargetID:     target.ID,
				Label:        link.Label,
				Href:         link.Href,
				Line:         link.Line,
				LinkTargetID: link.TargetID,
				TargetAnchor: link.TargetAnchor,
			})
		}
	}
	if bundle.SpecVersion == "0.2" {
		resourceNodes, resourceEdges := okfV02GraphResources(bundle.Files, paths)
		nodes = append(nodes, resourceNodes...)
		edges = append(edges, resourceEdges...)
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		if edges[i].Line != edges[j].Line {
			return edges[i].Line < edges[j].Line
		}
		if edges[i].Href != edges[j].Href {
			return edges[i].Href < edges[j].Href
		}
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		return edges[i].Label < edges[j].Label
	})

	return Graph{
		SchemaVersion: MachineSchemaVersion,
		Root:          bundle.Root,
		SpecVersion:   bundle.SpecVersion,
		Type:          GraphTypeSource,
		Nodes:         nodes,
		Edges:         edges,
		Issues:        bundle.Issues,
	}
}

func okfV02GraphResources(files []BundleFile, paths map[string]BundleFile) ([]GraphNode, []GraphEdge) {
	nodesByID := map[string]GraphNode{}
	var edges []GraphEdge
	for _, file := range files {
		if file.Reserved {
			continue
		}
		signals := DeriveOKFV02Signals(file.Frontmatter)
		for _, source := range signals.Sources {
			node, edge, ok := okfV02GraphResource(file, source.Resource, source.Title, source.ID, "source", paths)
			if !ok {
				continue
			}
			if node.ID != "" {
				nodesByID[node.ID] = node
			}
			edges = append(edges, edge)
		}
		if signals.Computation == nil {
			continue
		}
		resources := []struct {
			value string
			kind  string
		}{
			{value: signals.Computation.Path, kind: "computation"},
		}
		if signals.Computation.Executor != nil {
			resources = append(resources, struct {
				value string
				kind  string
			}{value: signals.Computation.Executor.Resource, kind: "executor"})
		}
		if signals.Computation.Attester != nil {
			resources = append(resources, struct {
				value string
				kind  string
			}{value: signals.Computation.Attester.Resource, kind: "attester"})
		}
		for _, resource := range resources {
			node, edge, ok := okfV02GraphResource(file, resource.value, "", "", resource.kind, paths)
			if !ok {
				continue
			}
			if node.ID != "" {
				nodesByID[node.ID] = node
			}
			edges = append(edges, edge)
		}
	}

	nodes := make([]GraphNode, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Path != nodes[j].Path {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, edges
}

func okfV02GraphResource(file BundleFile, resource string, title string, label string, kind string, paths map[string]BundleFile) (GraphNode, GraphEdge, bool) {
	target, targetPath, ok := okfV02ResourceTarget(file.Path, resource, paths)
	if !ok {
		return GraphNode{}, GraphEdge{}, false
	}
	if label == "" {
		label = title
	}
	edge := GraphEdge{
		Kind:     kind,
		Source:   file.Path,
		SourceID: file.ID,
		Label:    label,
		Href:     resource,
	}
	if target.ID != "" {
		edge.Target = target.Path
		edge.TargetID = target.ID
		return GraphNode{}, edge, true
	}
	resourceID := okfV02ResourceNodeID(targetPath)
	edge.Target = targetPath
	edge.TargetID = resourceID
	if title == "" {
		title = targetPath
	}
	return GraphNode{
		ID:       resourceID,
		Path:     targetPath,
		Kind:     "resource",
		Title:    title,
		Resource: resource,
	}, edge, true
}

func graphTargetFile(paths map[string]BundleFile, targetPath string) (BundleFile, bool) {
	if target, ok := paths[targetPath]; ok {
		return target, true
	}
	target, ok := paths[strings.TrimSuffix(targetPath, "/")+"/index.md"]
	return target, ok
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

	lookup := index.sectionLookup
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

	// Chunk-level link edges connect the section containing a link to the
	// content-bearing chunk addressed by its optional fragment.
	for _, section := range index.Sections {
		for _, link := range section.Links {
			target, ok := lookup.target(link)
			if !ok || target.ID == section.ID {
				continue
			}
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
				TargetAnchor: link.TargetAnchor,
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
		if graph.Edges[i].Target != graph.Edges[j].Target {
			return graph.Edges[i].Target < graph.Edges[j].Target
		}
		if graph.Edges[i].Line != graph.Edges[j].Line {
			return graph.Edges[i].Line < graph.Edges[j].Line
		}
		if graph.Edges[i].Href != graph.Edges[j].Href {
			return graph.Edges[i].Href < graph.Edges[j].Href
		}
		return graph.Edges[i].Label < graph.Edges[j].Label
	})
	return graph, nil
}
