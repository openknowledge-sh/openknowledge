package okf

import (
	"os"
	"path/filepath"
	"strings"
)

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
