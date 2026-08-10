package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type viewerHTMLExportOptions struct {
	HeadHTML          template.HTML
	OmitSourceArchive bool
}

func writeViewerHTMLWithVersion(root string, out string, version string) (okf.HTMLResult, error) {
	return writeViewerHTMLWithOptions(root, out, version, viewerHTMLExportOptions{})
}

func writeViewerHTMLWithOptions(root string, out string, version string, options viewerHTMLExportOptions) (okf.HTMLResult, error) {
	validation, err := okf.ValidateWithVersion(root, version)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	if err := okf.RequireValidBundle(validation); err != nil {
		return okf.HTMLResult{}, err
	}
	if err := okf.ValidateHTMLOutputBoundary(validation.Root, out); err != nil {
		return okf.HTMLResult{}, err
	}
	var result okf.HTMLResult
	absoluteOut, err := okf.WriteDirectoryAtomically(out, func(staging string) error {
		var writeErr error
		result, writeErr = writeViewerHTMLGeneration(root, staging, version, options, []string{out})
		return writeErr
	})
	if err != nil {
		return okf.HTMLResult{}, err
	}
	result.Out = absoluteOut
	return result, nil
}

func writeViewerHTMLGeneration(root string, out string, version string, options viewerHTMLExportOptions, sourceExcludes []string) (okf.HTMLResult, error) {
	bundle, err := okf.ParseBundleWithVersion(root, version)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	if _, err := okf.BuildPublicationSetWithVersion(bundle.Root, version); err != nil {
		return okf.HTMLResult{}, err
	}
	themeConfig, sourceConfig, siteConfig, err := loadViewerProjectConfig(bundle.Root)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	themeAsset, err := copyViewerThemeStylesheet(bundle.Root, absoluteOut, themeConfig)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	frontmatterByPath, err := viewerFrontmatterHTMLByPath(bundle.Root, bundle.Files, bundle.SpecVersion, viewerStaticMetadataLink)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	staticJSON, err := viewerStaticFilesJSON(bundle.Root, bundle.Files, bundle.SpecVersion, sourceConfig, frontmatterByPath)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	editorsJSON := viewerEditorsStaticJSON()
	graphJSON := viewerStaticGraphJSON(bundle.Files, bundle.SpecVersion)
	dataJS := viewerStaticDataScript(staticJSON, graphJSON, editorsJSON)

	var written []string
	for _, file := range bundle.Files {
		if !okf.ShouldPublishToTarget(file, okf.PublicationTargetViewer) {
			continue
		}
		target := filepath.Join(absoluteOut, filepath.FromSlash(viewerHTMLPath(file.Path)))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return okf.HTMLResult{}, err
		}

		data := viewerFileData{
			Title:       titleForMarkdownFile(file.Path),
			BrandName:   viewerKnowledgeBaseNameFromFiles(viewerFilesForTargets(bundle.Files, okf.PublicationTargetViewer), ""),
			HomeURL:     viewerStaticRelativeURL(file.Path, "index.md"),
			Root:        "",
			Path:        file.Path,
			FileURL:     viewerStaticRelativeURL(file.Path, file.Path),
			SourceURL:   viewerSourceURL(sourceConfig, file.Path),
			Frontmatter: frontmatterByPath[file.Path],
			Body:        template.HTML(viewerStaticFileBody(file, bundle.SpecVersion)),
			Tree:        viewerStaticTree(bundle.Files, file.Path),
			Theme:       viewerThemeForStaticPage(themeConfig, file.Path),
			HeadHTML:    options.HeadHTML,
			Scripts:     viewerStaticScriptURLs(file.Path),
		}

		var builder strings.Builder
		if err := viewerFileTemplate.Execute(&builder, data); err != nil {
			return okf.HTMLResult{}, err
		}
		if err := os.WriteFile(target, []byte(builder.String()), 0644); err != nil {
			return okf.HTMLResult{}, err
		}
		written = append(written, viewerRelPath(absoluteOut, target))
	}
	if themeAsset != "" {
		written = append(written, themeAsset)
	}
	publishedAssets, err := copyViewerPublishedAssets(bundle.Root, absoluteOut, version, themeAsset)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	written = append(written, publishedAssets...)
	scriptAssets, err := writeViewerScriptAssets(absoluteOut, dataJS)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	written = append(written, scriptAssets...)
	if !options.OmitSourceArchive {
		archiveResult, err := writeViewerExportBundleAssets(bundle.Root, absoluteOut, version, sourceExcludes)
		if err != nil {
			return okf.HTMLResult{}, err
		}
		written = append(written, archiveResult...)
	}
	discoveryResult, err := writeViewerDiscoveryFiles(bundle.Files, absoluteOut, siteConfig)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	written = append(written, discoveryResult...)

	sort.Strings(written)
	return okf.HTMLResult{Root: bundle.Root, Out: absoluteOut, Written: written}, nil
}

const (
	viewerThemeScriptAsset = "assets/openknowledge/viewer-theme.js"
	viewerStylesheetAsset  = "assets/openknowledge/viewer.css"
	viewerDataScriptAsset  = "assets/openknowledge/viewer-data.js"
	viewerAppScriptAsset   = "assets/openknowledge/viewer.js"
)

func viewerStaticScriptURLs(currentPath string) viewerScriptURLs {
	return viewerScriptURLs{
		Theme:      viewerStaticAssetURL(currentPath, viewerThemeScriptAsset),
		Stylesheet: viewerStaticAssetURL(currentPath, viewerStylesheetAsset),
		Data:       viewerStaticAssetURL(currentPath, viewerDataScriptAsset),
		App:        viewerStaticAssetURL(currentPath, viewerAppScriptAsset),
	}
}

func viewerStaticAssetURL(currentPath string, assetPath string) string {
	currentHTML := viewerHTMLPath(currentPath)
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(assetPath))
	if err != nil {
		return filepath.ToSlash(assetPath)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticDataScript(notes template.JS, graph template.JS, editors template.JS) string {
	return "window.OpenKnowledgeStaticData=Object.freeze({notes:" + string(notes) +
		",graph:" + string(graph) + ",editors:" + string(editors) + "});\n"
}

func writeViewerScriptAssets(out string, dataJS string) ([]string, error) {
	assets := []struct {
		path    string
		content string
	}{
		{path: viewerThemeScriptAsset, content: viewerThemeBootstrapJS},
		{path: viewerStylesheetAsset, content: viewerCSS},
		{path: viewerDataScriptAsset, content: dataJS},
		{path: viewerAppScriptAsset, content: viewerJS},
	}
	written := make([]string, 0, len(assets))
	for _, asset := range assets {
		target := filepath.Join(out, filepath.FromSlash(asset.path))
		if _, err := os.Stat(target); err == nil {
			return nil, fmt.Errorf("generated viewer script conflicts with published asset %s", asset.path)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(asset.content), 0644); err != nil {
			return nil, err
		}
		written = append(written, asset.path)
	}
	return written, nil
}

func copyViewerPublishedAssets(root string, out string, version string, alreadyWritten string) ([]string, error) {
	publication, err := okf.BuildPublicationSetWithVersion(root, version)
	if err != nil {
		return nil, err
	}
	written := make([]string, 0, len(publication.Assets))
	for _, rel := range publication.Assets {
		if rel == alreadyWritten {
			continue
		}
		source, err := okf.ResolveBundlePath(root, rel)
		if err != nil {
			return nil, fmt.Errorf("publish asset %s: %w", rel, err)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return nil, err
		}
		written = append(written, filepath.ToSlash(rel))
	}
	return written, nil
}

func writeViewerExportBundleAssets(root string, out string, version string, sourceExcludes []string) ([]string, error) {
	archiveRel := okf.BundleArchiveRelPath
	archivePath := filepath.Join(out, filepath.FromSlash(archiveRel))
	archive, err := okf.WritePublishedBundleTarGzipWithVersion(root, archivePath, version, append([]string{out}, sourceExcludes...))
	if err != nil {
		return nil, err
	}

	manifest, err := okf.BundleManifestForArchive(root, version, archiveRel, archive.SHA256)
	if err != nil {
		return nil, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestData = append(manifestData, '\n')
	manifestPath := filepath.Join(out, filepath.FromSlash(okf.BundleManifestRelPath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, err
	}

	return []string{
		filepath.ToSlash(okf.BundleArchiveRelPath),
		filepath.ToSlash(okf.BundleManifestRelPath),
	}, nil
}

func viewerRelPath(root string, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticFilesJSON(root string, files []okf.BundleFile, specVersion string, sourceConfig viewerSourceConfig, frontmatterByPath map[string]template.HTML) (template.JS, error) {
	payload := make([]viewerStaticPayload, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublishToTarget(file, okf.PublicationTargetViewer) || !okf.ShouldPublishToTarget(file, okf.PublicationTargetSearch) {
			continue
		}
		tags, err := viewerTagsForFile(root, file)
		if err != nil {
			return "", err
		}
		payload = append(payload, viewerStaticPayload{
			Title:       titleForMarkdownFile(file.Path),
			Path:        file.Path,
			HTMLPath:    viewerHTMLPath(file.Path),
			SourceURL:   viewerSourceURL(sourceConfig, file.Path),
			Tags:        tags,
			Frontmatter: string(frontmatterByPath[file.Path]),
			Body:        viewerStaticFileBody(file, specVersion),
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return template.JS(data), nil
}

func viewerStaticFileBody(file okf.BundleFile, version string) string {
	return viewerRenderedBody(file, version, okf.StaticHTMLLink)
}

func viewerRenderedBody(file okf.BundleFile, version string, resolve okf.LinkResolver) string {
	if version != "0.2" || file.Reserved {
		return okf.RenderMarkdown(file.Body, file.Path, resolve)
	}
	signals := okf.DeriveOKFV02Signals(file.Frontmatter)
	return okf.RenderMarkdownWithFootnotes(file.Body, file.Path, resolve, okf.OKFV02SourceFootnotes(signals))
}

func viewerStaticMetadataLink(currentRel string, href string) string {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.IsAbs() {
		return trimmed
	}
	target := viewerLinkTargetRel(currentRel, trimmed)
	if target == "" {
		return ""
	}
	if isMarkdownFile(target) {
		return viewerStaticRelativeURL(currentRel, target)
	}
	currentHTML := viewerHTMLPath(currentRel)
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(target))
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(relative)
}

func viewerHTMLPath(markdownPath string) string {
	extension := filepath.Ext(markdownPath)
	if extension == "" {
		return filepath.ToSlash(filepath.Join(markdownPath, "index.html"))
	}
	return strings.TrimSuffix(markdownPath, extension) + ".html"
}

func viewerStaticRelativeURL(currentPath string, targetPath string) string {
	currentHTML := viewerHTMLPath(currentPath)
	targetHTML := viewerHTMLPath(targetPath)
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(targetHTML))
	if err != nil {
		return filepath.ToSlash(targetHTML)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticTree(files []okf.BundleFile, currentPath string) []viewerTreeItem {
	entries := make([]okf.ListEntry, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublishToTarget(file, okf.PublicationTargetViewer) {
			continue
		}
		entries = append(entries, okf.ListEntry{Path: file.Path})
	}
	return viewerTreeWithURL(entries, func(path string) string {
		return viewerStaticRelativeURL(currentPath, path)
	})
}

func viewerGraphJSONFromBundleFiles(files []okf.BundleFile, entries []okf.ListEntry, specVersion string, fileURL func(string) string) template.JS {
	graph := viewerGraphFromBundleFiles(files, entries, specVersion, fileURL)
	data, err := json.Marshal(graph)
	if err != nil {
		return `{"nodes":[],"edges":[]}`
	}
	return template.JS(data)
}

func viewerStaticGraphJSON(files []okf.BundleFile, specVersion string) template.JS {
	entries := make([]okf.ListEntry, 0, len(files))
	publishedFiles := make([]okf.BundleFile, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublishToTarget(file, okf.PublicationTargetViewer) {
			continue
		}
		publishedFiles = append(publishedFiles, file)
		entries = append(entries, okf.ListEntry{Path: file.Path, Title: file.Title})
	}
	return viewerGraphJSONFromBundleFiles(publishedFiles, entries, specVersion, func(path string) string {
		return viewerStaticRelativeURL("index.md", path)
	})
}

func viewerFilesForTargets(files []okf.BundleFile, targets ...okf.PublicationTarget) []okf.BundleFile {
	selected := make([]okf.BundleFile, 0, len(files))
	for _, file := range files {
		allowed := true
		for _, target := range targets {
			if !okf.ShouldPublishToTarget(file, target) {
				allowed = false
				break
			}
		}
		if allowed {
			selected = append(selected, file)
		}
	}
	return selected
}

func viewerGraphFromBundleFiles(files []okf.BundleFile, entries []okf.ListEntry, specVersion string, fileURL func(string) string) viewerGraphData {
	graph := okf.GraphFromBundle(okf.Bundle{Files: files, SpecVersion: specVersion})
	titles := make(map[string]string, len(entries))
	paths := make(map[string]bool, len(entries))
	for _, entry := range entries {
		paths[entry.Path] = true
		titles[entry.Path] = entry.Title
	}

	nodes := make([]viewerGraphNode, 0, len(entries))
	for _, entry := range entries {
		title := strings.TrimSpace(titles[entry.Path])
		if title == "" {
			title = titleForMarkdownFile(entry.Path)
		}
		nodes = append(nodes, viewerGraphNode{
			Path:  entry.Path,
			Title: title,
			URL:   fileURL(entry.Path),
		})
	}

	seenEdges := map[string]bool{}
	var edges []viewerGraphEdge
	for _, edge := range graph.Edges {
		if !paths[edge.Source] || !paths[edge.Target] {
			continue
		}
		key := edge.Kind + "\x00" + edge.Source + "\x00" + edge.Target
		if seenEdges[key] {
			continue
		}
		seenEdges[key] = true
		edges = append(edges, viewerGraphEdge{
			Kind:   edge.Kind,
			Source: edge.Source,
			Target: edge.Target,
			Label:  edge.Label,
		})
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source == edges[j].Source {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Source < edges[j].Source
	})

	return viewerGraphData{Nodes: nodes, Edges: edges}
}

func viewerEditorsJSON() template.JS {
	data, err := json.Marshal(viewerEditors())
	if err != nil {
		return "[]"
	}
	return template.JS(data)
}

func viewerEditorsStaticJSON() template.JS {
	data, err := json.Marshal(viewerEditorsForStatic())
	if err != nil {
		return "[]"
	}
	return template.JS(data)
}
