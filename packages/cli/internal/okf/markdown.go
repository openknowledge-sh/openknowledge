package okf

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var inlineLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var orderedListItem = regexp.MustCompile(`^\d+[\.)]\s+`)

type LinkResolver func(currentRel string, href string) string

func RenderMarkdown(body string, currentRel string, resolve LinkResolver) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var builder strings.Builder
	listTag := ""
	inCode := false
	var paragraph []string
	var quote []string

	closeParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		builder.WriteString("<p>")
		builder.WriteString(renderInline(strings.Join(paragraph, " "), currentRel, resolve))
		builder.WriteString("</p>\n")
		paragraph = nil
	}
	closeList := func() {
		if listTag == "" {
			return
		}
		fmt.Fprintf(&builder, "</%s>\n", listTag)
		listTag = ""
	}
	openList := func(tag string) {
		if listTag == tag {
			return
		}
		closeList()
		fmt.Fprintf(&builder, "<%s>\n", tag)
		listTag = tag
	}
	closeQuote := func() {
		if len(quote) == 0 {
			return
		}
		builder.WriteString("<blockquote>\n")
		builder.WriteString(RenderMarkdown(strings.Join(quote, "\n"), currentRel, resolve))
		builder.WriteString("</blockquote>\n")
		quote = nil
	}

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			closeParagraph()
			closeList()
			closeQuote()
			if inCode {
				builder.WriteString("</code></pre>\n")
				inCode = false
			} else {
				builder.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			builder.WriteString(html.EscapeString(line))
			builder.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			closeParagraph()
			closeList()
			closeQuote()
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			closeParagraph()
			closeList()
			quote = append(quote, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))
			continue
		}
		closeQuote()
		if isHorizontalRule(trimmed) {
			closeParagraph()
			closeList()
			builder.WriteString("<hr>\n")
			continue
		}
		if tableHTML, next, ok := renderTable(lines, index, currentRel, resolve); ok {
			closeParagraph()
			closeList()
			builder.WriteString(tableHTML)
			index = next - 1
			continue
		}
		if level := HeadingLevel(trimmed); level > 0 {
			closeParagraph()
			closeList()
			content := strings.TrimSpace(trimmed[level:])
			fmt.Fprintf(&builder, "<h%d>%s</h%d>\n", level, renderInline(content, currentRel, resolve), level)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			closeParagraph()
			openList("ul")
			builder.WriteString("<li>")
			builder.WriteString(renderInline(strings.TrimSpace(trimmed[2:]), currentRel, resolve))
			builder.WriteString("</li>\n")
			continue
		}
		if match := orderedListItem.FindStringIndex(trimmed); match != nil {
			closeParagraph()
			openList("ol")
			builder.WriteString("<li>")
			builder.WriteString(renderInline(strings.TrimSpace(trimmed[match[1]:]), currentRel, resolve))
			builder.WriteString("</li>\n")
			continue
		}
		closeList()
		paragraph = append(paragraph, trimmed)
	}

	closeParagraph()
	closeList()
	closeQuote()
	if inCode {
		builder.WriteString("</code></pre>\n")
	}
	return builder.String()
}

func HeadingLevel(line string) int {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0
	}
	return level
}

func renderInline(text string, currentRel string, resolve LinkResolver) string {
	if resolve == nil {
		resolve = func(_ string, href string) string { return href }
	}

	parts := strings.Split(text, "`")
	var builder strings.Builder
	for index, part := range parts {
		if index%2 == 1 {
			builder.WriteString("<code>")
			builder.WriteString(html.EscapeString(part))
			builder.WriteString("</code>")
			continue
		}
		builder.WriteString(renderInlineMarkup(part, currentRel, resolve))
	}
	return builder.String()
}

func renderInlineMarkup(text string, currentRel string, resolve LinkResolver) string {
	var builder strings.Builder
	last := 0
	for _, match := range inlineLink.FindAllStringSubmatchIndex(text, -1) {
		builder.WriteString(renderEmphasis(text[last:match[0]]))
		label := text[match[2]:match[3]]
		href := text[match[4]:match[5]]
		builder.WriteString(`<a href="`)
		builder.WriteString(html.EscapeString(resolve(currentRel, href)))
		builder.WriteString(`">`)
		builder.WriteString(renderEmphasis(label))
		builder.WriteString("</a>")
		last = match[1]
	}
	builder.WriteString(renderEmphasis(text[last:]))
	return builder.String()
}

func renderEmphasis(text string) string {
	var builder strings.Builder
	for len(text) > 0 {
		if strings.HasPrefix(text, "**") {
			if end := strings.Index(text[2:], "**"); end >= 0 {
				builder.WriteString("<strong>")
				builder.WriteString(html.EscapeString(text[2 : 2+end]))
				builder.WriteString("</strong>")
				text = text[2+end+2:]
				continue
			}
		}
		if strings.HasPrefix(text, "*") {
			if end := strings.Index(text[1:], "*"); end >= 0 {
				builder.WriteString("<em>")
				builder.WriteString(html.EscapeString(text[1 : 1+end]))
				builder.WriteString("</em>")
				text = text[1+end+1:]
				continue
			}
		}

		next := strings.Index(text, "*")
		if next < 0 {
			builder.WriteString(html.EscapeString(text))
			break
		}
		if next > 0 {
			builder.WriteString(html.EscapeString(text[:next]))
			text = text[next:]
			continue
		}
		builder.WriteString(html.EscapeString(text[:1]))
		text = text[1:]
	}
	return builder.String()
}

func isHorizontalRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	first := line[0]
	if first != '-' && first != '*' && first != '_' {
		return false
	}
	for _, r := range line {
		if byte(r) != first && r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

func renderTable(lines []string, start int, currentRel string, resolve LinkResolver) (string, int, bool) {
	if start+1 >= len(lines) {
		return "", start, false
	}
	header := tableCells(lines[start])
	separator := tableCells(lines[start+1])
	if len(header) == 0 || len(separator) != len(header) || !isTableSeparator(separator) {
		return "", start, false
	}

	var builder strings.Builder
	builder.WriteString("<table>\n<thead>\n<tr>")
	for _, cell := range header {
		builder.WriteString("<th>")
		builder.WriteString(renderInline(cell, currentRel, resolve))
		builder.WriteString("</th>")
	}
	builder.WriteString("</tr>\n</thead>\n<tbody>\n")

	index := start + 2
	for index < len(lines) {
		cells := tableCells(lines[index])
		if len(cells) == 0 {
			break
		}
		builder.WriteString("<tr>")
		for column := range header {
			cell := ""
			if column < len(cells) {
				cell = cells[column]
			}
			builder.WriteString("<td>")
			builder.WriteString(renderInline(cell, currentRel, resolve))
			builder.WriteString("</td>")
		}
		builder.WriteString("</tr>\n")
		index++
	}

	builder.WriteString("</tbody>\n</table>\n")
	return builder.String(), index, true
}

func tableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.Contains(trimmed, "|") {
		return nil
	}
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func isTableSeparator(cells []string) bool {
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		cell = strings.Trim(cell, ":")
		if len(cell) < 3 {
			return false
		}
		for _, r := range cell {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func ViewerLink(currentRel string, href string) string {
	return rewriteMarkdownLink(currentRel, href, func(target string) string {
		return "/file/" + strings.TrimPrefix(path.Clean("/"+target), "/")
	})
}

func StaticHTMLLink(currentRel string, href string) string {
	return rewriteMarkdownLink(currentRel, href, func(target string) string {
		currentHTML := htmlPath(currentRel)
		targetHTML := htmlPath(target)
		relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(targetHTML))
		if err != nil {
			return filepath.ToSlash(targetHTML)
		}
		return filepath.ToSlash(relative)
	})
}

func rewriteMarkdownLink(currentRel string, href string, targetURL func(target string) string) string {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return "#"
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return trimmed
		}
		return "#"
	}
	if strings.HasPrefix(trimmed, "#") {
		return trimmed
	}

	linkPath := trimmed
	fragment := ""
	if hash := strings.Index(linkPath, "#"); hash >= 0 {
		fragment = linkPath[hash:]
		linkPath = linkPath[:hash]
	}

	target := linkTargetRel(currentRel, linkPath)
	if target == "" {
		return fragment
	}
	if isMarkdown(target) {
		return targetURL(target) + fragment
	}
	return trimmed
}

func htmlPath(markdownPath string) string {
	extension := filepath.Ext(markdownPath)
	if extension == "" {
		return filepath.ToSlash(filepath.Join(markdownPath, "index.html"))
	}
	return strings.TrimSuffix(markdownPath, extension) + ".html"
}
