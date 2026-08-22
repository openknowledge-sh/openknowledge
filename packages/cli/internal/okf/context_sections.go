package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"path/filepath"
	"strings"
)

type contextSectionBoundary struct {
	start       int
	level       int
	title       string
	anchor      string
	line        int
	headingPath []string
}

func splitContextSectionsFromASTDocument(entry ListEntry, document ASTDocument) []ContextSection {
	bodyLine := document.Frontmatter.BodyLine
	if bodyLine <= 0 {
		bodyLine = 1
	}

	boundaries := contextSectionBoundaries(document.Markdown.Sections, bodyLine)
	readerBody := astMarkdownReaderBody(document.Body, bodyLine, document.Markdown.Blocks)
	sections := contextSectionsFromBoundaries(entry, document.Frontmatter.Values, document.Frontmatter.Data, readerBody, document.Links, bodyLine, boundaries)
	attachContextSectionAnchors(sections, boundaries, document.Markdown.Headings, document.Markdown.ExplicitIDs)
	return sections
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
						anchor:      section.Anchor,
						line:        section.LineStart,
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

func contextSectionsFromBoundaries(entry ListEntry, frontmatter map[string]string, frontmatterData map[string]any, body string, links []Link, bodyLine int, boundaries []contextSectionBoundary) []ContextSection {
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
		return []ContextSection{newContextSection(entry, frontmatter, frontmatterData, "#top", "Top", nil, []string{"top"}, 0, bodyLine, bodyLine+len(lines)-1, text, links)}
	}

	var sections []ContextSection
	if top := strings.TrimSpace(strings.Join(lines[:boundaries[0].start], "\n")); top != "" {
		sections = append(sections, newContextSection(entry, frontmatter, frontmatterData, "#top", "Top", nil, []string{"top"}, 0, bodyLine, bodyLine+boundaries[0].start-1, top, linksInRange(links, bodyLine, bodyLine+boundaries[0].start-1)))
	}

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
		anchor := current.anchor
		if anchor == "" {
			anchor = "section"
		}
		id := entry.ID + "#" + anchor
		sections = append(sections, newContextSection(entry, frontmatter, frontmatterData, id, current.title, current.headingPath, []string{anchor}, current.level, lineStart, lineEnd, text, linksInRange(links, lineStart, lineEnd)))
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

func newContextSection(entry ListEntry, frontmatter map[string]string, frontmatterData map[string]any, id string, heading string, headingPath []string, anchors []string, level int, lineStart int, lineEnd int, text string, links []Link) ContextSection {
	if id == "#top" {
		id = entry.ID + "#top"
	}
	if len(headingPath) == 0 && heading != "" && heading != "Top" {
		headingPath = []string{heading}
	}
	return ContextSection{
		ID:              id,
		ContentSHA256:   sectionContentSHA256(text),
		Path:            entry.Path,
		Kind:            entry.Kind,
		Type:            entry.Type,
		Title:           entry.Title,
		Description:     entry.Description,
		Frontmatter:     frontmatter,
		FrontmatterData: frontmatterData,
		Heading:         heading,
		HeadingPath:     append([]string{}, headingPath...),
		Anchors:         append([]string{}, anchors...),
		HeadingLevel:    level,
		LineStart:       lineStart,
		LineEnd:         lineEnd,
		Text:            text,
		Links:           links,
		EstimatedTokens: estimateContextTokens(text),
	}
}

func attachContextSectionAnchors(sections []ContextSection, boundaries []contextSectionBoundary, headings []ASTMarkdownHeading, explicitIDs []ASTMarkdownExplicitID) {
	if len(sections) == 0 {
		return
	}
	add := func(index int, anchor string) {
		if anchor == "" {
			return
		}
		for _, existing := range sections[index].Anchors {
			if existing == anchor {
				return
			}
		}
		sections[index].Anchors = append(sections[index].Anchors, anchor)
	}

	// A conventional top fragment selects the first retrievable content even
	// when the document has no pre-heading chunk.
	add(0, "top")
	for _, explicit := range explicitIDs {
		assigned := false
		for _, boundary := range boundaries {
			if boundary.line <= explicit.Line || boundary.line-explicit.Line > 2 {
				continue
			}
			for index := range sections {
				if sections[index].LineStart >= boundary.line {
					add(index, explicit.ID)
					assigned = true
					break
				}
			}
			break
		}
		if assigned {
			continue
		}
		for index := range sections {
			if explicit.Line >= sections[index].LineStart && explicit.Line <= sections[index].LineEnd {
				add(index, explicit.ID)
				break
			}
		}
	}

	// Heading-only H1-H3 sections do not become noisy empty chunks. Preserve
	// their anchors by assigning them to the first content-bearing descendant.
	for _, boundary := range boundaries {
		found := false
		for _, section := range sections {
			for _, anchor := range section.Anchors {
				if anchor == boundary.anchor {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			continue
		}
		for index := range sections {
			section := sections[index]
			if section.LineStart <= boundary.line || section.HeadingLevel <= boundary.level || !headingPathHasPrefix(section.HeadingPath, boundary.headingPath) {
				continue
			}
			add(index, boundary.anchor)
			break
		}
	}

	// Lower-level headings remain inside their surrounding H1-H3 retrieval
	// chunk. Make their canonical anchors resolve to that owning chunk.
	for _, heading := range headings {
		if heading.Level <= 3 {
			continue
		}
		for index := range sections {
			if heading.Line >= sections[index].LineStart && heading.Line <= sections[index].LineEnd {
				add(index, heading.Anchor)
				break
			}
		}
	}
}

func headingPathHasPrefix(path []string, prefix []string) bool {
	if len(prefix) == 0 || len(path) < len(prefix) {
		return false
	}
	for index := range prefix {
		if path[index] != prefix[index] {
			return false
		}
	}
	return true
}

func sectionContentSHA256(text string) string {
	digest := sha256.Sum256([]byte(strings.ReplaceAll(text, "\r\n", "\n")))
	return hex.EncodeToString(digest[:])
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

func estimateContextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return int(math.Ceil(float64(len([]rune(text))) / 4.0))
}

type contextSectionLookup struct {
	byID        map[string]ContextSection
	firstByPath map[string]ContextSection
	byPath      map[string][]ContextSection
}

func newContextSectionLookup(sections []ContextSection) contextSectionLookup {
	lookup := contextSectionLookup{
		byID:        make(map[string]ContextSection, len(sections)),
		firstByPath: map[string]ContextSection{},
		byPath:      map[string][]ContextSection{},
	}
	for _, section := range sections {
		lookup.byID[section.ID] = section
		lookup.byPath[section.Path] = append(lookup.byPath[section.Path], section)
		if existing, ok := lookup.firstByPath[section.Path]; !ok || section.LineStart < existing.LineStart {
			lookup.firstByPath[section.Path] = section
		}
	}
	return lookup
}

func (lookup contextSectionLookup) target(link Link) (ContextSection, bool) {
	if (link.Kind != "local" && link.Kind != "anchor") || !link.Exists || link.TargetPath == "" {
		return ContextSection{}, false
	}
	targetPath := filepath.ToSlash(filepath.Clean(link.TargetPath))
	sections := lookup.byPath[targetPath]
	if len(sections) == 0 {
		targetPath = filepath.ToSlash(filepath.Join(targetPath, "index.md"))
		sections = lookup.byPath[targetPath]
	}
	if len(sections) == 0 {
		return ContextSection{}, false
	}
	if link.TargetAnchor == "" {
		target, ok := lookup.firstByPath[targetPath]
		return target, ok
	}
	for _, section := range sections {
		for _, anchor := range section.Anchors {
			if anchor == link.TargetAnchor {
				return section, true
			}
		}
	}
	return ContextSection{}, false
}
