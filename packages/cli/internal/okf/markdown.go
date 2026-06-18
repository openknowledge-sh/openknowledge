package okf

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type LinkResolver func(currentRel string, href string) string

func RenderMarkdown(body string, currentRel string, resolve LinkResolver) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var builder strings.Builder
	listKind := ""
	inCode := false
	var paragraph []string
	var blockquote []string

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
		if listKind == "" {
			return
		}
		builder.WriteString("</")
		builder.WriteString(listKind)
		builder.WriteString(">\n")
		listKind = ""
	}
	openList := func(kind string) {
		if listKind == kind {
			return
		}
		closeList()
		builder.WriteString("<")
		builder.WriteString(kind)
		builder.WriteString(">\n")
		listKind = kind
	}
	closeBlockquote := func() {
		if len(blockquote) == 0 {
			return
		}
		builder.WriteString("<blockquote>\n")
		builder.WriteString(RenderMarkdown(strings.Join(blockquote, "\n"), currentRel, resolve))
		builder.WriteString("</blockquote>\n")
		blockquote = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if quote, ok := blockquoteLine(trimmed); ok && !inCode {
			closeParagraph()
			closeList()
			blockquote = append(blockquote, quote)
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			closeParagraph()
			closeList()
			closeBlockquote()
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
			closeBlockquote()
			continue
		}
		if isThematicBreak(trimmed) {
			closeParagraph()
			closeList()
			closeBlockquote()
			builder.WriteString("<hr>\n")
			continue
		}
		if level := HeadingLevel(trimmed); level > 0 {
			closeParagraph()
			closeList()
			closeBlockquote()
			content := strings.TrimSpace(trimmed[level:])
			fmt.Fprintf(&builder, "<h%d>%s</h%d>\n", level, renderInline(content, currentRel, resolve), level)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			closeParagraph()
			closeBlockquote()
			openList("ul")
			builder.WriteString("<li>")
			builder.WriteString(renderInline(strings.TrimSpace(trimmed[2:]), currentRel, resolve))
			builder.WriteString("</li>\n")
			continue
		}
		if item, ok := orderedListItem(trimmed); ok {
			closeParagraph()
			closeBlockquote()
			openList("ol")
			builder.WriteString("<li>")
			builder.WriteString(renderInline(item, currentRel, resolve))
			builder.WriteString("</li>\n")
			continue
		}
		closeBlockquote()
		paragraph = append(paragraph, trimmed)
	}

	closeParagraph()
	closeList()
	closeBlockquote()
	if inCode {
		builder.WriteString("</code></pre>\n")
	}
	return builder.String()
}

func blockquoteLine(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, ">") {
		return "", false
	}
	content := strings.TrimPrefix(trimmed, ">")
	if strings.HasPrefix(content, " ") {
		content = strings.TrimPrefix(content, " ")
	}
	return content, true
}

func isThematicBreak(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	first := trimmed[0]
	if first != '-' && first != '*' && first != '_' {
		return false
	}
	for index := 0; index < len(trimmed); index++ {
		if trimmed[index] != first {
			return false
		}
	}
	return true
}

func orderedListItem(trimmed string) (string, bool) {
	index := 0
	for index < len(trimmed) && trimmed[index] >= '0' && trimmed[index] <= '9' {
		index++
	}
	if index == 0 || index+1 >= len(trimmed) || trimmed[index] != '.' || trimmed[index+1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[index+2:]), true
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

	var builder strings.Builder
	last := 0
	for _, match := range markdownLinkDetail.FindAllStringSubmatchIndex(text, -1) {
		builder.WriteString(renderInlineText(text[last:match[0]]))
		label := text[match[4]:match[5]]
		href := text[match[6]:match[7]]
		builder.WriteString(`<a href="`)
		builder.WriteString(html.EscapeString(resolve(currentRel, href)))
		builder.WriteString(`">`)
		builder.WriteString(renderInlineText(label))
		builder.WriteString("</a>")
		last = match[1]
	}
	builder.WriteString(renderInlineText(text[last:]))
	return builder.String()
}

func renderInlineText(text string) string {
	parts := strings.Split(text, "`")
	var builder strings.Builder
	for index, part := range parts {
		if index%2 == 1 {
			builder.WriteString("<code>")
			builder.WriteString(html.EscapeString(part))
			builder.WriteString("</code>")
			continue
		}
		builder.WriteString(renderInlineFormatting(part))
	}
	return builder.String()
}

type inlineFormat struct {
	start     int
	delimiter string
	openTag   string
	closeTag  string
}

func renderInlineFormatting(text string) string {
	var builder strings.Builder
	position := 0
	for position < len(text) {
		format := nextInlineFormat(text, position)
		if format.start < 0 {
			builder.WriteString(html.EscapeString(text[position:]))
			break
		}

		end := strings.Index(text[format.start+len(format.delimiter):], format.delimiter)
		if end < 0 {
			builder.WriteString(html.EscapeString(text[position:]))
			break
		}
		end += format.start + len(format.delimiter)

		builder.WriteString(html.EscapeString(text[position:format.start]))
		builder.WriteString(format.openTag)
		builder.WriteString(html.EscapeString(text[format.start+len(format.delimiter) : end]))
		builder.WriteString(format.closeTag)
		position = end + len(format.delimiter)
	}
	return builder.String()
}

func nextInlineFormat(text string, position int) inlineFormat {
	formats := []inlineFormat{
		{delimiter: "**", openTag: "<strong>", closeTag: "</strong>"},
		{delimiter: "__", openTag: "<strong>", closeTag: "</strong>"},
		{delimiter: "*", openTag: "<em>", closeTag: "</em>"},
		{delimiter: "_", openTag: "<em>", closeTag: "</em>"},
	}
	next := inlineFormat{start: -1}
	for _, format := range formats {
		index := strings.Index(text[position:], format.delimiter)
		if index < 0 {
			continue
		}
		format.start = position + index
		if next.start < 0 || format.start < next.start || (format.start == next.start && len(format.delimiter) > len(next.delimiter)) {
			next = format
		}
	}
	return next
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
