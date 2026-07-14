package main

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type viewerDiscoveryPage struct {
	Path     string
	Title    string
	HTMLPath string
	URL      string
}

func writeViewerDiscoveryFiles(files []okf.BundleFile, out string, siteConfig viewerSiteConfig) ([]string, error) {
	if err := os.MkdirAll(out, 0755); err != nil {
		return nil, err
	}

	llmsPath := filepath.Join(out, "llms.txt")
	if err := os.WriteFile(llmsPath, []byte(viewerLLMSText(files, siteConfig)), 0644); err != nil {
		return nil, err
	}

	written := []string{viewerRelPath(out, llmsPath)}
	if siteConfig.BaseURL == "" {
		return written, nil
	}

	sitemapPath := filepath.Join(out, "sitemap.xml")
	if err := os.WriteFile(sitemapPath, []byte(viewerSitemapXML(files, siteConfig)), 0644); err != nil {
		return nil, err
	}
	written = append(written, viewerRelPath(out, sitemapPath))
	return written, nil
}

func viewerLLMSText(files []okf.BundleFile, siteConfig viewerSiteConfig) string {
	title := viewerKnowledgeBaseNameFromFiles(files, "Open Knowledge")
	summary := viewerKnowledgeBaseSummaryFromFiles(files)
	if summary == "" {
		summary = fmt.Sprintf("%s is an Open Knowledge static wiki.", title)
	}

	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(escapeLLMSMarkdownText(title))
	builder.WriteString("\n\n> ")
	builder.WriteString(escapeLLMSBlockquoteText(summary))
	builder.WriteString("\n\n")
	builder.WriteString("This file lists the published Open Knowledge pages in this static wiki.\n\n")
	builder.WriteString("## Docs\n\n")
	for _, page := range viewerDiscoveryPages(files, siteConfig) {
		builder.WriteString("- [")
		builder.WriteString(escapeLLMSLinkLabel(page.Title))
		builder.WriteString("](")
		builder.WriteString(page.URL)
		builder.WriteString("): ")
		builder.WriteString(page.Path)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func viewerSitemapXML(files []okf.BundleFile, siteConfig viewerSiteConfig) string {
	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	builder.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, page := range viewerDiscoveryPages(files, siteConfig) {
		builder.WriteString("  <url>\n")
		builder.WriteString("    <loc>")
		builder.WriteString(xmlEscapedText(page.URL))
		builder.WriteString("</loc>\n")
		builder.WriteString("  </url>\n")
	}
	builder.WriteString("</urlset>\n")
	return builder.String()
}

func viewerDiscoveryPages(files []okf.BundleFile, siteConfig viewerSiteConfig) []viewerDiscoveryPage {
	pages := make([]viewerDiscoveryPage, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublish(file) {
			continue
		}
		htmlPath := viewerHTMLPath(file.Path)
		pages = append(pages, viewerDiscoveryPage{
			Path:     file.Path,
			Title:    viewerDiscoveryTitle(file),
			HTMLPath: htmlPath,
			URL:      viewerSiteURL(siteConfig, htmlPath),
		})
	}
	sort.Slice(pages, func(i int, j int) bool {
		if pages[i].Path == "index.md" {
			return true
		}
		if pages[j].Path == "index.md" {
			return false
		}
		return strings.ToLower(pages[i].Path) < strings.ToLower(pages[j].Path)
	})
	return pages
}

func viewerDiscoveryTitle(file okf.BundleFile) string {
	if title := viewerFrontmatterString(file.Frontmatter, "title"); title != "" {
		return title
	}
	if heading := firstMarkdownHeading(file.Body); heading != "" {
		return heading
	}
	if title := strings.TrimSpace(file.Title); title != "" {
		return title
	}
	return titleForMarkdownFile(file.Path)
}

func viewerKnowledgeBaseSummaryFromFiles(files []okf.BundleFile) string {
	for _, file := range files {
		if file.Path != "index.md" {
			continue
		}
		for _, key := range []string{"okf_bundle_purpose", "description"} {
			if summary := viewerFrontmatterString(file.Frontmatter, key); summary != "" {
				return summary
			}
		}
		return strings.TrimSpace(file.Description)
	}
	return ""
}

func viewerSiteURL(config viewerSiteConfig, relPath string) string {
	encodedPath := encodeViewerSitePath(relPath)
	if config.BaseURL == "" {
		return encodedPath
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil {
		return encodedPath
	}
	reference, err := url.Parse(encodedPath)
	if err != nil {
		return encodedPath
	}
	return base.ResolveReference(reference).String()
}

func encodeViewerSitePath(value string) string {
	segments := strings.Split(strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/"), "/")
	encoded := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		encoded = append(encoded, url.PathEscape(segment))
	}
	return strings.Join(encoded, "/")
}

func xmlEscapedText(value string) string {
	var builder strings.Builder
	_ = xml.EscapeText(&builder, []byte(value))
	return builder.String()
}

func escapeLLMSMarkdownText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func escapeLLMSBlockquoteText(value string) string {
	return strings.ReplaceAll(escapeLLMSMarkdownText(value), ">", "\\>")
}

func escapeLLMSLinkLabel(value string) string {
	value = escapeLLMSMarkdownText(value)
	replacer := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	return replacer.Replace(value)
}
