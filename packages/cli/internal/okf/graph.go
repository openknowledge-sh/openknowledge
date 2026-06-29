package okf

import "sort"

func BuildGraph(root string) (Graph, error) {
	return BuildGraphWithVersion(root, LatestSpecVersion)
}

func BuildGraphWithVersion(root string, version string) (Graph, error) {
	bundle, err := ParseBundleWithVersion(root, version)
	if err != nil {
		return Graph{}, err
	}
	return GraphFromBundle(bundle), nil
}

func GraphFromBundle(bundle Bundle) Graph {
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
		Nodes:       nodes,
		Edges:       edges,
		Issues:      bundle.Issues,
	}
}
