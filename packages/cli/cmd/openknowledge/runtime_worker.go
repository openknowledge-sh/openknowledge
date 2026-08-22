package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/insights"
	knowledgeintervention "github.com/openknowledge-sh/openknowledge/packages/cli/internal/intervention"
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
			if err := reconcileRuntimeInterventionPublication(config, mapped.ID, commit); err != nil {
				failures = append(failures, fmt.Errorf("publish %s intervention log: %w", mapped.ID, err))
			}
			continue
		}
		publicationChecks := append([]string{}, verifiedChecks...)
		if config.Worker.KnowledgeCI {
			if err := runtimeKnowledgeCIPass(config, checkout, mapped, commit); err != nil {
				failures = append(failures, fmt.Errorf("knowledge CI %s: %w", mapped.ID, err))
				continue
			}
			publicationChecks = append(publicationChecks, "openknowledge-runtime-ci")
			sort.Strings(publicationChecks)
		}
		out := filepath.Join(config.Runtime.StateDir, "builds", mapped.ID)
		result, err := buildRuntimeKnowledgeGenerationWithChecks(config, mapped, commit, out, true, publicationChecks)
		if err != nil {
			failures = append(failures, fmt.Errorf("publish %s: %w", mapped.ID, err))
			continue
		}
		if err := reconcileRuntimeInterventionPublication(config, mapped.ID, commit); err != nil {
			failures = append(failures, fmt.Errorf("publish %s intervention log: %w", mapped.ID, err))
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

func runtimeKnowledgeCIPass(config okruntime.Config, checkout string, knowledge okruntime.KnowledgeBaseConfig, commit string) (resultErr error) {
	reportDir := filepath.Join(config.Runtime.StateDir, "reports", "knowledge-ci", commit, knowledge.ID)
	if err := os.MkdirAll(reportDir, 0o700); err != nil {
		return err
	}
	index := fmt.Sprintf(`---
type: Open Knowledge Artifact
title: Runtime Knowledge CI
status: stable
---

# Runtime Knowledge CI

- Commit: %s
- Knowledge base: %s
- [Validation](validation.json)
- [Claim lifecycle](claims-validation.json)
- [Audit report](audit.md)
- [Audit data](audit.json)
- [Eval report](eval.md)
- [Eval data](eval.json)
`, commit, knowledge.ID)
	if err := writeOutputFileAtomically(filepath.Join(reportDir, "index.md"), []byte(index)); err != nil {
		return err
	}
	defer func() {
		if err := finalizeRuntimeKnowledgeCIArtifact(reportDir, commit); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	validation, err := okf.ValidateWithVersion(knowledge.Path, knowledge.Spec)
	if err != nil {
		return err
	}
	if err := writeRuntimeKnowledgeCIJSON(reportDir, "validation.json", validation); err != nil {
		return err
	}
	if err := okf.RequireValidBundle(validation); err != nil {
		return err
	}

	candidate, err := claimops.BuildIndex(knowledge.Path, knowledge.Spec, time.Now().UTC())
	if err != nil {
		return err
	}
	claimsReport := claimsValidationReport{
		SchemaVersion: okf.MachineSchemaVersion, Root: knowledge.Path, Valid: len(candidate.Issues) == 0,
		Issues: nonNilIssues(candidate.Issues), Lifecycle: []okf.Issue{}, AuthorityChanges: []claimops.AuthorityChange{},
	}
	baseCommit := ""
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	if _, activeRoot, activeErr := store.Active(knowledge.ID); activeErr == nil {
		manifest, manifestErr := okruntime.LoadAndValidateGeneration(activeRoot)
		if manifestErr != nil {
			return manifestErr
		}
		baseRoot := filepath.Join(activeRoot, "source")
		base, baseErr := claimops.BuildIndex(baseRoot, knowledge.Spec, time.Now().UTC())
		if baseErr != nil {
			return baseErr
		}
		lifecycle := claimops.CompareLifecycle(base, candidate)
		claimsReport.Against = baseRoot
		claimsReport.Lifecycle = lifecycle.Issues
		claimsReport.AuthorityChanges = lifecycle.AuthorityChanges
		claimsReport.Valid = claimsReport.Valid && lifecycle.Valid
		baseCommit = manifest.Commit
	} else if !os.IsNotExist(activeErr) {
		return activeErr
	}
	if err := writeRuntimeKnowledgeCIJSON(reportDir, "claims-validation.json", claimsReport); err != nil {
		return err
	}
	if !claimsReport.Valid {
		return fmt.Errorf("claim lifecycle gate failed")
	}

	baselinePath := filepath.Join(checkout, ".openknowledge", "audit-sources.json")
	baseline, err := knowledgeaudit.ReadBaseline(baselinePath)
	if err != nil {
		return fmt.Errorf("read audit baseline: %w", err)
	}
	auditReport, _, err := knowledgeaudit.Run(knowledgeaudit.Options{
		Root: knowledge.Path, Spec: knowledge.Spec, Baseline: &baseline, ObserveRemote: true,
	})
	if err != nil {
		return err
	}
	if err := writeRuntimeKnowledgeCIJSON(reportDir, "audit.json", auditReport); err != nil {
		return err
	}
	if err := writeOutputFileAtomically(filepath.Join(reportDir, "audit.md"), []byte(knowledgeaudit.RenderMarkdown(auditReport))); err != nil {
		return err
	}
	if auditFails(auditReport, "high") {
		return fmt.Errorf("audit high-severity gate failed")
	}

	datasetPath := filepath.Join(checkout, ".openknowledge", "evals", "knowledge.yaml")
	dataset, err := knowledgeeval.LoadDataset(datasetPath)
	if err != nil {
		return fmt.Errorf("load runtime eval dataset: %w", err)
	}
	if baseCommit == "" {
		report, evalErr := knowledgeeval.Run(knowledge.Path, knowledge.Spec, dataset)
		if err := writeRuntimeKnowledgeCIJSON(reportDir, "eval.json", report); err != nil {
			return err
		}
		if err := writeOutputFileAtomically(filepath.Join(reportDir, "eval.md"), []byte(knowledgeeval.RenderMarkdown(report))); err != nil {
			return err
		}
		if evalErr != nil {
			return evalErr
		}
		if report.Summary.Status != "pass" {
			return fmt.Errorf("runtime eval gate failed")
		}
	} else {
		report, evalErr := knowledgeeval.Compare(knowledge.Path, knowledge.Spec, dataset, baseCommit, knowledgeeval.GateRegressions)
		if err := writeRuntimeKnowledgeCIJSON(reportDir, "eval.json", report); err != nil {
			return err
		}
		if err := writeOutputFileAtomically(filepath.Join(reportDir, "eval.md"), []byte(knowledgeeval.RenderComparisonMarkdown(report))); err != nil {
			return err
		}
		if evalErr != nil {
			return evalErr
		}
		if report.Summary.Status != "pass" {
			return fmt.Errorf("runtime eval regression gate failed")
		}
	}
	return nil
}

func writeRuntimeKnowledgeCIJSON(directory string, name string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeOutputFileAtomically(filepath.Join(directory, name), append(content, '\n'))
}

func finalizeRuntimeKnowledgeCIArtifact(directory string, commit string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && entry.Name() != "artifact.json" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	manifest := map[string]any{
		"type": "openknowledge.artifact", "version": 1, "kind": "knowledge-ci",
		"runId": commit, "base": commit, "createdAt": time.Now().UTC().Format(time.RFC3339), "files": files,
	}
	return writeRuntimeKnowledgeCIJSON(directory, "artifact.json", manifest)
}

func reconcileRuntimeInterventionPublication(config okruntime.Config, knowledgeBase string, commit string) error {
	logRoot := filepath.Join(config.Runtime.StateDir, "interventions")
	events, err := knowledgeintervention.Read([]string{logRoot})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	mergedRuns, err := runtimeMergedInterventionRuns(config, commit)
	if err != nil {
		return err
	}
	if len(mergedRuns) == 0 {
		return nil
	}
	published := map[string]bool{}
	var proposals []knowledgeintervention.Event
	for _, event := range events {
		if event.Stage == "published" {
			published[event.InterventionID] = true
		}
		if event.Stage == "proposed" && event.KnowledgeBase == knowledgeBase && mergedRuns[event.Source.ID] {
			proposals = append(proposals, event)
		}
	}
	if len(proposals) == 0 {
		return nil
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	pointer, generationRoot, err := store.Active(knowledgeBase)
	if err != nil {
		return err
	}
	manifest, err := okruntime.LoadAndValidateGeneration(generationRoot)
	if err != nil {
		return err
	}
	if manifest.Commit != commit || pointer.ContentDigest != manifest.ContentDigest || len(manifest.Checks) == 0 {
		return fmt.Errorf("active generation does not prove intervention commit and checks")
	}
	activeInfo, err := os.Stat(filepath.Join(config.ArtifactStore.Path, knowledgeBase, okruntime.ActivePointerFile))
	if err != nil {
		return err
	}
	publishedAt := activeInfo.ModTime().UTC()
	recorder, err := knowledgeintervention.NewRecorder(logRoot)
	if err != nil {
		return err
	}
	for _, proposal := range proposals {
		if published[proposal.InterventionID] {
			continue
		}
		proposedAt, _ := time.Parse(time.RFC3339Nano, proposal.At)
		if !publishedAt.After(proposedAt) {
			return fmt.Errorf("active generation predates intervention proposal %s", proposal.InterventionID)
		}
		event := proposal
		event.ID = runtimeInterventionIdentity(proposal.Source.ID, knowledgeBase, "published")
		event.At = publishedAt.Format(time.RFC3339Nano)
		event.Stage = "published"
		event.Actor = knowledgeintervention.Actor{Kind: "system", ID: "runtime-publisher"}
		event.Publication = &knowledgeintervention.Publication{
			Generation: pointer.Generation, ContentDigest: manifest.ContentDigest,
			Checks: append([]string{}, manifest.Checks...), Automated: true, Verified: true,
		}
		if _, err := recorder.AppendIfMissing(event); err != nil {
			return err
		}
	}
	return nil
}

func runtimeMergedInterventionRuns(config okruntime.Config, commit string) (map[string]bool, error) {
	result := map[string]bool{}
	runsDir := filepath.Join(config.Worker.ExchangeDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(runsDir, entry.Name(), "published.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var publication runtimeGitHubPublication
		if err := okf.DecodeStrictJSON(content, &publication); err != nil {
			return nil, fmt.Errorf("invalid published agent exchange %s: %w", entry.Name(), err)
		}
		if publication.RunID != entry.Name() || !runtimeExchangeIdentifierPattern.MatchString(publication.RunID) || !runtimeExchangeSHA1Pattern.MatchString(publication.Commit) {
			return nil, fmt.Errorf("invalid published agent exchange identity: %s", entry.Name())
		}
		if publication.Merged && publication.Commit == commit {
			result[publication.RunID] = true
		}
	}
	return result, nil
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
	return err == nil && manifest.Commit == commit && equalStringLists(manifest.Checks, runtimeRequiredPublicationChecks(config))
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
	ProposedAt   string                      `json:"proposed_at,omitempty"`
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
	DetectedAt    string   `json:"detected_at,omitempty"`
}

type runtimeExchangeEval struct {
	Status         string `json:"status"`
	Dataset        string `json:"dataset"`
	Target         string `json:"target"`
	Base           string `json:"base"`
	Gate           string `json:"gate"`
	Regressions    int    `json:"regressions"`
	ProposedFailed int    `json:"proposed_failed"`
	Total          int    `json:"total"`
	BasePassed     int    `json:"base_passed"`
	ProposedPassed int    `json:"proposed_passed"`
}

type runtimeClaimReview struct {
	Changes     []runtimeClaimChange
	Authorities []runtimeAuthorityChange
}

type runtimeAuthorityChange struct {
	Knowledge  string
	Path       string
	SourceID   string
	Resource   string
	ApprovedBy string
}

type runtimeClaimChange struct {
	Knowledge    string
	ID           string
	Path         string
	BeforeValue  string
	AfterValue   string
	BeforeStatus string
	AfterStatus  string
	Sources      []string
	Documents    int
	Evals        int
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
		request := runtimeExchangeRequest{Version: 1, RunID: record.RunID, JobID: record.JobID, Branch: record.Plan.Branch, BaseSHA: record.Plan.BaseSHA, HeadSHA: headSHA, BundleSHA256: bundleSHA, VerifyCount: len(record.Verify), ProposedAt: record.FinishedAt.UTC().Format(time.RFC3339Nano), Maintenance: maintenance}
		if record.Eval != nil {
			request.Eval = &runtimeExchangeEval{
				Status: record.Eval.Status, Dataset: record.Eval.Dataset, Target: record.Eval.Target, Base: record.Eval.Base, Gate: record.Eval.Gate,
				Regressions: record.Eval.Regressions, ProposedFailed: record.Eval.ProposedFailed, Total: record.Eval.Total,
				BasePassed: record.Eval.BasePassed, ProposedPassed: record.Eval.ProposedPassed,
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
		if result.DetectedAt == "" || item.CreatedAt.UTC().Format(time.RFC3339Nano) < result.DetectedAt {
			result.DetectedAt = item.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
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
	if route.DetectedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, route.DetectedAt); err != nil {
			return fmt.Errorf("maintenance detection time is invalid")
		}
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
		if request.ProposedAt != "" {
			proposedAt, timeErr := time.Parse(time.RFC3339Nano, request.ProposedAt)
			if timeErr != nil {
				failures = append(failures, fmt.Errorf("invalid agent exchange proposal time for %s", request.RunID))
				continue
			}
			if request.Maintenance != nil && request.Maintenance.DetectedAt != "" {
				detectedAt, _ := time.Parse(time.RFC3339Nano, request.Maintenance.DetectedAt)
				if !detectedAt.Before(proposedAt) {
					failures = append(failures, fmt.Errorf("agent exchange proposal precedes its detection for %s", request.RunID))
					continue
				}
			}
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
		claimReview, err := validateRuntimeExchangeCommit(ctx, config, checkout, request.BaseSHA, head)
		if err != nil {
			failures = append(failures, fmt.Errorf("agent exchange validation failed for %s: %w", request.RunID, err))
			continue
		}
		if output, err := runtimeWorkerGit(ctx, config, token, checkout, "push", config.Worker.Remote, ref+":"+ref); err != nil {
			failures = append(failures, fmt.Errorf("agent run %s push branch: %w: %s", request.RunID, err, output))
			continue
		}
		if err := recordRuntimeInterventionProposal(ctx, config, checkout, request); err != nil {
			failures = append(failures, fmt.Errorf("agent run %s intervention log: %w", request.RunID, err))
			continue
		}
		publication, err := publishRuntimeGitHubRequest(ctx, config, token, request, claimReview)
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

func recordRuntimeInterventionProposal(ctx context.Context, config okruntime.Config, checkout string, request runtimeExchangeRequest) error {
	if request.Maintenance == nil {
		return nil
	}
	if request.ProposedAt == "" || request.Maintenance.DetectedAt == "" {
		return fmt.Errorf("maintenance exchange lacks detection or proposal time")
	}
	targets, err := runtimeInterventionTargets(ctx, config, checkout, request)
	if err != nil {
		return err
	}
	evidence := []string{"head-commit:" + request.HeadSHA, "job-run:" + request.RunID}
	for _, insight := range request.Maintenance.Insights {
		evidence = append(evidence, "insight:"+insight)
	}
	for _, finding := range request.Maintenance.Findings {
		evidence = append(evidence, "audit-finding:"+finding)
	}
	if request.Eval != nil {
		evidence = append(evidence, "eval:"+request.Eval.Dataset)
	}
	evidence = uniqueRuntimeStrings(evidence)
	recorder, err := knowledgeintervention.NewRecorder(filepath.Join(config.Runtime.StateDir, "interventions"))
	if err != nil {
		return err
	}
	knowledgeBases := make([]string, 0, len(targets))
	for knowledgeBase := range targets {
		knowledgeBases = append(knowledgeBases, knowledgeBase)
	}
	sort.Strings(knowledgeBases)
	for _, knowledgeBase := range knowledgeBases {
		paths := targets[knowledgeBase]
		interventionID := runtimeInterventionIdentity(request.RunID, knowledgeBase, "intervention")
		base := knowledgeintervention.Event{
			Type: knowledgeintervention.EventType, Version: knowledgeintervention.EventVersion,
			InterventionID: interventionID, KnowledgeBase: knowledgeBase,
			Actor:  knowledgeintervention.Actor{Kind: "agent", ID: "job:" + request.JobID},
			Source: knowledgeintervention.Source{Kind: "job-run", ID: request.RunID},
			Route: knowledgeintervention.Route{
				Risk: request.Maintenance.Risk, Approval: request.Maintenance.Approval,
				Confidence: request.Maintenance.Confidence, Owners: append([]string{}, request.Maintenance.Owners...),
			},
			Targets: paths, Evidence: evidence,
		}
		detected := base
		detected.ID = runtimeInterventionIdentity(request.RunID, knowledgeBase, "detected")
		detected.At, detected.Stage = request.Maintenance.DetectedAt, "detected"
		if _, err := recorder.AppendIfMissing(detected); err != nil {
			return err
		}
		proposed := base
		proposed.ID = runtimeInterventionIdentity(request.RunID, knowledgeBase, "proposed")
		proposed.At, proposed.Stage = request.ProposedAt, "proposed"
		if _, err := recorder.AppendIfMissing(proposed); err != nil {
			return err
		}
	}
	return nil
}

func runtimeInterventionTargets(ctx context.Context, config okruntime.Config, checkout string, request runtimeExchangeRequest) (map[string][]string, error) {
	changed, err := runtimeGitChangedPaths(ctx, checkout, request.BaseSHA, request.HeadSHA)
	if err != nil {
		return nil, err
	}
	result := map[string][]string{}
	for _, knowledge := range config.KnowledgeBases {
		if !knowledge.Publish {
			continue
		}
		prefix, err := filepath.Rel(config.Root, knowledge.Path)
		if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) || filepath.IsAbs(prefix) {
			return nil, fmt.Errorf("knowledge base %s cannot bind intervention paths", knowledge.ID)
		}
		prefix = filepath.ToSlash(filepath.Clean(prefix))
		for _, path := range changed {
			relative := path
			if prefix != "." {
				if !strings.HasPrefix(path, prefix+"/") {
					continue
				}
				relative = strings.TrimPrefix(path, prefix+"/")
			}
			if relative != "" {
				result[knowledge.ID] = append(result[knowledge.ID], relative)
			}
		}
		if len(result[knowledge.ID]) > 0 {
			result[knowledge.ID] = uniqueRuntimeStrings(result[knowledge.ID])
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("maintenance exchange changes no published knowledge base")
	}
	return result, nil
}

func runtimeInterventionIdentity(runID string, knowledgeBase string, stage string) string {
	digest := sha256.Sum256([]byte("openknowledge-intervention\x00" + runID + "\x00" + knowledgeBase + "\x00" + stage))
	return hex.EncodeToString(digest[:16])
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

func publishRuntimeGitHubRequest(ctx context.Context, config okruntime.Config, token string, request runtimeExchangeRequest, claimReview runtimeClaimReview) (runtimeGitHubPublication, error) {
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
		if request.Maintenance != nil && request.Maintenance.Approval == "auto" && !runtimeClaimReviewRequiresHuman(claimReview) {
			draft = false
		}
		created, err := client.CreateDraftPullRequest(ctx,
			"chore(knowledge): "+request.JobID,
			request.Branch,
			config.Worker.ProductionBranch,
			runtimeExchangePullRequestSummaryWithClaims(request, claimReview),
			draft,
		)
		if err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s create pull request: %w", request.RunID, err)
		}
		pull = &created
	}
	if request.Maintenance != nil && (request.Maintenance.Approval != "auto" || runtimeClaimReviewRequiresHuman(claimReview)) {
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
	if request.Maintenance != nil && request.Maintenance.Approval == "auto" && config.GitHub.AutoMergeLowRisk && !runtimeClaimReviewRequiresHuman(claimReview) {
		if _, err := client.RequireSuccessfulChecks(ctx, request.HeadSHA, config.GitHub.RequiredChecks); err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s low-risk auto-merge checks: %w", request.RunID, err)
		}
		commit, err := client.MergePullRequest(ctx, pull.Number, request.HeadSHA)
		if err != nil {
			return runtimeGitHubPublication{}, fmt.Errorf("agent run %s low-risk auto-merge: %w", request.RunID, err)
		}
		publication.Merged = true
		publication.Commit = commit
	}
	return publication, nil
}

func runtimeClaimReviewRequiresHuman(review runtimeClaimReview) bool {
	if len(review.Authorities) > 0 {
		return true
	}
	for _, change := range review.Changes {
		if change.AfterStatus == "extracted" || change.AfterStatus == "proposed" || change.AfterStatus == "supported" || change.AfterStatus == "disputed" {
			return true
		}
		if change.BeforeStatus == "verified" && (change.AfterStatus == "rejected" || change.AfterStatus == "superseded" || change.AfterStatus == "archived") {
			return true
		}
	}
	return false
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

func validateRuntimeExchangeCommit(ctx context.Context, config okruntime.Config, checkout string, base string, head string) (runtimeClaimReview, error) {
	review := runtimeClaimReview{Changes: []runtimeClaimChange{}, Authorities: []runtimeAuthorityChange{}}
	parent := filepath.Join(config.Runtime.StateDir, "verification-worktrees")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return review, err
	}
	worktree, err := addRuntimeVerificationWorktree(ctx, config, checkout, parent, ".candidate-*", head)
	if err != nil {
		return review, err
	}
	defer removeRuntimeVerificationWorktree(config, checkout, worktree)
	baseWorktree, err := addRuntimeVerificationWorktree(ctx, config, checkout, parent, ".base-*", base)
	if err != nil {
		return review, err
	}
	defer removeRuntimeVerificationWorktree(config, checkout, baseWorktree)
	now := time.Now().UTC()
	for _, knowledge := range config.KnowledgeBases {
		if !knowledge.Publish {
			continue
		}
		relative, err := filepath.Rel(config.Root, knowledge.Path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return review, fmt.Errorf("knowledge base %s path is outside repository", knowledge.ID)
		}
		validation, err := okf.ValidateWithVersion(filepath.Join(worktree, relative), knowledge.Spec)
		if err != nil {
			return review, err
		}
		if err := okf.RequireValidBundle(validation); err != nil {
			return review, err
		}
		if _, err := okf.BuildPublicationSetWithVersion(filepath.Join(worktree, relative), knowledge.Spec); err != nil {
			return review, fmt.Errorf("knowledge base %s publication contract: %w", knowledge.ID, err)
		}
		baseIndex, err := claimops.BuildIndex(filepath.Join(baseWorktree, relative), knowledge.Spec, now)
		if err != nil {
			return review, fmt.Errorf("knowledge base %s base claim index: %w", knowledge.ID, err)
		}
		candidateIndex, err := claimops.BuildIndex(filepath.Join(worktree, relative), knowledge.Spec, now)
		if err != nil {
			return review, fmt.Errorf("knowledge base %s candidate claim index: %w", knowledge.ID, err)
		}
		lifecycle := claimops.CompareLifecycle(baseIndex, candidateIndex)
		if !lifecycle.Valid {
			issue := lifecycle.Issues[0]
			return review, fmt.Errorf("knowledge base %s claim lifecycle: %s: %s", knowledge.ID, issue.Path, issue.Message)
		}
		for _, change := range lifecycle.AuthorityChanges {
			review.Authorities = append(review.Authorities, runtimeAuthorityChange{
				Knowledge: knowledge.ID, Path: change.Path, SourceID: change.SourceID,
				Resource: change.Resource, ApprovedBy: change.ApprovedBy,
			})
		}
		review.Changes = append(review.Changes, runtimeClaimChanges(knowledge.ID, baseIndex, candidateIndex, defaultClaimEvalRoots(filepath.Join(worktree, relative)))...)
	}
	sort.Slice(review.Changes, func(i, j int) bool {
		left, right := review.Changes[i], review.Changes[j]
		return left.Knowledge+"\x00"+left.ID+"\x00"+left.Path < right.Knowledge+"\x00"+right.ID+"\x00"+right.Path
	})
	sort.Slice(review.Authorities, func(i, j int) bool {
		left, right := review.Authorities[i], review.Authorities[j]
		return left.Knowledge+"\x00"+left.Path+"\x00"+left.SourceID < right.Knowledge+"\x00"+right.Path+"\x00"+right.SourceID
	})
	return review, nil
}

func runtimeClaimChanges(knowledge string, base claimops.Index, candidate claimops.Index, evalRoots []string) []runtimeClaimChange {
	baseByKey := runtimeClaimOccurrences(base)
	candidateByKey := runtimeClaimOccurrences(candidate)
	keys := make([]string, 0, len(baseByKey)+len(candidateByKey))
	seen := map[string]bool{}
	for key := range baseByKey {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range candidateByKey {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var changes []runtimeClaimChange
	for _, key := range keys {
		before, hasBefore := baseByKey[key]
		after, hasAfter := candidateByKey[key]
		beforeValue, afterValue := "", ""
		if hasBefore {
			beforeValue, _ = okf.NormalizeClaimObject(before.Claim.Object)
		}
		if hasAfter {
			afterValue, _ = okf.NormalizeClaimObject(after.Claim.Object)
		}
		if hasBefore && hasAfter && beforeValue == afterValue && before.Claim.Status == after.Claim.Status && reflect.DeepEqual(before.Claim.Evidence, after.Claim.Evidence) && reflect.DeepEqual(before.Claim.Relations, after.Claim.Relations) {
			continue
		}
		occurrence, index := after, candidate
		if !hasAfter {
			occurrence, index = before, base
		}
		impact, _ := claimops.BuildImpact(index, occurrence.Claim.ID, evalRoots)
		change := runtimeClaimChange{
			Knowledge: knowledge, ID: occurrence.Claim.ID, Path: occurrence.Path,
			BeforeValue: beforeValue, AfterValue: afterValue, Sources: runtimeClaimEvidenceSources(occurrence.Claim),
			Documents: len(impact.Documents), Evals: len(impact.Evals),
		}
		if hasBefore {
			change.BeforeStatus = before.Claim.Status
		}
		if hasAfter {
			change.AfterStatus = after.Claim.Status
		}
		changes = append(changes, change)
	}
	return changes
}

func runtimeClaimOccurrences(index claimops.Index) map[string]claimops.Occurrence {
	result := map[string]claimops.Occurrence{}
	for _, occurrence := range index.Occurrences {
		key := occurrence.Claim.ID
		result[key] = occurrence
	}
	return result
}

func runtimeClaimEvidenceSources(claim okf.Claim) []string {
	values := make([]string, 0, len(claim.Evidence))
	for _, evidence := range claim.Evidence {
		values = append(values, evidence.SourceRef)
	}
	sort.Strings(values)
	return values
}

func addRuntimeVerificationWorktree(ctx context.Context, config okruntime.Config, checkout string, parent string, pattern string, commit string) (string, error) {
	worktree, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", err
	}
	_ = os.Remove(worktree)
	if output, err := runtimeWorkerGit(ctx, config, "", checkout, "worktree", "add", "--detach", worktree, commit); err != nil {
		_ = os.RemoveAll(worktree)
		return "", fmt.Errorf("create verification worktree for %s: %w: %s", commit, err, output)
	}
	return worktree, nil
}

func removeRuntimeVerificationWorktree(config okruntime.Config, checkout string, worktree string) {
	_, _ = runtimeWorkerGit(context.Background(), config, "", checkout, "worktree", "remove", "--force", worktree)
	_ = os.RemoveAll(worktree)
}

func runtimeExchangePullRequestSummary(request runtimeExchangeRequest) string {
	return runtimeExchangePullRequestSummaryWithClaims(request, runtimeClaimReview{})
}

func runtimeExchangePullRequestSummaryWithClaims(request runtimeExchangeRequest, review runtimeClaimReview) string {
	summary := fmt.Sprintf("Automated Open Knowledge maintenance completed.\n\n- Job: `%s`\n- Run: `%s`\n- Base commit: `%s`\n- Agent verification commands reported: %d\n", request.JobID, request.RunID, request.BaseSHA, request.VerifyCount)
	if request.Maintenance != nil {
		summary += fmt.Sprintf("- Maintenance route: `%s` risk, `%s` approval, %.0f%% confidence\n- Owners: %s\n- Insight attestations: %d (%s)\n", request.Maintenance.Risk, request.Maintenance.Approval, request.Maintenance.Confidence*100, runtimeMarkdownInline(strings.Join(request.Maintenance.Owners, ", ")), len(request.Maintenance.Insights), request.Maintenance.Status)
		if len(request.Maintenance.Findings) > 0 {
			summary += "\n## Evidence-backed findings\n\n"
			for _, finding := range request.Maintenance.Findings {
				summary += "- " + runtimeMarkdownInline(finding) + "\n"
			}
		}
	}
	if request.Eval != nil {
		summary += fmt.Sprintf("- Knowledge eval `%s` reported: %d/%d → %d/%d passed (`%s`, %d regressions, %d proposed failures)\n", runtimeMarkdownInline(request.Eval.Dataset), request.Eval.BasePassed, request.Eval.Total, request.Eval.ProposedPassed, request.Eval.Total, request.Eval.Gate, request.Eval.Regressions, request.Eval.ProposedFailed)
	}
	summary += "- Publisher OKF, publication, and claim lifecycle validation: passed\n"
	if len(review.Changes) > 0 {
		summary += "\n## Knowledge claims\n\n| Claim | Status | Value | Evidence | Impact |\n| --- | --- | --- | --- | --- |\n"
		var decisions []string
		for _, change := range review.Changes {
			status := runtimeClaimTransition(change.BeforeStatus, change.AfterStatus)
			value := runtimeClaimTransition(change.BeforeValue, change.AfterValue)
			summary += fmt.Sprintf("| `%s` (`%s:%s`) | %s | %s | %s | %d docs, %d evals |\n", runtimeMarkdownInline(change.ID), runtimeMarkdownInline(change.Knowledge), runtimeMarkdownInline(change.Path), runtimeMarkdownInline(status), runtimeMarkdownInline(value), runtimeMarkdownInline(strings.Join(change.Sources, ", ")), change.Documents, change.Evals)
			if change.AfterStatus == "disputed" {
				decisions = append(decisions, fmt.Sprintf("- `%s`: sources disagree; an owner must decide the authoritative value.", runtimeMarkdownInline(change.ID)))
			}
			if change.AfterStatus == "extracted" || change.AfterStatus == "proposed" || change.AfterStatus == "supported" {
				decisions = append(decisions, fmt.Sprintf("- `%s`: verify accepted evidence before merge.", runtimeMarkdownInline(change.ID)))
			}
			if change.BeforeStatus == "verified" && (change.AfterStatus == "rejected" || change.AfterStatus == "superseded" || change.AfterStatus == "archived") {
				decisions = append(decisions, fmt.Sprintf("- `%s`: review the approved `%s` transition.", runtimeMarkdownInline(change.ID), runtimeMarkdownInline(change.AfterStatus)))
			}
		}
		if len(decisions) > 0 {
			summary += "\n## Human decision\n\n" + strings.Join(uniqueRuntimeStrings(decisions), "\n") + "\n"
		}
	}
	if len(review.Authorities) > 0 {
		summary += "\n## Source authority\n\n| Source | Document | Resource | Approval |\n| --- | --- | --- | --- |\n"
		for _, change := range review.Authorities {
			summary += fmt.Sprintf("| `%s` (`%s`) | `%s` | %s | `%s` |\n", runtimeMarkdownInline(change.SourceID), runtimeMarkdownInline(change.Knowledge), runtimeMarkdownInline(change.Path), runtimeMarkdownInline(change.Resource), runtimeMarkdownInline(change.ApprovedBy))
		}
		summary += "\n## Human decision\n\n- Review the newly granted source authority before merge.\n"
	}
	return summary + "\nRaw prompts, tool calls, environment metadata, and runtime logs remain private."
}

func runtimeClaimTransition(before string, after string) string {
	if before == after {
		return before
	}
	if before == "" {
		before = "—"
	}
	if after == "" {
		after = "—"
	}
	return before + " → " + after
}

func runtimeExchangeCheckSummary(request runtimeExchangeRequest, pullRequestURL string) string {
	eval := ""
	if request.Eval != nil {
		eval = fmt.Sprintf(" The worker reported eval `%s` improved from %d/%d to %d/%d passed, with gate `%s`, %d regressions, and %d proposed failures.", runtimeMarkdownInline(request.Eval.Dataset), request.Eval.BasePassed, request.Eval.Total, request.Eval.ProposedPassed, request.Eval.Total, request.Eval.Gate, request.Eval.Regressions, request.Eval.ProposedFailed)
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
	if eval.Status != "pass" || (eval.Gate != "all" && eval.Gate != "regressions") || eval.Regressions < 0 || eval.ProposedFailed < 0 || eval.Total < 0 || eval.BasePassed < 0 || eval.ProposedPassed < 0 || eval.BasePassed > eval.Total || eval.ProposedPassed > eval.Total || eval.ProposedPassed+eval.ProposedFailed != eval.Total {
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
	value = strings.NewReplacer("\r", " ", "\n", " ", "|", "\\|", "`", "'").Replace(value)
	return strings.TrimSpace(value)
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
