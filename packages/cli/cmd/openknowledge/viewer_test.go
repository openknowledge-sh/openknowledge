package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestViewerInjectsHeadHTMLWhenConfigured(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")

	defaultBody := getViewerBody(t, newViewerHandler(root), "/")
	if strings.Contains(defaultBody, "ok-head-test") {
		t.Fatalf("viewer should not include custom head HTML by default:\n%s", defaultBody)
	}

	headHTML, err := loadHeadInjection(headInjectionOptions{
		HTML:       `<meta name="ok-head-test" content="inline">`,
		ScriptSrcs: []string{"/analytics.js"},
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := newViewerHandlerWithOptions(root, viewerOptions{HeadHTML: headHTML})
	index := getViewerBody(t, handler, "/")
	if !strings.Contains(index, `<meta name="ok-head-test" content="inline">`) {
		t.Fatalf("viewer index did not include custom head HTML:\n%s", index)
	}
	if !strings.Contains(index, `<script src="/analytics.js"></script>`) {
		t.Fatalf("viewer index did not include script src:\n%s", index)
	}

	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `<meta name="ok-head-test" content="inline">`) {
		t.Fatalf("viewer page did not include custom head HTML:\n%s", page)
	}
}

func TestLoadHeadInjectionReadsFragmentFile(t *testing.T) {
	root := t.TempDir()
	headFile := filepath.Join(root, "head.html")
	if err := os.WriteFile(headFile, []byte(`<meta name="ok-head-file" content="1">`), 0644); err != nil {
		t.Fatal(err)
	}

	headHTML, err := loadHeadInjection(headInjectionOptions{File: headFile})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(headHTML), `name="ok-head-file"`) {
		t.Fatalf("expected head file content to be included, got %s", headHTML)
	}
}

func TestLoadHeadInjectionRejectsUnsupportedScriptScheme(t *testing.T) {
	_, err := loadHeadInjection(headInjectionOptions{
		ScriptSrcs: []string{"javascript:alert(1)"},
	})
	if err == nil {
		t.Fatal("expected unsupported script scheme to be rejected")
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
