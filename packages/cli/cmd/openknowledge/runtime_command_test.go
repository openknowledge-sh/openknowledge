package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type runtimeHandlerRoundTripper struct {
	handler http.Handler
}

func (transport runtimeHandlerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	transport.handler.ServeHTTP(response, request)
	return response.Result(), nil
}

func TestRuntimeInfoUsesStdout(t *testing.T) {
	stdout, stderr, code := captureMainOutput(t, func() int {
		runtimeInfof("runtime lifecycle %s\n", "ready")
		return 0
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "runtime lifecycle ready\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestEnsureRuntimeStateDirectorySkipsRedundantChmod(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	err := ensureRuntimeStateDirectoryWith(state, func(string, os.FileMode) error {
		called = true
		return fmt.Errorf("chmod should not be called")
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("private runtime state directory was chmodded again")
	}
}

func TestEnsureRuntimeStateDirectoryTightensExistingPermissions(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureRuntimeStateDirectory(state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("runtime state mode = %04o, want 0700", info.Mode().Perm())
	}
}

func TestRuntimeWorkerGitAuthorizationHeaderUsesGitHubBasicCredential(t *testing.T) {
	const expected = "AUTHORIZATION: basic eC1hY2Nlc3MtdG9rZW46dGVzdC10b2tlbg=="
	if got := runtimeWorkerGitAuthorizationHeader("test-token"); got != expected {
		t.Fatalf("authorization header = %q, want %q", got, expected)
	}
}

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
	for _, included := range []string{"public/index.html", "public/search-hidden.html", "public/mcp-hidden.html", "public/assets/openknowledge/viewer-theme.js", "public/assets/openknowledge/viewer-data.js", "public/assets/openknowledge/viewer.css", "public/assets/openknowledge/viewer.js", "public/assets/public/logo.svg", "source/index.md", "source/search-hidden.md", "source/mcp-hidden.md", "source/assets/public/logo.svg", "search/index.md", "search/mcp-hidden.md", "mcp/index.md", "mcp/search-hidden.md"} {
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
	if !strings.Contains(index.Header().Get("Content-Security-Policy"), "script-src 'self' https:") || strings.Contains(index.Body.String(), `<script>`) {
		t.Fatalf("runtime viewer must load generated scripts under its restrictive CSP: header=%q\n%s", index.Header().Get("Content-Security-Policy"), index.Body.String())
	}
	if index.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("runtime viewer cache policy = %q, want no-cache", index.Header().Get("Cache-Control"))
	}
	viewerScript := runtimeRequest(t, handler, http.MethodGet, "/assets/openknowledge/viewer.js", "", nil)
	if viewerScript.Code != http.StatusOK || !strings.Contains(viewerScript.Header().Get("Content-Type"), "javascript") || !strings.Contains(viewerScript.Body.String(), "OpenKnowledgeStaticData") {
		t.Fatalf("unexpected viewer script response %d %q: %s", viewerScript.Code, viewerScript.Header().Get("Content-Type"), viewerScript.Body.String())
	}
	if viewerScript.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("runtime viewer script cache policy = %q, want no-cache", viewerScript.Header().Get("Cache-Control"))
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

func TestRuntimeServeCachesSearchIndexPerGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v1\n\nAlpha search needle.\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve]
max_concurrency = 16
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/"
publish = true
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "first", filepath.Join(root, "first"), true); err != nil {
		t.Fatal(err)
	}
	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	var indexBuilds atomic.Int32
	handler.snapshots.buildSearchIndex = func(root string, version string) (okf.ContextIndex, error) {
		indexBuilds.Add(1)
		return okf.BuildContextIndexWithVersion(root, version)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("first activation failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("search index builds after first activation = %d, want 1", count)
	}
	before, ok := handler.snapshots.snapshot("wiki")
	if !ok || before.Search.Revision.IndexSHA256 == "" {
		t.Fatalf("expected activated snapshot to contain a search index: %#v", before)
	}

	// Generation directories are immutable in production. Mutating this test
	// fixture after activation proves requests use the snapshot's built index
	// instead of reparsing the projection on every search.
	searchPath := filepath.Join(before.Root, "search", "index.md")
	originalSearch, err := os.ReadFile(searchPath)
	if err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, before.Root, "search/index.md", "# Mutated after activation\n\nUncached mutation needle.\n")
	alpha := runtimeRequest(t, handler, http.MethodGet, "/_search?q=alpha", "", nil)
	var alphaResult okf.SearchResultSet
	if err := json.Unmarshal(alpha.Body.Bytes(), &alphaResult); err != nil {
		t.Fatal(err)
	}
	if alpha.Code != http.StatusOK || len(alphaResult.Results) == 0 {
		t.Fatalf("cached v1 search failed %d: %s", alpha.Code, alpha.Body.String())
	}
	mutated := runtimeRequest(t, handler, http.MethodGet, "/_search?q=uncached", "", nil)
	var mutatedResult okf.SearchResultSet
	if err := json.Unmarshal(mutated.Body.Bytes(), &mutatedResult); err != nil {
		t.Fatal(err)
	}
	if mutated.Code != http.StatusOK || len(mutatedResult.Results) != 0 {
		t.Fatalf("post-activation filesystem mutation reached cached search %d: %s", mutated.Code, mutated.Body.String())
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("search requests rebuilt snapshot index: build count = %d, want 1", count)
	}
	if err := os.WriteFile(searchPath, originalSearch, 0644); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("unchanged refresh failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("unchanged refresh rebuilt snapshot index: build count = %d, want 1", count)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v2\n\nBeta search needle.\n")
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "second", filepath.Join(root, "second"), true); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("second activation failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 2 {
		t.Fatalf("search index builds after second activation = %d, want 2", count)
	}
	after, ok := handler.snapshots.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest == before.Pointer.ContentDigest || after.Search.Revision == before.Search.Revision {
		t.Fatalf("expected second generation to replace the cached search index: before=%#v after=%#v", before, after)
	}
	beta := runtimeRequest(t, handler, http.MethodGet, "/_search?q=beta", "", nil)
	var betaResult okf.SearchResultSet
	if err := json.Unmarshal(beta.Body.Bytes(), &betaResult); err != nil {
		t.Fatal(err)
	}
	if beta.Code != http.StatusOK || len(betaResult.Results) == 0 {
		t.Fatalf("cached v2 search failed %d: %s", beta.Code, beta.Body.String())
	}

	const requests = 8
	var wait sync.WaitGroup
	errors := make(chan error, requests)
	for index := 0; index < requests; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/_search?q=beta", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "index.md") {
				errors <- fmt.Errorf("concurrent search response %d: %s", response.Code, response.Body.String())
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if count := indexBuilds.Load(); count != 2 {
		t.Fatalf("concurrent search requests rebuilt snapshot index: build count = %d, want 2", count)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v3\n\nGamma search needle.\n")
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "third", filepath.Join(root, "third"), true); err != nil {
		t.Fatal(err)
	}
	handler.snapshots.buildSearchIndex = func(string, string) (okf.ContextIndex, error) {
		indexBuilds.Add(1)
		return okf.ContextIndex{}, fmt.Errorf("injected index build failure")
	}
	if failures := handler.snapshots.refresh(); len(failures) == 0 || !strings.Contains(failures[0].Error(), "search index") {
		t.Fatalf("expected search index activation failure, got %v", failures)
	}
	retained, ok := handler.snapshots.snapshot("wiki")
	if !ok || retained.Pointer.ContentDigest != after.Pointer.ContentDigest || retained.Search.Revision != after.Search.Revision {
		t.Fatalf("failed index build replaced last valid snapshot: before=%#v after=%#v", after, retained)
	}
	if count := indexBuilds.Load(); count != 3 {
		t.Fatalf("search index builds after rejected generation = %d, want 3", count)
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

func TestRuntimePrivateTransportSynchronizesVerifiedGenerationAndSeparatesCapabilities(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Private transport v1\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "publisher-state"
[artifact_store]
type = "filesystem"
path = "publisher-artifacts"
[publisher_api]
enabled = true
address = "127.0.0.1:8090"
artifact_token_env = "TEST_ARTIFACT_TOKEN"
exchange_token_env = "TEST_EXCHANGE_TOKEN"
[worker]
exchange_dir = "publisher-exchange"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
publish = true
`)
	artifactCapability := strings.Repeat("a", 40)
	exchangeCapability := strings.Repeat("e", 40)
	t.Setenv("TEST_ARTIFACT_TOKEN", artifactCapability)
	t.Setenv("TEST_EXCHANGE_TOKEN", exchangeCapability)
	publisherConfig, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := buildRuntimeKnowledgeGeneration(publisherConfig, publisherConfig.KnowledgeBases[0], "first", filepath.Join(root, "first"), true)
	if err != nil {
		t.Fatal(err)
	}
	publisherHandler, err := newRuntimePublisherAPIHandler(publisherConfig)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/artifacts/wiki/active.json", "", map[string]string{"Authorization": "Bearer " + exchangeCapability})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("exchange capability read artifact endpoint: %d", unauthorized.Code)
	}
	crossScope := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/exchange/source.bundle", "", map[string]string{"Authorization": "Bearer " + artifactCapability})
	if crossScope.Code != http.StatusUnauthorized {
		t.Fatalf("artifact capability read exchange endpoint: %d", crossScope.Code)
	}
	if err := os.MkdirAll(publisherConfig.Worker.ExchangeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publisherConfig.Worker.ExchangeDir, "source.bundle"), []byte("private git bundle"), 0644); err != nil {
		t.Fatal(err)
	}
	source := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/exchange/source.bundle", "", map[string]string{"Authorization": "Bearer " + exchangeCapability})
	if source.Code != http.StatusOK || source.Body.String() != "private git bundle" {
		t.Fatalf("unexpected source exchange response %d: %q", source.Code, source.Body.String())
	}
	proposal := filepath.Join(root, "proposal")
	if err := os.MkdirAll(proposal, 0755); err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, proposal, "branch.bundle", "untrusted branch bundle")
	bundleDigest, err := okf.SHA256File(filepath.Join(proposal, "branch.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, proposal, "request.json", fmt.Sprintf(`{"version":1,"run_id":"run-1","job_id":"refresh","branch":"agent/refresh","base_sha":"%s","head_sha":"%s","bundle_sha256":"%s","verify_count":1}`+"\n", strings.Repeat("a", 40), strings.Repeat("b", 40), bundleDigest))
	var proposalArchive bytes.Buffer
	if err := okruntime.WriteDirectoryArchive(&proposalArchive, proposal); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPut, "/v1/exchange/runs/run-1", bytes.NewReader(proposalArchive.Bytes()))
		request.Header.Set("Authorization", "Bearer "+exchangeCapability)
		response := httptest.NewRecorder()
		publisherHandler.ServeHTTP(response, request)
		want := http.StatusCreated
		if attempt == 1 {
			want = http.StatusNoContent
		}
		if response.Code != want {
			t.Fatalf("exchange upload attempt %d = %d, want %d: %s", attempt, response.Code, want, response.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(publisherConfig.Worker.ExchangeDir, "runs", "run-1", "request.json")); err != nil {
		t.Fatalf("publisher did not atomically store proposal: %v", err)
	}

	serveConfig := publisherConfig
	serveConfig.PublisherAPI.Enabled = false
	serveConfig.ArtifactStore = okruntime.ArtifactStoreConfig{Type: "http", Path: filepath.Join(root, "serve-cache"), URL: "http://127.0.0.1:8090", TokenEnv: "TEST_ARTIFACT_TOKEN"}
	serveConfig.Runtime.StateDir = filepath.Join(root, "serve-state")
	handler, err := newRuntimeServeHandler(serveConfig)
	if err != nil {
		t.Fatal(err)
	}
	handler.snapshots.client.Transport = runtimeHandlerRoundTripper{handler: publisherHandler}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("remote activation failed: %v", failures)
	}
	page := runtimeRequest(t, handler, http.MethodGet, "/", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Private transport v1") {
		t.Fatalf("unexpected remotely synchronized page %d: %s", page.Code, page.Body.String())
	}
	before, ok := handler.snapshots.snapshot("wiki")
	if !ok || before.Pointer.ContentDigest != first.ContentDigest {
		t.Fatalf("unexpected first remote snapshot: %#v", before)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Private transport v2\n")
	second, err := buildRuntimeKnowledgeGeneration(publisherConfig, publisherConfig.KnowledgeBases[0], "second", filepath.Join(root, "second"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publisherConfig.ArtifactStore.Path, "wiki", "generations", second.Generation, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) == 0 {
		t.Fatal("expected publisher to refuse a tampered active generation")
	}
	after, ok := handler.snapshots.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest != before.Pointer.ContentDigest {
		t.Fatalf("serve did not retain last verified generation: %#v", after)
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
	agentCheckout, err := syncRuntimeAgentRepository(t.Context(), config, "codex")
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

func TestRuntimePlanReportsAndEnforcesRequiredAgentRuntimes(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, ".openknowledge", "jobs")
	writeViewerFile(t, root, ".openknowledge/jobs/codex.md", "---\nid: codex-job\nagent: {runtime: codex}\n---\nMaintain docs.\n")
	writeViewerFile(t, root, ".openknowledge/jobs/claude.md", "---\nid: claude-job\nagent: {runtime: claude}\n---\nMaintain docs.\n")
	config := okruntime.Config{Worker: okruntime.WorkerConfig{RunJobs: true, Repo: root, JobsPath: jobs, Runtimes: []string{"claude", "codex"}}}
	required, err := runtimeRequiredRuntimes(config)
	if err != nil || !reflect.DeepEqual(required, []string{"claude", "codex"}) {
		t.Fatalf("required=%#v err=%v", required, err)
	}
	config.Worker.Runtimes = []string{"codex"}
	if _, err := runtimeRequiredRuntimes(config); err == nil || !strings.Contains(err.Error(), "requires runtime claude") {
		t.Fatalf("expected missing worker refusal, got %v", err)
	}
}

func TestRuntimePlanReportsNoRequiredRuntimeWithoutJobDefinitions(t *testing.T) {
	root := t.TempDir()
	config := okruntime.Config{Worker: okruntime.WorkerConfig{
		RunJobs: true, Repo: root, JobsPath: filepath.Join(root, ".openknowledge", "jobs"), Runtimes: []string{"codex"},
	}}
	required, err := runtimeRequiredRuntimes(config)
	if err != nil || len(required) != 0 {
		t.Fatalf("required=%#v err=%v", required, err)
	}
}

func TestRuntimeAgentCleanupUsesRuntimeStateAndRetainsAuditRecord(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	if err := os.Mkdir(repository, 0755); err != nil {
		t.Fatal(err)
	}
	runtimeGitTest(t, repository, "init", "-b", "main")
	runtimeGitTest(t, repository, "config", "user.name", "Runtime Test")
	runtimeGitTest(t, repository, "config", "user.email", "runtime@example.test")
	writeViewerFile(t, repository, "README.md", "runtime cleanup\n")
	runtimeGitTest(t, repository, "add", "README.md")
	runtimeGitTest(t, repository, "commit", "-m", "initial")

	stateDir := filepath.Join(root, "state")
	jobsState := filepath.Join(stateDir, "jobs-codex")
	t.Setenv(agents.JobsStateDirEnv, jobsState)
	runsDir, err := agents.RepositoryRunDirectory(repository)
	if err != nil {
		t.Fatal(err)
	}
	runID := "aaaaaaaaaaaaaaaaaaaaaaaa"
	runDir := filepath.Join(runsDir, runID)
	worktree := filepath.Join(filepath.Dir(runsDir), "worktrees", runID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0700); err != nil {
		t.Fatal(err)
	}
	runtimeGitTest(t, repository, "worktree", "add", "-b", "agent/cleanup-test", worktree)
	for _, artifact := range []string{"home/cache.bin", "tmp/download.bin", "diff.patch"} {
		writeViewerFile(t, runDir, artifact, strings.Repeat("x", 1024))
	}
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	record := agents.RunRecord{
		SchemaVersion: "1",
		RunID:         runID,
		JobID:         "cleanup-test",
		Status:        "failed",
		ScheduledAt:   now,
		StartedAt:     now,
		FinishedAt:    now.Add(time.Minute),
		Plan: agents.RunPlan{
			RunID:    runID,
			JobID:    "cleanup-test",
			RepoRoot: repository,
			Worktree: worktree,
			RunDir:   runDir,
		},
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), content, 0600); err != nil {
		t.Fatal(err)
	}

	config := okruntime.Config{Runtime: okruntime.RuntimeConfig{StateDir: stateDir}}
	if err := cleanupRuntimeAgentRuns(t.Context(), config, repository, "codex"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("terminal worktree still exists: %v", err)
	}
	for _, artifact := range []string{"home", "tmp", "diff.patch"} {
		if _, err := os.Stat(filepath.Join(runDir, artifact)); !os.IsNotExist(err) {
			t.Fatalf("terminal artifact %s still exists: %v", artifact, err)
		}
	}
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Fatalf("audit record was removed: %v", err)
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
