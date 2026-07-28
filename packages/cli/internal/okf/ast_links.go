package okf

import (
	"os"
	"path/filepath"
	"strings"
)

func parseASTDocumentLinks(root string, document ASTDocument) ASTDocument {
	if document.ReadDiagnostic != nil {
		return document
	}
	document.Links = LinksFromASTMarkdown(root, document.Rel, document.Markdown)
	return document
}

func LinksFromASTMarkdown(root string, rel string, markdown ASTMarkdown) []Link {
	links := make([]Link, 0, len(markdown.Links))
	for _, markdownLink := range markdown.Links {
		href := strings.TrimSpace(markdownLink.Href)
		link := Link{
			Label:        strings.TrimSpace(markdownLink.Label),
			Href:         href,
			Kind:         markdownLink.Kind,
			Line:         markdownLink.Line,
			TargetAnchor: linkTargetAnchor(href),
		}
		if link.Kind == "" {
			link.Kind = linkKind(href)
		}

		targetRel := ""
		if link.Kind == "local" {
			targetRel = linkTargetRel(rel, href)
		} else if link.Kind == "anchor" && link.TargetAnchor != "" {
			targetRel = rel
			link.TargetPath = rel
			link.TargetID = trimMarkdownExtension(rel)
			link.Exists = true
		}
		if targetRel != "" {
			if link.Kind != "anchor" {
				link.Kind = "local"
			}
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
