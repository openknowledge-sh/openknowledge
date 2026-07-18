package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

var runtimeWorkerTokenCache struct {
	sync.Mutex
	key       string
	token     string
	expiresAt time.Time
}

var runtimeExchangeIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var runtimeExchangeSHA1Pattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

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
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	once := flags.Bool("once", false, "run one reconciliation pass and exit")
	role := flags.String("role", "publisher", "worker role: publisher, jobs, or all")
	agentRuntime := flags.String("runtime", "", "jobs harness runtime: codex, claude, or opencode")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "runtime worker accepts no positional arguments")
		return 2
	}
	if *role != "publisher" && *role != "jobs" && *role != "all" {
		fmt.Fprintln(os.Stderr, "--role must be publisher, jobs, or all")
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
			fmt.Fprintf(os.Stderr, "runtime worker %s pass failed: %v\n", *role, passErr)
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
	fmt.Fprintf(os.Stderr, "runtime worker synchronized %s at %s\n", config.Worker.ProductionBranch, commit)
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
		result, err := buildRuntimeKnowledgeGeneration(config, mapped, commit, out, true)
		if err != nil {
			failures = append(failures, fmt.Errorf("publish %s: %w", mapped.ID, err))
			continue
		}
		fmt.Fprintf(os.Stderr, "runtime worker published %s generation %s\n", mapped.ID, result.Generation)
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
	if err := runRuntimeAgentPass(ctx, config, checkout, runtimeName); err != nil {
		return err
	}
	return exportRuntimeAgentPullRequests(ctx, config, checkout)
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
			"GIT_CONFIG_VALUE_0=AUTHORIZATION: bearer "+token,
		)
	}
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
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
	command.Stderr = os.Stderr
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
}

type runtimeExchangeRequest struct {
	Version      int    `json:"version"`
	RunID        string `json:"run_id"`
	JobID        string `json:"job_id"`
	Branch       string `json:"branch"`
	BaseSHA      string `json:"base_sha"`
	HeadSHA      string `json:"head_sha"`
	BundleSHA256 string `json:"bundle_sha256"`
	VerifyCount  int    `json:"verify_count"`
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

func exportRuntimeAgentPullRequests(ctx context.Context, config okruntime.Config, checkout string) error {
	runs, issues, err := listRuntimeAgentRuns(config, checkout)
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
			}
			_ = writePrivateRuntimeJSON(marker, map[string]any{"run_id": record.RunID, "exported": true})
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
		request := runtimeExchangeRequest{Version: 1, RunID: record.RunID, JobID: record.JobID, Branch: record.Plan.Branch, BaseSHA: record.Plan.BaseSHA, HeadSHA: headSHA, BundleSHA256: bundleSHA, VerifyCount: len(record.Verify)}
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
		}
		_ = writePrivateRuntimeJSON(marker, map[string]any{"run_id": record.RunID, "exported": true})
		fmt.Fprintf(os.Stderr, "runtime agent worker exported run %s for private publication\n", record.RunID)
	}
	return errors.Join(failures...)
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
		fmt.Fprintf(os.Stderr, "runtime publisher published agent run %s as draft PR #%d\n", request.RunID, publication.PR)
	}
	return errors.Join(failures...)
}

func listRuntimeAgentRuns(config okruntime.Config, checkout string) ([]agents.RunSummary, []agents.RunIssue, error) {
	state := filepath.Join(config.Runtime.StateDir, "jobs")
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
	client := okruntime.GitHubClient{APIURL: config.GitHub.APIURL, Repository: config.GitHub.Repository, Token: token}
	owner := strings.SplitN(config.GitHub.Repository, "/", 2)[0]
	pull, err := client.FindOpenPullRequest(ctx, owner, request.Branch, config.Worker.ProductionBranch)
	if err != nil {
		return runtimeGitHubPublication{}, fmt.Errorf("agent run %s find pull request: %w", request.RunID, err)
	}
	if pull == nil {
		created, err := client.CreateDraftPullRequest(ctx,
			"chore(knowledge): "+request.JobID,
			request.Branch,
			config.Worker.ProductionBranch,
			runtimeExchangePullRequestSummary(request),
			config.GitHub.DraftPullRequest,
		)
		if err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s create pull request: %w", request.RunID, err)
		}
		pull = &created
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
	return runtimeGitHubPublication{RunID: request.RunID, Commit: request.HeadSHA, PR: pull.Number, PRURL: pull.HTMLURL, Checked: checked}, nil
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
	return fmt.Sprintf("Automated Open Knowledge maintenance completed.\n\n- Job: `%s`\n- Run: `%s`\n- Base commit: `%s`\n- Agent verification commands reported: %d\n- Publisher OKF and publication validation: passed\n\nRaw prompts, tool calls, environment metadata, and runtime logs remain private.",
		request.JobID, request.RunID, request.BaseSHA, request.VerifyCount)
}

func runtimeExchangeCheckSummary(request runtimeExchangeRequest, pullRequestURL string) string {
	return fmt.Sprintf("Job `%s` reported %d verification commands and the credentialed publisher independently validated every OKF bundle and public publication contract. Draft pull request: %s. Raw execution data remains in private agent storage.",
		request.JobID, request.VerifyCount, pullRequestURL)
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
	return `openknowledge runtime worker --config runtime.toml [--once] [--role publisher|jobs|all]

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
