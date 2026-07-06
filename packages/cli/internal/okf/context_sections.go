package okf

import (
	"math"
	"strconv"
	"strings"
)

type contextSectionBoundary struct {
	start       int
	level       int
	title       string
	headingPath []string
}

func splitContextSectionsFromASTDocument(entry ListEntry, document ASTDocument) []ContextSection {
	bodyLine := document.Frontmatter.BodyLine
	if bodyLine <= 0 {
		bodyLine = 1
	}

	boundaries := contextSectionBoundaries(document.Markdown.Sections, bodyLine)
	return contextSectionsFromBoundaries(entry, document.Frontmatter.Values, document.Body, document.Links, bodyLine, boundaries)
}

func contextSectionBoundaries(sections []ASTMarkdownSection, bodyLine int) []contextSectionBoundary {
	var boundaries []contextSectionBoundary
	var walk func([]ASTMarkdownSection, []string)
	walk = func(nodes []ASTMarkdownSection, parents []string) {
		for _, section := range nodes {
			// Keep the human heading trail on each chunk; search and graph
			// output use this as navigational context without reparsing text.
			path := append(append([]string{}, parents...), section.Heading)
			if section.Level > 0 && section.Level <= 3 {
				start := section.LineStart - bodyLine
				if start >= 0 {
					boundaries = append(boundaries, contextSectionBoundary{
						start:       start,
						level:       section.Level,
						title:       section.Heading,
						headingPath: path,
					})
				}
			}
			walk(section.Children, path)
		}
	}
	walk(sections, nil)
	return boundaries
}

func contextSectionsFromBoundaries(entry ListEntry, frontmatter map[string]string, body string, links []Link, bodyLine int, boundaries []contextSectionBoundary) []ContextSection {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if bodyLine <= 0 {
		bodyLine = 1
	}

	if len(boundaries) == 0 {
		text := strings.TrimSpace(normalized)
		if text == "" {
			return nil
		}
		return []ContextSection{newContextSection(entry, frontmatter, "#top", "Top", nil, 0, bodyLine, bodyLine+len(lines)-1, text, links)}
	}

	var sections []ContextSection
	if top := strings.TrimSpace(strings.Join(lines[:boundaries[0].start], "\n")); top != "" {
		sections = append(sections, newContextSection(entry, frontmatter, "#top", "Top", nil, 0, bodyLine, bodyLine+boundaries[0].start-1, top, linksInRange(links, bodyLine, bodyLine+boundaries[0].start-1)))
	}

	usedIDs := map[string]int{}
	for index, current := range boundaries {
		end := len(lines) - 1
		if index+1 < len(boundaries) {
			end = boundaries[index+1].start - 1
		}
		text := strings.TrimSpace(strings.Join(lines[current.start:end+1], "\n"))
		if text == "" || !hasContextSectionContent(text) {
			continue
		}
		lineStart := bodyLine + current.start
		lineEnd := bodyLine + end
		id := contextSectionID(entry.ID, current.title, usedIDs)
		sections = append(sections, newContextSection(entry, frontmatter, id, current.title, current.headingPath, current.level, lineStart, lineEnd, text, linksInRange(links, lineStart, lineEnd)))
	}
	return sections
}

func hasContextSectionContent(text string) bool {
	// Heading-only parent sections add noise to ranked retrieval and graph
	// chunk output. Keep sections only when they contain usable source content.
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return true
	}
	return false
}

func newContextSection(entry ListEntry, frontmatter map[string]string, id string, heading string, headingPath []string, level int, lineStart int, lineEnd int, text string, links []Link) ContextSection {
	if id == "#top" {
		id = entry.ID + "#top"
	}
	if len(headingPath) == 0 && heading != "" && heading != "Top" {
		headingPath = []string{heading}
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
		HeadingPath:     append([]string{}, headingPath...),
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
