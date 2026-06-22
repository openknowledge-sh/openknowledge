package okf

import (
	"strconv"
	"strings"
)

type astMarkdownFenceState struct {
	marker byte
	length int
	start  int
	info   string
	lines  []string
}

func ParseASTMarkdown(body string, bodyLine int) ASTMarkdown {
	if bodyLine <= 0 {
		bodyLine = 1
	}
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	markdown := ASTMarkdown{}
	usedAnchors := map[string]int{}

	var fence *astMarkdownFenceState
	paragraphStart := -1
	var paragraphLines []string
	flushParagraph := func(endIndex int) {
		if paragraphStart < 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(paragraphLines, "\n"))
		if text != "" {
			lineStart := bodyLine + paragraphStart
			lineEnd := bodyLine + endIndex
			block := ASTMarkdownBlock{
				Kind:      "paragraph",
				LineStart: lineStart,
				LineEnd:   lineEnd,
				Text:      text,
				Links:     parseASTMarkdownLinks(text, lineStart),
			}
			markdown.Blocks = append(markdown.Blocks, block)
			markdown.Links = append(markdown.Links, block.Links...)
		}
		paragraphStart = -1
		paragraphLines = nil
	}

	for index, line := range lines {
		lineNumber := bodyLine + index
		trimmed := strings.TrimSpace(line)

		if marker, length, ok := markdownFenceMarker(trimmed); ok {
			if fence == nil {
				flushParagraph(index - 1)
				fence = &astMarkdownFenceState{
					marker: marker,
					length: length,
					start:  index,
					info:   strings.TrimSpace(trimmed[length:]),
				}
				continue
			}
			if marker == fence.marker && length >= fence.length {
				codeBlock := astMarkdownCodeBlock(fence, bodyLine, lineNumber)
				markdown.CodeBlocks = append(markdown.CodeBlocks, codeBlock)
				markdown.Blocks = append(markdown.Blocks, ASTMarkdownBlock{
					Kind:      "code",
					LineStart: codeBlock.LineStart,
					LineEnd:   codeBlock.LineEnd,
					Text:      codeBlock.Text,
					CodeBlock: &codeBlock,
				})
				fence = nil
				continue
			}
		}

		if fence != nil {
			fence.lines = append(fence.lines, line)
			continue
		}

		if trimmed == "" {
			flushParagraph(index - 1)
			continue
		}

		if level := HeadingLevel(trimmed); level > 0 {
			flushParagraph(index - 1)
			text := strings.TrimSpace(trimmed[level:])
			heading := ASTMarkdownHeading{
				Level:  level,
				Text:   text,
				Anchor: astMarkdownAnchor(text, usedAnchors),
				Line:   lineNumber,
			}
			links := parseASTMarkdownLinks(text, lineNumber)
			markdown.Headings = append(markdown.Headings, heading)
			markdown.Links = append(markdown.Links, links...)
			markdown.Blocks = append(markdown.Blocks, ASTMarkdownBlock{
				Kind:      "heading",
				LineStart: lineNumber,
				LineEnd:   lineNumber,
				Text:      text,
				Heading:   &heading,
				Links:     links,
			})
			continue
		}

		if paragraphStart < 0 {
			paragraphStart = index
		}
		paragraphLines = append(paragraphLines, line)
	}

	if fence != nil {
		codeBlock := astMarkdownCodeBlock(fence, bodyLine, bodyLine+len(lines)-1)
		markdown.CodeBlocks = append(markdown.CodeBlocks, codeBlock)
		markdown.Blocks = append(markdown.Blocks, ASTMarkdownBlock{
			Kind:      "code",
			LineStart: codeBlock.LineStart,
			LineEnd:   codeBlock.LineEnd,
			Text:      codeBlock.Text,
			CodeBlock: &codeBlock,
		})
	}
	flushParagraph(len(lines) - 1)
	return markdown
}

func astMarkdownCodeBlock(fence *astMarkdownFenceState, bodyLine int, lineEnd int) ASTMarkdownCodeBlock {
	info := strings.TrimSpace(fence.info)
	language := ""
	if fields := strings.Fields(info); len(fields) > 0 {
		language = strings.ToLower(fields[0])
	}
	return ASTMarkdownCodeBlock{
		Info:      info,
		Language:  language,
		Text:      strings.Join(fence.lines, "\n"),
		LineStart: bodyLine + fence.start,
		LineEnd:   lineEnd,
		Mermaid:   language == "mermaid",
	}
}

func astMarkdownAnchor(text string, used map[string]int) string {
	slug := strings.ReplaceAll(normalizeSearchText(text), " ", "-")
	if slug == "" {
		slug = "section"
	}
	used[slug]++
	if used[slug] == 1 {
		return slug
	}
	return slug + "-" + strconv.Itoa(used[slug])
}

func parseASTMarkdownLinks(text string, line int) []ASTMarkdownLink {
	var links []ASTMarkdownLink
	for _, match := range markdownLinkDetail.FindAllStringSubmatchIndex(text, -1) {
		label := strings.TrimSpace(text[match[4]:match[5]])
		href := strings.TrimSpace(text[match[6]:match[7]])
		image := match[2] >= 0 && strings.TrimSpace(text[match[2]:match[3]]) == "!"
		links = append(links, ASTMarkdownLink{
			Label: label,
			Href:  href,
			Kind:  linkKind(href),
			Line:  line,
			Image: image,
		})
	}
	return links
}
