package okf

import (
	"sort"
	"strings"
)

func scoreContextSections(sections []ContextSection, query string) []contextCandidate {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	normalizedQuery := normalizeSearchText(query)
	var candidates []contextCandidate
	for _, section := range sections {
		score := scoreContextSection(section, terms, normalizedQuery)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, contextCandidate{section: section, score: roundSearchScore(score)})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].section.Path != candidates[j].section.Path {
			return candidates[i].section.Path < candidates[j].section.Path
		}
		return candidates[i].section.LineStart < candidates[j].section.LineStart
	})
	return candidates
}

func scoreContextSection(section ContextSection, terms []string, normalizedQuery string) float64 {
	fields := []searchField{
		newSearchField("title", section.Title, 14),
		newSearchField("heading", section.Heading, 10),
		newSearchField("path", section.Path+" "+section.ID, 9),
		newSearchField("type", section.Type+" "+section.Kind, 6),
		newSearchField("description", section.Description, 5),
		newSearchField("metadata", frontmatterSearchText(section.Frontmatter), 3),
		newSearchField("body", section.Text, 1.2),
	}
	score := 0.0
	matchedTerms := map[string]struct{}{}
	for _, field := range fields {
		if field.text == "" {
			continue
		}
		if normalizedQuery != "" && strings.Contains(field.text, normalizedQuery) {
			score += field.weight * 4
		}
		for _, term := range terms {
			fieldScore, ok := scoreSearchField(field, term, true)
			if !ok {
				continue
			}
			score += fieldScore
			matchedTerms[term] = struct{}{}
		}
	}
	if len(matchedTerms) == 0 {
		return 0
	}
	if len(matchedTerms) == len(terms) {
		score *= 1.25
	}
	if isIndexMarkdownSearchResult(section.Path) {
		score *= 0.55
	}
	return score
}

func packContextCandidates(candidates []contextCandidate, budget int, limit int, neighbor bool) []ContextMatch {
	var matches []ContextMatch
	remaining := budget
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if len(matches) >= limit || remaining <= 0 {
			break
		}
		if _, ok := seen[candidate.section.ID]; ok {
			continue
		}
		match := contextMatch(candidate.section, candidate.score, neighbor)
		if match.EstimatedTokens > remaining {
			if len(matches) > 0 {
				continue
			}
			match = truncateContextMatch(match, remaining)
		}
		matches = append(matches, match)
		seen[match.ID] = struct{}{}
		remaining -= match.EstimatedTokens
	}
	return matches
}

func appendContextNeighbors(matches []ContextMatch, sections []ContextSection, budget int, limit int) []ContextMatch {
	usedTokens := 0
	seen := map[string]struct{}{}
	for _, match := range matches {
		usedTokens += match.EstimatedTokens
		seen[match.ID] = struct{}{}
	}
	remaining := budget - usedTokens
	if remaining <= 0 || len(matches) >= limit {
		return matches
	}

	byPath := map[string]ContextSection{}
	for _, section := range sections {
		if _, ok := byPath[section.Path]; !ok {
			byPath[section.Path] = section
		}
	}
	var candidates []contextCandidate
	for _, match := range matches {
		for _, link := range match.Links {
			if link.Kind != "local" || link.TargetPath == "" || !link.Exists {
				continue
			}
			section, ok := byPath[link.TargetPath]
			if !ok {
				continue
			}
			if _, ok := seen[section.ID]; ok {
				continue
			}
			candidates = append(candidates, contextCandidate{section: section, score: roundSearchScore(match.Score * 0.55)})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].section.Path < candidates[j].section.Path
	})
	for _, match := range packContextCandidates(candidates, remaining, limit-len(matches), true) {
		if _, ok := seen[match.ID]; ok {
			continue
		}
		matches = append(matches, match)
		seen[match.ID] = struct{}{}
	}
	return matches
}

func contextMatch(section ContextSection, score float64, neighbor bool) ContextMatch {
	return ContextMatch{
		ID:              section.ID,
		Path:            section.Path,
		Kind:            section.Kind,
		Type:            section.Type,
		Title:           section.Title,
		Heading:         section.Heading,
		HeadingPath:     append([]string{}, section.HeadingPath...),
		HeadingLevel:    section.HeadingLevel,
		LineStart:       section.LineStart,
		LineEnd:         section.LineEnd,
		Score:           score,
		EstimatedTokens: section.EstimatedTokens,
		Text:            section.Text,
		Links:           section.Links,
		Neighbor:        neighbor,
	}
}

func truncateContextMatch(match ContextMatch, budget int) ContextMatch {
	lines := strings.Split(match.Text, "\n")
	var selected []string
	estimated := 0
	for _, line := range lines {
		next := estimateContextTokens(strings.Join(append(selected, line), "\n"))
		if next > budget {
			break
		}
		selected = append(selected, line)
		estimated = next
	}
	if len(selected) == 0 {
		text := []rune(match.Text)
		if len(text) > budget*4 {
			text = text[:budget*4]
		}
		match.Text = strings.TrimSpace(string(text))
		match.LineEnd = match.LineStart
		match.EstimatedTokens = estimateContextTokens(match.Text)
		return match
	}
	match.Text = strings.TrimSpace(strings.Join(selected, "\n"))
	match.LineEnd = match.LineStart + len(selected) - 1
	match.EstimatedTokens = estimated
	match.Links = linksInRange(match.Links, match.LineStart, match.LineEnd)
	return match
}
