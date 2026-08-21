package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/insights"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

var runtimeWorkerTokenCache struct {
	sync.Mutex
	key       string
	token     string
	expiresAt time.Time
}

var runtimeWorkerGitHubHTTPClient *http.Client

var runtimeExchangeIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var runtimeExchangeSHA1Pattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var runtimeGitHubReviewerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,99}$`)

const runtimeExchangeBundleMaxBytes int64 = 512 << 20

func runtimeListed(runtimes []string, candidate string) bool {
	for _, runtimeName := range runtimes {
		if runtimeName == candidate {
			return true
		}
	}
	return false
}

func runRuntimeWorker(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeWorkerHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime worker", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	once := flags.Bool("once", false, "run one reconciliation pass and exit")
	role := flags.String("role", "publisher", "worker role: publisher, jobs, or all")
	agentRuntime := flags.String("runtime", "", "jobs harness runtime: codex, claude, or opencode")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "runtime worker accepts no positional arguments")
		return 2
	}
	if *role != "publisher" && *role != "jobs" && *role != "all" {
		fmt.Fprintln(stderrOutput(), "--role must be publisher, jobs, or all")
		return 2
	}
	config, err := okruntime.LoadConfig(*configPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	if *role == "all" && config.GitHub.Enabled {
		return printAgentCommandError(fmt.Errorf("--role all cannot run with GitHub credentials; run separate publisher and jobs roles"))
	}
	*agentRuntime = strings.ToLower(strings.TrimSpace(*agentRuntime))
	if *role != "jobs" && *agentRuntime != "" {
		return printAgentCommandError(fmt.Errorf("--runtime is only valid with --role jobs"))
	}
	if *agentRuntime != "" {
		if _, err := agents.HarnessForRuntime(*agentRuntime); err != nil {
			return printAgentCommandError(err)
		}
		if !runtimeListed(config.Worker.Runtimes, *agentRuntime) {
			return printAgentCommandError(fmt.Errorf("runtime %s is not enabled by worker.runtimes", *agentRuntime))
		}
	}
	if *role == "jobs" && *agentRuntime == "" {
		if len(config.Worker.Runtimes) != 1 {
			return printAgentCommandError(fmt.Errorf("--runtime is required when worker.runtimes contains more than one runtime"))
		}
		*agentRuntime = config.Worker.Runtimes[0]
	}
	if err := ensureRuntimeStateDirectory(config.Runtime.StateDir); err != nil {
		return printAgentCommandError(err)
	}
	lockName := "worker-" + *role
	if *agentRuntime != "" {
		lockName += "-" + *agentRuntime
	}
	lock := flock.New(filepath.Join(config.Runtime.StateDir, lockName+".lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return printAgentCommandError(err)
	}
	if !locked {
		return printAgentCommandError(fmt.Errorf("another worker owns %s", lock.Path()))
	}
	defer lock.Unlock()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var publisherAPIErrors <-chan error
	if config.PublisherAPI.Enabled && (*role == "publisher" || *role == "all") && !*once {
		publisherAPIErrors, err = startRuntimePublisherAPIServer(ctx, config)
		if err != nil {
			return printAgentCommandError(err)
		}
	}
	interval, _ := time.ParseDuration(config.Worker.PollInterval)
	for {
		var passErr error
		switch *role {
		case "publisher":
			passErr = runtimePublisherPass(ctx, config)
		case "jobs":
			passErr = runtimeAgentWorkerPass(ctx, config, *agentRuntime)
		default:
			passErr = runtimeWorkerPass(ctx, config)
		}
		if passErr != nil {
			fmt.Fprintf(stderrOutput(), "runtime worker %s pass failed: %v\n", *role, passErr)
			if *once {
				return 1
			}
		}
		if *once {
			return 0
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0
		case serverErr, ok := <-publisherAPIErrors:
			timer.Stop()
			if ok && serverErr != nil {
				return printAgentCommandError(fmt.Errorf("publisher private API: %w", serverErr))
			}
			publisherAPIErrors = nil
		case <-timer.C:
		}
	}
}

func ensureRuntimeStateDirectory(path string) error {
	return ensureRuntimeStateDirectoryWith(path, os.Chmod)
}

func ensureRuntimeStateDirectoryWith(path string, chmod func(string, os.FileMode) error) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("runtime state path is not a directory: %s", path)
	}
	if info.Mode().Perm() == 0700 {
		return nil
	}
	return chmod(path, 0700)
}

func runtimeWorkerPass(ctx context.Context, config okruntime.Config) error {
	if err := runtimePublisherPass(ctx, config); err != nil {
		return err
	}
	for _, runtimeName := range config.Worker.Runtimes {
		if err := runtimeAgentWorkerPass(ctx, config, runtimeName); err != nil {
			return err
		}
	}
	if !config.GitHub.Enabled {
		return nil
	}
	token, err := runtimeWorkerToken(ctx, config)
	if err != nil {
		return err
	}
	checkout := filepath.Join(config.Runtime.StateDir, "publisher-repository")
	return publishRuntimeExchangePullRequests(ctx, config, checkout, token)
}

func runtimePublisherPass(ctx context.Context, config okruntime.Config) error {
	token, err := runtimeWorkerToken(ctx, config)
	if err != nil {
		return err
	}
	checkout, commit, err := syncRuntimeRepository(ctx, config, token)
	if err != nil {
		return err
	}
	runtimeInfof("runtime worker synchronized %s at %s\n", config.Worker.ProductionBranch, commit)
	verifiedChecks := []string{}
	if len(config.GitHub.RequiredChecks) > 0 {
		client := okruntime.GitHubClient{APIURL: config.GitHub.APIURL, Repository: config.GitHub.Repository, Token: token}
		verifiedChecks, err = client.RequireSuccessfulChecks(ctx, commit, config.GitHub.RequiredChecks)
		if err != nil {
			return fmt.Errorf("production commit %s is not publishable: %w", commit, err)
		}
	}
	if err := publishRuntimeSourceBundle(ctx, config, checkout); err != nil {
		return err
	}
	var failures []error
	for _, knowledge := range config.KnowledgeBases {
		if !knowledge.Publish {
			continue
		}
		mapped, err := mapRuntimeKnowledgeToCheckout(config, knowledge, checkout)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", knowledge.ID, err))
			continue
		}
		if runtimeStoreAlreadyPublishes(config, mapped.ID, commit) {
			continue
		}
		out := filepath.Join(config.Runtime.StateDir, "builds", mapped.ID)
		result, err := buildRuntimeKnowledgeGenerationWithChecks(config, mapped, commit, out, true, verifiedChecks)
		if err != nil {
			failures = append(failures, fmt.Errorf("publish %s: %w", mapped.ID, err))
			continue
		}
		runtimeInfof("runtime worker published %s generation %s\n", mapped.ID, result.Generation)
	}
	if config.Worker.RunJobs && config.GitHub.Enabled {
		if err := publishRuntimeExchangePullRequests(ctx, config, checkout, token); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func runtimeAgentWorkerPass(ctx context.Context, config okruntime.Config, runtimeName string) error {
	if !config.Worker.RunJobs {
		return fmt.Errorf("worker.run_jobs must be true for the jobs role")
	}
	if config.Worker.ExchangeURL != "" {
		if err := downloadRuntimeSourceBundle(ctx, config); err != nil {
			return err
		}
	}
	checkout, err := syncRuntimeAgentRepository(ctx, config, runtimeName)
	if err != nil {
		return err
	}
	runErr := runRuntimeAgentPass(ctx, config, checkout, runtimeName)
	exportErr := exportRuntimeAgentPullRequests(ctx, config, checkout, runtimeName)
	cleanupErr := cleanupRuntimeAgentRuns(ctx, config, checkout, runtimeName)
	return errors.Join(runErr, exportErr, cleanupErr)
}

func syncRuntimeRepository(ctx context.Context, config okruntime.Config, token string) (string, string, error) {
	checkout := filepath.Join(config.Runtime.StateDir, "publisher-repository")
	gitDir := filepath.Join(checkout, ".git")
	if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
		source := strings.TrimSpace(config.Worker.RepositoryURL)
		if source == "" {
			source = config.Worker.Repo
		}
		if source == "" {
			return "", "", fmt.Errorf("worker.repo or worker.repository_url is required")
		}
		if err := os.MkdirAll(filepath.Dir(checkout), 0700); err != nil {
			return "", "", err
		}
		if output, err := runtimeWorkerGit(ctx, config, token, "", "clone", "--no-local", "--branch", config.Worker.ProductionBranch, "--single-branch", "--", source, checkout); err != nil {
			return "", "", fmt.Errorf("clone worker repository: %w: %s", err, output)
		}
	} else if err != nil {
		return "", "", err
	}
	remote := config.Worker.Remote
	branch := config.Worker.ProductionBranch
	refspec := "+refs/heads/" + branch + ":refs/remotes/" + remote + "/" + branch
	if output, err := runtimeWorkerGit(ctx, config, token, checkout, "fetch", "--prune", remote, refspec); err != nil {
		return "", "", fmt.Errorf("fetch production branch: %w: %s", err, output)
	}
	if output, err := runtimeWorkerGit(ctx, config, token, checkout, "checkout", "-B", branch, remote+"/"+branch); err != nil {
		return "", "", fmt.Errorf("checkout production branch: %w: %s", err, output)
	}
	commit, err := runtimeWorkerGit(ctx, config, token, checkout, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	return checkout, strings.TrimSpace(commit), nil
}

func runtimeWorkerGit(ctx context.Context, config okruntime.Config, token string, directory string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = directory
	command.Env = os.Environ()
	if token != "" {
		command.Env = append(command.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
			"GIT_CONFIG_VALUE_0="+runtimeWorkerGitAuthorizationHeader(token),
		)
	}
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runtimeWorkerGitAuthorizationHeader(token string) string {
	credential := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return "AUTHORIZATION: basic " + credential
}

func runtimeWorkerToken(ctx context.Context, config okruntime.Config) (string, error) {
	if config.GitHub.Enabled {
		if config.GitHub.TokenEnv != "" {
			if token := strings.TrimSpace(os.Getenv(config.GitHub.TokenEnv)); token != "" {
				return token, nil
			}
		}
		key := fmt.Sprintf("%s|%d|%d|%s", config.GitHub.APIURL, config.GitHub.AppID, config.GitHub.InstallationID, config.GitHub.PrivateKeyFile)
		runtimeWorkerTokenCache.Lock()
		if runtimeWorkerTokenCache.key == key && runtimeWorkerTokenCache.token != "" && time.Now().Add(2*time.Minute).Before(runtimeWorkerTokenCache.expiresAt) {
			token := runtimeWorkerTokenCache.token
			runtimeWorkerTokenCache.Unlock()
			return token, nil
		}
		runtimeWorkerTokenCache.Unlock()
		credential, err := okruntime.ResolveGitHubCredential(ctx, config.GitHub)
		if err != nil {
			return "", fmt.Errorf("authenticate GitHub App: %w", err)
		}
		if credential.ExpiresAt.IsZero() || !credential.ExpiresAt.After(time.Now().Add(2*time.Minute)) {
			return "", fmt.Errorf("authenticate GitHub App: installation token expiration is missing or too soon")
		}
		runtimeWorkerTokenCache.Lock()
		runtimeWorkerTokenCache.key = key
		runtimeWorkerTokenCache.token = credential.Token
		runtimeWorkerTokenCache.expiresAt = credential.ExpiresAt
		runtimeWorkerTokenCache.Unlock()
		return credential.Token, nil
	}
	if name := strings.TrimSpace(config.Worker.GitTokenEnv); name != "" {
		return strings.TrimSpace(os.Getenv(name)), nil
	}
	return "", nil
}

func mapRuntimeKnowledgeToCheckout(config okruntime.Config, knowledge okruntime.KnowledgeBaseConfig, checkout string) (okruntime.KnowledgeBaseConfig, error) {
	relative, err := filepath.Rel(config.Root, knowledge.Path)
	if err != nil {
		return okruntime.KnowledgeBaseConfig{}, err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return okruntime.KnowledgeBaseConfig{}, fmt.Errorf("path must be inside the runtime configuration repository: %s", knowledge.Path)
	}
	knowledge.Path = filepath.Join(checkout, relative)
	return knowledge, nil
}

func runtimeStoreAlreadyPublishes(config okruntime.Config, knowledgeID string, commit string) bool {
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	_, root, err := store.Active(knowledgeID)
	if err != nil {
		return false
	}
	manifest, err := okruntime.LoadAndValidateGeneration(root)
	return err == nil && manifest.Commit == commit
}

func runRuntimeAgentPass(ctx context.Context, config okruntime.Config, checkout string, runtimeName string) error {
	jobs := config.Worker.JobsPath
	if !filepath.IsAbs(jobs) {
		jobs = filepath.Join(checkout, jobs)
	}
	if _, err := os.Stat(jobs); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executable, "jobs", "daemon", jobs, "--once", "--runtime", runtimeName)
	command.Dir = checkout
	environment := runtimeEnvironmentWithout(os.Environ(), config.Worker.GitTokenEnv, config.GitHub.TokenEnv, config.Worker.ExchangeTokenEnv)
	command.Env = runtimeEnvironmentWith(environment, agents.JobsStateDirEnv, filepath.Join(config.Runtime.StateDir, "jobs-"+runtimeName))
	command.Stdout = os.Stdout
	command.Stderr = stderrOutput()
	if err := command.Run(); err != nil {
		return fmt.Errorf("scheduled agent pass: %w", err)
	}
	return nil
}

type runtimeGitHubPublication struct {
	RunID   string `json:"run_id"`
	Commit  string `json:"commit"`
	PR      int    `json:"pull_request"`
	PRURL   string `json:"pull_request_url"`
	Checked bool   `json:"check_published"`
	Merged  bool   `json:"merged"`
}

type runtimeExchangeRequest struct {
	Version      int                         `json:"version"`
	RunID        string                      `json:"run_id"`
	JobID        string                      `json:"job_id"`
	Branch       string                      `json:"branch"`
	BaseSHA      string                      `json:"base_sha"`
	HeadSHA      string                      `json:"head_sha"`
	BundleSHA256 string                      `json:"bundle_sha256"`
	VerifyCount  int                         `json:"verify_count"`
	Eval         *runtimeExchangeEval        `json:"eval,omitempty"`
	Maintenance  *runtimeExchangeMaintenance `json:"maintenance,omitempty"`
}

type runtimeExchangeMaintenance struct {
	Risk          string   `json:"risk"`
	Approval      string   `json:"approval"`
	Confidence    float64  `json:"confidence"`
	Owners        []string `json:"owners"`
	Insights      []string `json:"insights"`
	Findings      []string `json:"findings"`
	Paths         []string `json:"paths"`
	ExpertTargets []string `json:"expert_targets"`
	Status        string   `json:"status"`
}

type runtimeExchangeEval struct {
	Status         string `json:"status"`
	Dataset        string `json:"dataset"`
	Target         string `json:"target"`
	Base           string `json:"base"`
	Gate           string `json:"gate"`
	Regressions    int    `json:"regressions"`
	ProposedFailed int    `json:"proposed_failed"`
}

func publishRuntimeSourceBundle(ctx context.Context, config okruntime.Config, checkout string) error {
	if err := os.MkdirAll(config.Worker.ExchangeDir, 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(config.Worker.ExchangeDir, ".source-*.bundle")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	_ = temp.Close()
	_ = os.Remove(tempPath)
	defer os.Remove(tempPath)
	ref := "refs/heads/" + config.Worker.ProductionBranch
	if output, err := runtimeWorkerGit(ctx, config, "", checkout, "bundle", "create", tempPath, ref); err != nil {
		return fmt.Errorf("create source exchange bundle: %w: %s", err, output)
	}
	if err := os.Chmod(tempPath, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, filepath.Join(config.Worker.ExchangeDir, "source.bundle"))
}

func syncRuntimeAgentRepository(ctx context.Context, config okruntime.Config, runtimeName string) (string, error) {
	bundle := filepath.Join(config.Worker.ExchangeDir, "source.bundle")
	if _, err := os.Stat(bundle); err != nil {
		return "", fmt.Errorf("publisher source bundle is not ready: %w", err)
	}
	checkout := filepath.Join(config.Runtime.StateDir, "agent-repository-"+runtimeName)
	if _, err := os.Stat(filepath.Join(checkout, ".git")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(checkout), 0700); err != nil {
			return "", err
		}
		if output, err := runtimeWorkerGit(ctx, config, "", "", "clone", "--no-local", "--branch", config.Worker.ProductionBranch, "--single-branch", "--", bundle, checkout); err != nil {
			return "", fmt.Errorf("clone agent source bundle: %w: %s", err, output)
		}
	} else if err != nil {
		return "", err
	}
	branch := config.Worker.ProductionBranch
	refspec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
	if output, err := runtimeWorkerGit(ctx, config, "", checkout, "fetch", bundle, refspec); err != nil {
		return "", fmt.Errorf("refresh agent source bundle: %w: %s", err, output)
	}
	if output, err := runtimeWorkerGit(ctx, config, "", checkout, "checkout", "-B", branch, "refs/remotes/origin/"+branch); err != nil {
		return "", fmt.Errorf("activate agent source: %w: %s", err, output)
	}
	return checkout, nil
}

func exportRuntimeAgentPullRequests(ctx context.Context, config okruntime.Config, checkout string, runtimeName string) error {
	runs, issues, err := listRuntimeAgentRuns(config, checkout, runtimeName)
	if err != nil {
		return err
	}
	var failures []error
	for _, issue := range issues {
		failures = append(failures, fmt.Errorf("agent run %s: %s", issue.Path, issue.Error))
	}
	for _, summary := range runs {
		if summary.Status != "succeeded" {
			continue
		}
		content, err := os.ReadFile(summary.RunRecord)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var record agents.RunRecord
		if err := json.Unmarshal(content, &record); err != nil {
			failures = append(failures, err)
			continue
		}
		if !record.Plan.Output.PR {
			continue
		}
		marker := filepath.Join(filepath.Dir(summary.RunRecord), "exchange.json")
		if _, err := os.Stat(marker); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
			continue
		}
		headSHA, err := runtimeWorkerGit(ctx, config, "", record.Plan.Worktree, "rev-parse", "HEAD")
		if err != nil {
			failures = append(failures, fmt.Errorf("agent run %s resolve commit: %w", record.RunID, err))
			continue
		}
		if headSHA == record.Plan.BaseSHA {
			if err := writePrivateRuntimeJSON(marker, map[string]any{"run_id": record.RunID, "empty": true}); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		runsDir := filepath.Join(config.Worker.ExchangeDir, "runs")
		if err := os.MkdirAll(runsDir, 0755); err != nil {
			failures = append(failures, err)
			continue
		}
		target := filepath.Join(runsDir, record.RunID)
		if _, err := os.Stat(target); err == nil {
			if config.Worker.ExchangeURL != "" {
				if err := uploadRuntimeExchangeRun(ctx, config, record.RunID, target); err != nil {
					failures = append(failures, err)
					continue
				}
				if err := os.RemoveAll(target); err != nil {
					failures = append(failures, fmt.Errorf("remove uploaded agent exchange %s: %w", record.RunID, err))
					continue
				}
			}
			if err := writePrivateRuntimeJSON(marker, map[string]any{"run_id": record.RunID, "exported": true}); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		staging, err := os.MkdirTemp(runsDir, ".incoming-*")
		if err != nil {
			failures = append(failures, err)
			continue
		}
		bundlePath := filepath.Join(staging, "branch.bundle")
		ref := "refs/heads/" + record.Plan.Branch
		if output, err := runtimeWorkerGit(ctx, config, "", record.Plan.Worktree, "bundle", "create", bundlePath, ref); err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, fmt.Errorf("agent run %s create exchange bundle: %w: %s", record.RunID, err, output))
			continue
		}
		bundleSHA, err := okf.SHA256File(bundlePath)
		if err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, err)
			continue
		}
		maintenance, err := runtimeMaintenanceAttestation(ctx, record.Plan.Worktree, record.Plan.BaseSHA, headSHA)
		if err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, fmt.Errorf("agent run %s maintenance attestation: %w", record.RunID, err))
			continue
		}
		request := runtimeExchangeRequest{Version: 1, RunID: record.RunID, JobID: record.JobID, Branch: record.Plan.Branch, BaseSHA: record.Plan.BaseSHA, HeadSHA: headSHA, BundleSHA256: bundleSHA, VerifyCount: len(record.Verify), Maintenance: maintenance}
		if record.Eval != nil {
			request.Eval = &runtimeExchangeEval{
				Status: record.Eval.Status, Dataset: record.Eval.Dataset, Target: record.Eval.Target, Base: record.Eval.Base, Gate: record.Eval.Gate,
				Regressions: record.Eval.Regressions, ProposedFailed: record.Eval.ProposedFailed,
			}
		}
		if err := writeExchangeJSON(filepath.Join(staging, "request.json"), request); err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, err)
			continue
		}
		if err := os.Chmod(staging, 0755); err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, err)
			continue
		}
		if err := os.Rename(staging, target); err != nil {
			_ = os.RemoveAll(staging)
			failures = append(failures, err)
			continue
		}
		if config.Worker.ExchangeURL != "" {
			if err := uploadRuntimeExchangeRun(ctx, config, record.RunID, target); err != nil {
				failures = append(failures, err)
				continue
			}
			if err := os.RemoveAll(target); err != nil {
				failures = append(failures, fmt.Errorf("remove uploaded agent exchange %s: %w", record.RunID, err))
				continue
			}
		}
		if err := writePrivateRuntimeJSON(marker, map[string]any{"run_id": record.RunID, "exported": true}); err != nil {
			failures = append(failures, err)
			continue
		}
		runtimeInfof("runtime agent worker exported run %s for private publication\n", record.RunID)
	}
	return errors.Join(failures...)
}

func runtimeMaintenanceAttestation(ctx context.Context, worktree string, baseSHA string, headSHA string) (*runtimeExchangeMaintenance, error) {
	paths, err := runtimeGitChangedPaths(ctx, worktree, baseSHA, headSHA)
	if err != nil {
		return nil, err
	}
	var routed []insights.Insight
	var insightPaths []string
	for _, path := range paths {
		if !strings.HasSuffix(strings.ToLower(path), ".md") || !strings.Contains("/"+filepath.ToSlash(path)+"/", "/insights/") {
			continue
		}
		content, readErr := runtimeGitBlob(ctx, worktree, headSHA, path)
		if readErr != nil {
			return nil, fmt.Errorf("read changed insight %s from exchange head: %w", path, readErr)
		}
		item, parseErr := insights.ParseContent(path, content)
		if parseErr != nil {
			return nil, fmt.Errorf("parse changed insight %s: %w", path, parseErr)
		}
		if item.Status != "resolved" && item.Status != "blocked" {
			return nil, fmt.Errorf("changed insight must be resolved or blocked: %s", path)
		}
		item.Path = filepath.ToSlash(path)
		routed = append(routed, item)
		insightPaths = append(insightPaths, filepath.ToSlash(path))
	}
	if len(routed) == 0 {
		return nil, nil
	}
	result := &runtimeExchangeMaintenance{Confidence: 1, Paths: uniqueRuntimeStrings(insightPaths)}
	highest := 0
	hasResolved, hasBlocked := false, false
	for _, item := range routed {
		rank := map[string]int{"low": 1, "medium": 2, "high": 3}[item.Route.Risk]
		if rank > highest {
			highest = rank
			result.Risk = item.Route.Risk
			result.Approval = item.Route.Approval
		}
		if item.Route.Confidence < result.Confidence {
			result.Confidence = item.Route.Confidence
		}
		result.Owners = append(result.Owners, item.Route.Owners...)
		result.Insights = append(result.Insights, item.ID)
		if item.FindingID != "" {
			result.Findings = append(result.Findings, item.FindingID)
		}
		if item.Route.Approval == "expert" {
			prefix := strings.Split(item.Path, "/insights/")[0]
			for _, target := range item.Targets {
				if target == "." {
					result.ExpertTargets = append(result.ExpertTargets, prefix)
				} else if prefix == "" {
					result.ExpertTargets = append(result.ExpertTargets, target)
				} else {
					result.ExpertTargets = append(result.ExpertTargets, prefix+"/"+target)
				}
			}
		}
		hasResolved = hasResolved || item.Status == "resolved"
		hasBlocked = hasBlocked || item.Status == "blocked"
	}
	result.Owners = uniqueRuntimeStrings(result.Owners)
	result.Insights = uniqueRuntimeStrings(result.Insights)
	result.Findings = uniqueRuntimeStrings(result.Findings)
	result.ExpertTargets = uniqueRuntimeStrings(result.ExpertTargets)
	switch {
	case hasBlocked && hasResolved:
		result.Status = "mixed"
	case hasBlocked:
		result.Status = "escalated"
	default:
		result.Status = "proposed"
	}
	if err := validateRuntimeExchangeMaintenance(result); err != nil {
		return nil, err
	}
	return result, nil
}

func runtimeGitBlob(ctx context.Context, repository string, revision string, path string) ([]byte, error) {
	object := revision + ":" + path
	sizeCommand := exec.CommandContext(ctx, "git", "cat-file", "-s", object)
	sizeCommand.Dir = repository
	sizeOutput, err := sizeCommand.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect blob: %w: %s", err, strings.TrimSpace(string(sizeOutput)))
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
	if err != nil || size < 0 || size > 4<<20 {
		return nil, fmt.Errorf("changed insight is not a bounded blob")
	}
	showCommand := exec.CommandContext(ctx, "git", "show", object)
	showCommand.Dir = repository
	content, err := showCommand.Output()
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	if int64(len(content)) != size {
		return nil, fmt.Errorf("changed insight blob size changed while reading")
	}
	return content, nil
}

func runtimeGitChangedPaths(ctx context.Context, repository string, baseSHA string, headSHA string) ([]string, error) {
	command := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=ACMRD", "-z", baseSHA, headSHA, "--")
	command.Dir = repository
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect maintenance paths: %w: %s", err, strings.TrimSpace(string(output)))
	}
	parts := strings.Split(string(output), "\x00")
	paths := make([]string, 0, len(parts))
	for _, item := range parts {
		if item == "" {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(item) || strings.ContainsAny(item, "\r\n") {
			return nil, fmt.Errorf("maintenance diff contains an unsafe path")
		}
		paths = append(paths, clean)
	}
	return uniqueRuntimeStrings(paths), nil
}

func validateRuntimeExchangeMaintenance(route *runtimeExchangeMaintenance) error {
	if route == nil {
		return nil
	}
	normalized, err := insights.NormalizeMaintenanceRoute(insights.MaintenanceRoute{
		Risk: route.Risk, Approval: route.Approval, Confidence: route.Confidence, Owners: route.Owners,
	})
	if err != nil || normalized.Risk != route.Risk || normalized.Approval != route.Approval || !equalStringLists(normalized.Owners, route.Owners) {
		if err == nil {
			err = fmt.Errorf("maintenance route is not normalized")
		}
		return err
	}
	for _, owner := range route.Owners {
		if suffix, present := strings.CutPrefix(owner, "github:"); present && !runtimeGitHubReviewerPattern.MatchString(suffix) {
			return fmt.Errorf("maintenance GitHub reviewer is invalid")
		}
		if suffix, present := strings.CutPrefix(owner, "github-team:"); present && !runtimeGitHubReviewerPattern.MatchString(suffix) {
			return fmt.Errorf("maintenance GitHub team reviewer is invalid")
		}
	}
	if route.Status != "proposed" && route.Status != "escalated" && route.Status != "mixed" {
		return fmt.Errorf("maintenance status must be proposed, escalated, or mixed")
	}
	if route.Approval == "expert" && route.Status == "proposed" {
		return fmt.Errorf("expert maintenance must be escalated")
	}
	for name, values := range map[string][]string{"insights": route.Insights, "findings": route.Findings, "paths": route.Paths, "expert_targets": route.ExpertTargets} {
		if len(values) > 100 || !sort.StringsAreSorted(values) {
			return fmt.Errorf("maintenance %s must be a sorted bounded list", name)
		}
		for index, value := range values {
			if value == "" || len(value) > 512 || strings.ContainsAny(value, "\x00\r\n") || (index > 0 && value == values[index-1]) {
				return fmt.Errorf("maintenance %s contains an invalid value", name)
			}
		}
	}
	if len(route.Insights) == 0 || len(route.Paths) == 0 {
		return fmt.Errorf("maintenance attestation requires insights and paths")
	}
	return nil
}

func validateRuntimeMaintenanceClaim(expected *runtimeExchangeMaintenance, claimed *runtimeExchangeMaintenance) error {
	if !reflect.DeepEqual(expected, claimed) {
		return fmt.Errorf("maintenance attestation does not match the exchanged commits")
	}
	return nil
}

func validateRuntimeExpertBoundary(ctx context.Context, repository string, baseSHA string, headSHA string, route *runtimeExchangeMaintenance) error {
	if route == nil || route.Approval != "expert" {
		return nil
	}
	changed, err := runtimeGitChangedPaths(ctx, repository, baseSHA, headSHA)
	if err != nil {
		return err
	}
	insightPaths := make(map[string]bool, len(route.Paths))
	for _, path := range route.Paths {
		insightPaths[path] = true
	}
	for _, path := range changed {
		if insightPaths[path] {
			continue
		}
		for _, target := range route.ExpertTargets {
			if target == "." || path == target || strings.HasPrefix(path, strings.TrimSuffix(target, "/")+"/") {
				return fmt.Errorf("expert-only insight changed knowledge target %s", path)
			}
		}
	}
	return nil
}

func uniqueRuntimeStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func publishRuntimeExchangePullRequests(ctx context.Context, config okruntime.Config, checkout string, token string) error {
	runsDir := filepath.Join(config.Worker.ExchangeDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		root := filepath.Join(runsDir, entry.Name())
		publishedMarker := filepath.Join(root, "published.json")
		if _, err := os.Stat(publishedMarker); err == nil {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, "request.json"))
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var request runtimeExchangeRequest
		if err := okf.DecodeStrictJSON(content, &request); err != nil || request.Version != 1 || request.RunID != entry.Name() {
			failures = append(failures, fmt.Errorf("invalid agent exchange request %s", entry.Name()))
			continue
		}
		if !runtimeExchangeIdentifierPattern.MatchString(request.RunID) || !runtimeExchangeIdentifierPattern.MatchString(request.JobID) ||
			!runtimeExchangeSHA1Pattern.MatchString(request.BaseSHA) || !runtimeExchangeSHA1Pattern.MatchString(request.HeadSHA) ||
			request.VerifyCount < 0 || request.VerifyCount > 1000 {
			failures = append(failures, fmt.Errorf("invalid agent exchange fields for %s", entry.Name()))
			continue
		}
		if err := validateRuntimeExchangeEval(request.Eval); err != nil {
			failures = append(failures, fmt.Errorf("invalid agent exchange eval for %s: %w", request.RunID, err))
			continue
		}
		if err := validateRuntimeExchangeMaintenance(request.Maintenance); err != nil {
			failures = append(failures, fmt.Errorf("invalid agent exchange maintenance for %s: %w", request.RunID, err))
			continue
		}
		bundlePath := filepath.Join(root, "branch.bundle")
		bundleInfo, err := os.Stat(bundlePath)
		if err != nil || !bundleInfo.Mode().IsRegular() || bundleInfo.Size() <= 0 || bundleInfo.Size() > runtimeExchangeBundleMaxBytes {
			failures = append(failures, fmt.Errorf("invalid agent exchange bundle for %s", request.RunID))
			continue
		}
		digest, err := okf.SHA256File(bundlePath)
		if err != nil || digest != request.BundleSHA256 {
			failures = append(failures, fmt.Errorf("agent exchange bundle digest mismatch for %s", request.RunID))
			continue
		}
		if _, err := runtimeWorkerGit(ctx, config, "", checkout, "check-ref-format", "--branch", request.Branch); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange branch is invalid for %s", request.RunID))
			continue
		}
		if request.Branch == config.Worker.ProductionBranch {
			failures = append(failures, fmt.Errorf("agent exchange branch cannot be the production branch for %s", request.RunID))
			continue
		}
		ref := "refs/heads/" + request.Branch
		if output, err := runtimeWorkerGit(ctx, config, "", checkout, "fetch", bundlePath, ref+":"+ref); err != nil {
			failures = append(failures, fmt.Errorf("import agent exchange %s: %w: %s", request.RunID, err, output))
			continue
		}
		head, err := runtimeWorkerGit(ctx, config, "", checkout, "rev-parse", ref)
		if err != nil || head != request.HeadSHA {
			failures = append(failures, fmt.Errorf("agent exchange head mismatch for %s", request.RunID))
			continue
		}
		if _, err := runtimeWorkerGit(ctx, config, "", checkout, "merge-base", "--is-ancestor", request.BaseSHA, head); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange base is not an ancestor for %s", request.RunID))
			continue
		}
		if _, err := runtimeWorkerGit(ctx, config, "", checkout, "merge-base", "--is-ancestor", request.BaseSHA, "refs/heads/"+config.Worker.ProductionBranch); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange base is not production history for %s", request.RunID))
			continue
		}
		expectedMaintenance, err := runtimeMaintenanceAttestation(ctx, checkout, request.BaseSHA, head)
		if err != nil {
			failures = append(failures, fmt.Errorf("agent exchange maintenance reconstruction failed for %s: %w", request.RunID, err))
			continue
		}
		if err := validateRuntimeMaintenanceClaim(expectedMaintenance, request.Maintenance); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange maintenance claim failed for %s: %w", request.RunID, err))
			continue
		}
		if err := validateRuntimeExpertBoundary(ctx, checkout, request.BaseSHA, head, request.Maintenance); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange expert boundary failed for %s: %w", request.RunID, err))
			continue
		}
		if err := validateRuntimeExchangeCommit(ctx, config, checkout, head); err != nil {
			failures = append(failures, fmt.Errorf("agent exchange validation failed for %s: %w", request.RunID, err))
			continue
		}
		if output, err := runtimeWorkerGit(ctx, config, token, checkout, "push", config.Worker.Remote, ref+":"+ref); err != nil {
			failures = append(failures, fmt.Errorf("agent run %s push branch: %w: %s", request.RunID, err, output))
			continue
		}
		publication, err := publishRuntimeGitHubRequest(ctx, config, token, request)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := writeExchangeJSON(publishedMarker, publication); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := os.Remove(bundlePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove published agent exchange bundle %s: %w", request.RunID, err))
		}
		runtimeInfof("runtime publisher published agent run %s as draft PR #%d\n", request.RunID, publication.PR)
	}
	return errors.Join(failures...)
}

func listRuntimeAgentRuns(config okruntime.Config, checkout string, runtimeName string) ([]agents.RunSummary, []agents.RunIssue, error) {
	state := filepath.Join(config.Runtime.StateDir, "jobs-"+runtimeName)
	previous, present := os.LookupEnv(agents.JobsStateDirEnv)
	if err := os.Setenv(agents.JobsStateDirEnv, state); err != nil {
		return nil, nil, err
	}
	defer func() {
		if present {
			_ = os.Setenv(agents.JobsStateDirEnv, previous)
		} else {
			_ = os.Unsetenv(agents.JobsStateDirEnv)
		}
	}()
	runs, issues, _, err := agents.ListRuns(checkout)
	return runs, issues, err
}

func cleanupRuntimeAgentRuns(ctx context.Context, config okruntime.Config, checkout string, runtimeName string) error {
	runs, issues, err := listRuntimeAgentRuns(config, checkout, runtimeName)
	if err != nil {
		return err
	}
	var failures []error
	for _, issue := range issues {
		failures = append(failures, fmt.Errorf("agent run %s: %s", issue.Path, issue.Error))
	}
	removedWorktree := false
	for _, summary := range runs {
		switch summary.Status {
		case "running", "stopping", "killing":
			continue
		}
		content, err := os.ReadFile(summary.RunRecord)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var record agents.RunRecord
		if err := json.Unmarshal(content, &record); err != nil {
			failures = append(failures, err)
			continue
		}
		if summary.Status == "succeeded" && record.Plan.Output.PR {
			if _, err := os.Stat(filepath.Join(filepath.Dir(summary.RunRecord), "exchange.json")); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				failures = append(failures, err)
				continue
			}
		}
		if record.Plan.Worktree != "" {
			runRoot := filepath.Dir(summary.RunRecord)
			repositoryStateRoot := filepath.Dir(filepath.Dir(runRoot))
			worktreesRoot := filepath.Join(repositoryStateRoot, "worktrees")
			if !runtimeWorkerPathInside(worktreesRoot, record.Plan.Worktree) {
				failures = append(failures, fmt.Errorf("agent run %s worktree is outside runtime state: %s", record.RunID, record.Plan.Worktree))
				continue
			}
			if _, err := os.Stat(record.Plan.Worktree); err == nil {
				if output, removeErr := runtimeWorkerGit(ctx, config, "", checkout, "worktree", "remove", "--force", record.Plan.Worktree); removeErr != nil {
					if fallbackErr := os.RemoveAll(record.Plan.Worktree); fallbackErr != nil {
						failures = append(failures, fmt.Errorf("remove agent run %s worktree: %w: %s; fallback: %v", record.RunID, removeErr, output, fallbackErr))
						continue
					}
				}
				removedWorktree = true
			} else if !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, err)
				continue
			}
		}
		for _, artifact := range []string{"home", "tmp", "diff.patch"} {
			if err := os.RemoveAll(filepath.Join(filepath.Dir(summary.RunRecord), artifact)); err != nil {
				failures = append(failures, fmt.Errorf("remove agent run %s artifact %s: %w", record.RunID, artifact, err))
			}
		}
	}
	if removedWorktree {
		if output, err := runtimeWorkerGit(ctx, config, "", checkout, "worktree", "prune"); err != nil {
			failures = append(failures, fmt.Errorf("prune agent worktrees: %w: %s", err, output))
		}
	}
	return errors.Join(failures...)
}

func runtimeWorkerPathInside(root string, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))))
}

func runtimeEnvironmentWith(environment []string, name string, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func runtimeEnvironmentWithout(environment []string, names ...string) []string {
	blocked := make(map[string]bool, len(names))
	for _, name := range names {
		if name != "" {
			blocked[name] = true
		}
	}
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		name := item
		if equals := strings.IndexByte(item, '='); equals >= 0 {
			name = item[:equals]
		}
		if !blocked[name] {
			result = append(result, item)
		}
	}
	return result
}

func publishRuntimeGitHubRequest(ctx context.Context, config okruntime.Config, token string, request runtimeExchangeRequest) (runtimeGitHubPublication, error) {
	if !config.GitHub.Enabled || token == "" {
		return runtimeGitHubPublication{}, fmt.Errorf("agent run %s requests output.pr but github integration is not enabled", request.RunID)
	}
	client := okruntime.GitHubClient{APIURL: config.GitHub.APIURL, Repository: config.GitHub.Repository, Token: token, HTTPClient: runtimeWorkerGitHubHTTPClient}
	owner := strings.SplitN(config.GitHub.Repository, "/", 2)[0]
	pull, err := client.FindOpenPullRequest(ctx, owner, request.Branch, config.Worker.ProductionBranch)
	if err != nil {
		return runtimeGitHubPublication{}, fmt.Errorf("agent run %s find pull request: %w", request.RunID, err)
	}
	if pull == nil {
		draft := config.GitHub.DraftPullRequest
		if request.Maintenance != nil && request.Maintenance.Approval == "auto" {
			draft = false
		}
		created, err := client.CreateDraftPullRequest(ctx,
			"chore(knowledge): "+request.JobID,
			request.Branch,
			config.Worker.ProductionBranch,
			runtimeExchangePullRequestSummary(request),
			draft,
		)
		if err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s create pull request: %w", request.RunID, err)
		}
		pull = &created
	}
	if request.Maintenance != nil && request.Maintenance.Approval != "auto" {
		reviewers, teams := runtimeGitHubReviewers(request.Maintenance.Owners)
		if len(reviewers)+len(teams) > 0 {
			if err := client.RequestReviewers(ctx, pull.Number, reviewers, teams); err != nil {
				return runtimeGitHubPublication{}, fmt.Errorf("agent run %s request maintenance reviewers: %w", request.RunID, err)
			}
		}
	}
	checked := false
	if config.GitHub.Checks {
		if err := client.CreateCompletedCheck(ctx,
			"Open Knowledge / "+request.JobID,
			request.HeadSHA,
			"Maintenance validation passed",
			runtimeExchangeCheckSummary(request, pull.HTMLURL),
			"success",
		); err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s create check: %w", request.RunID, err)
		}
		checked = true
	}
	publication := runtimeGitHubPublication{RunID: request.RunID, Commit: request.HeadSHA, PR: pull.Number, PRURL: pull.HTMLURL, Checked: checked}
	if request.Maintenance != nil && request.Maintenance.Approval == "auto" && config.GitHub.AutoMergeLowRisk {
		if _, err := client.RequireSuccessfulChecks(ctx, request.HeadSHA, config.GitHub.RequiredChecks); err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s low-risk auto-merge checks: %w", request.RunID, err)
		}
		if err := client.MergePullRequest(ctx, pull.Number, request.HeadSHA); err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s low-risk auto-merge: %w", request.RunID, err)
		}
		publication.Merged = true
	}
	return publication, nil
}

func runtimeGitHubReviewers(owners []string) ([]string, []string) {
	var reviewers []string
	var teams []string
	for _, owner := range owners {
		switch {
		case strings.HasPrefix(owner, "github-team:"):
			teams = append(teams, strings.TrimPrefix(owner, "github-team:"))
		case strings.HasPrefix(owner, "github:"):
			reviewers = append(reviewers, strings.TrimPrefix(owner, "github:"))
		}
	}
	return uniqueRuntimeStrings(reviewers), uniqueRuntimeStrings(teams)
}

func validateRuntimeExchangeCommit(ctx context.Context, config okruntime.Config, checkout string, head string) error {
	parent := filepath.Join(config.Runtime.StateDir, "verification-worktrees")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	worktree, err := os.MkdirTemp(parent, ".verify-*")
	if err != nil {
		return err
	}
	_ = os.Remove(worktree)
	if output, err := runtimeWorkerGit(ctx, config, "", checkout, "worktree", "add", "--detach", worktree, head); err != nil {
		return fmt.Errorf("create verification worktree: %w: %s", err, output)
	}
	defer func() {
		_, _ = runtimeWorkerGit(context.Background(), config, "", checkout, "worktree", "remove", "--force", worktree)
		_ = os.RemoveAll(worktree)
	}()
	for _, knowledge := range config.KnowledgeBases {
		if !knowledge.Publish {
			continue
		}
		relative, err := filepath.Rel(config.Root, knowledge.Path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("knowledge base %s path is outside repository", knowledge.ID)
		}
		validation, err := okf.ValidateWithVersion(filepath.Join(worktree, relative), knowledge.Spec)
		if err != nil {
			return err
		}
		if err := okf.RequireValidBundle(validation); err != nil {
			return err
		}
		if _, err := okf.BuildPublicationSetWithVersion(filepath.Join(worktree, relative), knowledge.Spec); err != nil {
			return fmt.Errorf("knowledge base %s publication contract: %w", knowledge.ID, err)
		}
	}
	return nil
}

func runtimeExchangePullRequestSummary(request runtimeExchangeRequest) string {
	summary := fmt.Sprintf("Automated Open Knowledge maintenance completed.\n\n- Job: `%s`\n- Run: `%s`\n- Base commit: `%s`\n- Agent verification commands reported: %d\n", request.JobID, request.RunID, request.BaseSHA, request.VerifyCount)
	if request.Maintenance != nil {
		summary += fmt.Sprintf("- Maintenance route: `%s` risk, `%s` approval, %.0f%% confidence\n- Owners: %s\n- Insight attestations: %d (%s)\n", request.Maintenance.Risk, request.Maintenance.Approval, request.Maintenance.Confidence*100, runtimeMarkdownInline(strings.Join(request.Maintenance.Owners, ", ")), len(request.Maintenance.Insights), request.Maintenance.Status)
	}
	if request.Eval != nil {
		summary += fmt.Sprintf("- Knowledge eval `%s` reported: passed (`%s`, %d regressions, %d proposed failures)\n", runtimeMarkdownInline(request.Eval.Dataset), request.Eval.Gate, request.Eval.Regressions, request.Eval.ProposedFailed)
	}
	return summary + "- Publisher OKF and publication validation: passed\n\nRaw prompts, tool calls, environment metadata, and runtime logs remain private."
}

func runtimeExchangeCheckSummary(request runtimeExchangeRequest, pullRequestURL string) string {
	eval := ""
	if request.Eval != nil {
		eval = fmt.Sprintf(" The worker reported eval `%s` passed gate `%s` with %d regressions and %d proposed failures.", runtimeMarkdownInline(request.Eval.Dataset), request.Eval.Gate, request.Eval.Regressions, request.Eval.ProposedFailed)
	}
	route := ""
	if request.Maintenance != nil {
		route = fmt.Sprintf(" Maintenance route is `%s/%s` at %.0f%% confidence for %d insight attestations.", request.Maintenance.Risk, request.Maintenance.Approval, request.Maintenance.Confidence*100, len(request.Maintenance.Insights))
	}
	return fmt.Sprintf("Job `%s` reported %d verification commands.%s%s The credentialed publisher independently validated every OKF bundle and public publication contract. Pull request: %s. Raw execution data remains in private agent storage.",
		request.JobID, request.VerifyCount, eval, route, pullRequestURL)
}

func validateRuntimeExchangeEval(eval *runtimeExchangeEval) error {
	if eval == nil {
		return nil
	}
	if eval.Status != "pass" || (eval.Gate != "all" && eval.Gate != "regressions") || eval.Regressions < 0 || eval.ProposedFailed < 0 {
		return fmt.Errorf("eval must report a valid passing gate")
	}
	if !runtimeExchangeSHA1Pattern.MatchString(eval.Base) {
		return fmt.Errorf("eval base must be an immutable commit SHA")
	}
	for _, item := range []struct{ field, value string }{{"dataset", eval.Dataset}, {"target", eval.Target}} {
		field, value := item.field, item.value
		clean := filepath.ToSlash(filepath.Clean(value))
		if value == "" || len(value) > 4096 || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsAny(value, "\r\n`") {
			return fmt.Errorf("eval %s must be a safe repository-relative path", field)
		}
	}
	return nil
}

func runtimeMarkdownInline(value string) string {
	return strings.ReplaceAll(value, "`", "'")
}

func writeExchangeJSON(target string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0644); err != nil {
		return err
	}
	return os.Chmod(target, 0644)
}

func writePrivateRuntimeJSON(target string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.WriteFile(target, content, 0600); err != nil {
		return err
	}
	return os.Chmod(target, 0600)
}

func runtimeWorkerHelpText() string {
	return `openknowledge automation runtime worker --config runtime.toml [--once] [--role publisher|jobs|all]

Run the private, ingress-free reconciliation loop. Production Docker deployment
uses separate publisher and jobs processes with isolated state volumes and an
untrusted Git-bundle exchange: publisher alone receives GitHub credentials and
artifact write access; jobs alone receive the model credential. The combined
all role is for local use without GitHub credentials.

Agent commands receive only each job's explicit sandbox.env allowlist. Raw run
records and logs remain under the private state directory and are never copied
into a public generation.
`
}
