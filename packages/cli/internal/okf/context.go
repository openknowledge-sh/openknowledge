package okf

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

const DefaultContextBudget = 2400

type ContextOptions struct {
	Query  string
	Budget int
	Limit  int
}

type ContextResult struct {
	Root            string         `json:"root"`
	Query           string         `json:"query"`
	Budget          int            `json:"budget"`
	EstimatedTokens int            `json:"estimatedTokens"`
	Results         []ContextMatch `json:"results"`
	Issues          []Issue        `json:"issues,omitempty"`
}

type ContextMatch struct {
	ID              string  `json:"id"`
	Path            string  `json:"path"`
	Kind            string  `json:"kind"`
	Type            string  `json:"type,omitempty"`
	Title           string  `json:"title"`
	Heading         string  `json:"heading"`
	HeadingLevel    int     `json:"headingLevel,omitempty"`
	LineStart       int     `json:"lineStart"`
	LineEnd         int     `json:"lineEnd"`
	Score           float64 `json:"score"`
	EstimatedTokens int     `json:"estimatedTokens"`
	Text            string  `json:"text"`
	Links           []Link  `json:"links,omitempty"`
	Neighbor        bool    `json:"neighbor,omitempty"`
}

type ContextIndex struct {
	Root     string
	Sections []ContextSection
	Issues   []Issue
}

type ContextSection struct {
	ID              string
	Path            string
	Kind            string
	Type            string
	Title           string
	Description     string
	Frontmatter     map[string]string
	Heading         string
	HeadingLevel    int
	LineStart       int
	LineEnd         int
	Text            string
	Links           []Link
	EstimatedTokens int
}

type contextCandidate struct {
	section ContextSection
	score   float64
}

func BuildContextIndex(root string) (ContextIndex, error) {
	return BuildContextIndexWithVersion(root, LatestSpecVersion)
}

func BuildContextIndexWithVersion(root string, version string) (ContextIndex, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return ContextIndex{}, err
	}

	issues := append([]Issue{}, validation.Errors...)
	issues = append(issues, validation.Warnings...)
	var sections []ContextSection
	for _, document := range ast.Documents {
		if document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		metadata := document.Metadata
		if document.FrontmatterDiagnostic != nil {
			metadata = astDocumentMetadata{}
		}
		entry := listEntryFromASTSummary(summarizeASTDocument(document, metadata))
		sections = append(sections, splitContextSections(entry, document.Frontmatter.Values, document.Body, document.Links, document.Frontmatter.BodyLine)...)
	}

	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Path != sections[j].Path {
			return sections[i].Path < sections[j].Path
		}
		return sections[i].LineStart < sections[j].LineStart
	})
	return ContextIndex{Root: validation.Root, Sections: sections, Issues: issues}, nil
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

func splitContextSections(entry ListEntry, frontmatter map[string]string, body string, links []Link, bodyLine int) []ContextSection {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if bodyLine <= 0 {
		bodyLine = 1
	}

	type boundary struct {
		start int
		level int
		title string
	}
	var boundaries []boundary
	inCode := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		level := HeadingLevel(trimmed)
		if level <= 0 || level > 3 {
			continue
		}
		boundaries = append(boundaries, boundary{
			start: index,
			level: level,
			title: strings.TrimSpace(trimmed[level:]),
		})
	}

	if len(boundaries) == 0 {
		text := strings.TrimSpace(normalized)
		if text == "" {
			return nil
		}
		return []ContextSection{newContextSection(entry, frontmatter, "#top", "Top", 0, bodyLine, bodyLine+len(lines)-1, text, links)}
	}

	var sections []ContextSection
	if top := strings.TrimSpace(strings.Join(lines[:boundaries[0].start], "\n")); top != "" {
		sections = append(sections, newContextSection(entry, frontmatter, "#top", "Top", 0, bodyLine, bodyLine+boundaries[0].start-1, top, linksInRange(links, bodyLine, bodyLine+boundaries[0].start-1)))
	}

	usedIDs := map[string]int{}
	for index, current := range boundaries {
		end := len(lines) - 1
		if index+1 < len(boundaries) {
			end = boundaries[index+1].start - 1
		}
		text := strings.TrimSpace(strings.Join(lines[current.start:end+1], "\n"))
		if text == "" {
			continue
		}
		lineStart := bodyLine + current.start
		lineEnd := bodyLine + end
		id := contextSectionID(entry.ID, current.title, usedIDs)
		sections = append(sections, newContextSection(entry, frontmatter, id, current.title, current.level, lineStart, lineEnd, text, linksInRange(links, lineStart, lineEnd)))
	}
	return sections
}

func newContextSection(entry ListEntry, frontmatter map[string]string, id string, heading string, level int, lineStart int, lineEnd int, text string, links []Link) ContextSection {
	if id == "#top" {
		id = entry.ID + "#top"
	}
	return ContextSection{
		ID:              id,
		Path:            entry.Path,
		Kind:            entry.Kind,
		Type:            entry.Type,
		Title:           entry.Title,
		Description:     entry.Description,
		Frontmatter:     frontmatter,
		Heading:         heading,
		HeadingLevel:    level,
		LineStart:       lineStart,
		LineEnd:         lineEnd,
		Text:            text,
		Links:           links,
		EstimatedTokens: estimateContextTokens(text),
	}
}

func linksInRange(links []Link, lineStart int, lineEnd int) []Link {
	var filtered []Link
	for _, link := range links {
		if link.Line >= lineStart && link.Line <= lineEnd {
			filtered = append(filtered, link)
		}
	}
	return filtered
}

func contextSectionID(fileID string, heading string, used map[string]int) string {
	slug := strings.ReplaceAll(normalizeSearchText(heading), " ", "-")
	if slug == "" {
		slug = "section"
	}
	used[slug]++
	if used[slug] > 1 {
		slug = slug + "-" + strconv.Itoa(used[slug])
	}
	return fileID + "#" + slug
}

func estimateContextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return int(math.Ceil(float64(len([]rune(text))) / 4.0))
}

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
