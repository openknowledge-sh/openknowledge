package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestViewerRendersIndexAndMarkdownFile(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nSee [Workflow](workflows/docs.md) and [Concepts](concepts/).\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs\n---\n\n# Docs\n\n- Update docs\n")
	writeViewerFile(t, root, "concepts/index.md", "# Concepts\n")

	handler := newViewerHandler(root)

	index := getViewerBody(t, handler, "/")
	if !strings.Contains(index, "index.md") || !strings.Contains(index, "workflows/docs.md") {
		t.Fatalf("viewer index did not include markdown files:\n%s", index)
	}
	if !strings.Contains(index, `id="viewer-search"`) {
		t.Fatalf("viewer index did not include search input:\n%s", index)
	}

	page := getViewerBody(t, handler, "/file/index.md")
	if strings.Contains(page, "okf_version") {
		t.Fatalf("viewer should strip frontmatter:\n%s", page)
	}
	if !strings.Contains(page, "<h1>Home</h1>") {
		t.Fatalf("viewer did not render heading:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/workflows/docs.md"`) {
		t.Fatalf("viewer did not rewrite relative markdown link:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/concepts/index.md"`) {
		t.Fatalf("viewer did not rewrite directory index link:\n%s", page)
	}
}

func TestViewerSearchAPI(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nRead the workflow docs.\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\n---\n\n# Docs\n\nRun validation before publishing.\n")

	handler := newViewerHandler(root)
	payload := getViewerSearch(t, handler, "/api/search?q=validaton&limit=4")
	if payload.Query != "validaton" {
		t.Fatalf("expected query echo, got %#v", payload)
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected fuzzy search results, got %#v", payload)
	}
	if payload.Results[0].Path != "workflows/docs.md" || payload.Results[0].URL != "/file/workflows/docs.md" {
		t.Fatalf("unexpected search result: %#v", payload.Results[0])
	}
}

func TestViewerSearchRefreshesAfterMarkdownChanges(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")

	handler := newViewerHandler(root)
	first := getViewerSearch(t, handler, "/api/search?q=draft")
	if len(first.Results) != 0 {
		t.Fatalf("expected no draft results before file is written, got %#v", first)
	}

	writeViewerFile(t, root, "notes/draft.md", "---\ntype: Note\ntitle: Draft Note\n---\n\n# Draft\n\nFresh searchable content.\n")
	second := getViewerSearch(t, handler, "/api/search?q=draft")
	if len(second.Results) == 0 || second.Results[0].Path != "notes/draft.md" {
		t.Fatalf("expected refreshed search result, got %#v", second)
	}
}

func TestViewerRejectsTraversalAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes.txt", "not markdown\n")
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := newViewerHandler(root)

	if _, ok := safeMarkdownPath(root, "../outside.md"); ok {
		t.Fatal("expected traversal path to be rejected")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/file/notes.txt", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected non-markdown file to return 404, got %d", recorder.Code)
	}
}

func TestRegistryViewerRendersWorkspaceSelectorAndSwitchesBases(t *testing.T) {
	personal := t.TempDir()
	work := t.TempDir()
	writeViewerFile(t, personal, "index.md", "# Personal\n")
	writeViewerFile(t, personal, "only-personal.md", "---\ntype: Note\n---\n\n# Personal note\n")
	writeViewerFile(t, work, "index.md", "# Work\n\nSee [Guide](notes/guide.md).\n")
	writeViewerFile(t, work, "notes/guide.md", "---\ntype: Note\n---\n\n# Guide\n")

	handler := newRegistryViewerHandler([]okf.RegistryEntry{
		{Name: "personal", Path: personal},
		{Name: "work", Path: work},
	})

	index := getViewerBody(t, handler, "/")
	for _, required := range []string{
		"Knowledge bases",
		`href="/kb/personal/"`,
		`href="/kb/work/"`,
		"only-personal.md",
	} {
		if !strings.Contains(index, required) {
			t.Fatalf("registry index missing %q:\n%s", required, index)
		}
	}

	workIndex := getViewerBody(t, handler, "/kb/work/")
	if !strings.Contains(workIndex, `class="workspace active" href="/kb/work/"`) {
		t.Fatalf("work knowledge base was not active:\n%s", workIndex)
	}
	if !strings.Contains(workIndex, "notes/guide.md") || strings.Contains(workIndex, "only-personal.md") {
		t.Fatalf("work index did not switch file listing:\n%s", workIndex)
	}

	workPage := getViewerBody(t, handler, "/kb/work/file/index.md")
	if !strings.Contains(workPage, `href="/kb/work/file/notes/guide.md"`) {
		t.Fatalf("registry viewer did not prefix markdown links:\n%s", workPage)
	}
}

func TestRegistryViewerEmptyRegistry(t *testing.T) {
	body := getViewerBody(t, newRegistryViewerHandler(nil), "/")
	if !strings.Contains(body, "No registered knowledge bases") {
		t.Fatalf("empty registry page did not explain the empty state:\n%s", body)
	}
}

func writeViewerFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func getViewerBody(t *testing.T, handler http.Handler, target string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d: %s", target, response.StatusCode, string(body))
	}
	return string(body)
}

func getViewerSearch(t *testing.T, handler http.Handler, target string) viewerSearchResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	response := recorder.Result()
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d: %s", target, response.StatusCode, string(body))
	}

	var payload viewerSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
