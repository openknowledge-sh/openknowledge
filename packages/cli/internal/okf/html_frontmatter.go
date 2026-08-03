package okf

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
)

func renderPlainFrontmatter(data map[string]any, currentRel string) string {
	if len(data) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("<details>\n<summary>Metadata</summary>\n")
	writePlainFrontmatterMap(&builder, data, currentRel)
	builder.WriteString("</details>\n")
	return builder.String()
}

func writePlainFrontmatterMap(builder *strings.Builder, values map[string]any, currentRel string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder.WriteString("<dl>\n")
	for _, key := range keys {
		builder.WriteString("<dt><code>")
		builder.WriteString(html.EscapeString(key))
		builder.WriteString("</code></dt>\n<dd>")
		writePlainFrontmatterValue(builder, values[key], currentRel, key)
		builder.WriteString("</dd>\n")
	}
	builder.WriteString("</dl>\n")
}

func writePlainFrontmatterValue(builder *strings.Builder, value any, currentRel string, key string) {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		builder.WriteString(html.EscapeString(fmt.Sprint(typed)))
	case string:
		writePlainFrontmatterString(builder, typed, currentRel, key)
	case []any:
		builder.WriteString("<ol>\n")
		for _, item := range typed {
			builder.WriteString("<li")
			if strings.EqualFold(key, "sources") {
				if source, ok := item.(map[string]any); ok {
					if anchor := OKFV02SourceAnchor(okfV02String(source["id"])); anchor != "" {
						builder.WriteString(` id="`)
						builder.WriteString(html.EscapeString(anchor))
						builder.WriteString(`"`)
					}
				}
			}
			builder.WriteString(">")
			writePlainFrontmatterValue(builder, item, currentRel, key)
			builder.WriteString("</li>\n")
		}
		builder.WriteString("</ol>\n")
	case map[string]any:
		writePlainFrontmatterMap(builder, typed, currentRel)
	default:
		builder.WriteString(html.EscapeString(fmt.Sprint(typed)))
	}
}

func writePlainFrontmatterString(builder *strings.Builder, value string, currentRel string, key string) {
	value = strings.TrimSpace(value)
	if plainFrontmatterLinkKey(key) && value != "" {
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host == "" && !parsed.IsAbs() {
			writePlainFrontmatterLink(builder, StaticHTMLLink(currentRel, value), value)
			return
		}
		if err == nil && parsed.IsAbs() && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			writePlainFrontmatterLink(builder, value, value)
			return
		}
	}
	builder.WriteString(html.EscapeString(value))
}

func writePlainFrontmatterLink(builder *strings.Builder, href string, label string) {
	builder.WriteString(`<a href="`)
	builder.WriteString(html.EscapeString(href))
	builder.WriteString(`">`)
	builder.WriteString(html.EscapeString(label))
	builder.WriteString("</a>")
}

func plainFrontmatterLinkKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "resource", "computation":
		return true
	default:
		return false
	}
}
