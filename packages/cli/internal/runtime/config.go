package runtime

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	"github.com/pelletier/go-toml/v2"
)

const DefaultConfigFile = "runtime.toml"

type Config struct {
	Path           string                `toml:"-" json:"-"`
	Root           string                `toml:"-" json:"-"`
	Runtime        RuntimeConfig         `toml:"runtime" json:"runtime"`
	ArtifactStore  ArtifactStoreConfig   `toml:"artifact_store" json:"artifact_store"`
	PublisherAPI   PublisherAPIConfig    `toml:"publisher_api" json:"publisher_api"`
	Serve          ServeConfig           `toml:"serve" json:"serve"`
	Worker         WorkerConfig          `toml:"worker" json:"worker"`
	GitHub         GitHubConfig          `toml:"github" json:"github"`
	KnowledgeBases []KnowledgeBaseConfig `toml:"knowledge_bases" json:"knowledge_bases"`
	AccessProfiles []AccessProfileConfig `toml:"access_profiles" json:"access_profiles"`
}

type RuntimeConfig struct {
	StateDir              string `toml:"state_dir" json:"state_dir"`
	RequireResolvedClaims bool   `toml:"require_resolved_claims" json:"require_resolved_claims"`
}

type ArtifactStoreConfig struct {
	Type     string `toml:"type" json:"type"`
	Path     string `toml:"path" json:"path"`
	URL      string `toml:"url" json:"url,omitempty"`
	TokenEnv string `toml:"token_env" json:"token_env,omitempty"`
}

// PublisherAPIConfig controls the authenticated, private-network transport used
// when a deployment platform cannot share one filesystem volume between roles.
// Artifact and exchange credentials are deliberately separate capabilities.
type PublisherAPIConfig struct {
	Enabled          bool   `toml:"enabled" json:"enabled"`
	Address          string `toml:"address" json:"address"`
	ArtifactTokenEnv string `toml:"artifact_token_env" json:"artifact_token_env,omitempty"`
	ExchangeTokenEnv string `toml:"exchange_token_env" json:"exchange_token_env,omitempty"`
}

type ServeConfig struct {
	Address         string                `toml:"address" json:"address"`
	PollInterval    string                `toml:"poll_interval" json:"poll_interval"`
	RequestTimeout  string                `toml:"request_timeout" json:"request_timeout"`
	MaxConcurrency  int                   `toml:"max_concurrency" json:"max_concurrency"`
	MCPAccess       string                `toml:"mcp_access" json:"mcp_access"`
	MCPTokenEnv     string                `toml:"mcp_token_env" json:"mcp_token_env,omitempty"`
	AllowedOrigins  []string              `toml:"allowed_origins" json:"allowed_origins,omitempty"`
	RetrievalPolicy RetrievalPolicyConfig `toml:"retrieval_policy" json:"retrieval_policy"`
	UsageEvents     UsageEventsConfig     `toml:"usage_events" json:"usage_events"`
}

type RetrievalPolicyConfig struct {
	MinimumTrust    string   `toml:"minimum_trust" json:"minimum_trust"`
	AllowStale      bool     `toml:"allow_stale" json:"allow_stale"`
	AllowedStatuses []string `toml:"allowed_statuses" json:"allowed_statuses"`
	RequireSources  bool     `toml:"require_sources" json:"require_sources"`
}

type AccessProfileConfig struct {
	ID              string                 `toml:"id" json:"id"`
	TokenEnv        string                 `toml:"token_env" json:"token_env"`
	KnowledgeBases  []string               `toml:"knowledge_bases" json:"knowledge_bases"`
	Agents          []string               `toml:"agents" json:"agents"`
	Teams           []string               `toml:"teams" json:"teams"`
	UseCases        []string               `toml:"use_cases" json:"use_cases"`
	RetrievalPolicy *RetrievalPolicyConfig `toml:"retrieval_policy" json:"retrieval_policy,omitempty"`
}

type UsageEventsConfig struct {
	Enabled        bool   `toml:"enabled" json:"enabled"`
	CaptureQueries bool   `toml:"capture_queries" json:"capture_queries"`
	Retention      string `toml:"retention" json:"retention"`
}

type WorkerConfig struct {
	Repo             string   `toml:"repo" json:"repo"`
	RepositoryURL    string   `toml:"repository_url" json:"repository_url,omitempty"`
	Remote           string   `toml:"remote" json:"remote"`
	ProductionBranch string   `toml:"production_branch" json:"production_branch"`
	PollInterval     string   `toml:"poll_interval" json:"poll_interval"`
	RunJobs          bool     `toml:"run_jobs" json:"run_jobs"`
	KnowledgeCI      bool     `toml:"knowledge_ci" json:"knowledge_ci"`
	JobsPath         string   `toml:"jobs_path" json:"jobs_path"`
	Runtimes         []string `toml:"runtimes" json:"runtimes,omitempty"`
	GitTokenEnv      string   `toml:"git_token_env" json:"git_token_env,omitempty"`
	ExchangeDir      string   `toml:"exchange_dir" json:"exchange_dir"`
	ExchangeURL      string   `toml:"exchange_url" json:"exchange_url,omitempty"`
	ExchangeTokenEnv string   `toml:"exchange_token_env" json:"exchange_token_env,omitempty"`
}

type GitHubConfig struct {
	Enabled          bool     `toml:"enabled" json:"enabled"`
	APIURL           string   `toml:"api_url" json:"api_url"`
	Repository       string   `toml:"repository" json:"repository"`
	AppID            int64    `toml:"app_id" json:"app_id,omitempty"`
	InstallationID   int64    `toml:"installation_id" json:"installation_id,omitempty"`
	PrivateKeyFile   string   `toml:"private_key_file" json:"private_key_file,omitempty"`
	TokenEnv         string   `toml:"token_env" json:"token_env,omitempty"`
	DraftPullRequest bool     `toml:"draft_pull_request" json:"draft_pull_request"`
	Checks           bool     `toml:"checks" json:"checks"`
	RequiredChecks   []string `toml:"required_checks" json:"required_checks,omitempty"`
	AutoMergeLowRisk bool     `toml:"auto_merge_low_risk" json:"auto_merge_low_risk"`
}

type KnowledgeBaseConfig struct {
	ID      string `toml:"id" json:"id"`
	Path    string `toml:"path" json:"path"`
	Route   string `toml:"route" json:"route"`
	Spec    string `toml:"spec" json:"spec"`
	Publish bool   `toml:"publish" json:"publish"`
	MCP     bool   `toml:"mcp" json:"mcp"`
}

func LoadConfig(file string) (Config, error) {
	if strings.HasPrefix(file, "env:") {
		name := strings.TrimSpace(strings.TrimPrefix(file, "env:"))
		if !validEnvironmentName(name) {
			return Config{}, fmt.Errorf("invalid runtime configuration environment variable: %s", name)
		}
		content, present := os.LookupEnv(name)
		if !present || strings.TrimSpace(content) == "" {
			return Config{}, fmt.Errorf("runtime configuration environment variable %s is empty", name)
		}
		config, err := ParseConfig([]byte(content))
		if err != nil {
			return Config{}, fmt.Errorf("env:%s: %w", name, err)
		}
		root := strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_RUNTIME_ROOT"))
		if root == "" {
			root = "/workspace"
		}
		absoluteRoot, err := filepath.Abs(root)
		if err != nil {
			return Config{}, err
		}
		config.Path = "env:" + name
		config.Root = absoluteRoot
		if err := config.resolvePaths(absoluteRoot); err != nil {
			return Config{}, fmt.Errorf("env:%s: %w", name, err)
		}
		return config, nil
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return Config{}, err
	}
	config, err := ParseConfig(content)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", file, err)
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return Config{}, err
	}
	config.Path = absolute
	config.Root = filepath.Dir(absolute)
	if err := config.resolvePaths(filepath.Dir(absolute)); err != nil {
		return Config{}, fmt.Errorf("%s: %w", file, err)
	}
	return config, nil
}

func ParseConfig(content []byte) (Config, error) {
	config := defaultConfig()
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}
	config.normalizeAccessProfiles()
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func defaultConfig() Config {
	return Config{
		Runtime:        RuntimeConfig{RequireResolvedClaims: true},
		AccessProfiles: []AccessProfileConfig{},
		ArtifactStore:  ArtifactStoreConfig{Type: "filesystem"},
		PublisherAPI:   PublisherAPIConfig{Address: "127.0.0.1:8090"},
		Serve: ServeConfig{
			Address:        "127.0.0.1:8080",
			PollInterval:   "5s",
			RequestTimeout: "15s",
			MaxConcurrency: 32,
			MCPAccess:      "public",
			MCPTokenEnv:    "OPENKNOWLEDGE_MCP_TOKEN",
			RetrievalPolicy: RetrievalPolicyConfig{
				MinimumTrust:    okf.OKFV02TrustUnverified,
				AllowStale:      true,
				AllowedStatuses: []string{"draft", "stable", "deprecated"},
			},
			UsageEvents: UsageEventsConfig{Retention: "720h"},
		},
		Worker: WorkerConfig{
			Repo:             ".",
			Remote:           "origin",
			ProductionBranch: "main",
			PollInterval:     "30s",
			JobsPath:         ".openknowledge/jobs",
			GitTokenEnv:      "GITHUB_TOKEN",
		},
		GitHub: GitHubConfig{
			APIURL:           "https://api.github.com",
			DraftPullRequest: true,
			Checks:           true,
		},
	}
}

func (config *Config) normalizeAccessProfiles() {
	for index := range config.AccessProfiles {
		profile := &config.AccessProfiles[index]
		profile.KnowledgeBases = append([]string{}, profile.KnowledgeBases...)
		profile.Agents = append([]string{}, profile.Agents...)
		profile.Teams = append([]string{}, profile.Teams...)
		profile.UseCases = append([]string{}, profile.UseCases...)
		for _, values := range [][]string{profile.KnowledgeBases, profile.Agents, profile.Teams, profile.UseCases} {
			for item := range values {
				values[item] = strings.TrimSpace(values[item])
			}
			sort.Strings(values)
		}
		if profile.RetrievalPolicy != nil {
			sort.Strings(profile.RetrievalPolicy.AllowedStatuses)
		}
	}
	sort.Slice(config.AccessProfiles, func(i, j int) bool { return config.AccessProfiles[i].ID < config.AccessProfiles[j].ID })
}

func (config *Config) resolvePaths(base string) error {
	resolve := func(value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			return "", nil
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(base, value)
		}
		return filepath.Abs(value)
	}
	var err error
	config.Runtime.StateDir, err = resolve(config.Runtime.StateDir)
	if err != nil {
		return err
	}
	config.ArtifactStore.Path, err = resolve(config.ArtifactStore.Path)
	if err != nil {
		return err
	}
	config.Worker.Repo, err = resolve(config.Worker.Repo)
	if err != nil {
		return err
	}
	config.Worker.ExchangeDir, err = resolve(config.Worker.ExchangeDir)
	if err != nil {
		return err
	}
	if config.Worker.ExchangeDir == "" {
		config.Worker.ExchangeDir = filepath.Join(config.Runtime.StateDir, "exchange")
	}
	config.GitHub.PrivateKeyFile, err = resolve(config.GitHub.PrivateKeyFile)
	if err != nil {
		return err
	}
	for index := range config.KnowledgeBases {
		config.KnowledgeBases[index].Path, err = resolve(config.KnowledgeBases[index].Path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.Runtime.StateDir) == "" {
		return fmt.Errorf("runtime.state_dir is required")
	}
	if config.ArtifactStore.Type != "filesystem" && config.ArtifactStore.Type != "http" {
		return fmt.Errorf("artifact_store.type must be filesystem or http")
	}
	if strings.TrimSpace(config.ArtifactStore.Path) == "" {
		return fmt.Errorf("artifact_store.path is required")
	}
	if config.ArtifactStore.Type == "http" {
		if err := validatePrivateRuntimeURL("artifact_store.url", config.ArtifactStore.URL); err != nil {
			return err
		}
		if !validEnvironmentName(config.ArtifactStore.TokenEnv) {
			return fmt.Errorf("artifact_store.token_env is required for http stores")
		}
	} else if config.ArtifactStore.URL != "" || config.ArtifactStore.TokenEnv != "" {
		return fmt.Errorf("artifact_store.url and token_env require type = http")
	}
	if config.PublisherAPI.Enabled {
		if config.ArtifactStore.Type != "filesystem" {
			return fmt.Errorf("publisher_api requires a filesystem artifact store")
		}
		if _, _, err := net.SplitHostPort(config.PublisherAPI.Address); err != nil {
			return fmt.Errorf("publisher_api.address must be host:port: %w", err)
		}
		if !validEnvironmentName(config.PublisherAPI.ArtifactTokenEnv) || !validEnvironmentName(config.PublisherAPI.ExchangeTokenEnv) {
			return fmt.Errorf("publisher_api artifact_token_env and exchange_token_env are required")
		}
		if config.PublisherAPI.ArtifactTokenEnv == config.PublisherAPI.ExchangeTokenEnv {
			return fmt.Errorf("publisher_api artifact and exchange token environments must be different")
		}
	}
	if _, _, err := net.SplitHostPort(config.Serve.Address); err != nil {
		return fmt.Errorf("serve.address must be host:port: %w", err)
	}
	if err := positiveDuration("serve.poll_interval", config.Serve.PollInterval); err != nil {
		return err
	}
	if err := positiveDuration("serve.request_timeout", config.Serve.RequestTimeout); err != nil {
		return err
	}
	if config.Serve.MaxConcurrency < 1 || config.Serve.MaxConcurrency > 10_000 {
		return fmt.Errorf("serve.max_concurrency must be between 1 and 10000")
	}
	if config.Serve.MCPAccess != "off" && config.Serve.MCPAccess != "public" && config.Serve.MCPAccess != "token" {
		return fmt.Errorf("serve.mcp_access must be off, public, or token")
	}
	if config.Serve.MCPAccess == "token" && strings.TrimSpace(config.Serve.MCPTokenEnv) == "" {
		return fmt.Errorf("serve.mcp_token_env is required for token access")
	}
	if err := validateRetrievalPolicy("serve.retrieval_policy", config.Serve.RetrievalPolicy); err != nil {
		return err
	}
	for index, origin := range config.Serve.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("serve.allowed_origins[%d] must be an absolute http(s) origin", index)
		}
		if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != origin {
			return fmt.Errorf("serve.allowed_origins[%d] must not contain credentials, path, query, or fragment", index)
		}
	}
	if err := positiveDuration("serve.usage_events.retention", config.Serve.UsageEvents.Retention); err != nil {
		return err
	}
	if config.Serve.UsageEvents.CaptureQueries && !config.Serve.UsageEvents.Enabled {
		return fmt.Errorf("serve.usage_events.capture_queries requires enabled = true")
	}
	if strings.TrimSpace(config.Worker.Remote) == "" {
		return fmt.Errorf("worker.remote is required")
	}
	if !validGitBranch(config.Worker.ProductionBranch) {
		return fmt.Errorf("worker.production_branch is invalid")
	}
	if err := positiveDuration("worker.poll_interval", config.Worker.PollInterval); err != nil {
		return err
	}
	seenRuntimes := make(map[string]bool)
	for index, runtimeName := range config.Worker.Runtimes {
		runtimeName = strings.ToLower(strings.TrimSpace(runtimeName))
		if _, err := agents.HarnessForRuntime(runtimeName); err != nil {
			return fmt.Errorf("worker.runtimes[%d]: %w", index, err)
		}
		if seenRuntimes[runtimeName] {
			return fmt.Errorf("worker.runtimes[%d] is duplicated: %s", index, runtimeName)
		}
		seenRuntimes[runtimeName] = true
		config.Worker.Runtimes[index] = runtimeName
	}
	sort.Strings(config.Worker.Runtimes)
	if config.Worker.RunJobs && len(config.Worker.Runtimes) == 0 {
		return fmt.Errorf("worker.runtimes must list at least one supported runtime when run_jobs = true")
	}
	if config.Worker.ExchangeURL != "" {
		if err := validatePrivateRuntimeURL("worker.exchange_url", config.Worker.ExchangeURL); err != nil {
			return err
		}
		if !validEnvironmentName(config.Worker.ExchangeTokenEnv) {
			return fmt.Errorf("worker.exchange_token_env is required with exchange_url")
		}
	} else if config.Worker.ExchangeTokenEnv != "" {
		return fmt.Errorf("worker.exchange_token_env requires exchange_url")
	}
	if config.GitHub.Enabled {
		if !strings.HasPrefix(config.GitHub.APIURL, "https://") && !strings.HasPrefix(config.GitHub.APIURL, "http://127.0.0.1:") {
			return fmt.Errorf("github.api_url must use HTTPS")
		}
		parts := strings.Split(config.GitHub.Repository, "/")
		if len(parts) != 2 || !validID(parts[0]) || !validID(parts[1]) {
			return fmt.Errorf("github.repository must be owner/name")
		}
		hasEnvironmentToken := strings.TrimSpace(config.GitHub.TokenEnv) != ""
		hasApp := config.GitHub.AppID > 0 && config.GitHub.InstallationID > 0 && strings.TrimSpace(config.GitHub.PrivateKeyFile) != ""
		if !hasEnvironmentToken && !hasApp {
			return fmt.Errorf("github requires token_env or app_id, installation_id, and private_key_file")
		}
		if len(config.GitHub.RequiredChecks) > 100 {
			return fmt.Errorf("github.required_checks must contain at most 100 names")
		}
		seenChecks := make(map[string]bool)
		for index, check := range config.GitHub.RequiredChecks {
			check = strings.TrimSpace(check)
			if check == "" || len(check) > 256 || strings.ContainsAny(check, "\r\n") {
				return fmt.Errorf("github.required_checks[%d] must be a non-empty single-line check name", index)
			}
			if seenChecks[check] {
				return fmt.Errorf("github.required_checks[%d] is duplicated: %s", index, check)
			}
			seenChecks[check] = true
			config.GitHub.RequiredChecks[index] = check
		}
		sort.Strings(config.GitHub.RequiredChecks)
	} else if len(config.GitHub.RequiredChecks) > 0 {
		return fmt.Errorf("github.required_checks requires github.enabled = true")
	}
	if config.GitHub.AutoMergeLowRisk && (!config.GitHub.Enabled || !config.GitHub.Checks || len(config.GitHub.RequiredChecks) == 0) {
		return fmt.Errorf("github.auto_merge_low_risk requires GitHub checks and non-empty github.required_checks")
	}
	if len(config.KnowledgeBases) == 0 {
		return fmt.Errorf("at least one knowledge_bases entry is required")
	}
	ids := map[string]bool{}
	publishedIDs := map[string]bool{}
	routes := map[string]bool{}
	for index := range config.KnowledgeBases {
		knowledge := &config.KnowledgeBases[index]
		if !validID(knowledge.ID) {
			return fmt.Errorf("knowledge_bases[%d].id must contain only letters, numbers, dots, underscores, or hyphens", index)
		}
		if ids[knowledge.ID] {
			return fmt.Errorf("knowledge_bases[%d].id is duplicated: %s", index, knowledge.ID)
		}
		ids[knowledge.ID] = true
		publishedIDs[knowledge.ID] = knowledge.Publish
		if strings.TrimSpace(knowledge.Path) == "" {
			return fmt.Errorf("knowledge_bases[%d].path is required", index)
		}
		route, err := normalizeRoute(knowledge.Route)
		if err != nil {
			return fmt.Errorf("knowledge_bases[%d].route: %w", index, err)
		}
		knowledge.Route = route
		if routes[route] {
			return fmt.Errorf("knowledge_bases[%d].route is duplicated: %s", index, route)
		}
		routes[route] = true
		resolved, ok := okf.ResolveSpecVersion(knowledge.Spec)
		if knowledge.Spec == "" {
			resolved, ok = okf.ResolveSpecVersion("latest")
		}
		if !ok {
			return fmt.Errorf("knowledge_bases[%d].spec is unsupported: %s", index, knowledge.Spec)
		}
		knowledge.Spec = resolved
	}
	sort.Slice(config.KnowledgeBases, func(i, j int) bool {
		return config.KnowledgeBases[i].ID < config.KnowledgeBases[j].ID
	})
	profileIDs := map[string]bool{}
	tokenEnvironments := map[string]bool{}
	for index, profile := range config.AccessProfiles {
		field := fmt.Sprintf("access_profiles[%d]", index)
		if !validID(profile.ID) || profileIDs[profile.ID] {
			return fmt.Errorf("%s.id must be a unique bounded ID", field)
		}
		profileIDs[profile.ID] = true
		if !validEnvironmentName(profile.TokenEnv) || tokenEnvironments[profile.TokenEnv] {
			return fmt.Errorf("%s.token_env must be a unique environment variable", field)
		}
		tokenEnvironments[profile.TokenEnv] = true
		if len(profile.KnowledgeBases) == 0 || len(profile.KnowledgeBases) > 100 {
			return fmt.Errorf("%s.knowledge_bases must contain between 1 and 100 IDs", field)
		}
		if len(profile.Agents)+len(profile.Teams)+len(profile.UseCases) == 0 {
			return fmt.Errorf("%s must route at least one agent, team, or use case", field)
		}
		for name, values := range map[string][]string{"knowledge_bases": profile.KnowledgeBases, "agents": profile.Agents, "teams": profile.Teams, "use_cases": profile.UseCases} {
			if len(values) > 100 {
				return fmt.Errorf("%s.%s must contain at most 100 IDs", field, name)
			}
			seen := map[string]bool{}
			for item, value := range values {
				if !validID(value) || seen[value] {
					return fmt.Errorf("%s.%s[%d] must be a unique bounded ID", field, name, item)
				}
				seen[value] = true
				if name == "knowledge_bases" && !ids[value] {
					return fmt.Errorf("%s.knowledge_bases[%d] references unknown knowledge base %s", field, item, value)
				}
				if name == "knowledge_bases" && !publishedIDs[value] {
					return fmt.Errorf("%s.knowledge_bases[%d] references unpublished knowledge base %s", field, item, value)
				}
			}
		}
		if profile.RetrievalPolicy != nil {
			if err := validateRetrievalPolicy(field+".retrieval_policy", *profile.RetrievalPolicy); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRetrievalPolicy(field string, policy RetrievalPolicyConfig) error {
	trustTiers := map[string]bool{
		okf.OKFV02TrustUnverified: true, okf.OKFV02TrustMachineConfirmed: true, okf.OKFV02TrustHumanReviewed: true,
	}
	if !trustTiers[policy.MinimumTrust] {
		return fmt.Errorf("%s.minimum_trust must be unverified, machine-confirmed, or human-reviewed", field)
	}
	if len(policy.AllowedStatuses) == 0 {
		return fmt.Errorf("%s.allowed_statuses must not be empty", field)
	}
	allowedStatuses := map[string]bool{"draft": true, "stable": true, "deprecated": true}
	seenStatuses := make(map[string]bool)
	for index, status := range policy.AllowedStatuses {
		if !allowedStatuses[status] {
			return fmt.Errorf("%s.allowed_statuses[%d] must be draft, stable, or deprecated", field, index)
		}
		if seenStatuses[status] {
			return fmt.Errorf("%s.allowed_statuses[%d] is duplicated: %s", field, index, status)
		}
		seenStatuses[status] = true
	}
	return nil
}

func positiveDuration(field string, value string) error {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fmt.Errorf("%s must be a positive duration", field)
	}
	return nil
}

func normalizeRoute(value string) (string, error) {
	if value == "" {
		value = "/"
	}
	if !strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return "", fmt.Errorf("must start with / and use forward slashes")
	}
	clean := path.Clean(value)
	if clean != value && !(value != "/" && clean+"/" == value) {
		return "", fmt.Errorf("must be a clean URL path")
	}
	if clean != "/" {
		clean += "/"
	}
	return clean, nil
}

func validID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validGitBranch(value string) bool {
	return value != "" && value != "@" && !strings.HasPrefix(value, "-") &&
		!strings.HasPrefix(value, ".") && !strings.HasSuffix(value, ".") &&
		!strings.HasSuffix(value, "/") && !strings.Contains(value, "..") &&
		!strings.Contains(value, "//") && !strings.Contains(value, "@{") &&
		!strings.Contains(value, "\\") && !strings.ContainsAny(value, " ~^:?*[") &&
		!strings.ContainsFunc(value, func(character rune) bool { return character < 0x20 || character == 0x7f })
}

func validEnvironmentName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func validatePrivateRuntimeURL(field string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%s must be an absolute private http(s) URL", field)
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must not contain credentials, path, query, or fragment", field)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("%s must use HTTPS or private-network HTTP", field)
	}
	host := parsed.Hostname()
	if host == "localhost" || strings.HasSuffix(host, ".railway.internal") {
		return nil
	}
	if address := net.ParseIP(host); address != nil && (address.IsLoopback() || address.IsPrivate()) {
		return nil
	}
	return fmt.Errorf("%s permits plain HTTP only for loopback, private IPs, or *.railway.internal", field)
}
