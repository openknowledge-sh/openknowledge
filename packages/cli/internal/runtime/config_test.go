package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestParseConfigIsStrictAndAppliesSafeDefaults(t *testing.T) {
	config, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"

[artifact_store]
type = "filesystem"
path = "artifacts"

[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/docs"
`))
	if err != nil {
		t.Fatal(err)
	}
	if config.Serve.Address != "127.0.0.1:8080" || config.Serve.MCPAccess != "public" {
		t.Fatalf("unexpected serve defaults: %#v", config.Serve)
	}
	if config.Runtime.ReleasePolicy != ReleasePolicyFollowMain {
		t.Fatalf("unexpected release policy default: %#v", config.Runtime)
	}
	policy := config.Serve.RetrievalPolicy
	if policy.MinimumTrust != okf.OKFV02TrustUnverified || !policy.AllowStale || policy.RequireSources || !reflect.DeepEqual(policy.AllowedStatuses, []string{"draft", "stable", "deprecated"}) {
		t.Fatalf("unexpected permissive retrieval policy defaults: %#v", policy)
	}
	if config.Serve.UsageEvents.Enabled || config.Serve.UsageEvents.CaptureQueries || config.Serve.UsageEvents.Retention != "720h" {
		t.Fatalf("unexpected privacy-safe usage event defaults: %#v", config.Serve.UsageEvents)
	}
	if config.KnowledgeBases[0].Route != "/docs/" || config.KnowledgeBases[0].Spec != okf.LatestSpecVersion {
		t.Fatalf("unexpected normalized knowledge base: %#v", config.KnowledgeBases[0])
	}
	if _, err := ParseConfig([]byte("[runtime]\nstate_dir='state'\nunknown=true\n")); err == nil || !strings.Contains(err.Error(), "missing in the target struct") {
		t.Fatalf("expected unknown field refusal, got %v", err)
	}
}

func TestParseConfigValidatesReleasePolicy(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
release_policy = %q
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	for _, policy := range []string{ReleasePolicyFollowMain, ReleasePolicyLastPassing} {
		config, err := ParseConfig([]byte(fmt.Sprintf(base, policy)))
		if err != nil || config.Runtime.ReleasePolicy != policy {
			t.Fatalf("release policy %q: config=%#v err=%v", policy, config.Runtime, err)
		}
	}
	if _, err := ParseConfig([]byte(fmt.Sprintf(base, "latest"))); err == nil || !strings.Contains(err.Error(), "release_policy") {
		t.Fatalf("expected invalid release policy refusal, got %v", err)
	}
}

func TestParseConfigRejectsMCPAccessAsPublicationSwitch(t *testing.T) {
	_, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve]
mcp_access = "off"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`))
	if err == nil || !strings.Contains(err.Error(), "must be public or token") {
		t.Fatalf("expected MCP access policy refusal, got %v", err)
	}
}

func TestParseConfigNormalizesPermissionAwareAccessProfiles(t *testing.T) {
	config, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve]
mcp_access = "token"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
[[access_profiles]]
id = "support"
token_env = "SUPPORT_KNOWLEDGE_TOKEN"
knowledge_bases = ["wiki"]
agents = ["support-agent"]
teams = ["support"]
use_cases = ["customer-support"]
[access_profiles.retrieval_policy]
minimum_trust = "human-reviewed"
allow_stale = false
allowed_statuses = ["stable"]
require_sources = true
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(config.AccessProfiles) != 1 || config.AccessProfiles[0].ID != "support" || config.AccessProfiles[0].RetrievalPolicy == nil || config.AccessProfiles[0].RetrievalPolicy.MinimumTrust != "human-reviewed" {
		t.Fatalf("unexpected access profiles: %#v", config.AccessProfiles)
	}
	invalid := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
[[access_profiles]]
id = "support"
token_env = "SUPPORT_KNOWLEDGE_TOKEN"
knowledge_bases = ["missing"]
agents = ["support-agent"]
`
	if _, err := ParseConfig([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "unknown knowledge base") {
		t.Fatalf("expected unknown access route refusal, got %v", err)
	}
}

func TestLoadConfigRejectsAccessProfileForLocalOnlyBundle(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "runtime.toml")
	content := `[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
[[access_profiles]]
id = "support"
token_env = "SUPPORT_KNOWLEDGE_TOKEN"
knowledge_bases = ["wiki"]
agents = ["support-agent"]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(configPath); err == nil || !strings.Contains(err.Error(), "local-only knowledge base") {
		t.Fatalf("expected local-only access route refusal, got %v", err)
	}
}

func TestParseConfigDoesNotAcceptRemovedRunAgentsKey(t *testing.T) {
	_, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[worker]
run_agents = true
`))
	if err == nil {
		t.Fatalf("removed run_agents key was accepted: %v", err)
	}
}

func TestParseConfigRejectsRemovedKnowledgeBasePublicationSwitches(t *testing.T) {
	for _, field := range []string{"publish = true", "mcp = true"} {
		_, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
` + field + "\n"))
		if err == nil || !strings.Contains(err.Error(), "missing in the target struct") {
			t.Fatalf("expected removed %q field refusal, got %v", field, err)
		}
	}
}

func TestParseConfigRequiresExplicitSupportedWorkerRuntimes(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[worker]
run_jobs = true
%s
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	if _, err := ParseConfig([]byte(fmt.Sprintf(base, ""))); err == nil || !strings.Contains(err.Error(), "worker.runtimes") {
		t.Fatalf("expected missing runtime refusal, got %v", err)
	}
	config, err := ParseConfig([]byte(fmt.Sprintf(base, `runtimes = ["opencode", "claude", "codex"]`)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config.Worker.Runtimes, []string{"claude", "codex", "opencode"}) {
		t.Fatalf("unexpected normalized runtimes: %#v", config.Worker.Runtimes)
	}
	if _, err := ParseConfig([]byte(fmt.Sprintf(base, `runtimes = ["unsupported"]`))); err == nil || !strings.Contains(err.Error(), "unsupported agent runtime") {
		t.Fatalf("expected unknown runtime refusal, got %v", err)
	}
}

func TestLoadConfigSupportsSecretFreeEnvironmentConfiguration(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiki, okf.ValidationConfigFile), []byte("[release]\noutputs = [\"mcp\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENKNOWLEDGE_RUNTIME_ROOT", root)
	t.Setenv("TEST_RUNTIME_CONFIG", `
[runtime]
state_dir = "state"
[artifact_store]
type = "http"
path = "cache"
url = "http://publisher.railway.internal:8090"
token_env = "ARTIFACT_TOKEN"
[worker]
exchange_url = "http://publisher.railway.internal:8090"
exchange_token_env = "EXCHANGE_TOKEN"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	config, err := LoadConfig("env:TEST_RUNTIME_CONFIG")
	if err != nil {
		t.Fatal(err)
	}
	if config.Path != "env:TEST_RUNTIME_CONFIG" || config.Root != root {
		t.Fatalf("unexpected environment config identity: %#v", config)
	}
	if config.ArtifactStore.Path != filepath.Join(root, "cache") || config.KnowledgeBases[0].Path != filepath.Join(root, "Wiki") {
		t.Fatalf("environment config paths were not rooted safely: %#v", config)
	}
}

func TestParseConfigRejectsPublicPlainHTTPTransport(t *testing.T) {
	_, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"
[artifact_store]
type = "http"
path = "cache"
url = "http://public.example.com"
token_env = "TOKEN"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`))
	if err == nil || !strings.Contains(err.Error(), "plain HTTP") {
		t.Fatalf("expected public HTTP refusal, got %v", err)
	}
}

func TestParseConfigRejectsAmbiguousOrUnsafeValues(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	tests := []struct {
		name    string
		replace string
		with    string
		want    string
	}{
		{name: "store", replace: `type = "filesystem"`, with: `type = "s3"`, want: "must be filesystem"},
		{name: "route", replace: `path = "Wiki"`, with: "path = \"Wiki\"\nroute = \"../private\"", want: "must start with /"},
		{name: "id", replace: `id = "wiki"`, with: `id = "../wiki"`, want: "must contain only"},
		{name: "dot id", replace: `id = "wiki"`, with: `id = ".."`, want: "must contain only"},
		{name: "branch traversal", replace: `path = "Wiki"`, with: "path = \"Wiki\"\n[worker]\nproduction_branch = \"feature/../main\"", want: "production_branch is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(strings.Replace(base, test.replace, test.with, 1)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestParseConfigValidatesRetrievalPolicy(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve.retrieval_policy]
minimum_trust = %q
allow_stale = false
allowed_statuses = %s
require_sources = true
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	config, err := ParseConfig([]byte(fmt.Sprintf(base, "machine-confirmed", `["stable"]`)))
	if err != nil {
		t.Fatal(err)
	}
	if config.Serve.RetrievalPolicy.MinimumTrust != "machine-confirmed" || config.Serve.RetrievalPolicy.AllowStale || !config.Serve.RetrievalPolicy.RequireSources {
		t.Fatalf("unexpected retrieval policy: %#v", config.Serve.RetrievalPolicy)
	}
	for _, test := range []struct {
		trust    string
		statuses string
		want     string
	}{
		{trust: "trusted", statuses: `["stable"]`, want: "minimum_trust"},
		{trust: "unverified", statuses: `[]`, want: "must not be empty"},
		{trust: "unverified", statuses: `["stable", "stable"]`, want: "duplicated"},
		{trust: "unverified", statuses: `["archived"]`, want: "must be draft"},
	} {
		_, err := ParseConfig([]byte(fmt.Sprintf(base, test.trust, test.statuses)))
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %q for trust=%q statuses=%s, got %v", test.want, test.trust, test.statuses, err)
		}
	}
}

func TestParseConfigRequiresExplicitUsageQueryCapture(t *testing.T) {
	content := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve.usage_events]
enabled = false
capture_queries = true
retention = "24h"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	if _, err := ParseConfig([]byte(content)); err == nil || !strings.Contains(err.Error(), "capture_queries requires enabled") {
		t.Fatalf("expected explicit usage query capture refusal, got %v", err)
	}
}

func TestParseConfigValidatesRequiredGitHubChecks(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[github]
enabled = %t
repository = "owner/repo"
token_env = "GITHUB_TOKEN"
required_checks = %s
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	config, err := ParseConfig([]byte(fmt.Sprintf(base, true, `["Verify", "Knowledge Eval"]`)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config.GitHub.RequiredChecks, []string{"Knowledge Eval", "Verify"}) {
		t.Fatalf("required checks were not normalized: %#v", config.GitHub.RequiredChecks)
	}
	for _, test := range []struct {
		enabled bool
		checks  string
		want    string
	}{
		{enabled: false, checks: `["Verify"]`, want: "requires github.enabled"},
		{enabled: true, checks: `["Verify", "Verify"]`, want: "duplicated"},
		{enabled: true, checks: `[""]`, want: "non-empty"},
	} {
		_, err := ParseConfig([]byte(fmt.Sprintf(base, test.enabled, test.checks)))
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %q, got %v", test.want, err)
		}
	}
}

func TestParseConfigRequiresChecksForLowRiskAutoMerge(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[github]
enabled = true
repository = "owner/repo"
token_env = "GITHUB_TOKEN"
checks = %t
required_checks = %s
auto_merge_low_risk = true
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`
	if _, err := ParseConfig([]byte(fmt.Sprintf(base, true, `["Verify"]`))); err != nil {
		t.Fatalf("valid low-risk auto merge rejected: %v", err)
	}
	for _, value := range []string{fmt.Sprintf(base, false, `["Verify"]`), fmt.Sprintf(base, true, `[]`)} {
		if _, err := ParseConfig([]byte(value)); err == nil || !strings.Contains(err.Error(), "auto_merge_low_risk") {
			t.Fatalf("expected low-risk auto merge gate, got %v", err)
		}
	}
}
