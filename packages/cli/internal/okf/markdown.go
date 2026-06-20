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
	var listItem []string
	inCode := false
	codeLanguage := ""
	var codeLines []string
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
	flushListItem := func() {
		if len(listItem) == 0 {
			return
		}
		builder.WriteString("<li>")
		builder.WriteString(renderInline(strings.Join(listItem, " "), currentRel, resolve))
		builder.WriteString("</li>\n")
		listItem = nil
	}
	closeList := func() {
		if listTag == "" {
			return
		}
		flushListItem()
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
	startListItem := func(tag string, text string) {
		closeParagraph()
		if listTag == tag {
			flushListItem()
		}
		openList(tag)
		listItem = []string{text}
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
	closeCode := func() {
		if !inCode {
			return
		}
		builder.WriteString(RenderCodeBlock(strings.Join(codeLines, "\n"), codeLanguage))
		inCode = false
		codeLanguage = ""
		codeLines = nil
	}

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			closeParagraph()
			closeList()
			closeQuote()
			if inCode {
				closeCode()
			} else {
				inCode = true
				codeLanguage = codeFenceLanguage(trimmed)
				codeLines = nil
			}
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
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
			startListItem("ul", strings.TrimSpace(trimmed[2:]))
			continue
		}
		if match := orderedListItem.FindStringIndex(trimmed); match != nil {
			startListItem("ol", strings.TrimSpace(trimmed[match[1]:]))
			continue
		}
		if listTag != "" && len(listItem) > 0 && isListContinuation(line) {
			listItem = append(listItem, trimmed)
			continue
		}
		closeList()
		paragraph = append(paragraph, trimmed)
	}

	closeParagraph()
	closeList()
	closeQuote()
	if inCode {
		closeCode()
	}
	return builder.String()
}

func isListContinuation(line string) bool {
	return strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")
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
	for _, match := range inlineLink.FindAllStringSubmatchIndex(text, -1) {
		if insideInlineCodeSpan(text, match[0]) {
			continue
		}
		builder.WriteString(renderInlineText(text[last:match[0]]))
		label := text[match[2]:match[3]]
		href := text[match[4]:match[5]]
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
		builder.WriteString(renderEmphasis(part))
	}
	return builder.String()
}

func insideInlineCodeSpan(text string, offset int) bool {
	return strings.Count(text[:offset], "`")%2 == 1
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
	alignments := tableAlignments(separator)

	var builder strings.Builder
	builder.WriteString(`<div class="ok-table-wrap" data-ok-table-wrap>`)
	builder.WriteString("\n<div class=\"ok-table-scroller\">\n<table class=\"ok-table\" data-ok-table>\n<thead>\n<tr>")
	for column, cell := range header {
		writeTableCellStart(&builder, "th", alignments[column], ` scope="col"`)
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
			writeTableCellStart(&builder, "td", alignments[column], "")
			builder.WriteString(renderInline(cell, currentRel, resolve))
			builder.WriteString("</td>")
		}
		builder.WriteString("</tr>\n")
		index++
	}

	builder.WriteString("</tbody>\n</table>\n</div>\n</div>\n")
	return builder.String(), index, true
}

func writeTableCellStart(builder *strings.Builder, tag string, alignment string, attributes string) {
	builder.WriteString("<")
	builder.WriteString(tag)
	builder.WriteString(attributes)
	if alignment != "" {
		builder.WriteString(` data-align="`)
		builder.WriteString(alignment)
		builder.WriteString(`"`)
	}
	builder.WriteString(">")
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

func tableAlignments(separator []string) []string {
	alignments := make([]string, len(separator))
	for index, cell := range separator {
		cell = strings.TrimSpace(cell)
		left := strings.HasPrefix(cell, ":")
		right := strings.HasSuffix(cell, ":")
		switch {
		case left && right:
			alignments[index] = "center"
		case right:
			alignments[index] = "right"
		case left:
			alignments[index] = "left"
		}
	}
	return alignments
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

func RenderCodeBlock(content string, language string) string {
	language = NormalizeCodeLanguage(language)
	className := "code-block"
	if classLanguage := codeLanguageClass(language); classLanguage != "" {
		className += " language-" + classLanguage
	}
	label := language
	if label == "" {
		label = "code"
	}
	return `<pre class="` + className + `" data-language="` + html.EscapeString(label) + `"><code>` + highlightCode(content, language) + "</code></pre>\n"
}

func codeLanguageClass(language string) string {
	var builder strings.Builder
	for _, character := range language {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func CodeLanguageForPath(name string) string {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".go":
		return "go"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".json", ".jsonc":
		return "json"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".sql":
		return "sql"
	case ".yml", ".yaml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".xml", ".svg":
		return "xml"
	case ".md", ".markdown":
		return "markdown"
	default:
		return ""
	}
}

func NormalizeCodeLanguage(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language == "" {
		return ""
	}
	language = strings.Fields(language)[0]
	language = strings.Trim(language, "{}")
	language = strings.TrimPrefix(language, ".")
	switch language {
	case "js", "mjs", "cjs", "node":
		return "javascript"
	case "ts", "mts", "cts":
		return "typescript"
	case "py":
		return "python"
	case "bash", "zsh", "sh":
		return "shell"
	case "yml":
		return "yaml"
	case "md":
		return "markdown"
	case "htm":
		return "html"
	default:
		return language
	}
}

func codeFenceLanguage(fence string) string {
	return NormalizeCodeLanguage(strings.TrimSpace(strings.TrimPrefix(fence, "```")))
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

func highlightCode(content string, language string) string {
	language = NormalizeCodeLanguage(language)
	keywords := codeKeywords(language)
	if len(keywords) == 0 && !codeLanguageHasPrimitiveTokens(language) {
		return html.EscapeString(content)
	}

	lines := strings.Split(content, "\n")
	var builder strings.Builder
	for index, line := range lines {
		builder.WriteString(highlightCodeLine(line, language, keywords))
		if index < len(lines)-1 {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func highlightCodeLine(line string, language string, keywords map[string]bool) string {
	if language == "shell" {
		return highlightShellLine(line, keywords)
	}

	var builder strings.Builder
	for index := 0; index < len(line); {
		if codeLineCommentStart(line[index:], language) {
			builder.WriteString(codeToken("comment", line[index:]))
			break
		} else if strings.HasPrefix(line[index:], "/*") && codeSupportsBlockComments(language) {
			end := strings.Index(line[index+2:], "*/")
			if end < 0 {
				builder.WriteString(codeToken("comment", line[index:]))
				break
			}
			endIndex := index + 2 + end + 2
			builder.WriteString(codeToken("comment", line[index:endIndex]))
			index = endIndex
			continue
		}

		character := line[index]
		if codeQuote(character, language) {
			end := consumeCodeString(line, index, character)
			builder.WriteString(codeToken("string", line[index:end]))
			index = end
			continue
		}
		if isCodeDigit(character) {
			end := consumeCodeNumber(line, index)
			builder.WriteString(codeToken("number", line[index:end]))
			index = end
			continue
		}
		if isCodeIdentifierStart(character) {
			end := consumeCodeIdentifier(line, index)
			word := line[index:end]
			if keywords[word] {
				builder.WriteString(codeToken("keyword", word))
			} else {
				builder.WriteString(html.EscapeString(word))
			}
			index = end
			continue
		}
		builder.WriteString(html.EscapeString(line[index : index+1]))
		index++
	}
	return builder.String()
}

func highlightShellLine(line string, keywords map[string]bool) string {
	var builder strings.Builder
	expectCommand := true
	for index := 0; index < len(line); {
		if codeLineCommentStart(line[index:], "shell") {
			builder.WriteString(codeToken("comment", line[index:]))
			break
		}
		if isShellWhitespace(line[index]) {
			end := index + 1
			for end < len(line) && isShellWhitespace(line[end]) {
				end++
			}
			builder.WriteString(html.EscapeString(line[index:end]))
			index = end
			continue
		}
		if end, ok := consumeShellSeparator(line, index); ok {
			builder.WriteString(html.EscapeString(line[index:end]))
			index = end
			expectCommand = true
			continue
		}
		if end, ok := writeShellAssignmentToken(&builder, line, index); ok {
			index = end
			continue
		}

		character := line[index]
		if codeQuote(character, "shell") {
			end := consumeCodeString(line, index, character)
			builder.WriteString(codeToken("string", line[index:end]))
			index = end
			expectCommand = false
			continue
		}
		if end := consumeShellVariable(line, index); end > index {
			builder.WriteString(codeToken("variable", line[index:end]))
			index = end
			expectCommand = false
			continue
		}
		if end := consumeShellFlag(line, index); end > index {
			builder.WriteString(codeToken("flag", line[index:end]))
			index = end
			expectCommand = false
			continue
		}
		if isCodeDigit(character) {
			end := consumeCodeNumber(line, index)
			builder.WriteString(codeToken("number", line[index:end]))
			index = end
			expectCommand = false
			continue
		}

		end := consumeShellWord(line, index)
		if end == index {
			builder.WriteString(html.EscapeString(line[index : index+1]))
			index++
			continue
		}

		word := line[index:end]
		switch {
		case keywords[word]:
			builder.WriteString(codeToken("keyword", word))
		case expectCommand && shellLooksLikeCommand(word):
			builder.WriteString(codeToken("command", word))
		default:
			builder.WriteString(html.EscapeString(word))
		}
		index = end
		expectCommand = false
	}
	return builder.String()
}

func writeShellAssignmentToken(builder *strings.Builder, line string, start int) (int, bool) {
	nameEnd := shellAssignmentNameEnd(line, start)
	if nameEnd <= start {
		return start, false
	}
	builder.WriteString(codeToken("variable", line[start:nameEnd]))
	builder.WriteByte('=')
	index := nameEnd + 1
	for index < len(line) && !isShellTokenBoundary(line[index]) {
		character := line[index]
		if codeQuote(character, "shell") {
			end := consumeCodeString(line, index, character)
			builder.WriteString(codeToken("string", line[index:end]))
			index = end
			continue
		}
		if end := consumeShellVariable(line, index); end > index {
			builder.WriteString(codeToken("variable", line[index:end]))
			index = end
			continue
		}
		end := index + 1
		for end < len(line) && !isShellTokenBoundary(line[end]) && line[end] != '"' && line[end] != '\'' && line[end] != '`' && line[end] != '$' {
			end++
		}
		builder.WriteString(html.EscapeString(line[index:end]))
		index = end
	}
	return index, true
}

func shellAssignmentNameEnd(line string, start int) int {
	if start >= len(line) || !isShellNameStart(line[start]) {
		return -1
	}
	index := start + 1
	for index < len(line) && isShellNamePart(line[index]) {
		index++
	}
	if index < len(line) && line[index] == '=' {
		return index
	}
	return -1
}

func consumeShellVariable(line string, start int) int {
	if start >= len(line) || line[start] != '$' || start+1 >= len(line) {
		return start
	}
	if line[start+1] == '{' {
		index := start + 2
		if index >= len(line) || !isShellNameStart(line[index]) {
			return start
		}
		for index < len(line) && isShellNamePart(line[index]) {
			index++
		}
		if index < len(line) && line[index] == '}' {
			return index + 1
		}
		return start
	}
	if !isShellNameStart(line[start+1]) {
		return start
	}
	index := start + 2
	for index < len(line) && isShellNamePart(line[index]) {
		index++
	}
	return index
}

func consumeShellFlag(line string, start int) int {
	if start >= len(line) || line[start] != '-' || start+1 >= len(line) {
		return start
	}
	next := line[start+1]
	if next == '-' {
		if start+2 >= len(line) || isShellTokenBoundary(line[start+2]) {
			return start
		}
		return consumeShellWord(line, start)
	}
	if isShellFlagCharacter(next) {
		return consumeShellWord(line, start)
	}
	return start
}

func consumeShellWord(line string, start int) int {
	index := start
	for index < len(line) && !isShellTokenBoundary(line[index]) {
		switch line[index] {
		case '"', '\'', '`', '$':
			return index
		}
		index++
	}
	return index
}

func consumeShellSeparator(line string, start int) (int, bool) {
	if start >= len(line) {
		return start, false
	}
	if start+1 < len(line) {
		switch line[start : start+2] {
		case "&&", "||":
			return start + 2, true
		}
	}
	switch line[start] {
	case ';', '|':
		return start + 1, true
	default:
		return start, false
	}
}

func isShellTokenBoundary(character byte) bool {
	return isShellWhitespace(character) || character == ';' || character == '|'
}

func isShellWhitespace(character byte) bool {
	return character == ' ' || character == '\t'
}

func isShellNameStart(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_'
}

func isShellNamePart(character byte) bool {
	return isShellNameStart(character) || isCodeDigit(character)
}

func isShellFlagCharacter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || isCodeDigit(character)
}

func shellLooksLikeCommand(word string) bool {
	if word == "" || strings.HasPrefix(word, "-") {
		return false
	}
	for index := 0; index < len(word); index++ {
		character := word[index]
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || isCodeDigit(character) {
			return true
		}
	}
	return false
}

func codeLineCommentStart(line string, language string) bool {
	if strings.HasPrefix(line, "//") && codeSupportsSlashComments(language) {
		return true
	}
	if strings.HasPrefix(line, "#") && codeSupportsHashComments(language) {
		return true
	}
	if strings.HasPrefix(line, "--") && language == "sql" {
		return true
	}
	return false
}

func codeQuote(character byte, language string) bool {
	if character == '"' || character == '\'' {
		return true
	}
	if character != '`' {
		return false
	}
	switch language {
	case "go", "javascript", "jsx", "typescript", "tsx", "shell":
		return true
	default:
		return false
	}
}

func consumeCodeString(line string, start int, quote byte) int {
	for index := start + 1; index < len(line); index++ {
		if line[index] == '\\' {
			index++
			continue
		}
		if line[index] == quote {
			return index + 1
		}
	}
	return len(line)
}

func consumeCodeNumber(line string, start int) int {
	index := start + 1
	for index < len(line) {
		character := line[index]
		if isCodeDigit(character) || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F') || character == '.' || character == '_' || character == 'x' || character == 'X' || character == '+' || character == '-' {
			index++
			continue
		}
		break
	}
	return index
}

func consumeCodeIdentifier(line string, start int) int {
	index := start + 1
	for index < len(line) && isCodeIdentifierPart(line[index]) {
		index++
	}
	return index
}

func isCodeIdentifierStart(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_'
}

func isCodeIdentifierPart(character byte) bool {
	return isCodeIdentifierStart(character) || isCodeDigit(character) || character == '-'
}

func isCodeDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func codeToken(kind string, value string) string {
	return `<span class="tok-` + kind + `">` + html.EscapeString(value) + `</span>`
}

func codeLanguageHasPrimitiveTokens(language string) bool {
	switch language {
	case "go", "javascript", "jsx", "typescript", "tsx", "json", "html", "xml", "css", "shell", "python", "ruby", "rust", "java", "kotlin", "swift", "sql", "yaml", "toml":
		return true
	default:
		return false
	}
}

func codeSupportsSlashComments(language string) bool {
	switch language {
	case "go", "javascript", "jsx", "typescript", "tsx", "java", "kotlin", "swift", "rust":
		return true
	default:
		return false
	}
}

func codeSupportsHashComments(language string) bool {
	switch language {
	case "shell", "python", "ruby", "yaml":
		return true
	default:
		return false
	}
}

func codeSupportsBlockComments(language string) bool {
	switch language {
	case "go", "javascript", "jsx", "typescript", "tsx", "java", "kotlin", "swift", "rust", "css", "sql":
		return true
	default:
		return false
	}
}

func codeKeywords(language string) map[string]bool {
	words := []string{}
	switch language {
	case "go":
		words = []string{"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"}
	case "javascript", "jsx", "typescript", "tsx":
		words = []string{"async", "await", "break", "case", "catch", "class", "const", "continue", "default", "else", "export", "extends", "finally", "for", "from", "function", "if", "import", "in", "instanceof", "let", "new", "of", "return", "switch", "throw", "try", "typeof", "var", "void", "while", "yield", "true", "false", "null", "undefined", "type", "interface"}
	case "json":
		words = []string{"true", "false", "null"}
	case "shell":
		words = []string{"case", "do", "done", "elif", "else", "esac", "export", "fi", "for", "function", "if", "in", "local", "then", "while"}
	case "python":
		words = []string{"and", "as", "assert", "async", "await", "break", "class", "continue", "def", "elif", "else", "except", "false", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "none", "nonlocal", "not", "or", "pass", "raise", "return", "true", "try", "while", "with", "yield"}
	case "ruby":
		words = []string{"begin", "class", "def", "do", "else", "elsif", "end", "ensure", "false", "if", "module", "nil", "rescue", "return", "self", "true", "unless", "until", "when", "while", "yield"}
	case "rust":
		words = []string{"as", "async", "await", "break", "const", "continue", "crate", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while"}
	case "java", "kotlin":
		words = []string{"abstract", "break", "case", "catch", "class", "const", "continue", "data", "default", "do", "else", "enum", "extends", "false", "final", "finally", "for", "fun", "if", "implements", "import", "in", "interface", "new", "null", "object", "override", "package", "private", "protected", "public", "return", "static", "super", "switch", "this", "throw", "true", "try", "val", "var", "void", "when", "while"}
	case "swift":
		words = []string{"as", "associatedtype", "break", "case", "catch", "class", "continue", "defer", "do", "else", "enum", "extension", "false", "for", "func", "guard", "if", "import", "in", "let", "nil", "protocol", "return", "self", "static", "struct", "switch", "throw", "true", "try", "typealias", "var", "where", "while"}
	case "sql":
		words = []string{"and", "as", "by", "case", "create", "delete", "desc", "distinct", "drop", "else", "end", "from", "group", "having", "in", "insert", "into", "is", "join", "left", "like", "limit", "not", "null", "on", "or", "order", "outer", "right", "select", "set", "table", "then", "union", "update", "values", "when", "where"}
	}
	if len(words) == 0 {
		return nil
	}
	keywords := make(map[string]bool, len(words)*2)
	for _, word := range words {
		keywords[word] = true
		keywords[strings.ToUpper(word)] = true
	}
	return keywords
}

func htmlPath(markdownPath string) string {
	extension := filepath.Ext(markdownPath)
	if extension == "" {
		return filepath.ToSlash(filepath.Join(markdownPath, "index.html"))
	}
	return strings.TrimSuffix(markdownPath, extension) + ".html"
}
