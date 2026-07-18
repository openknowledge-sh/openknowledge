package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

const railwayDeployStateVersion = 1

var railwayVersionPattern = regexp.MustCompile(`(?i)railway\s+(?:app\s+)?v?(\d+)\.(\d+)\.(\d+)`)
var deployImageTagPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

type deployProvider interface {
	Apply(context.Context, deployPlan, deploySecrets) (deployResult, error)
}

type deployPlan struct {
	SchemaVersion string              `json:"schemaVersion"`
	Provider      string              `json:"provider"`
	DryRun        bool                `json:"dryRun"`
	Project       deployProject       `json:"project"`
	Repository    string              `json:"repository"`
	GitHubRepo    string              `json:"githubRepository"`
	Branch        string              `json:"branch"`
	KnowledgeBase deployKnowledgeBase `json:"knowledgeBase"`
	Services      []deployService     `json:"services"`
	Endpoint      deployEndpoint      `json:"publicEndpoint"`
	StateFile     string              `json:"stateFile"`
	Requirements  []string            `json:"credentialRequirements"`
	Runtimes      []string            `json:"agentRuntimes,omitempty"`
}

type deployProject struct {
	Name      string `json:"name"`
	ID        string `json:"id,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

type deployKnowledgeBase struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Spec string `json:"spec"`
}

type deployService struct {
	Name          string   `json:"name"`
	Role          string   `json:"role"`
	Image         string   `json:"image"`
	Public        bool     `json:"public"`
	Port          int      `json:"port,omitempty"`
	VolumePath    string   `json:"volumePath,omitempty"`
	VariableNames []string `json:"variableNames"`
	Config        string   `json:"-"`
}

type deployEndpoint struct {
	Mode       string            `json:"mode"`
	Domain     string            `json:"domain,omitempty"`
	URL        string            `json:"url,omitempty"`
	DNSRecords []deployDNSRecord `json:"dnsRecords,omitempty"`
}

type deployDNSRecord struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status,omitempty"`
}

type deploySecrets struct {
	GitHubToken   string
	AgentKeys     map[string]string
	MCPToken      string
	ArtifactToken string
	ExchangeToken string
}

type deployResult struct {
	SchemaVersion string         `json:"schemaVersion"`
	Provider      string         `json:"provider"`
	Project       deployProject  `json:"project"`
	Endpoint      deployEndpoint `json:"publicEndpoint"`
	Services      []string       `json:"services"`
	StateFile     string         `json:"stateFile"`
	Status        string         `json:"status"`
}

type railwayDeployState struct {
	Version   int                            `json:"version"`
	Complete  bool                           `json:"complete"`
	Project   deployProject                  `json:"project"`
	Services  map[string]railwayServiceState `json:"services"`
	Endpoint  deployEndpoint                 `json:"publicEndpoint"`
	UpdatedAt string                         `json:"updatedAt"`
}

type railwayServiceState struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Image    string `json:"image"`
	Volume   bool   `json:"volume"`
	Deployed bool   `json:"deployed"`
}

type railwayRunner interface {
	Run(context.Context, string, io.Reader, ...string) ([]byte, error)
}

type railwayExecRunner struct{}

func (railwayExecRunner) Run(ctx context.Context, directory string, stdin io.Reader, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "railway", arguments...)
	command.Dir = directory
	command.Stdin = stdin
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil {
		if len(arguments) >= 2 && arguments[0] == "variable" && arguments[1] == "set" {
			return nil, fmt.Errorf("railway %s: %w (provider output suppressed because stdin may contain a secret)", strings.Join(arguments, " "), err)
		}
		providerOutput := strings.TrimSpace(strings.TrimSpace(stderr.String()) + "\n" + strings.TrimSpace(stdout.String()))
		return nil, fmt.Errorf("railway %s: %w: %s", strings.Join(arguments, " "), err, providerOutput)
	}
	return stdout.Bytes(), nil
}

type railwayProvider struct {
	runner railwayRunner
}

func runDeploy(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, deployHelpText())
		return 0
	}
	if args[0] != "railway" {
		fmt.Fprintf(os.Stderr, "unsupported deploy provider: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, deployHelpText())
		return 2
	}
	return runDeployRailway(args[1:])
}

func runDeployRailway(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, deployRailwayHelpText())
		return 0
	}
	// The standard flag package stops at the first positional argument. Accept
	// the natural `deploy railway Wiki --dry-run` form by moving that one path
	// behind the options before parsing.
	if len(args) > 1 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string(nil), args[1:]...), args[0])
	}
	flags := flag.NewFlagSet("deploy railway", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	name := flags.String("name", "", "Railway project and service name prefix")
	project := flags.String("project", "", "reuse an existing Railway project ID")
	workspace := flags.String("workspace", "", "Railway workspace ID or name for a new project")
	branch := flags.String("production-branch", "main", "production Git branch")
	repository := flags.String("repository", "", "GitHub repository URL (defaults to origin)")
	withoutWorker := flags.Bool("without-worker", false, "deploy serving and publishing without the agent worker")
	mcpAccess := flags.String("mcp", "public", "MCP access: public, token, or off")
	domain := flags.String("domain", "", "attach a user-owned custom domain")
	noPublicEndpoint := flags.Bool("no-public-endpoint", false, "do not create a Railway public endpoint")
	githubTokenEnv := flags.String("github-token-env", "GITHUB_TOKEN", "environment variable containing the GitHub token")
	runtimes := flags.String("runtimes", "", "comma-separated worker runtimes; inferred from jobs when omitted")
	codexKeyEnv := flags.String("codex-key-env", "CODEX_API_KEY", "environment variable containing the Codex API key")
	claudeKeyEnv := flags.String("claude-key-env", "ANTHROPIC_API_KEY", "environment variable containing the Claude API key")
	opencodeKeyEnv := flags.String("opencode-key-env", "OPENCODE_API_KEY", "environment variable containing the OpenCode provider key")
	mcpTokenEnv := flags.String("mcp-token-env", "OPENKNOWLEDGE_MCP_TOKEN", "environment variable containing the MCP bearer token")
	imagePrefix := flags.String("image-prefix", "ghcr.io/openknowledge-sh/openknowledge-runtime", "runtime image prefix")
	imageTag := flags.String("image-tag", "latest", "runtime image tag")
	statePath := flags.String("state", "", "deployment state file")
	dryRun := flags.Bool("dry-run", false, "validate and print the plan without changing Railway")
	confirmed := flags.Bool("yes", false, "confirm provider resource changes")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "deploy railway accepts at most one knowledge base path")
		return 2
	}
	knowledgePath := "."
	if flags.NArg() == 1 {
		knowledgePath = flags.Arg(0)
	}
	options := railwayDeployOptions{
		Name: *name, Project: *project, Workspace: *workspace, Branch: *branch, Repository: *repository,
		WithoutWorker: *withoutWorker, MCPAccess: *mcpAccess, Domain: *domain,
		NoPublicEndpoint: *noPublicEndpoint, GitHubTokenEnv: *githubTokenEnv,
		Runtimes: *runtimes, CodexKeyEnv: *codexKeyEnv, ClaudeKeyEnv: *claudeKeyEnv, OpenCodeKeyEnv: *opencodeKeyEnv, MCPTokenEnv: *mcpTokenEnv,
		ImagePrefix: *imagePrefix, ImageTag: *imageTag, StatePath: *statePath, DryRun: *dryRun,
	}
	plan, err := buildRailwayDeployPlan(options, knowledgePath)
	if err != nil {
		return printAgentCommandError(err)
	}
	if *dryRun {
		if err := printJSON(plan); err != nil {
			return printAgentCommandError(err)
		}
		return 0
	}
	if !*confirmed {
		fmt.Fprintln(os.Stderr, "deploy railway changes provider resources; review --dry-run and rerun with --yes")
		return 2
	}
	secrets, err := resolveRailwayDeploySecrets(options, plan)
	if err != nil {
		return printAgentCommandError(err)
	}
	result, err := (railwayProvider{runner: railwayExecRunner{}}).Apply(context.Background(), plan, secrets)
	if err != nil {
		return printAgentCommandError(err)
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

type railwayDeployOptions struct {
	Name, Project, Workspace, Branch, Repository                           string
	MCPAccess, Domain                                                      string
	Runtimes                                                               string
	GitHubTokenEnv, CodexKeyEnv, ClaudeKeyEnv, OpenCodeKeyEnv, MCPTokenEnv string
	ImagePrefix, ImageTag, StatePath                                       string
	WithoutWorker, NoPublicEndpoint, DryRun                                bool
}

func buildRailwayDeployPlan(options railwayDeployOptions, knowledgeInput string) (deployPlan, error) {
	if options.MCPAccess != "public" && options.MCPAccess != "token" && options.MCPAccess != "off" {
		return deployPlan{}, fmt.Errorf("--mcp must be public, token, or off")
	}
	if options.NoPublicEndpoint && strings.TrimSpace(options.Domain) != "" {
		return deployPlan{}, fmt.Errorf("--domain and --no-public-endpoint are mutually exclusive")
	}
	if !validDeployBranch(options.Branch) {
		return deployPlan{}, fmt.Errorf("--production-branch is invalid")
	}
	for flagName, name := range map[string]string{"--github-token-env": options.GitHubTokenEnv, "--codex-key-env": options.CodexKeyEnv, "--claude-key-env": options.ClaudeKeyEnv, "--opencode-key-env": options.OpenCodeKeyEnv, "--mcp-token-env": options.MCPTokenEnv} {
		if !validDeployEnvironmentName(name) {
			return deployPlan{}, fmt.Errorf("%s must be an uppercase environment variable name", flagName)
		}
	}
	if strings.TrimSpace(options.Project) != "" && !validDeployOpaqueProviderValue(options.Project) {
		return deployPlan{}, fmt.Errorf("--project is invalid")
	}
	if strings.TrimSpace(options.Workspace) != "" && !validDeployOpaqueProviderValue(options.Workspace) {
		return deployPlan{}, fmt.Errorf("--workspace is invalid")
	}
	if strings.TrimSpace(options.ImagePrefix) == "" || strings.ContainsAny(options.ImagePrefix, " \t\r\n") || strings.HasPrefix(options.ImagePrefix, "-") {
		return deployPlan{}, fmt.Errorf("--image-prefix is invalid")
	}
	if !deployImageTagPattern.MatchString(options.ImageTag) {
		return deployPlan{}, fmt.Errorf("--image-tag is invalid")
	}
	root, err := okf.ResolveKnowledgeRoot(knowledgeInput)
	if err != nil {
		return deployPlan{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return deployPlan{}, err
	}
	if evaluated, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = evaluated
	}
	validation, err := okf.ValidateWithVersion(root, "latest")
	if err != nil {
		return deployPlan{}, err
	}
	if err := okf.RequireValidBundle(validation); err != nil {
		return deployPlan{}, err
	}
	if _, err := okf.BuildPublicationSetWithVersion(root, "latest"); err != nil {
		return deployPlan{}, fmt.Errorf("publication preflight: %w", err)
	}
	repoRoot, err := runtimeGitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return deployPlan{}, fmt.Errorf("knowledge base must be inside a Git repository: %w", err)
	}
	if evaluated, evalErr := filepath.EvalSymlinks(repoRoot); evalErr == nil {
		repoRoot = evaluated
	}
	relative, err := filepath.Rel(repoRoot, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return deployPlan{}, fmt.Errorf("knowledge base path must be inside the Git repository")
	}
	if err := validateDeployProductionSnapshot(repoRoot, relative, options.Branch); err != nil {
		return deployPlan{}, fmt.Errorf("production branch preflight: %w", err)
	}
	agentRuntimes, err := resolveDeployAgentRuntimes(repoRoot, options.Runtimes, options.WithoutWorker)
	if err != nil {
		return deployPlan{}, err
	}
	repository := strings.TrimSpace(options.Repository)
	if repository == "" {
		repository, err = runtimeGitOutput(repoRoot, "remote", "get-url", "origin")
		if err != nil {
			return deployPlan{}, fmt.Errorf("resolve origin repository (or pass --repository): %w", err)
		}
	}
	cloneURL, githubRepo, err := normalizeGitHubDeployRepository(repository)
	if err != nil {
		return deployPlan{}, err
	}
	projectName := sanitizeDeployName(options.Name)
	if projectName == "" {
		projectName = sanitizeDeployName(filepath.Base(repoRoot) + "-knowledge")
	}
	if projectName == "" {
		return deployPlan{}, fmt.Errorf("cannot derive deployment name; pass --name")
	}
	knowledgeID := sanitizeDeployName(filepath.Base(root))
	if knowledgeID == "" {
		knowledgeID = "knowledge"
	}
	statePath := strings.TrimSpace(options.StatePath)
	if statePath == "" {
		statePath = filepath.Join(repoRoot, ".openknowledge", "deployments", "railway.json")
	} else if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(repoRoot, statePath)
	}
	statePath, err = filepath.Abs(statePath)
	if err != nil {
		return deployPlan{}, err
	}
	publisherName := projectName + "-publisher"
	serveName := projectName + "-serve"
	endpoint := deployEndpoint{Mode: "generated"}
	if options.NoPublicEndpoint {
		endpoint.Mode = "none"
	} else if domain := strings.TrimSpace(options.Domain); domain != "" {
		if err := validateCustomDeployDomain(domain); err != nil {
			return deployPlan{}, err
		}
		endpoint.Mode = "custom"
		endpoint.Domain = domain
	}
	knowledgeRelative := filepath.ToSlash(relative)
	if knowledgeRelative == "." {
		knowledgeRelative = "."
	}
	knowledge := deployKnowledgeBase{ID: knowledgeID, Path: knowledgeRelative, Spec: "latest"}
	services := []deployService{
		{
			Name: publisherName, Role: "publisher", Image: deployImage(options.ImagePrefix, "publisher", options.ImageTag),
			VolumePath:    "/var/lib/openknowledge",
			VariableNames: []string{"OPENKNOWLEDGE_RUNTIME_CONFIG", "OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN", "OPENKNOWLEDGE_EXCHANGE_TOKEN", "GITHUB_TOKEN"},
		},
		{
			Name: serveName, Role: "serve", Image: deployImage(options.ImagePrefix, "serve", options.ImageTag), Public: endpoint.Mode != "none", Port: 8080,
			VariableNames: []string{"OPENKNOWLEDGE_RUNTIME_CONFIG", "OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN", "PORT"},
		},
	}
	if options.MCPAccess == "token" {
		services[1].VariableNames = append(services[1].VariableNames, "OPENKNOWLEDGE_MCP_TOKEN")
	}
	if !options.WithoutWorker {
		for _, runtimeName := range agentRuntimes {
			credential := deployRuntimeCredentialEnvironment(runtimeName)
			services = append(services, deployService{
				Name: projectName + "-worker-" + runtimeName, Role: "worker-" + runtimeName,
				Image: deployImage(options.ImagePrefix, "worker-"+runtimeName, options.ImageTag), VolumePath: "/var/lib/openknowledge",
				VariableNames: []string{"OPENKNOWLEDGE_RUNTIME_CONFIG", "OPENKNOWLEDGE_EXCHANGE_TOKEN", credential},
			})
		}
	}
	plan := deployPlan{
		SchemaVersion: okf.MachineSchemaVersion, Provider: "railway", DryRun: options.DryRun,
		Project:    deployProject{Name: projectName, ID: strings.TrimSpace(options.Project), Workspace: strings.TrimSpace(options.Workspace)},
		Repository: cloneURL, GitHubRepo: githubRepo, Branch: options.Branch,
		KnowledgeBase: knowledge, Services: services, Endpoint: endpoint, StateFile: statePath,
		Requirements: []string{"Railway CLI v5+ authentication", options.GitHubTokenEnv + " (or gh auth token)"},
		Runtimes:     agentRuntimes,
	}
	if !options.WithoutWorker {
		for _, runtimeName := range agentRuntimes {
			plan.Requirements = append(plan.Requirements, deployRuntimeCredentialSource(options, runtimeName))
		}
	}
	if options.MCPAccess == "token" {
		plan.Requirements = append(plan.Requirements, options.MCPTokenEnv)
	}
	for index := range plan.Services {
		plan.Services[index].Config = railwayRuntimeConfig(plan, plan.Services[index].Role, options.MCPAccess)
		if _, err := okruntime.ParseConfig([]byte(plan.Services[index].Config)); err != nil {
			return deployPlan{}, fmt.Errorf("generated %s runtime configuration is invalid: %w", plan.Services[index].Role, err)
		}
	}
	return plan, nil
}

func deployImage(prefix string, role string, tag string) string {
	return strings.TrimSuffix(strings.TrimSpace(prefix), "-") + "-" + role + ":" + strings.TrimSpace(tag)
}

func resolveDeployAgentRuntimes(repoRoot string, requested string, withoutWorker bool) ([]string, error) {
	if withoutWorker {
		return nil, nil
	}
	set := make(map[string]bool)
	if strings.TrimSpace(requested) != "" {
		for _, value := range strings.Split(requested, ",") {
			runtimeName := strings.ToLower(strings.TrimSpace(value))
			if runtimeName == "" {
				return nil, fmt.Errorf("--runtimes must not contain empty entries")
			}
			if _, err := agents.HarnessForRuntime(runtimeName); err != nil {
				return nil, err
			}
			set[runtimeName] = true
		}
	} else {
		jobsPath := filepath.Join(repoRoot, ".openknowledge", "jobs")
		jobs, failures, err := agents.DiscoverJobsLenient(jobsPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("discover deployment jobs: %w", err)
		}
		if len(failures) > 0 {
			return nil, fmt.Errorf("discover deployment jobs: %w", failures[0])
		}
		for _, job := range jobs {
			if job.Enabled {
				set[job.Agent.Runtime] = true
			}
		}
	}
	result := make([]string, 0, len(set))
	for runtimeName := range set {
		result = append(result, runtimeName)
	}
	sort.Strings(result)
	return result, nil
}

func deployRuntimeCredentialEnvironment(runtimeName string) string {
	switch runtimeName {
	case agents.RuntimeClaude:
		return "ANTHROPIC_API_KEY"
	case agents.RuntimeOpenCode:
		return "OPENCODE_API_KEY"
	default:
		return "CODEX_API_KEY"
	}
}

func deployRuntimeCredentialSource(options railwayDeployOptions, runtimeName string) string {
	switch runtimeName {
	case agents.RuntimeClaude:
		return options.ClaudeKeyEnv
	case agents.RuntimeOpenCode:
		return options.OpenCodeKeyEnv
	default:
		return options.CodexKeyEnv
	}
}

func validateDeployProductionSnapshot(repoRoot string, relative string, branch string) error {
	commit := ""
	for _, reference := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		resolved, err := runtimeGitOutput(repoRoot, "rev-parse", "--verify", reference+"^{commit}")
		if err == nil {
			commit = resolved
			break
		}
	}
	if commit == "" {
		return fmt.Errorf("branch %s is not available locally", branch)
	}
	temp, err := os.MkdirTemp("", "openknowledge-deploy-preflight-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	archivePath := filepath.Join(temp, "source.tar.gz")
	arguments := []string{"-C", repoRoot, "archive", "--format=tar.gz", "-o", archivePath, commit}
	if relative != "." {
		arguments = append(arguments, "--", filepath.ToSlash(relative))
	}
	command := exec.Command("git", arguments...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("archive production snapshot: %w: %s", err, strings.TrimSpace(string(output)))
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	extractRoot := filepath.Join(temp, "repository")
	extractErr := okruntime.ExtractDirectoryArchive(archive, extractRoot, runtimeTransportArchiveMaxBytes)
	closeErr := archive.Close()
	if extractErr != nil {
		return extractErr
	}
	if closeErr != nil {
		return closeErr
	}
	knowledgeRoot := extractRoot
	if relative != "." {
		knowledgeRoot = filepath.Join(extractRoot, relative)
	}
	validation, err := okf.ValidateWithVersion(knowledgeRoot, "latest")
	if err != nil {
		return err
	}
	if err := okf.RequireValidBundle(validation); err != nil {
		return err
	}
	if _, err := okf.BuildPublicationSetWithVersion(knowledgeRoot, "latest"); err != nil {
		return err
	}
	return nil
}

func railwayRuntimeConfig(plan deployPlan, role string, mcpAccess string) string {
	publisher := plan.Project.Name + "-publisher"
	volumeRoot := "/var/lib/openknowledge"
	state := "/tmp/openknowledge"
	artifactType := "filesystem"
	artifactPath := "/tmp/openknowledge/artifacts"
	artifactURL := ""
	exchangeDir := "/tmp/openknowledge/exchange"
	exchangeURL := ""
	repositoryURL := ""
	githubEnabled := false
	publisherAPI := false
	runAgents := false
	address := "127.0.0.1:8080"
	if role == "publisher" {
		state = volumeRoot + "/state"
		artifactPath = volumeRoot + "/artifacts"
		exchangeDir = volumeRoot + "/exchange"
		repositoryURL = plan.Repository
		githubEnabled = true
		publisherAPI = true
		runAgents = len(plan.Runtimes) > 0
	} else if role == "serve" {
		artifactType = "http"
		artifactURL = "http://" + publisher + ".railway.internal:8090"
		address = "0.0.0.0:8080"
	} else if strings.HasPrefix(role, "worker-") {
		state = volumeRoot + "/state"
		exchangeDir = volumeRoot + "/exchange"
		exchangeURL = "http://" + publisher + ".railway.internal:8090"
		runAgents = true
	}
	var output strings.Builder
	fmt.Fprintf(&output, "[runtime]\nstate_dir = %s\n\n", strconv.Quote(state))
	fmt.Fprintf(&output, "[artifact_store]\ntype = %s\npath = %s\n", strconv.Quote(artifactType), strconv.Quote(artifactPath))
	if artifactURL != "" {
		fmt.Fprintf(&output, "url = %s\ntoken_env = \"OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN\"\n", strconv.Quote(artifactURL))
	}
	if publisherAPI {
		fmt.Fprint(&output, "\n[publisher_api]\nenabled = true\naddress = \"[::]:8090\"\nartifact_token_env = \"OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN\"\nexchange_token_env = \"OPENKNOWLEDGE_EXCHANGE_TOKEN\"\n")
	}
	fmt.Fprintf(&output, "\n[serve]\naddress = %s\npoll_interval = \"5s\"\nrequest_timeout = \"30s\"\nmax_concurrency = 64\nmcp_access = %s\nmcp_token_env = \"OPENKNOWLEDGE_MCP_TOKEN\"\n", strconv.Quote(address), strconv.Quote(mcpAccess))
	fmt.Fprintf(&output, "\n[worker]\nrepo = \"/workspace\"\nremote = \"origin\"\nproduction_branch = %s\npoll_interval = \"30s\"\nrun_jobs = %t\njobs_path = \".openknowledge/jobs\"\nexchange_dir = %s\n", strconv.Quote(plan.Branch), runAgents, strconv.Quote(exchangeDir))
	if runAgents {
		encoded, _ := json.Marshal(plan.Runtimes)
		fmt.Fprintf(&output, "runtimes = %s\n", encoded)
	}
	if repositoryURL != "" {
		fmt.Fprintf(&output, "repository_url = %s\n", strconv.Quote(repositoryURL))
	}
	if exchangeURL != "" {
		fmt.Fprintf(&output, "exchange_url = %s\nexchange_token_env = \"OPENKNOWLEDGE_EXCHANGE_TOKEN\"\n", strconv.Quote(exchangeURL))
	}
	if githubEnabled {
		fmt.Fprintf(&output, "\n[github]\nenabled = true\nrepository = %s\ntoken_env = \"GITHUB_TOKEN\"\ndraft_pull_request = true\nchecks = true\n", strconv.Quote(plan.GitHubRepo))
	}
	path := "/workspace"
	if plan.KnowledgeBase.Path != "." {
		path += "/" + plan.KnowledgeBase.Path
	}
	fmt.Fprintf(&output, "\n[[knowledge_bases]]\nid = %s\npath = %s\nroute = \"/\"\nspec = %s\npublish = true\nmcp = %t\n", strconv.Quote(plan.KnowledgeBase.ID), strconv.Quote(path), strconv.Quote(plan.KnowledgeBase.Spec), mcpAccess != "off")
	return output.String()
}

func normalizeGitHubDeployRepository(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ".git")
	var slug string
	switch {
	case strings.HasPrefix(trimmed, "git@github.com:"):
		slug = strings.TrimPrefix(trimmed, "git@github.com:")
	case strings.HasPrefix(trimmed, "ssh://git@github.com/"):
		slug = strings.TrimPrefix(trimmed, "ssh://git@github.com/")
	case strings.HasPrefix(trimmed, "https://github.com/"):
		slug = strings.TrimPrefix(trimmed, "https://github.com/")
	case strings.HasPrefix(trimmed, "http://github.com/"):
		slug = strings.TrimPrefix(trimmed, "http://github.com/")
	default:
		return "", "", fmt.Errorf("deploy railway currently requires a GitHub repository URL")
	}
	parts := strings.Split(slug, "/")
	if len(parts) != 2 || !validGitHubDeploySlugPart(parts[0]) || !validGitHubDeploySlugPart(parts[1]) {
		return "", "", fmt.Errorf("GitHub repository must be owner/name")
	}
	return "https://github.com/" + strings.Join(parts, "/") + ".git", strings.Join(parts, "/"), nil
}

func sanitizeDeployName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var output strings.Builder
	previousDash := false
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			output.WriteRune(character)
			previousDash = false
			continue
		}
		if !previousDash && output.Len() > 0 {
			output.WriteByte('-')
			previousDash = true
		}
	}
	result := strings.Trim(output.String(), "-")
	if len(result) > 40 {
		result = strings.TrimRight(result[:40], "-")
	}
	return result
}

func validDeployBranch(value string) bool {
	return value != "" && value != "@" && !strings.HasPrefix(value, "-") &&
		!strings.HasPrefix(value, ".") && !strings.HasSuffix(value, ".") &&
		!strings.HasSuffix(value, "/") && !strings.Contains(value, "..") &&
		!strings.Contains(value, "//") && !strings.Contains(value, "@{") &&
		!strings.Contains(value, "\\") && !strings.ContainsAny(value, " ~^:?*[")
}

func validDeployEnvironmentName(value string) bool {
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

func validDeployOpaqueProviderValue(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "-") && !strings.ContainsAny(value, "\r\n\x00")
}

func validGitHubDeploySlugPart(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validateCustomDeployDomain(value string) error {
	if len(value) > 253 || strings.Contains(value, ":") || strings.Contains(value, "/") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return fmt.Errorf("--domain must be a hostname owned by the user, without scheme, path, or port")
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return fmt.Errorf("--domain must be a fully qualified hostname")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("--domain contains an invalid DNS label")
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return fmt.Errorf("--domain contains an invalid DNS character")
		}
	}
	return nil
}

func resolveRailwayDeploySecrets(options railwayDeployOptions, plan deployPlan) (deploySecrets, error) {
	githubToken := strings.TrimSpace(os.Getenv(options.GitHubTokenEnv))
	if githubToken == "" {
		command := exec.Command("gh", "auth", "token")
		output, err := command.Output()
		if err == nil {
			githubToken = strings.TrimSpace(string(output))
		}
	}
	if githubToken == "" {
		return deploySecrets{}, fmt.Errorf("GitHub credential is required in %s or gh auth", options.GitHubTokenEnv)
	}
	secrets := deploySecrets{GitHubToken: githubToken, AgentKeys: make(map[string]string)}
	if !options.WithoutWorker {
		for _, runtimeName := range plan.Runtimes {
			source := deployRuntimeCredentialSource(options, runtimeName)
			value := strings.TrimSpace(os.Getenv(source))
			if value == "" {
				return deploySecrets{}, fmt.Errorf("%s agent worker requires %s", runtimeName, source)
			}
			secrets.AgentKeys[runtimeName] = value
		}
	}
	if options.MCPAccess == "token" {
		secrets.MCPToken = strings.TrimSpace(os.Getenv(options.MCPTokenEnv))
		if secrets.MCPToken == "" {
			return deploySecrets{}, fmt.Errorf("token-protected MCP requires %s", options.MCPTokenEnv)
		}
	}
	var err error
	secrets.ArtifactToken, err = randomDeployToken()
	if err != nil {
		return deploySecrets{}, err
	}
	secrets.ExchangeToken, err = randomDeployToken()
	if err != nil {
		return deploySecrets{}, err
	}
	return secrets, nil
}

func randomDeployToken() (string, error) {
	content := make([]byte, 32)
	if _, err := rand.Read(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil
}

func (provider railwayProvider) Apply(ctx context.Context, plan deployPlan, secrets deploySecrets) (deployResult, error) {
	if provider.runner == nil {
		return deployResult{}, fmt.Errorf("Railway command runner is not configured")
	}
	versionOutput, err := provider.runner.Run(ctx, "", nil, "--version")
	if err != nil {
		return deployResult{}, fmt.Errorf("Railway CLI is required: %w", err)
	}
	if err := requireRailwayVersion(string(versionOutput)); err != nil {
		return deployResult{}, err
	}
	if _, err := provider.runner.Run(ctx, "", nil, "whoami", "--json"); err != nil {
		return deployResult{}, fmt.Errorf("Railway authentication is required: %w", err)
	}
	working, err := os.MkdirTemp("", "openknowledge-railway-*")
	if err != nil {
		return deployResult{}, err
	}
	defer os.RemoveAll(working)
	state, present, err := loadRailwayDeployState(plan.StateFile)
	if err != nil {
		return deployResult{}, err
	}
	if !present {
		state = railwayDeployState{Version: railwayDeployStateVersion, Project: plan.Project, Services: make(map[string]railwayServiceState)}
	}
	if state.Version != railwayDeployStateVersion {
		return deployResult{}, fmt.Errorf("unsupported Railway deployment state version: %d", state.Version)
	}
	if state.Services == nil {
		state.Services = make(map[string]railwayServiceState)
	}
	if state.Project.Name != "" && state.Project.Name != plan.Project.Name {
		return deployResult{}, fmt.Errorf("deployment state belongs to project %s, not %s", state.Project.Name, plan.Project.Name)
	}
	desiredRoles := make(map[string]bool, len(plan.Services))
	for _, service := range plan.Services {
		desiredRoles[service.Role] = true
	}
	for role := range state.Services {
		if !desiredRoles[role] {
			return deployResult{}, fmt.Errorf("deployment state contains the %s service, but the new plan omits it; remove provider resources explicitly before narrowing topology", role)
		}
	}
	state.Complete = false
	state.Project.Name = plan.Project.Name
	if state.Project.ID != "" {
		if plan.Project.ID != "" && state.Project.ID != plan.Project.ID {
			return deployResult{}, fmt.Errorf("--project does not match deployment state project ID")
		}
		if _, err := provider.runner.Run(ctx, working, nil, "link", "--project", state.Project.ID, "--environment", "production"); err != nil {
			return deployResult{}, err
		}
		if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
			return deployResult{}, err
		}
	} else if plan.Project.ID != "" {
		state.Project.ID = plan.Project.ID
		if _, err := provider.runner.Run(ctx, working, nil, "link", "--project", state.Project.ID, "--environment", "production"); err != nil {
			return deployResult{}, err
		}
		if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
			return deployResult{}, err
		}
	} else {
		arguments := []string{"init", "--name", plan.Project.Name}
		if plan.Project.Workspace != "" {
			arguments = append(arguments, "--workspace", plan.Project.Workspace)
		}
		arguments = append(arguments, "--json")
		output, err := provider.runner.Run(ctx, working, nil, arguments...)
		if err != nil {
			return deployResult{}, err
		}
		state.Project.ID = railwayJSONField(output, "id")
		if state.Project.ID == "" {
			return deployResult{}, fmt.Errorf("Railway project creation did not return an ID")
		}
		if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
			return deployResult{}, err
		}
	}
	for _, service := range plan.Services {
		current, exists := state.Services[service.Role]
		if exists && (current.Name != service.Name || current.Role != service.Role) {
			return deployResult{}, fmt.Errorf("deployment state service mismatch for role %s", service.Role)
		}
		if exists && current.Image != "" && current.Image != service.Image {
			return deployResult{}, fmt.Errorf("service %s uses image %s in deployment state; image source changes require an explicit provider migration", service.Name, current.Image)
		}
		if exists && current.Image == "" {
			current.Image = service.Image
			state.Services[service.Role] = current
		}
		if !exists || current.ID == "" {
			output, err := provider.runner.Run(ctx, working, nil, "add", "--image", service.Image, "--service", service.Name, "--json")
			if err != nil {
				return deployResult{}, err
			}
			current = railwayServiceState{ID: railwayJSONField(output, "id"), Name: service.Name, Role: service.Role, Image: service.Image}
			if current.ID == "" {
				return deployResult{}, fmt.Errorf("Railway service creation did not return an ID for %s", service.Name)
			}
			state.Services[service.Role] = current
			if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
				return deployResult{}, err
			}
		}
		if service.VolumePath != "" && !current.Volume {
			if _, err := provider.runner.Run(ctx, working, nil, "volume", "--service", current.ID, "add", "--mount-path", service.VolumePath, "--json"); err != nil {
				return deployResult{}, err
			}
			current.Volume = true
			state.Services[service.Role] = current
			if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
				return deployResult{}, err
			}
		}
	}
	for _, service := range plan.Services {
		variables := railwayServiceVariables(service, secrets)
		names := make([]string, 0, len(variables))
		for name := range variables {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, err := provider.runner.Run(ctx, working, strings.NewReader(variables[name]), "variable", "set", name, "--stdin", "--service", service.Name, "--skip-deploys", "--json"); err != nil {
				return deployResult{}, fmt.Errorf("set Railway variable %s on %s: %w", name, service.Name, err)
			}
		}
		if _, err := provider.runner.Run(ctx, working, nil, "redeploy", "--service", service.Name, "--yes", "--json"); err != nil {
			return deployResult{}, err
		}
		current := state.Services[service.Role]
		current.Deployed = true
		state.Services[service.Role] = current
		if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
			return deployResult{}, err
		}
	}
	endpoint := state.Endpoint
	if plan.Endpoint.Mode == "none" {
		endpoint = plan.Endpoint
	} else if endpoint.Mode == "" {
		serveName := railwayServiceName(plan.Services, "serve")
		arguments := []string{"domain"}
		if plan.Endpoint.Mode == "custom" {
			arguments = append(arguments, plan.Endpoint.Domain)
		}
		arguments = append(arguments, "--service", serveName, "--port", "8080", "--json")
		output, err := provider.runner.Run(ctx, working, nil, arguments...)
		if err != nil {
			return deployResult{}, err
		}
		endpoint = plan.Endpoint
		if endpoint.Domain == "" {
			endpoint.Domain = railwayJSONField(output, "domain")
		}
		if endpoint.Domain == "" {
			return deployResult{}, fmt.Errorf("Railway endpoint creation did not return a domain")
		}
		endpoint.URL = "https://" + endpoint.Domain
		if endpoint.Mode == "custom" {
			endpoint.DNSRecords = railwayDNSRecords(output)
			if len(endpoint.DNSRecords) == 0 {
				return deployResult{}, fmt.Errorf("Railway custom domain creation did not return the required DNS records")
			}
		}
	} else if endpoint.Mode != plan.Endpoint.Mode || (endpoint.Mode == "custom" && endpoint.Domain != plan.Endpoint.Domain) {
		return deployResult{}, fmt.Errorf("requested public endpoint differs from deployment state; remove the old binding explicitly before changing it")
	}
	state.Endpoint = endpoint
	state.Complete = true
	if err := saveRailwayDeployState(plan.StateFile, state); err != nil {
		return deployResult{}, err
	}
	serviceNames := make([]string, 0, len(plan.Services))
	for _, service := range plan.Services {
		serviceNames = append(serviceNames, service.Name)
	}
	return deployResult{SchemaVersion: okf.MachineSchemaVersion, Provider: "railway", Project: state.Project, Endpoint: endpoint, Services: serviceNames, StateFile: plan.StateFile, Status: "deployment-triggered"}, nil
}

func railwayServiceVariables(service deployService, secrets deploySecrets) map[string]string {
	variables := map[string]string{"OPENKNOWLEDGE_RUNTIME_CONFIG": service.Config}
	switch service.Role {
	case "publisher":
		variables["OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN"] = secrets.ArtifactToken
		variables["OPENKNOWLEDGE_EXCHANGE_TOKEN"] = secrets.ExchangeToken
		variables["GITHUB_TOKEN"] = secrets.GitHubToken
	case "serve":
		variables["OPENKNOWLEDGE_ARTIFACT_SYNC_TOKEN"] = secrets.ArtifactToken
		variables["PORT"] = strconv.Itoa(service.Port)
		if secrets.MCPToken != "" {
			variables["OPENKNOWLEDGE_MCP_TOKEN"] = secrets.MCPToken
		}
	default:
		if strings.HasPrefix(service.Role, "worker-") {
			variables["OPENKNOWLEDGE_EXCHANGE_TOKEN"] = secrets.ExchangeToken
			runtimeName := strings.TrimPrefix(service.Role, "worker-")
			variables[deployRuntimeCredentialEnvironment(runtimeName)] = secrets.AgentKeys[runtimeName]
		}
	}
	return variables
}

func requireRailwayVersion(output string) error {
	matches := railwayVersionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) != 4 {
		return fmt.Errorf("cannot parse Railway CLI version %q; install Railway CLI v5 or newer", strings.TrimSpace(output))
	}
	major, _ := strconv.Atoi(matches[1])
	if major < 5 {
		return fmt.Errorf("Railway CLI v5 or newer is required; found %s.%s.%s", matches[1], matches[2], matches[3])
	}
	return nil
}

func railwayJSONField(content []byte, field string) string {
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return ""
	}
	var find func(any) string
	find = func(candidate any) string {
		switch typed := candidate.(type) {
		case map[string]any:
			if direct, ok := typed[field].(string); ok && direct != "" {
				return direct
			}
			for _, nested := range typed {
				if found := find(nested); found != "" {
					return found
				}
			}
		case []any:
			for _, nested := range typed {
				if found := find(nested); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return find(value)
}

func railwayDNSRecords(content []byte) []deployDNSRecord {
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return nil
	}
	var records []deployDNSRecord
	var visit func(any)
	visit = func(candidate any) {
		switch typed := candidate.(type) {
		case map[string]any:
			typeName := firstRailwayString(typed, "type", "recordType")
			name := firstRailwayString(typed, "name", "host", "hostname")
			value := firstRailwayString(typed, "value", "target")
			if typeName != "" && name != "" && value != "" {
				records = append(records, deployDNSRecord{Type: typeName, Name: name, Value: value, Status: firstRailwayString(typed, "status")})
			}
			for _, nested := range typed {
				visit(nested)
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		}
	}
	visit(value)
	return records
}

func firstRailwayString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if candidate, ok := value[key].(string); ok && candidate != "" {
			return candidate
		}
	}
	return ""
}

func railwayServiceName(services []deployService, role string) string {
	for _, service := range services {
		if service.Role == role {
			return service.Name
		}
	}
	return ""
}

func loadRailwayDeployState(path string) (railwayDeployState, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return railwayDeployState{}, false, nil
	}
	if err != nil {
		return railwayDeployState{}, false, err
	}
	var state railwayDeployState
	if err := okf.DecodeStrictJSON(content, &state); err != nil {
		return railwayDeployState{}, false, fmt.Errorf("invalid Railway deployment state: %w", err)
	}
	return state, true, nil
}

func saveRailwayDeployState(path string, state railwayDeployState) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".railway-state-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func deployHelpText() string {
	return `openknowledge deploy <provider>

Create a self-hosted Open Knowledge runtime from an explicitly publishable
knowledge base. Open Knowledge provisions services and a provider endpoint; it
never purchases or registers a custom domain.

Providers:
  railway    Deploy isolated serve, publisher, and optional agent services.

Run "openknowledge deploy railway --help" for provider options.
`
}

func deployRailwayHelpText() string {
	return `openknowledge deploy railway [path] [options]

Validate a public knowledge artifact, then deploy the isolated runtime to
Railway. By default Railway assigns its own technical service URL. Pass a domain
you already own with --domain, or disable all public ingress with
--no-public-endpoint. Open Knowledge never registers a domain.

Options:
  --name NAME                  Project/service prefix (derived from Git by default).
  --project ID                 Reuse an existing Railway project.
  --workspace ID|NAME          Workspace for a newly created project.
  --production-branch BRANCH   Production branch (default: main).
  --repository URL             GitHub repository (default: origin).
  --without-worker             Omit all agent workers.
  --runtimes LIST              Worker runtimes; infer enabled jobs when omitted.
  --mcp public|token|off       MCP exposure mode (default: public).
  --domain HOSTNAME            Attach a custom domain already owned by the user.
  --no-public-endpoint         Do not create a public Railway endpoint.
  --github-token-env NAME      GitHub token environment (default: GITHUB_TOKEN).
  --codex-key-env NAME         Codex key environment (default: CODEX_API_KEY).
  --claude-key-env NAME        Claude key environment (default: ANTHROPIC_API_KEY).
  --opencode-key-env NAME      OpenCode provider key environment (default: OPENCODE_API_KEY).
  --mcp-token-env NAME         MCP token environment when --mcp token.
  --image-prefix IMAGE         Runtime image prefix.
  --image-tag TAG              Runtime image tag (default: latest).
  --state PATH                 Idempotent deployment state path.
  --dry-run                    Print a secret-free plan without provider changes.
  --yes                        Confirm provider resource changes.
`
}
