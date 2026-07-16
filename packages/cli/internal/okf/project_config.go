package okf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ProjectConfig struct {
	Path       string
	Rules      RuleCatalogConfig
	Validation ValidationOptions
	HTML       ProjectHTMLConfig
	Publish    ProjectPublishConfig
}

type ProjectPublishConfig struct {
	Enabled bool
	Assets  []string
}

type ProjectHTMLConfig struct {
	Theme  ProjectHTMLThemeConfig
	Source ProjectHTMLSourceConfig
	Site   ProjectHTMLSiteConfig
}

type ProjectHTMLThemeConfig struct {
	Name       string `toml:"name"`
	Stylesheet string `toml:"stylesheet"`
}

type ProjectHTMLSourceConfig struct {
	GitHubBase string `toml:"github_base"`
	Entry      string `toml:"entry"`
}

type ProjectHTMLSiteConfig struct {
	BaseURL string `toml:"base_url"`
}

type projectConfigDocument struct {
	Rules      *projectRulesDocument      `toml:"rules"`
	Validation *projectValidationDocument `toml:"validation"`
	HTML       *projectHTMLDocument       `toml:"html"`
	Publish    *projectPublishDocument    `toml:"publish"`
}

type projectPublishDocument struct {
	Enabled bool `toml:"enabled"`
	Assets  any  `toml:"assets"`
}

type projectRulesDocument struct {
	Paths   any `toml:"paths"`
	Enabled any `toml:"enabled"`
}

type projectValidationDocument struct {
	Rules map[string]string `toml:"rules"`
}

type projectHTMLDocument struct {
	Theme  ProjectHTMLThemeConfig  `toml:"theme"`
	Source ProjectHTMLSourceConfig `toml:"source"`
	Site   ProjectHTMLSiteConfig   `toml:"site"`
}

func LoadProjectConfig(root string) (ProjectConfig, error) {
	configuredPath := filepath.Join(root, ValidationConfigFile)
	if _, err := os.Lstat(configuredPath); err != nil {
		if os.IsNotExist(err) {
			return ProjectConfig{}, nil
		}
		return ProjectConfig{}, err
	}
	resolvedPath, err := ResolveBundlePath(root, ValidationConfigFile)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("resolve %s: %w", ValidationConfigFile, err)
	}
	return LoadProjectConfigFile(resolvedPath)
}

func LoadProjectConfigFile(path string) (ProjectConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProjectConfig{}, nil
		}
		return ProjectConfig{}, err
	}
	config, err := ParseProjectConfig(string(content))
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	config.Path = path
	if config.Rules.PathsConfigured || config.Rules.EnabledConfigured {
		config.Rules.ConfigPath = path
	}
	if len(config.Validation.Rules) > 0 {
		config.Validation.ConfigPath = path
	}
	return config, nil
}

func ParseProjectConfig(content string) (ProjectConfig, error) {
	var document projectConfigDocument
	decoder := toml.NewDecoder(bytes.NewBufferString(content)).DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return ProjectConfig{}, err
	}

	config := ProjectConfig{Rules: defaultRuleCatalogConfig()}
	if document.Rules != nil {
		if document.Rules.Paths != nil {
			values, err := projectConfigStringList("rules.paths", document.Rules.Paths)
			if err != nil {
				return ProjectConfig{}, err
			}
			paths, err := normalizeRulePaths(values)
			if err != nil {
				return ProjectConfig{}, err
			}
			config.Rules.Paths = paths
			config.Rules.PathsConfigured = true
		}
		if document.Rules.Enabled != nil {
			values, err := projectConfigStringList("rules.enabled", document.Rules.Enabled)
			if err != nil {
				return ProjectConfig{}, err
			}
			enabled, err := normalizeConfiguredRuleIDs(values)
			if err != nil {
				return ProjectConfig{}, err
			}
			config.Rules.Enabled = enabled
			config.Rules.EnabledConfigured = true
		}
	}
	if document.Validation != nil {
		rules := make([]string, 0, len(document.Validation.Rules))
		for rule := range document.Validation.Rules {
			rules = append(rules, rule)
		}
		sort.Strings(rules)
		for _, rule := range rules {
			if err := SetValidationRuleSeverity(&config.Validation, rule, document.Validation.Rules[rule]); err != nil {
				return ProjectConfig{}, fmt.Errorf("validation.rules.%s: %w", rule, err)
			}
		}
	}
	if document.HTML != nil {
		config.HTML = ProjectHTMLConfig{
			Theme:  document.HTML.Theme,
			Source: document.HTML.Source,
			Site:   document.HTML.Site,
		}
	}
	if document.Publish != nil {
		config.Publish.Enabled = document.Publish.Enabled
		if document.Publish.Assets != nil {
			values, err := projectConfigStringList("publish.assets", document.Publish.Assets)
			if err != nil {
				return ProjectConfig{}, err
			}
			assets, err := normalizePublishAssetPatterns(values)
			if err != nil {
				return ProjectConfig{}, err
			}
			config.Publish.Assets = assets
		}
	}
	return config, nil
}

func projectConfigStringList(field string, value any) ([]string, error) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, fmt.Errorf("%s must not be empty", field)
		}
		return []string{typed}, nil
	case []any:
		values := make([]string, 0, len(typed))
		for index, item := range typed {
			stringValue, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", field, index)
			}
			values = append(values, stringValue)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("%s must be a string or array of strings", field)
	}
}
