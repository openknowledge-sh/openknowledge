package okf

import "strings"

func prioritizeContextResults(sections []ContextSection, direct []SearchResult, query string, limit int, includeDocumentContext bool) []SearchResult {
	if len(direct) == 0 || limit <= 1 || (len(direct) == 1 && !includeDocumentContext) {
		return direct
	}

	byID := make(map[string]ContextSection, len(sections))
	byPath := make(map[string][]ContextSection)
	for _, section := range sections {
		byID[section.ID] = section
		byPath[section.Path] = append(byPath[section.Path], section)
	}
	directByID := make(map[string]SearchResult, len(direct))
	for _, result := range direct {
		directByID[result.ID] = result
	}
	selected := make([]SearchResult, 0, minInt(limit, len(direct)))
	selectedIDs := map[string]struct{}{}
	add := func(result SearchResult, priority int) bool {
		if len(selected) >= limit {
			return false
		}
		if _, exists := selectedIDs[result.ID]; exists {
			return true
		}
		result.contextPriority = priority
		selectedIDs[result.ID] = struct{}{}
		selected = append(selected, result)
		return true
	}

	terms := searchTerms(query)

	// Preserve the strongest direct evidence before adding context. This keeps
	// ranking and context mode aligned while still leaving room for document
	// cohesion under the default source limit.
	for _, result := range direct[:minInt(5, len(direct))] {
		add(result, 2)
	}

	// Long documents commonly answer one question across several sibling
	// sections. Bring the next two lexical matches from the strongest
	// document forward before unrelated lower-ranked documents consume the
	// source limit.
	topPath := direct[0].Path
	samePathAdds := 0
	for _, result := range direct {
		if len(selected) >= limit || samePathAdds >= 2 {
			break
		}
		if result.Path != topPath {
			continue
		}
		if _, exists := selectedIDs[result.ID]; exists {
			continue
		}
		if add(result, 1) {
			samePathAdds++
		}
	}

	// A focused child often relies on its parent's short explanation, while a
	// matching parent can keep its decisive details in child sections. Bring
	// the closest parent and immediate children of the strongest seeds forward.
	// These structurally selected sections are explicit document context and
	// are omitted by --no-expand.
	hierarchyAdds := 0
	hierarchySeeds := make([]SearchResult, 0, len(selected))
	for _, seed := range selected {
		if seed.Path == topPath {
			hierarchySeeds = append(hierarchySeeds, seed)
		}
	}
	for _, seed := range selected {
		if seed.Path != topPath {
			hierarchySeeds = append(hierarchySeeds, seed)
		}
	}
	for _, seed := range hierarchySeeds {
		if !includeDocumentContext || len(selected) >= limit || hierarchyAdds >= 4 {
			break
		}
		seedSection, ok := byID[seed.ID]
		if !ok || len(seedSection.HeadingPath) == 0 {
			continue
		}

		var ancestor *ContextSection
		var children []ContextSection
		for _, candidate := range byPath[seed.Path] {
			if candidate.ID == seed.ID {
				continue
			}
			if strictHeadingPathPrefix(candidate.HeadingPath, seedSection.HeadingPath) &&
				(ancestor == nil || len(candidate.HeadingPath) > len(ancestor.HeadingPath)) {
				candidateCopy := candidate
				ancestor = &candidateCopy
			}
			if strictHeadingPathPrefix(seedSection.HeadingPath, candidate.HeadingPath) &&
				len(candidate.HeadingPath) == len(seedSection.HeadingPath)+1 {
				children = append(children, candidate)
			}
		}
		if ancestor != nil {
			if _, exists := selectedIDs[ancestor.ID]; !exists && add(contextResultForSection(*ancestor, seed, directByID, terms, query), 1) {
				hierarchyAdds++
			}
		}
		for _, child := range children[:minInt(2, len(children))] {
			if hierarchyAdds >= 4 || len(selected) >= limit {
				break
			}
			if _, exists := selectedIDs[child.ID]; !exists && add(contextResultForSection(child, seed, directByID, terms, query), 1) {
				hierarchyAdds++
			}
		}
	}

	// Use any remaining slots for lexical sections that add uncovered query
	// evidence. Ties retain the original deterministic ranking.
	covered := map[string]struct{}{}
	for _, result := range selected {
		for term := range contextCoveredTerms(byID[result.ID], terms) {
			covered[term] = struct{}{}
		}
	}
	for len(selected) < limit {
		bestIndex := -1
		bestGain := 0
		for index, candidate := range direct {
			if _, exists := selectedIDs[candidate.ID]; exists {
				continue
			}
			gain := 0
			for term := range contextCoveredTerms(byID[candidate.ID], terms) {
				if _, exists := covered[term]; !exists {
					gain++
				}
			}
			if gain > bestGain {
				bestIndex = index
				bestGain = gain
			}
		}
		if bestIndex < 0 || bestGain == 0 {
			break
		}
		candidate := direct[bestIndex]
		add(candidate, 1)
		for term := range contextCoveredTerms(byID[candidate.ID], terms) {
			covered[term] = struct{}{}
		}
	}

	// Preserve lower-ranked direct evidence after the prioritized prefix so a
	// large explicit budget can continue filling in deterministic score order.
	for _, result := range direct {
		if _, exists := selectedIDs[result.ID]; exists {
			continue
		}
		selected = append(selected, result)
	}
	return selected
}

func contextResultForSection(section ContextSection, seed SearchResult, directByID map[string]SearchResult, terms []string, query string) SearchResult {
	if result, ok := directByID[section.ID]; ok {
		return result
	}
	return searchResultFromContextSection(
		section,
		roundSearchScore(seed.Score*0.8),
		[]string{"hierarchy"},
		true,
		"document-context",
		terms,
		normalizeSearchText(query),
		true,
	)
}

func contextCoveredTerms(section ContextSection, terms []string) map[string]struct{} {
	covered := map[string]struct{}{}
	if section.ID == "" {
		return covered
	}
	text := normalizeSearchText(strings.Join([]string{
		section.Heading,
		strings.Join(section.HeadingPath, " "),
		section.Text,
	}, " "))
	for _, term := range terms {
		if snippetMatchesTerm(text, term) {
			covered[term] = struct{}{}
		}
	}
	return covered
}

func strictHeadingPathPrefix(prefix []string, path []string) bool {
	return len(prefix) < len(path) && headingPathHasPrefix(path, prefix)
}

func packContextSources(sections []ContextSection, direct []SearchResult, neighbors []SearchResult, budget int, limit int) []ContextSource {
	byID := make(map[string]ContextSection, len(sections))
	for _, section := range sections {
		byID[section.ID] = section
	}

	sources := make([]ContextSource, 0, minInt(limit, len(direct)+len(neighbors)))
	remaining := budget
	var deferredSeeds []ContextSource
	var deferred []ContextSource
	pack := func(results []SearchResult) bool {
		for _, result := range results {
			if len(sources) >= limit || remaining <= 0 {
				return false
			}
			if result.contextPriority < 2 && len(deferredSeeds) > 0 {
				source := truncateContextSource(deferredSeeds[0], remaining)
				if source.EstimatedTokens > 0 && strings.TrimSpace(source.Markdown) != "" {
					sources = append(sources, source)
				}
				return false
			}
			section, ok := byID[result.ID]
			if !ok {
				continue
			}
			source := contextSourceFromSearchResult(section, result)
			if source.EstimatedTokens > remaining {
				switch result.contextPriority {
				case 2:
					deferredSeeds = append(deferredSeeds, source)
					continue
				case 1:
					source = truncateContextSource(source, remaining)
					if source.EstimatedTokens > 0 && strings.TrimSpace(source.Markdown) != "" {
						sources = append(sources, source)
					}
					return false
				default:
					deferred = append(deferred, source)
					continue
				}
			}
			sources = append(sources, source)
			remaining -= source.EstimatedTokens
		}
		return true
	}

	if pack(direct) {
		pack(neighbors)
	}
	if len(sources) < limit && remaining > 0 {
		candidates := deferredSeeds
		if len(candidates) == 0 {
			candidates = deferred
		}
		if len(candidates) > 0 {
			source := truncateContextSource(candidates[0], remaining)
			if source.EstimatedTokens > 0 && strings.TrimSpace(source.Markdown) != "" {
				sources = append(sources, source)
			}
		}
	}
	return sources
}

func contextSourceFromSearchResult(section ContextSection, result SearchResult) ContextSource {
	relation := result.Relation
	if relation == "" {
		relation = "direct"
	}
	title := result.Title
	if title == "" {
		title = deriveTitle(section.Path)
	}
	return ContextSource{
		ID:              section.ID,
		Locator:         section.Locator,
		ContentSHA256:   section.ContentSHA256,
		Path:            section.Path,
		Kind:            section.Kind,
		Type:            section.Type,
		Title:           title,
		Heading:         section.Heading,
		HeadingPath:     append([]string{}, section.HeadingPath...),
		HeadingLevel:    section.HeadingLevel,
		LineStart:       section.LineStart,
		LineEnd:         section.LineEnd,
		Score:           result.Score,
		EstimatedTokens: section.EstimatedTokens,
		Relation:        relation,
		Markdown:        section.Text,
	}
}

func truncateContextSource(source ContextSource, budget int) ContextSource {
	if budget <= 0 {
		source.Markdown = ""
		source.EstimatedTokens = 0
		return source
	}

	lines := strings.Split(source.Markdown, "\n")
	var selected []string
	for _, line := range lines {
		candidate := strings.Join(append(selected, line), "\n")
		if estimateContextTokens(candidate) > budget {
			break
		}
		selected = append(selected, line)
	}
	if len(selected) > 0 {
		source.Markdown = strings.TrimSpace(strings.Join(selected, "\n"))
		source.LineEnd = source.LineStart + len(selected) - 1
		source.EstimatedTokens = estimateContextTokens(source.Markdown)
		return source
	}

	runes := []rune(source.Markdown)
	limit := budget * 4
	if len(runes) > limit {
		runes = runes[:limit]
	}
	source.Markdown = strings.TrimSpace(string(runes))
	source.LineEnd = source.LineStart
	source.EstimatedTokens = estimateContextTokens(source.Markdown)
	return source
}
