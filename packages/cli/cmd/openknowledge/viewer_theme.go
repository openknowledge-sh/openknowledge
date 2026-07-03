package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const viewerThemeConfigFile = "openknowledge.toml"

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
	config := defaultViewerThemeConfig()
	content, err := os.ReadFile(filepath.Join(root, viewerThemeConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return viewerThemeConfig{}, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	section := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripTomlComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "html.theme" {
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return viewerThemeConfig{}, fmt.Errorf("%s:%d expected key = value in [html.theme]", viewerThemeConfigFile, lineNumber)
		}
		value, err := parseTomlStringValue(strings.TrimSpace(rawValue))
		if err != nil {
			return viewerThemeConfig{}, fmt.Errorf("%s:%d %w", viewerThemeConfigFile, lineNumber, err)
		}

		switch strings.TrimSpace(key) {
		case "name":
			config.Name = strings.TrimSpace(value)
		case "stylesheet", "css":
			config.Stylesheet = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return viewerThemeConfig{}, err
	}
	if config.Name == "" {
		config.Name = "default"
	}
	if config.Stylesheet == "" {
		return config, nil
	}
	stylesheet, external, err := normalizeViewerThemeStylesheet(config.Stylesheet)
	if err != nil {
		return viewerThemeConfig{}, err
	}
	config.Stylesheet = stylesheet
	config.External = external
	return config, nil
}

func loadViewerSourceConfig(root string) (viewerSourceConfig, error) {
	content, err := os.ReadFile(filepath.Join(root, viewerThemeConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return viewerSourceConfig{}, nil
		}
		return viewerSourceConfig{}, err
	}

	var config viewerSourceConfig
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	section := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripTomlComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "html.source" {
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return viewerSourceConfig{}, fmt.Errorf("%s:%d expected key = value in [html.source]", viewerThemeConfigFile, lineNumber)
		}
		value, err := parseTomlStringValue(strings.TrimSpace(rawValue))
		if err != nil {
			return viewerSourceConfig{}, fmt.Errorf("%s:%d %w", viewerThemeConfigFile, lineNumber, err)
		}

		switch strings.TrimSpace(key) {
		case "github_base", "githubBase", "github_base_url", "github":
			config.GitHubBase = strings.TrimSpace(value)
		case "entry":
			config.Entry = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return viewerSourceConfig{}, err
	}
	return normalizeViewerSourceConfig(config)
}

func loadViewerSiteConfig(root string) (viewerSiteConfig, error) {
	content, err := os.ReadFile(filepath.Join(root, viewerThemeConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return viewerSiteConfig{}, nil
		}
		return viewerSiteConfig{}, err
	}

	var config viewerSiteConfig
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	section := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripTomlComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "html.site" {
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return viewerSiteConfig{}, fmt.Errorf("%s:%d expected key = value in [html.site]", viewerThemeConfigFile, lineNumber)
		}
		value, err := parseTomlStringValue(strings.TrimSpace(rawValue))
		if err != nil {
			return viewerSiteConfig{}, fmt.Errorf("%s:%d %w", viewerThemeConfigFile, lineNumber, err)
		}

		switch strings.TrimSpace(key) {
		case "base_url", "baseURL", "site_url", "url":
			config.BaseURL = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return viewerSiteConfig{}, err
	}
	return normalizeViewerSiteConfig(config)
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

func stripTomlComment(line string) string {
	quote := rune(0)
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == '#' {
			return line[:index]
		}
	}
	return line
}

func parseTomlStringValue(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("expected a quoted string value")
	}
	if strings.HasPrefix(value, `"`) {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string")
		}
		return parsed, nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return value[1 : len(value)-1], nil
	}
	return "", fmt.Errorf("expected a quoted string value")
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

	source := filepath.Join(root, filepath.FromSlash(config.Stylesheet))
	relative, err := filepath.Rel(root, source)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("theme stylesheet must stay inside the bundle")
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
