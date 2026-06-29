package okf

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const maxContextKeyPoints = 8

type contextKeyPointCandidate struct {
	point ContextKeyPoint
	score float64
	order int
}

type contextKeyPointBlock struct {
	text      string
	lineStart int
}

func buildContextBriefing(result ContextResult) ContextBriefing {
	briefing := ContextBriefing{
		Summary:          contextBriefingSummary(result),
		KeyPoints:        contextKeyPoints(result.Query, result.Results),
		Related:          contextRelatedSources(result.Results),
		Gaps:             contextGaps(result),
		ValidationIssues: len(result.Issues),
	}
	return briefing
}

func contextBriefingSummary(result ContextResult) string {
	if len(result.Results) == 0 {
		return "No matching sections were selected for this query."
	}

	files := map[string]struct{}{}
	neighbors := 0
	for _, match := range result.Results {
		files[match.Path] = struct{}{}
		if match.Neighbor {
			neighbors++
		}
	}

	parts := []string{
		fmt.Sprintf("Selected %d source-grounded section%s", len(result.Results), pluralSuffix(len(result.Results))),
		fmt.Sprintf("from %d file%s", len(files), pluralSuffix(len(files))),
		fmt.Sprintf("within an estimated %d of %d tokens", result.EstimatedTokens, result.Budget),
	}
	if neighbors > 0 {
		parts = append(parts, fmt.Sprintf("including %d linked neighbor section%s", neighbors, pluralSuffix(neighbors)))
	}
	return strings.Join(parts, ", ") + "."
}

func contextKeyPoints(query string, matches []ContextMatch) []ContextKeyPoint {
	terms := searchTerms(query)
	normalizedQuery := normalizeSearchText(query)
	var candidates []contextKeyPointCandidate
	seen := map[string]struct{}{}
	order := 0

	for _, match := range matches {
		blocks := contextKeyPointBlocks(match.Text, match.LineStart)
		matchAdded := false
		for _, block := range blocks {
			text := block.text
			if !contextKeyPointRelevant(text, terms, normalizedQuery) {
				continue
			}
			key := normalizeSearchText(text)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			score := contextKeyPointScore(text, terms, normalizedQuery, match)
			candidates = append(candidates, contextKeyPointCandidate{
				point: ContextKeyPoint{
					Text:     truncateContextKeyPoint(text),
					Path:     match.Path,
					Line:     block.lineStart,
					Heading:  match.Heading,
					Neighbor: match.Neighbor,
				},
				score: score,
				order: order,
			})
			matchAdded = true
			order++
		}
		if !matchAdded && !match.Neighbor && len(blocks) > 0 {
			text := blocks[0].text
			key := normalizeSearchText(text)
			if key != "" {
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					candidates = append(candidates, contextKeyPointCandidate{
						point: ContextKeyPoint{
							Text:    truncateContextKeyPoint(text),
							Path:    match.Path,
							Line:    blocks[0].lineStart,
							Heading: match.Heading,
						},
						score: match.Score * 0.7,
						order: order,
					})
					order++
				}
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].order < candidates[j].order
	})

	if len(candidates) > maxContextKeyPoints {
		candidates = candidates[:maxContextKeyPoints]
	}

	points := make([]ContextKeyPoint, 0, len(candidates))
	for _, candidate := range candidates {
		points = append(points, candidate.point)
	}
	return points
}

func contextKeyPointRelevant(text string, terms []string, normalizedQuery string) bool {
	normalized := normalizeSearchText(text)
	if normalizedQuery != "" && strings.Contains(normalized, normalizedQuery) {
		return true
	}
	for _, term := range terms {
		if strings.Contains(normalized, term) {
			return true
		}
	}
	return strings.Contains(normalized, "openknowledge")
}

func contextKeyPointBlocks(text string, lineStart int) []contextKeyPointBlock {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var blocks []contextKeyPointBlock
	var pending []string
	pendingLine := 0
	inFence := false

	flush := func() {
		if len(pending) == 0 {
			return
		}
		joined := strings.Join(pending, " ")
		if contextKeyPointLineUseful(joined) {
			blocks = append(blocks, contextKeyPointBlock{text: joined, lineStart: pendingLine})
		}
		pending = nil
		pendingLine = 0
	}

	for index, line := range lines {
		currentLine := lineStart + index
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			flush()
			inFence = !inFence
			continue
		}
		if contextKeyPointLineIgnored(trimmed) {
			flush()
			continue
		}

		normalized := normalizeContextKeyPointText(trimmed)
		if inFence {
			flush()
			if contextKeyPointLineUseful(normalized) {
				blocks = append(blocks, contextKeyPointBlock{text: normalized, lineStart: currentLine})
			}
			continue
		}

		if isMarkdownListLine(trimmed) {
			flush()
			pendingLine = currentLine
			pending = append(pending, normalized)
			continue
		}
		if len(pending) == 0 {
			pendingLine = currentLine
		}
		pending = append(pending, normalized)
	}
	flush()
	return blocks
}

func contextKeyPointScore(text string, terms []string, normalizedQuery string, match ContextMatch) float64 {
	normalized := normalizeSearchText(text)
	score := match.Score
	if match.Neighbor {
		score *= 0.6
	}
	if normalizedQuery != "" && strings.Contains(normalized, normalizedQuery) {
		score += 24
	}
	for _, term := range terms {
		if strings.Contains(normalized, term) {
			score += 8
		}
	}
	if strings.Contains(text, "`") {
		score += 4
	}
	return score
}

func normalizeContextKeyPointText(line string) string {
	text := strings.TrimSpace(line)
	text = strings.TrimPrefix(text, ">")
	text = strings.TrimSpace(text)
	text = strings.TrimLeftFunc(text, func(r rune) bool {
		return r == '-' || r == '*' || r == '+'
	})
	text = strings.TrimSpace(text)
	if dot := strings.Index(text, ". "); dot > 0 && dot < 4 {
		prefix := text[:dot]
		if allDigits(prefix) {
			text = strings.TrimSpace(text[dot+2:])
		}
	}
	return strings.TrimSpace(text)
}

func contextKeyPointLineUseful(text string) bool {
	if text == "" {
		return false
	}
	if contextKeyPointLineIgnored(text) {
		return false
	}
	return len([]rune(text)) >= 10
}

func contextKeyPointLineIgnored(text string) bool {
	if strings.HasPrefix(text, "#") || strings.HasPrefix(text, "```") || strings.HasPrefix(text, "---") {
		return true
	}
	if strings.HasPrefix(text, "<!--") {
		return true
	}
	if strings.Trim(text, "-:| ") == "" {
		return true
	}
	return false
}

func isMarkdownListLine(text string) bool {
	if strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "* ") || strings.HasPrefix(text, "+ ") {
		return true
	}
	dot := strings.Index(text, ". ")
	if dot <= 0 || dot > 3 {
		return false
	}
	return allDigits(text[:dot])
}

func truncateContextKeyPoint(text string) string {
	const limit = 220
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	cut := limit
	for cut > 120 && !unicode.IsSpace(runes[cut-1]) {
		cut--
	}
	if cut <= 120 {
		cut = limit
	}
	return strings.TrimSpace(string(runes[:cut])) + "..."
}

func contextRelatedSources(matches []ContextMatch) []ContextBriefingSource {
	var related []ContextBriefingSource
	for _, match := range matches {
		if !match.Neighbor {
			continue
		}
		related = append(related, contextBriefingSource(match))
	}
	return related
}

func contextGaps(result ContextResult) []string {
	var gaps []string
	if len(result.Results) == 0 {
		gaps = append(gaps, "No source sections matched the query.")
	}
	if result.Budget > 0 && result.EstimatedTokens >= result.Budget && len(result.Results) > 0 {
		gaps = append(gaps, "The selected context filled the token budget, so lower-ranked matches may be omitted.")
	}
	if len(result.Issues) > 0 {
		gaps = append(gaps, fmt.Sprintf("The bundle has %d validation issue%s; run openknowledge validate for details.", len(result.Issues), pluralSuffix(len(result.Issues))))
	}
	return gaps
}

func contextBriefingSource(match ContextMatch) ContextBriefingSource {
	return ContextBriefingSource{
		Path:      match.Path,
		LineStart: match.LineStart,
		LineEnd:   match.LineEnd,
		Title:     match.Title,
		Heading:   match.Heading,
		Neighbor:  match.Neighbor,
	}
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
