package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestRailwayExecRunnerKeepsJSONStdoutSeparateFromProgressStderr(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "railway")
	if runtime.GOOS == "windows" {
		executable += ".cmd"
		content := "@echo off\r\necho progress 1>&2\r\necho {\"id\":\"service-1\"}\r\n"
		if err := os.WriteFile(executable, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		content := "#!/bin/sh\necho progress >&2\nprintf '%s\\n' '{\"id\":\"service-1\"}'\n"
		if err := os.WriteFile(executable, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	output, err := (railwayExecRunner{}).Run(t.Context(), directory, nil, "add", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := railwayJSONField(output, "id"); got != "service-1" {
		t.Fatalf("service id = %q, output=%q", got, output)
	}
}

type fakeRailwayCall struct {
	Arguments []string
	Stdin     string
}

type fakeRailwayRunner struct {
	Version string
	Calls   []fakeRailwayCall
	FailOn  string
}

func (runner *fakeRailwayRunner) Run(_ context.Context, _ string, stdin io.Reader, arguments ...string) ([]byte, error) {
	content := ""
	if stdin != nil {
		bytes, _ := io.ReadAll(stdin)
		content = string(bytes)
	}
	runner.Calls = append(runner.Calls, fakeRailwayCall{Arguments: append([]string(nil), arguments...), Stdin: content})
	joined := strings.Join(arguments, " ")
	if runner.FailOn != "" && strings.Contains(joined, runner.FailOn) {
		return nil, fmt.Errorf("injected Railway failure")
	}
	if len(arguments) == 1 && arguments[0] == "--version" {
		version := runner.Version
		if version == "" {
			version = "railway 5.26.2"
		}
		return []byte(version), nil
	}
	if len(arguments) > 0 && arguments[0] == "init" {
		return []byte(`{"id":"project-1"}`), nil
	}
	if len(arguments) > 0 && arguments[0] == "add" {
		name := fakeRailwayArgument(arguments, "--service")
		return []byte(fmt.Sprintf(`{"id":"service-%s"}`, name)), nil
	}
	if len(arguments) > 0 && arguments[0] == "domain" {
		if len(arguments) > 1 && !strings.HasPrefix(arguments[1], "-") {
			return []byte(fmt.Sprintf(`{"domain":%q,"dnsRecords":[{"type":"CNAME","name":%q,"value":"target.up.railway.app"},{"type":"TXT","name":"_railway","value":"verify"}]}`, arguments[1], arguments[1])), nil
		}
		return []byte(`{"domain":"generated.up.railway.app"}`), nil
	}
	return []byte(`{}`), nil
}

func fakeRailwayArgument(arguments []string, name string) string {
	for index := range arguments {
		if arguments[index] == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func TestRailwayDeployPlanIsSecretFreeAndModelsProviderEndpoint(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "state.json"))
	options.DryRun = true
	options.Domain = "docs.example.com"
	options.MCPAccess = "token"
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(content)
	if strings.Contains(serialized, "github-secret") || strings.Contains(serialized, "openai-secret") || strings.Contains(serialized, "mcp-secret") {
		t.Fatalf("dry-run plan contains secret material: %s", serialized)
	}
	if plan.Endpoint.Mode != "custom" || plan.Endpoint.Domain != "docs.example.com" {
		t.Fatalf("unexpected custom endpoint: %#v", plan.Endpoint)
	}
	if len(plan.Services) != 3 || plan.Services[0].Role != "publisher" || plan.Services[1].Role != "serve" || plan.Services[2].Role != "worker-codex" {
		t.Fatalf("unexpected isolated services: %#v", plan.Services)
	}
	if plan.Services[0].Public || !plan.Services[1].Public || plan.Services[2].Public {
		t.Fatalf("only serve may be public: %#v", plan.Services)
	}
	if plan.Services[0].VolumePath == "" || plan.Services[2].VolumePath == "" || plan.Services[1].VolumePath != "" {
		t.Fatalf("Railway volumes must be owned by private stateful roles: %#v", plan.Services)
	}
	if !strings.Contains(plan.Services[0].Config, `state_dir = "/tmp/openknowledge"`) {
		t.Fatalf("publisher checkout and build state must stay off the Railway volume: %s", plan.Services[0].Config)
	}
	if !strings.Contains(plan.Services[2].Config, `state_dir = "/var/lib/openknowledge/state"`) {
		t.Fatalf("worker state must use a process-owned directory below the Railway mount: %s", plan.Services[2].Config)
	}
	if strings.Contains(plan.Services[2].Config, `state_dir = "/var/lib/openknowledge"`) {
		t.Fatalf("worker must not chmod the provider-owned Railway mount root: %s", plan.Services[2].Config)
	}
	if !strings.Contains(plan.Services[0].Config, `path = "/var/lib/openknowledge/artifacts"`) ||
		!strings.Contains(plan.Services[0].Config, `exchange_dir = "/var/lib/openknowledge/exchange"`) {
		t.Fatalf("publisher stores must remain on the persistent Railway volume: %s", plan.Services[0].Config)
	}
}

func TestRailwayDeployInfersOneIsolatedServicePerJobRuntime(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	writeViewerFile(t, repository, ".openknowledge/jobs/claude.md", "---\nid: claude-refresh\nagent: {runtime: claude}\n---\nRefresh docs.\n")
	writeViewerFile(t, repository, ".openknowledge/jobs/opencode.md", "---\nid: opencode-research\nagent: {runtime: opencode, model: custom/research}\n---\nResearch updates.\n")
	runtimeGitTest(t, repository, "add", ".openknowledge/jobs")
	runtimeGitTest(t, repository, "commit", "-m", "add agent jobs")
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "multi-state.json"))
	options.Runtimes = ""
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Runtimes, []string{"claude", "opencode"}) {
		t.Fatalf("unexpected inferred runtimes: %#v", plan.Runtimes)
	}
	roles := make([]string, 0, len(plan.Services))
	for _, service := range plan.Services {
		roles = append(roles, service.Role)
	}
	if !reflect.DeepEqual(roles, []string{"publisher", "serve", "worker-claude", "worker-opencode"}) {
		t.Fatalf("unexpected services: %#v", plan.Services)
	}
	if !strings.Contains(plan.Services[2].Config, `runtimes = ["claude","opencode"]`) || plan.Services[3].Source.Repository != "example/knowledge" {
		t.Fatalf("runtime plan did not configure isolated workers: %#v", plan.Services)
	}
}

func TestRailwayDeployOmitsWorkersWhenNoEnabledJobsAreInferred(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "no-workers-state.json"))
	options.Runtimes = ""
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Runtimes) != 0 || len(plan.Services) != 2 {
		t.Fatalf("expected publisher and serve only, got runtimes=%#v services=%#v", plan.Runtimes, plan.Services)
	}
	if strings.Contains(plan.Services[0].Config, "runtimes =") || strings.Contains(plan.Services[0].Config, "run_jobs = true") {
		t.Fatalf("publisher config unexpectedly enabled jobs: %s", plan.Services[0].Config)
	}
}

func TestRailwayDeployScopesOpenCodeCredentialsSeparately(t *testing.T) {
	options := defaultRailwayDeployTestOptions("state.json")
	if got := deployRuntimeCredentialEnvironment("opencode"); got != "OPENCODE_API_KEY" {
		t.Fatalf("OpenCode credential target = %q", got)
	}
	if got := deployRuntimeCredentialSource(options, "opencode"); got != options.OpenCodeKeyEnv {
		t.Fatalf("OpenCode credential source = %q", got)
	}
}

func TestRailwayProviderIsIdempotentAndNeverPlacesSecretsInArgumentsOrState(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	statePath := filepath.Join(repository, ".openknowledge", "deployments", "railway.json")
	options := defaultRailwayDeployTestOptions(statePath)
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	secrets := deploySecrets{GitHubToken: "github-secret", AgentKeys: map[string]string{"codex": "openai-secret"}, ArtifactToken: "artifact-secret", ExchangeToken: "exchange-secret"}
	runner := &fakeRailwayRunner{}
	provider := railwayProvider{runner: runner}
	first, err := provider.Apply(t.Context(), plan, secrets)
	if err != nil {
		t.Fatal(err)
	}
	if first.Endpoint.URL != "https://generated.up.railway.app" {
		t.Fatalf("unexpected provider endpoint: %#v", first.Endpoint)
	}
	if first.Status != "deployment-triggered" {
		t.Fatalf("unexpected deployment status: %q", first.Status)
	}
	if _, err := provider.Apply(t.Context(), plan, secrets); err != nil {
		t.Fatal(err)
	}
	addCount, volumeCount, sourceCount, domainCount := 0, 0, 0, 0
	for _, call := range runner.Calls {
		joined := strings.Join(call.Arguments, " ")
		if strings.HasPrefix(joined, "add ") {
			addCount++
		}
		if strings.HasPrefix(joined, "volume ") && strings.Contains(joined, " add ") {
			volumeCount++
		}
		if strings.HasPrefix(joined, "service source connect ") {
			sourceCount++
			if !strings.Contains(joined, "--repo example/knowledge --branch main") {
				t.Fatalf("source connection omitted repository or branch: %s", joined)
			}
		}
		if strings.HasPrefix(joined, "domain ") {
			domainCount++
		}
		for _, secret := range []string{"github-secret", "openai-secret", "artifact-secret", "exchange-secret"} {
			if strings.Contains(joined, secret) {
				t.Fatalf("secret appeared in Railway argv: %s", joined)
			}
		}
	}
	if addCount != 3 || volumeCount != 2 || sourceCount != 3 || domainCount != 1 {
		t.Fatalf("provider duplicated resources: add=%d volume=%d source=%d domain=%d calls=%#v", addCount, volumeCount, sourceCount, domainCount, runner.Calls)
	}
	state, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"github-secret", "openai-secret", "artifact-secret", "exchange-secret"} {
		if strings.Contains(string(state), secret) {
			t.Fatalf("secret appeared in deployment state: %s", state)
		}
	}
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("deployment state mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestRailwayProviderMigratesV1ImageStateToRepositorySource(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	statePath := filepath.Join(repository, "legacy-state.json")
	options := defaultRailwayDeployTestOptions(statePath)
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	legacy := railwayDeployState{
		Version: 1, Project: deployProject{Name: plan.Project.Name, ID: "project-1"},
		Services: make(map[string]railwayServiceState),
	}
	for _, service := range plan.Services {
		legacy.Services[service.Role] = railwayServiceState{
			ID: "legacy-" + service.Role, Name: service.Name, Role: service.Role,
			Image:  "ghcr.io/openknowledge-sh/openknowledge-runtime-" + service.Role + ":0.6.2",
			Volume: service.VolumePath != "", Deployed: true,
		}
	}
	if err := saveRailwayDeployState(statePath, legacy); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRailwayRunner{}
	secrets := deploySecrets{GitHubToken: "github", AgentKeys: map[string]string{"codex": "openai"}, ArtifactToken: "artifact", ExchangeToken: "exchange"}
	if _, err := (railwayProvider{runner: runner}).Apply(t.Context(), plan, secrets); err != nil {
		t.Fatal(err)
	}
	migrated, present, err := loadRailwayDeployState(statePath)
	if err != nil || !present {
		t.Fatalf("missing migrated state: present=%t err=%v", present, err)
	}
	if migrated.Version != railwayDeployStateVersion {
		t.Fatalf("state version = %d, want %d", migrated.Version, railwayDeployStateVersion)
	}
	for role, service := range migrated.Services {
		if service.Image != "" || !service.SourceConnected || service.Source.Repository != "example/knowledge" {
			t.Fatalf("service %s was not migrated to repository source: %#v", role, service)
		}
	}
	addCount, sourceCount := 0, 0
	for _, call := range runner.Calls {
		joined := strings.Join(call.Arguments, " ")
		if strings.HasPrefix(joined, "add ") {
			addCount++
		}
		if strings.HasPrefix(joined, "service source connect ") {
			sourceCount++
		}
	}
	if addCount != 0 || sourceCount != len(plan.Services) {
		t.Fatalf("legacy migration recreated services or omitted source connections: add=%d source=%d calls=%#v", addCount, sourceCount, runner.Calls)
	}
}

func TestRailwayProviderRejectsOldCLIWithoutCreatingResources(t *testing.T) {
	runner := &fakeRailwayRunner{Version: "railway 4.3.0"}
	_, err := (railwayProvider{runner: runner}).Apply(t.Context(), deployPlan{}, deploySecrets{})
	if err == nil || !strings.Contains(err.Error(), "v5 or newer") {
		t.Fatalf("expected actionable version error, got %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("old CLI must fail before provider mutations: %#v", runner.Calls)
	}
}

func TestRailwayProviderReturnsCustomDomainDNSRecords(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "custom-state.json"))
	options.Domain = "docs.example.com"
	options.WithoutWorker = true
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (railwayProvider{runner: &fakeRailwayRunner{}}).Apply(t.Context(), plan, deploySecrets{GitHubToken: "github", ArtifactToken: "artifact", ExchangeToken: "exchange"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Endpoint.URL != "https://docs.example.com" || len(result.Endpoint.DNSRecords) != 2 {
		t.Fatalf("custom endpoint omitted DNS handoff: %#v", result.Endpoint)
	}
	if result.Endpoint.DNSRecords[0].Type != "CNAME" || result.Endpoint.DNSRecords[1].Type != "TXT" {
		t.Fatalf("unexpected DNS records: %#v", result.Endpoint.DNSRecords)
	}
}

func TestRailwayProviderPersistsRecoverableIncompleteStateOnFailure(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	statePath := filepath.Join(repository, "failed-state.json")
	options := defaultRailwayDeployTestOptions(statePath)
	plan, err := buildRailwayDeployPlan(options, wiki)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRailwayRunner{FailOn: "variable set GITHUB_TOKEN"}
	provider := railwayProvider{runner: runner}
	secrets := deploySecrets{GitHubToken: "github", AgentKeys: map[string]string{"codex": "openai"}, ArtifactToken: "artifact", ExchangeToken: "exchange"}
	if _, err := provider.Apply(t.Context(), plan, secrets); err == nil {
		t.Fatal("expected injected provider failure")
	}
	state, present, err := loadRailwayDeployState(statePath)
	if err != nil || !present {
		t.Fatalf("missing recovery state: present=%t err=%v", present, err)
	}
	if state.Complete || len(state.Services) != 3 {
		t.Fatalf("failure was recorded as success or lost resources: %#v", state)
	}
	runner.FailOn = ""
	if _, err := provider.Apply(t.Context(), plan, secrets); err != nil {
		t.Fatal(err)
	}
	addCount := 0
	for _, call := range runner.Calls {
		if len(call.Arguments) > 0 && call.Arguments[0] == "add" {
			addCount++
		}
	}
	if addCount != 3 {
		t.Fatalf("recovery duplicated services: %d", addCount)
	}
}

func TestRailwayDeployRejectsDomainCreationAmbiguity(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "state.json"))
	options.Domain = "docs.example.com"
	options.NoPublicEndpoint = true
	if _, err := buildRailwayDeployPlan(options, wiki); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected endpoint mode conflict, got %v", err)
	}
}

func TestRailwayDeployPreflightsCommittedProductionSnapshot(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	writeViewerFile(t, repository, "Wiki/openknowledge.toml", "[publish]\nenabled = false\n")
	runtimeGitTest(t, repository, "add", "Wiki/openknowledge.toml")
	runtimeGitTest(t, repository, "commit", "-m", "disable production publication")
	// A valid uncommitted working copy must not conceal that the deployed branch
	// still refuses publication.
	writeViewerFile(t, repository, "Wiki/openknowledge.toml", "[publish]\nenabled = true\n")
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "state.json"))
	_, err := buildRailwayDeployPlan(options, wiki)
	if err == nil || !strings.Contains(err.Error(), "production branch preflight") {
		t.Fatalf("expected committed production refusal, got %v", err)
	}
}

func TestRailwayDeployRequiresCommittedProjectRuntimeScaffold(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	if err := os.RemoveAll(filepath.Join(repository, filepath.FromSlash(deployRuntimeDirectory))); err != nil {
		t.Fatal(err)
	}
	runtimeGitTest(t, repository, "add", "-A")
	runtimeGitTest(t, repository, "commit", "-m", "remove runtime scaffold")
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "state.json"))
	_, err := buildRailwayDeployPlan(options, wiki)
	if err == nil || !strings.Contains(err.Error(), "deploy railway init") {
		t.Fatalf("expected committed runtime scaffold preflight, got %v", err)
	}
}

func TestRailwayDeployRejectsWorkerMissingFromCommittedRuntime(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	writeViewerFile(t, repository, deployRuntimeDockerfile, "FROM node:22-bookworm-slim\n")
	runtimeGitTest(t, repository, "add", deployRuntimeDockerfile)
	runtimeGitTest(t, repository, "commit", "-m", "remove codex runtime")
	options := defaultRailwayDeployTestOptions(filepath.Join(repository, "state.json"))
	_, err := buildRailwayDeployPlan(options, wiki)
	if err == nil || !strings.Contains(err.Error(), "matching --runtimes") {
		t.Fatalf("expected runtime/scaffold mismatch, got %v", err)
	}
}

func defaultRailwayDeployTestOptions(state string) railwayDeployOptions {
	return railwayDeployOptions{
		Name: "test-knowledge", Branch: "main", MCPAccess: "public",
		Runtimes:       "codex",
		GitHubTokenEnv: "GITHUB_TOKEN", CodexKeyEnv: "CODEX_API_KEY", ClaudeKeyEnv: "ANTHROPIC_API_KEY", OpenCodeKeyEnv: "OPENCODE_API_KEY", MCPTokenEnv: "OPENKNOWLEDGE_MCP_TOKEN",
		StatePath: state,
	}
}

func newDeployTestRepository(t *testing.T) (string, string) {
	t.Helper()
	repository := t.TempDir()
	wiki := filepath.Join(repository, "Wiki")
	enablePublicArtifactTest(t, wiki)
	writeViewerFile(t, repository, "Wiki/index.md", "# Deployable knowledge\n")
	writeViewerFile(t, repository, deployRuntimeDockerfile, "FROM node:22-bookworm-slim\nRUN npm install --global \"@openai/codex@${CODEX_VERSION}\" \"@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}\" \"opencode-ai@${OPENCODE_VERSION}\"\n")
	writeViewerFile(t, repository, deployRuntimeEntrypoint, "#!/bin/sh\n")
	runtimeGitTest(t, repository, "init", "-b", "main")
	runtimeGitTest(t, repository, "config", "user.name", "Deploy Test")
	runtimeGitTest(t, repository, "config", "user.email", "deploy@example.test")
	runtimeGitTest(t, repository, "remote", "add", "origin", "git@github.com:example/knowledge.git")
	runtimeGitTest(t, repository, "add", ".")
	runtimeGitTest(t, repository, "commit", "-m", "deployable knowledge")
	return repository, wiki
}
