package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/insights"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestViewerFeedbackRespectsIntegrationOriginAndReadOnly(t *testing.T) {
	repo := t.TempDir()
	insightGitOutput(t, repo, "init")
	root := filepath.Join(repo, "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.InstallProject(root); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, origin, body string
		readOnly                   bool
		status                     int
	}{
		{"readonly", "POST", "http://localhost", `{"summary":"Fix guide","path":"index.md"}`, true, 403},
		{"cross-origin", "POST", "http://example.com", `{"summary":"Fix guide"}`, false, 403},
		{"missing-origin", "POST", "", `{"summary":"Fix guide"}`, false, 403},
		{"method", "GET", "http://localhost", "", false, 405},
		{"traversal", "POST", "http://localhost", `{"summary":"Fix guide","path":"../secret"}`, false, 400},
		{"empty", "POST", "http://localhost", `{"summary":"  "}`, false, 400},
		{"trailing", "POST", "http://localhost", `{"summary":"Fix guide"}{}`, false, 400},
		{"valid", "POST", "http://localhost", `{"summary":"Explain the first step more clearly","path":"index.md","query":"How do I start?"}`, false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "http://localhost/api/feedback", strings.NewReader(tc.body))
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			if !serveViewerProductAPI(response, request, root, "api/feedback", tc.readOnly) || response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	pending, err := insights.Pending(root)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%#v error=%v", pending, err)
	}
	response := httptest.NewRecorder()
	serveViewerProductAPI(response, httptest.NewRequest("GET", "http://localhost/api/health", nil), root, "api/health", false)
	var health viewerHealth
	if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil || !health.FeedbackAvailable || len(health.Pending) != 1 {
		t.Fatalf("health=%#v error=%v", health, err)
	}
	if viewerFeedbackAvailable(repo, false) {
		t.Fatal("repository root is not the integrated knowledge root")
	}
}

func TestCheckSeparatesViewerAndMissingRuntimeReadiness(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, root, okf.ValidationConfigFile, "[release]\noutputs = [\"viewer\", \"mcp\"]\n\n[rules]\nenabled = [\"project\", \"writing\"]\n")
	report, err := buildKnowledgeCheckReportWithRuntime(root, filepath.Join(t.TempDir(), "missing.toml"), "")
	if err != nil {
		t.Fatal(err)
	}
	layers := map[string]string{}
	for _, layer := range report.Layers {
		layers[layer.Name] = layer.Status
	}
	if layers["Publication"] != checkReady || layers["Target viewer"] != checkReady || layers["Target mcp"] != checkNeedsAttention || report.Overall != checkNeedsAttention {
		t.Fatalf("report=%#v", report)
	}
	_, stderr, code := captureMainOutput(t, func() int { return runPublish([]string{root, "--target", "viewer", "--plan"}) })
	if code != 0 {
		t.Fatalf("viewer plan failed: %s", stderr)
	}
}
