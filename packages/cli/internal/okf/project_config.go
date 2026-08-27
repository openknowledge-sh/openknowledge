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
	Path        string
	Rules       RuleCatalogConfig
	Validation  ValidationOptions
	HTML        ProjectHTMLConfig
	Publish     ProjectPublishConfig
	Release     ProjectReleaseConfig
	Maintenance ProjectMaintenanceConfig
}

const (
	ReleasePolicyFollowMain  = "follow-main"
	ReleasePolicyLastPassing = "last-passing"

	MaintenanceModeOff        = "off"
	MaintenanceModePropose    = "propose"
	MaintenanceModeAutonomous = "autonomous"

	MaintenanceDeliveryPullRequest = "pull-request"
	MaintenanceAgentCodex          = "codex"
	MaintenanceAgentClaude         = "claude"
	MaintenanceAgentOpenCode       = "opencode"
)

type ProjectReleaseConfig struct {
	Branch  string
	Policy  string
	Outputs []string
}

type ProjectMaintenanceConfig struct {
	Mode      string
	Agent     string
	Delivery  string
	AutoMerge bool
}

type ProjectPublishConfig struct {
	Assets []string
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
	Rules       *projectRulesDocument       `toml:"rules"`
	Validation  *projectValidationDocument  `toml:"validation"`
	HTML        *projectHTMLDocument        `toml:"html"`
	Publish     *projectPublishDocument     `toml:"publish"`
	Release     *projectReleaseDocument     `toml:"release"`
	Maintenance *projectMaintenanceDocument `toml:"maintenance"`
}

type projectReleaseDocument struct {
	Branch  *string `toml:"branch"`
	Policy  *string `toml:"policy"`
	Outputs any     `toml:"outputs"`
}

type projectMaintenanceDocument struct {
	Mode      *string `toml:"mode"`
	Agent     *string `toml:"agent"`
	Delivery  *string `toml:"delivery"`
	AutoMerge *bool   `toml:"auto_merge"`
}

type projectPublishDocument struct {
	Assets any `toml:"assets"`
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
			return defaultProjectConfig(), nil
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
			return defaultProjectConfig(), nil
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

	config := defaultProjectConfig()
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
			if err := setConfiguredValidationRuleSeverity(&config.Validation, rule, document.Validation.Rules[rule]); err != nil {
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
	if document.Release != nil {
		if document.Release.Branch != nil {
			branch := strings.TrimSpace(*document.Release.Branch)
			if branch == "" {
				return ProjectConfig{}, fmt.Errorf("release.branch must not be empty")
			}
			config.Release.Branch = branch
		}
		if document.Release.Policy != nil {
			switch *document.Release.Policy {
			case ReleasePolicyFollowMain, ReleasePolicyLastPassing:
				config.Release.Policy = *document.Release.Policy
			default:
				return ProjectConfig{}, fmt.Errorf("release.policy must be %s or %s", ReleasePolicyFollowMain, ReleasePolicyLastPassing)
			}
		}
		if document.Release.Outputs != nil {
			values, err := projectConfigStringArray("release.outputs", document.Release.Outputs)
			if err != nil {
				return ProjectConfig{}, err
			}
			outputs, err := normalizeReleaseOutputs(values)
			if err != nil {
				return ProjectConfig{}, err
			}
			config.Release.Outputs = outputs
		}
	}
	if document.Maintenance != nil {
		if document.Maintenance.Mode != nil {
			switch *document.Maintenance.Mode {
			case MaintenanceModeOff, MaintenanceModePropose, MaintenanceModeAutonomous:
				config.Maintenance.Mode = *document.Maintenance.Mode
			default:
				return ProjectConfig{}, fmt.Errorf("maintenance.mode must be %s, %s, or %s", MaintenanceModeOff, MaintenanceModePropose, MaintenanceModeAutonomous)
			}
		}
		if document.Maintenance.Agent != nil {
			agent := strings.TrimSpace(*document.Maintenance.Agent)
			switch agent {
			case MaintenanceAgentCodex, MaintenanceAgentClaude, MaintenanceAgentOpenCode:
				config.Maintenance.Agent = agent
			default:
				return ProjectConfig{}, fmt.Errorf("maintenance.agent must be codex, claude, or opencode")
			}
		}
		if document.Maintenance.Delivery != nil {
			if *document.Maintenance.Delivery != MaintenanceDeliveryPullRequest {
				return ProjectConfig{}, fmt.Errorf("maintenance.delivery must be %s", MaintenanceDeliveryPullRequest)
			}
			config.Maintenance.Delivery = *document.Maintenance.Delivery
		}
		if document.Maintenance.AutoMerge != nil {
			config.Maintenance.AutoMerge = *document.Maintenance.AutoMerge
		}
	}
	return config, nil
}

const (
	ReleaseOutputViewer = "viewer"
	ReleaseOutputMCP    = "mcp"
)

func (config ProjectReleaseConfig) HasOutput(output string) bool {
	for _, candidate := range config.Outputs {
		if candidate == output {
			return true
		}
	}
	return false
}

func normalizeReleaseOutputs(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	outputs := make([]string, 0, len(values))
	for index, value := range values {
		output := strings.TrimSpace(value)
		if output != ReleaseOutputViewer && output != ReleaseOutputMCP {
			return nil, fmt.Errorf("release.outputs[%d] must be viewer or mcp", index)
		}
		if seen[output] {
			return nil, fmt.Errorf("release.outputs[%d] is duplicated: %s", index, output)
		}
		seen[output] = true
		outputs = append(outputs, output)
	}
	sort.Strings(outputs)
	return outputs, nil
}

func defaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		Rules: defaultRuleCatalogConfig(),
		Release: ProjectReleaseConfig{
			Branch:  "main",
			Policy:  ReleasePolicyFollowMain,
			Outputs: []string{},
		},
		Maintenance: ProjectMaintenanceConfig{
			Mode:     MaintenanceModeOff,
			Agent:    MaintenanceAgentCodex,
			Delivery: MaintenanceDeliveryPullRequest,
		},
	}
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

func projectConfigStringArray(field string, value any) ([]string, error) {
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", field)
	}
	result := make([]string, 0, len(values))
	for index, item := range values {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", field, index)
		}
		result = append(result, text)
	}
	return result, nil
}
