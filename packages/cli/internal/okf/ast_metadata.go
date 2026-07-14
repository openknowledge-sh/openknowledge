package okf

import (
	"fmt"
	"strings"
)

func astDocumentMetadataFromFrontmatter(frontmatter ASTFrontmatter) ASTDocumentMetadata {
	return ASTDocumentMetadata{
		Type:        frontmatterString(frontmatter, "type"),
		Title:       frontmatterString(frontmatter, "title"),
		Description: frontmatterString(frontmatter, "description"),
		Resource:    frontmatterString(frontmatter, "resource"),
		Tags:        frontmatterStringList(frontmatter, "tags"),
		UseWhen:     frontmatterStringList(frontmatter, "use_when"),
		Bundle:      bundleMetadataFromFrontmatter(frontmatter),
	}
}

func frontmatterString(frontmatter ASTFrontmatter, key string) string {
	if value, exists := frontmatter.Data[key]; exists {
		if typed, ok := value.(string); ok {
			return strings.TrimSpace(typed)
		}
		return ""
	}
	return strings.TrimSpace(frontmatter.Values[key])
}

func frontmatterStringList(frontmatter ASTFrontmatter, key string) []string {
	if value, exists := frontmatter.Data[key]; exists {
		switch typed := value.(type) {
		case []any:
			values := make([]string, 0, len(typed))
			for _, item := range typed {
				switch scalar := item.(type) {
				case string:
					values = append(values, strings.TrimSpace(scalar))
				case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
					values = append(values, fmt.Sprint(scalar))
				}
			}
			return compactStrings(values)
		case string:
			return compactStrings([]string{strings.TrimSpace(typed)})
		default:
			return nil
		}
	}
	return parseFlowStringList(frontmatter.Values[key])
}
