package okf

import "strings"

func packContextSources(sections []ContextSection, direct []SearchResult, neighbors []SearchResult, budget int, limit int) []ContextSource {
	byID := make(map[string]ContextSection, len(sections))
	for _, section := range sections {
		byID[section.ID] = section
	}

	sources := make([]ContextSource, 0, minInt(limit, len(direct)+len(neighbors)))
	remaining := budget
	var deferred []ContextSource
	pack := func(results []SearchResult) bool {
		for _, result := range results {
			if len(sources) >= limit || remaining <= 0 {
				return false
			}
			section, ok := byID[result.ID]
			if !ok {
				continue
			}
			source := contextSourceFromSearchResult(section, result)
			if source.EstimatedTokens > remaining {
				if len(sources) == 0 {
					source = truncateContextSource(source, remaining)
					if source.EstimatedTokens > 0 && strings.TrimSpace(source.Markdown) != "" {
						sources = append(sources, source)
					}
					return false
				}
				deferred = append(deferred, source)
				continue
			}
			sources = append(sources, source)
			remaining -= source.EstimatedTokens
		}
		return true
	}

	if pack(direct) {
		pack(neighbors)
	}
	if len(sources) < limit && remaining > 0 && len(deferred) > 0 {
		source := truncateContextSource(deferred[0], remaining)
		if source.EstimatedTokens > 0 && strings.TrimSpace(source.Markdown) != "" {
			sources = append(sources, source)
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
