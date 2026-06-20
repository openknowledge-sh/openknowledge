package okf

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var markdownLinkDetail = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^\s)]+)(?:\s+"[^"]*")?\)`)

type Link struct {
	Label      string `json:"label"`
	Href       string `json:"href"`
	Kind       string `json:"kind"`
	Line       int    `json:"line"`
	TargetPath string `json:"targetPath,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Exists     bool   `json:"exists,omitempty"`
}

func ExtractLinks(root string, rel string, content string) []Link {
	linkContent := maskFencedCode(content)
	var links []Link
	for _, match := range markdownLinkDetail.FindAllStringSubmatchIndex(linkContent, -1) {
		label := content[match[4]:match[5]]
		href := content[match[6]:match[7]]
		link := Link{
			Label: strings.TrimSpace(label),
			Href:  strings.TrimSpace(href),
			Kind:  linkKind(href),
			Line:  lineForOffset(content, match[0]),
		}

		targetRel := ""
		if link.Kind == "local" {
			targetRel = linkTargetRel(rel, href)
		}
		if targetRel != "" {
			link.Kind = "local"
			link.TargetPath = targetRel
			link.TargetID = trimMarkdownExtension(targetRel)
			targetPath := filepath.Join(root, filepath.FromSlash(targetRel))
			if insideRoot(root, targetPath) {
				if info, err := os.Stat(targetPath); err == nil {
					if info.IsDir() {
						_, err = os.Stat(filepath.Join(targetPath, "index.md"))
						link.Exists = err == nil
					} else {
						link.Exists = true
					}
				}
			}
		}

		links = append(links, link)
	}
	return links
}

func linkKind(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "#") {
		return "anchor"
	}
	if shouldSkipLink(href) {
		return "external"
	}
	return "local"
}

func maskFencedCode(content string) string {
	lines := strings.SplitAfter(content, "\n")
	var builder strings.Builder
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			builder.WriteString(maskLinePreservingNewline(line))
			continue
		}
		if inFence {
			builder.WriteString(maskLinePreservingNewline(line))
			continue
		}
		builder.WriteString(line)
	}
	return builder.String()
}

func maskLinePreservingNewline(line string) string {
	var builder strings.Builder
	for _, r := range line {
		if r == '\n' || r == '\r' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte(' ')
		}
	}
	return builder.String()
}

func shouldSkipLink(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "//") {
		return true
	}
	if schemeIndex := strings.Index(href, ":"); schemeIndex > 0 {
		slashIndex := strings.Index(href, "/")
		if slashIndex < 0 || schemeIndex < slashIndex {
			return true
		}
	}
	return false
}

func linkTargetRel(sourceRel string, href string) string {
	target := strings.TrimSpace(href)
	if hash := strings.Index(target, "#"); hash >= 0 {
		target = target[:hash]
	}
	if query := strings.Index(target, "?"); query >= 0 {
		target = target[:query]
	}
	if target == "" {
		return ""
	}

	var clean string
	if strings.HasPrefix(target, "/") {
		clean = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(target, "/")))
	} else {
		base := filepath.Dir(sourceRel)
		if base == "." {
			base = ""
		}
		clean = filepath.ToSlash(filepath.Clean(filepath.Join(base, target)))
	}
	if clean == "." {
		clean = ""
	}
	if strings.HasSuffix(target, "/") {
		clean = filepath.ToSlash(filepath.Join(clean, "index.md"))
	}
	return clean
}

func lineForOffset(content string, offset int) int {
	line := 1
	for index := 0; index < offset && index < len(content); index++ {
		if content[index] == '\n' {
			line++
		}
	}
	return line
}
