package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveBundlePath resolves an existing bundle-relative path without
// traversing symbolic links. Bundle roots themselves may be reached through a
// symlink, but every entry below the resolved root must be a real filesystem
// entry so untrusted bundles cannot redirect reads outside their boundary.
func ResolveBundlePath(root string, relative string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", err
	}

	relative = filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
	if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside the bundle: %s", relative)
	}

	current := resolvedRoot
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symbolic links are not supported inside knowledge bundles: %s", filepath.ToSlash(relative))
		}
	}
	return current, nil
}

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
	if isMarkdown(path) {
		return strings.TrimSuffix(path, extension)
	}
	return path
}

func isMarkdown(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".md" || extension == ".markdown"
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func insideRoot(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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
