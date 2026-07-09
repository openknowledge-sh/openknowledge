package main

import (
	"fmt"
	"html"
	"html/template"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func viewerFrontmatterHTMLForFile(root string, file okf.BundleFile) (template.HTML, error) {
	if len(file.Frontmatter) == 0 {
		return "", nil
	}

	filePath, ok := safeViewerPath(root, file.Path)
	if !ok {
		return "", fmt.Errorf("invalid frontmatter path: %s", file.Path)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	document, parseErr := okf.ParseFrontmatterDocument(content)
	data := document.Data
	fallback := false
	if parseErr != nil || len(data) == 0 {
		data = viewerFrontmatterScalarData(document.Values)
		if len(data) == 0 {
			data = viewerFrontmatterScalarData(file.Frontmatter)
		}
		fallback = parseErr != nil
	}
	if !document.Has && len(data) == 0 {
		return "", nil
	}

	order := viewerFrontmatterTopLevelOrder(string(content), data)
	return renderViewerFrontmatter(data, order, fallback), nil
}

func viewerFrontmatterHTMLByPath(root string, files []okf.BundleFile) (map[string]template.HTML, error) {
	result := make(map[string]template.HTML)
	for _, file := range files {
		frontmatter, err := viewerFrontmatterHTMLForFile(root, file)
		if err != nil {
			return nil, err
		}
		if frontmatter != "" {
			result[file.Path] = frontmatter
		}
	}
	return result, nil
}

func viewerFrontmatterScalarData(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	data := make(map[string]any, len(values))
	for key, value := range values {
		data[key] = value
	}
	return data
}

func viewerFrontmatterTopLevelOrder(content string, data map[string]any) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}

	seen := make(map[string]struct{}, len(data))
	order := make([]string, 0, len(data))
	for _, raw := range lines[1:] {
		if strings.TrimSpace(raw) == "---" {
			break
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") || strings.HasPrefix(trimmed, "- ") {
			continue
		}
		colon := strings.Index(raw, ":")
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(raw[:colon])
		if _, exists := data[key]; !exists {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		order = append(order, key)
	}
	return order
}

func renderViewerFrontmatter(data map[string]any, order []string, fallback bool) template.HTML {
	if len(data) == 0 {
		return ""
	}

	var builder strings.Builder
	count := len(data)
	fmt.Fprintf(&builder, `<details class="ok-frontmatter" data-frontmatter><summary class="ok-frontmatter-summary"><span class="ok-frontmatter-title">Frontmatter</span><span class="ok-frontmatter-count">%d %s</span></summary><div class="ok-frontmatter-body">`, count, viewerFrontmatterNoun(count, "field", "fields"))
	if fallback {
		builder.WriteString(`<p class="ok-frontmatter-notice">Structured preview is unavailable for this YAML subset; showing compatible scalar values.</p>`)
	}
	writeViewerFrontmatterMap(&builder, data, order, 0)
	builder.WriteString(`</div></details>`)
	return template.HTML(builder.String())
}

func viewerFrontmatterNoun(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func writeViewerFrontmatterMap(builder *strings.Builder, values map[string]any, preferredOrder []string, depth int) {
	builder.WriteString(`<dl class="ok-frontmatter-map">`)
	for _, key := range viewerFrontmatterKeys(values, preferredOrder) {
		value := values[key]
		valueType := viewerFrontmatterType(value)
		builder.WriteString(`<div class="ok-frontmatter-row">`)
		fmt.Fprintf(builder, `<dt class="ok-frontmatter-key"><code>%s</code></dt>`, html.EscapeString(key))
		fmt.Fprintf(builder, `<dd class="ok-frontmatter-value" data-frontmatter-type="%s">`, valueType)
		if depth != 0 || !strings.EqualFold(key, "tags") || !writeViewerFrontmatterTags(builder, value, depth+1) {
			writeViewerFrontmatterValue(builder, value, depth+1)
		}
		builder.WriteString(`</dd></div>`)
	}
	builder.WriteString(`</dl>`)
}

func writeViewerFrontmatterTags(builder *strings.Builder, value any, depth int) bool {
	values, ok := value.([]any)
	if !ok || !viewerFrontmatterArrayIsScalar(values) {
		return false
	}
	if len(values) == 0 {
		return false
	}

	builder.WriteString(`<ul class="ok-frontmatter-chips ok-frontmatter-tags" role="list">`)
	for _, item := range values {
		tag, isTag := item.(string)
		tag = strings.TrimSpace(tag)
		if isTag && tag != "" {
			fmt.Fprintf(builder, `<li class="ok-frontmatter-chip ok-frontmatter-tag"><a class="ok-frontmatter-tag-link" href="?ok-tag=%s" data-frontmatter-tag="%s" data-direct-link="true">%s</a></li>`, url.QueryEscape(tag), html.EscapeString(tag), html.EscapeString(tag))
			continue
		}
		fmt.Fprintf(builder, `<li class="ok-frontmatter-chip" data-frontmatter-type="%s">`, viewerFrontmatterType(item))
		writeViewerFrontmatterValue(builder, item, depth+1)
		builder.WriteString(`</li>`)
	}
	builder.WriteString(`</ul>`)
	return true
}

func viewerTagsForFile(root string, file okf.BundleFile) ([]string, error) {
	filePath, ok := safeViewerPath(root, file.Path)
	if !ok {
		return nil, fmt.Errorf("invalid frontmatter path: %s", file.Path)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	document, err := okf.ParseFrontmatterDocument(content)
	if err != nil {
		return nil, nil
	}
	return viewerTagValues(document.Data["tags"]), nil
}

func viewerTagValues(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, item := range values {
		tag, ok := item.(string)
		tag = strings.TrimSpace(tag)
		if !ok || tag == "" {
			continue
		}
		normalized := strings.ToLower(tag)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func viewerFrontmatterKeys(values map[string]any, preferredOrder []string) []string {
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, key := range preferredOrder {
		if _, exists := values[key]; !exists {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	rest := make([]string, 0, len(values)-len(keys))
	for key := range values {
		if _, exists := seen[key]; !exists {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func viewerFrontmatterType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "string"
	}
}

func writeViewerFrontmatterValue(builder *strings.Builder, value any, depth int) {
	switch typed := value.(type) {
	case nil:
		builder.WriteString(`<span class="ok-frontmatter-scalar ok-frontmatter-null">null</span>`)
	case bool:
		fmt.Fprintf(builder, `<span class="ok-frontmatter-scalar ok-frontmatter-boolean" data-value="%t"><span class="ok-frontmatter-boolean-dot" aria-hidden="true"></span>%t</span>`, typed, typed)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		fmt.Fprintf(builder, `<span class="ok-frontmatter-scalar ok-frontmatter-number">%s</span>`, html.EscapeString(fmt.Sprint(typed)))
	case []any:
		writeViewerFrontmatterArray(builder, typed, depth)
	case map[string]any:
		if len(typed) == 0 {
			builder.WriteString(`<span class="ok-frontmatter-scalar ok-frontmatter-empty">{}</span>`)
			return
		}
		writeViewerFrontmatterMap(builder, typed, nil, depth)
	default:
		writeViewerFrontmatterString(builder, fmt.Sprint(typed))
	}
}

func writeViewerFrontmatterArray(builder *strings.Builder, values []any, depth int) {
	if len(values) == 0 {
		builder.WriteString(`<span class="ok-frontmatter-scalar ok-frontmatter-empty">[]</span>`)
		return
	}
	if viewerFrontmatterArrayIsScalar(values) {
		builder.WriteString(`<ul class="ok-frontmatter-chips" role="list">`)
		for _, value := range values {
			fmt.Fprintf(builder, `<li class="ok-frontmatter-chip" data-frontmatter-type="%s">`, viewerFrontmatterType(value))
			writeViewerFrontmatterValue(builder, value, depth+1)
			builder.WriteString(`</li>`)
		}
		builder.WriteString(`</ul>`)
		return
	}

	builder.WriteString(`<ol class="ok-frontmatter-list">`)
	for index, value := range values {
		fmt.Fprintf(builder, `<li class="ok-frontmatter-list-item"><span class="ok-frontmatter-index">%d</span><div class="ok-frontmatter-list-value" data-frontmatter-type="%s">`, index+1, viewerFrontmatterType(value))
		writeViewerFrontmatterValue(builder, value, depth+1)
		builder.WriteString(`</div></li>`)
	}
	builder.WriteString(`</ol>`)
}

func viewerFrontmatterArrayIsScalar(values []any) bool {
	for _, value := range values {
		switch value.(type) {
		case []any, map[string]any:
			return false
		}
	}
	return true
}

func writeViewerFrontmatterString(builder *strings.Builder, value string) {
	escaped := html.EscapeString(value)
	if target := viewerFrontmatterURL(value); target != "" {
		fmt.Fprintf(builder, `<a class="ok-frontmatter-link" href="%s" target="_blank" rel="noreferrer" data-direct-link="true">%s</a>`, html.EscapeString(target), escaped)
		return
	}
	if viewerFrontmatterTimestamp(value) {
		fmt.Fprintf(builder, `<time class="ok-frontmatter-scalar ok-frontmatter-string" datetime="%s">%s</time>`, html.EscapeString(value), escaped)
		return
	}
	className := "ok-frontmatter-scalar ok-frontmatter-string"
	if strings.Contains(value, "\n") {
		className += " ok-frontmatter-multiline"
	}
	fmt.Fprintf(builder, `<span class="%s">%s</span>`, className, escaped)
}

func viewerFrontmatterURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}

func viewerFrontmatterTimestamp(value string) bool {
	trimmed := strings.TrimSpace(value)
	if _, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", trimmed)
	return err == nil
}
