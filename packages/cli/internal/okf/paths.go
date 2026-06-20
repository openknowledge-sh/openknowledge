package okf

import (
	"path/filepath"
	"strings"
)

func isReserved(path string) bool {
	_, _, reserved := classifyDocument(path)
	return reserved
}

func classifyDocument(rel string) (string, string, bool) {
	name := filepath.Base(rel)
	if strings.EqualFold(name, "index.md") {
		return trimMarkdownExtension(rel), "index", true
	}
	if strings.EqualFold(name, "log.md") {
		return trimMarkdownExtension(rel), "log", true
	}
	return trimMarkdownExtension(rel), "concept", false
}

func trimMarkdownExtension(path string) string {
	extension := filepath.Ext(path)
	if strings.EqualFold(extension, ".md") {
		return strings.TrimSuffix(path, extension)
	}
	return path
}

func deriveTitle(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	if base == "" {
		return "Untitled"
	}
	return strings.ToUpper(base[:1]) + base[1:]
}
