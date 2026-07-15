package main

import (
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

//go:embed viewer_theme.css
var viewerDefaultThemeCSS string

type viewerThemeConfig struct {
	Name       string
	Stylesheet string
	External   bool
}

type viewerSourceConfig struct {
	GitHubBase string
	Entry      string
}

type viewerSiteConfig struct {
	BaseURL string
}

type viewerThemeData struct {
	Name       string
	Stylesheet string
}

func defaultViewerThemeConfig() viewerThemeConfig {
	return viewerThemeConfig{Name: "default"}
}

func loadViewerThemeConfig(root string) (viewerThemeConfig, error) {
	config, _, _, err := loadViewerProjectConfig(root)
	return config, err
}

func loadViewerSourceConfig(root string) (viewerSourceConfig, error) {
	_, config, _, err := loadViewerProjectConfig(root)
	return config, err
}

func loadViewerSiteConfig(root string) (viewerSiteConfig, error) {
	_, _, config, err := loadViewerProjectConfig(root)
	return config, err
}

func loadViewerProjectConfig(root string) (viewerThemeConfig, viewerSourceConfig, viewerSiteConfig, error) {
	project, err := okf.LoadProjectConfig(root)
	if err != nil {
		return viewerThemeConfig{}, viewerSourceConfig{}, viewerSiteConfig{}, err
	}
	config := defaultViewerThemeConfig()
	config.Name = strings.TrimSpace(project.HTML.Theme.Name)
	config.Stylesheet = strings.TrimSpace(project.HTML.Theme.Stylesheet)
	if config.Name == "" {
		config.Name = "default"
	}
	if config.Stylesheet == "" {
		source, err := normalizeViewerSourceConfig(viewerSourceConfig{
			GitHubBase: project.HTML.Source.GitHubBase,
			Entry:      project.HTML.Source.Entry,
		})
		if err != nil {
			return viewerThemeConfig{}, viewerSourceConfig{}, viewerSiteConfig{}, err
		}
		site, err := normalizeViewerSiteConfig(viewerSiteConfig{BaseURL: project.HTML.Site.BaseURL})
		return config, source, site, err
	}
	stylesheet, external, err := normalizeViewerThemeStylesheet(config.Stylesheet)
	if err != nil {
		return viewerThemeConfig{}, viewerSourceConfig{}, viewerSiteConfig{}, err
	}
	config.Stylesheet = stylesheet
	config.External = external
	source, err := normalizeViewerSourceConfig(viewerSourceConfig{
		GitHubBase: project.HTML.Source.GitHubBase,
		Entry:      project.HTML.Source.Entry,
	})
	if err != nil {
		return viewerThemeConfig{}, viewerSourceConfig{}, viewerSiteConfig{}, err
	}
	site, err := normalizeViewerSiteConfig(viewerSiteConfig{BaseURL: project.HTML.Site.BaseURL})
	if err != nil {
		return viewerThemeConfig{}, viewerSourceConfig{}, viewerSiteConfig{}, err
	}
	return config, source, site, nil
}

func normalizeViewerSourceConfig(config viewerSourceConfig) (viewerSourceConfig, error) {
	if strings.TrimSpace(config.GitHubBase) == "" {
		return viewerSourceConfig{}, nil
	}
	base := strings.TrimRight(strings.TrimSpace(config.GitHubBase), "/")
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return viewerSourceConfig{}, fmt.Errorf("html.source.github_base must be an http(s) URL")
	}

	entry := strings.TrimSpace(config.Entry)
	if entry != "" {
		if strings.HasPrefix(entry, "/") {
			return viewerSourceConfig{}, fmt.Errorf("html.source.entry must be a relative repository path")
		}
		entry = path.Clean(strings.ReplaceAll(entry, "\\", "/"))
		entry = strings.TrimPrefix(entry, "./")
		if entry == "." {
			entry = ""
		}
		if hasParentSegment(entry) {
			return viewerSourceConfig{}, fmt.Errorf("html.source.entry must stay inside the repository")
		}
	}

	return viewerSourceConfig{GitHubBase: base, Entry: entry}, nil
}

func normalizeViewerSiteConfig(config viewerSiteConfig) (viewerSiteConfig, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return viewerSiteConfig{}, nil
	}
	base := strings.TrimSpace(config.BaseURL)
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return viewerSiteConfig{}, fmt.Errorf("html.site.base_url must be an http(s) URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return viewerSiteConfig{}, fmt.Errorf("html.site.base_url must not include a query string or fragment")
	}
	return viewerSiteConfig{BaseURL: strings.TrimRight(base, "/") + "/"}, nil
}

func viewerSourceURL(config viewerSourceConfig, filePath string) string {
	if config.GitHubBase == "" {
		return ""
	}
	parts := []string{strings.TrimRight(config.GitHubBase, "/")}
	if config.Entry != "" {
		parts = append(parts, encodeViewerSourcePath(config.Entry))
	}
	parts = append(parts, encodeViewerSourcePath(filePath))
	return strings.Join(parts, "/")
}

func encodeViewerSourcePath(value string) string {
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

func normalizeViewerThemeStylesheet(value string) (string, bool, error) {
	if isExternalThemeStylesheet(value) {
		return strings.TrimSpace(value), true, nil
	}
	clean := strings.TrimSpace(value)
	if clean == "" || strings.HasPrefix(clean, "/") {
		return "", false, fmt.Errorf("html.theme.stylesheet must be a relative bundle path or an http(s) URL")
	}
	clean = path.Clean(strings.ReplaceAll(clean, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." || clean == "" || hasParentSegment(clean) {
		return "", false, fmt.Errorf("html.theme.stylesheet must stay inside the bundle")
	}
	return clean, false, nil
}

func isExternalThemeStylesheet(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return (scheme == "http" || scheme == "https") && parsed.Host != ""
}

func viewerThemeForServer(root string, linkPrefix string) (viewerThemeData, error) {
	config, err := loadViewerThemeConfig(root)
	if err != nil {
		return viewerThemeData{Name: "default"}, err
	}
	if err := validateViewerThemeStylesheet(root, config); err != nil {
		return viewerThemeData{Name: "default"}, err
	}
	if config.Stylesheet == "" {
		return viewerThemeData{Name: config.Name}, nil
	}
	if config.External {
		return viewerThemeData{Name: config.Name, Stylesheet: config.Stylesheet}, nil
	}
	return viewerThemeData{Name: config.Name, Stylesheet: rawURLWithPrefix(linkPrefix, config.Stylesheet)}, nil
}

func viewerThemeForStaticPage(config viewerThemeConfig, currentPath string) viewerThemeData {
	if config.Stylesheet == "" || config.External {
		return viewerThemeData{Name: config.Name, Stylesheet: config.Stylesheet}
	}
	currentHTML := viewerHTMLPath(currentPath)
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(config.Stylesheet))
	if err != nil {
		return viewerThemeData{Name: config.Name, Stylesheet: filepath.ToSlash(config.Stylesheet)}
	}
	return viewerThemeData{Name: config.Name, Stylesheet: filepath.ToSlash(relative)}
}

func copyViewerThemeStylesheet(root string, out string, config viewerThemeConfig) (string, error) {
	if config.Stylesheet == "" || config.External {
		return "", nil
	}
	if err := validateViewerThemeStylesheet(root, config); err != nil {
		return "", err
	}

	source := filepath.Join(root, filepath.FromSlash(config.Stylesheet))
	target := filepath.Join(out, filepath.FromSlash(config.Stylesheet))
	content, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, content, 0644); err != nil {
		return "", err
	}
	return viewerRelPath(out, target), nil
}

func validateViewerThemeStylesheet(root string, config viewerThemeConfig) error {
	if config.Stylesheet == "" || config.External {
		return nil
	}

	source, err := okf.ResolveBundlePath(root, config.Stylesheet)
	if err != nil {
		return fmt.Errorf("theme stylesheet %s: %w", config.Stylesheet, err)
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("theme stylesheet %s: %w", config.Stylesheet, err)
	}
	if info.IsDir() {
		return fmt.Errorf("theme stylesheet %s is a directory", config.Stylesheet)
	}
	return nil
}
