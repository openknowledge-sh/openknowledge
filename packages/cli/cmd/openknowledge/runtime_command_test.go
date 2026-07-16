package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

func TestRuntimeBuildAndServeUseOnlyVerifiedPublicGeneration(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "Wiki/index.md", "# Runtime Knowledge\n\nSearchable public guidance.\n")
	writeViewerFile(t, root, "Wiki/guide.md", "---\ntype: Guide\n---\n\n# Operations\n\nImmutable snapshot activation.\n")
	writeViewerFile(t, root, "Wiki/draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Private draft\n")
	writeViewerFile(t, root, "Wiki/search-hidden.md", "---\ntype: Guide\nokf_targets:\n  search: false\n---\n\n# Search Hidden\n\nUnique forbidden search needle.\n")
	writeViewerFile(t, root, "Wiki/mcp-hidden.md", "---\ntype: Guide\nokf_targets:\n  mcp: false\n---\n\n# MCP Hidden\n\nUnique forbidden MCP needle.\n")
	writeViewerFile(t, root, "Wiki/assets/public/logo.svg", "<svg/>\n")
	writeViewerFile(t, root, "Wiki/secret.txt", "private\n")
	writeViewerFile(t, root, "Wiki/.openknowledge/agent.log", "private log\n")
	writeViewerFile(t, root, "Wiki/openknowledge.toml", "[publish]\nenabled = true\nassets = [\"assets/public/**\"]\n")
	configPath := filepath.Join(root, "runtime.toml")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"

[artifact_store]
type = "filesystem"
path = "artifacts"

[serve]
address = "127.0.0.1:8080"
mcp_access = "public"
allowed_origins = ["https://allowed.example"]

[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/"
publish = true
mcp = true
`)
	config, err := okruntime.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "generation"), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published == nil {
		t.Fatal("expected generation promotion")
	}
	for _, included := range []string{"public/index.html", "public/search-hidden.html", "public/mcp-hidden.html", "public/assets/public/logo.svg", "source/index.md", "source/search-hidden.md", "source/mcp-hidden.md", "source/assets/public/logo.svg", "search/index.md", "search/mcp-hidden.md", "mcp/index.md", "mcp/search-hidden.md"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(included))); err != nil {
			t.Fatalf("expected %s in generation: %v", included, err)
		}
	}
	for _, excluded := range []string{"source/draft.md", "source/secret.txt", "source/openknowledge.toml", "source/.openknowledge/agent.log", "search/search-hidden.md", "mcp/mcp-hidden.md"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(excluded))); !os.IsNotExist(err) {
			t.Fatalf("expected %s outside generation, got %v", excluded, err)
		}
	}

	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("unexpected activation failures: %v", failures)
	}
	index := runtimeRequest(t, handler, http.MethodGet, "/", "", nil)
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "Runtime Knowledge") {
		t.Fatalf("unexpected viewer response %d: %s", index.Code, index.Body.String())
	}
	search := runtimeRequest(t, handler, http.MethodGet, "/_search?q=immutable", "", nil)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "guide.md") {
		t.Fatalf("unexpected search response %d: %s", search.Code, search.Body.String())
	}
	hiddenSearch := runtimeRequest(t, handler, http.MethodGet, "/_search?q=forbidden+search+needle", "", nil)
	if hiddenSearch.Code != http.StatusOK || strings.Contains(hiddenSearch.Body.String(), "search-hidden.md") {
		t.Fatalf("search=false page leaked into runtime search %d: %s", hiddenSearch.Code, hiddenSearch.Body.String())
	}
	forbidden := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, map[string]string{"Origin": "https://evil.example"})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected origin refusal, got %d", forbidden.Code)
	}

	initializeBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	initialize := runtimeRequest(t, handler, http.MethodPost, "/_mcp", initializeBody, nil)
	if initialize.Code != http.StatusOK || initialize.Header().Get("Mcp-Session-Id") == "" {
		t.Fatalf("unexpected MCP initialize response %d: %s", initialize.Code, initialize.Body.String())
	}
	var response mcpResponse
	if err := json.Unmarshal(initialize.Body.Bytes(), &response); err != nil || response.Error != nil {
		t.Fatalf("unexpected MCP response %#v err=%v", response, err)
	}
	session := initialize.Header().Get("Mcp-Session-Id")
	notification := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{"Mcp-Session-Id": session})
	if notification.Code != http.StatusAccepted {
		t.Fatalf("expected accepted notification, got %d", notification.Code)
	}
	tools := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, map[string]string{"Mcp-Session-Id": session})
	if tools.Code != http.StatusOK || !strings.Contains(tools.Body.String(), "openknowledge_search") {
		t.Fatalf("unexpected MCP tools response %d: %s", tools.Code, tools.Body.String())
	}
	resources := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`, map[string]string{"Mcp-Session-Id": session})
	if resources.Code != http.StatusOK || strings.Contains(resources.Body.String(), "mcp-hidden.md") || !strings.Contains(resources.Body.String(), "search-hidden.md") {
		t.Fatalf("unexpected MCP target projection %d: %s", resources.Code, resources.Body.String())
	}
}

func TestRuntimeServeRetainsLastValidGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Stable\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
publish = true
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "stable", filepath.Join(root, "generation"), true); err != nil {
		t.Fatal(err)
	}
	manager := newRuntimeSnapshotManager(config)
	if failures := manager.refresh(); len(failures) != 0 {
		t.Fatal(failures)
	}
	before, _ := manager.snapshot("wiki")
	if err := os.WriteFile(filepath.Join(before.Root, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if failures := manager.refresh(); len(failures) == 0 {
		t.Fatal("expected invalid active generation refresh to fail")
	}
	after, ok := manager.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest != before.Pointer.ContentDigest {
		t.Fatalf("expected last valid snapshot to remain active: %#v", after)
	}
}

func TestRuntimeWorkerReconcilesProductionBranchIntoArtifactStore(t *testing.T) {
	base := t.TempDir()
	repository := filepath.Join(base, "repository-source")
	enablePublicArtifactTest(t, filepath.Join(repository, "Wiki"))
	writeViewerFile(t, repository, "Wiki/index.md", "# First generation\n")
	writeViewerFile(t, repository, "runtime.toml", `
[runtime]
state_dir = "../worker-state"
[artifact_store]
type = "filesystem"
path = "../artifacts"
[worker]
repo = "."
production_branch = "main"
poll_interval = "1s"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
publish = true
`)
	runtimeGitTest(t, repository, "init", "-b", "main")
	runtimeGitTest(t, repository, "config", "user.name", "Runtime Test")
	runtimeGitTest(t, repository, "config", "user.email", "runtime@example.test")
	runtimeGitTest(t, repository, "add", ".")
	runtimeGitTest(t, repository, "commit", "-m", "first")
	config, err := okruntime.LoadConfig(filepath.Join(repository, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimePublisherPass(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	publisherCheckout := filepath.Join(config.Runtime.StateDir, "publisher-repository")
	agentCheckout, err := syncRuntimeAgentRepository(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	if agentCheckout == publisherCheckout {
		t.Fatal("publisher and agent must never share a checkout")
	}
	publisherGit, err := os.Stat(filepath.Join(publisherCheckout, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	agentGit, err := os.Stat(filepath.Join(agentCheckout, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(publisherGit, agentGit) {
		t.Fatal("publisher and agent must never share Git metadata")
	}
	writeViewerFile(t, agentCheckout, "agent-only.txt", "untrusted workspace mutation\n")
	if _, err := os.Stat(filepath.Join(publisherCheckout, "agent-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("agent mutation reached credentialed publisher checkout: %v", err)
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	_, firstRoot, err := store.Active("wiki")
	if err != nil {
		t.Fatal(err)
	}
	first, err := okruntime.LoadAndValidateGeneration(firstRoot)
	if err != nil {
		t.Fatal(err)
	}

	writeViewerFile(t, repository, "Wiki/guide.md", "---\ntype: Guide\n---\n\n# New guide\n")
	runtimeGitTest(t, repository, "add", "Wiki/guide.md")
	runtimeGitTest(t, repository, "commit", "-m", "second")
	if err := runtimePublisherPass(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	_, secondRoot, err := store.Active("wiki")
	if err != nil {
		t.Fatal(err)
	}
	second, err := okruntime.LoadAndValidateGeneration(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Commit == second.Commit || first.ContentDigest == second.ContentDigest {
		t.Fatalf("expected production reconciliation to activate new generation: first=%#v second=%#v", first, second)
	}
	if _, err := os.Stat(filepath.Join(secondRoot, "source", "guide.md")); err != nil {
		t.Fatalf("expected second generation content: %v", err)
	}
}

func runtimeRequest(t *testing.T, handler http.Handler, method string, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func runtimeGitTest(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
