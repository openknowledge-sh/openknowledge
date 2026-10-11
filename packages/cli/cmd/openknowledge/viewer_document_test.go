package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/authoring"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func viewerDocumentFixture(t *testing.T) string {
	t.Helper()
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")
	return root
}

func viewerDocumentRequest(t *testing.T, handler http.Handler, method, route, body, origin, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://localhost"+route, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestViewerDocumentRoutesReadAndSave(t *testing.T) {
	root := viewerDocumentFixture(t)
	for _, tc := range []struct {
		name, prefix string
		handler      http.Handler
	}{
		{"direct", "", newViewerHandler(root)},
		{"direct-alias", "/docs", newViewerHandlerWithAlias(root, "docs")},
		{"registry", "/kb/docs", newRegistryViewerHandler([]okf.RegistryEntry{{Name: "docs", Path: root, Access: "write"}})},
		{"registry-alias", "/docs", newRegistryViewerHandler([]okf.RegistryEntry{{Name: "docs", Path: root, Access: "write"}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route := tc.prefix + "/api/document"
			read := viewerDocumentRequest(t, tc.handler, "GET", route+"?path=guide.md", "", "", "")
			var document authoring.Document
			if read.Code != 200 || json.Unmarshal(read.Body.Bytes(), &document) != nil || !document.Writable {
				t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
			}
			body, _ := json.Marshal(authoring.SaveRequest{Path: document.Path, Text: document.Text + "\nSaved from " + tc.name + ".\n", ExpectedRevision: document.Revision})
			saved := viewerDocumentRequest(t, tc.handler, "POST", route, string(body), "http://localhost", "application/json")
			var result authoring.SaveResult
			if saved.Code != 200 || json.Unmarshal(saved.Body.Bytes(), &result) != nil || result.Revision == document.Revision {
				t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
			}
			stale := viewerDocumentRequest(t, tc.handler, "POST", route, string(body), "http://localhost", "application/json")
			var conflict struct {
				Code    string
				Current authoring.Document
			}
			if stale.Code != 409 || json.Unmarshal(stale.Body.Bytes(), &conflict) != nil || conflict.Code != "conflict" || conflict.Current.Text != result.Text {
				t.Fatalf("conflict status=%d body=%s", stale.Code, stale.Body.String())
			}
		})
	}
}

func TestViewerDocumentRejectsUnsafeRequests(t *testing.T) {
	root := viewerDocumentFixture(t)
	handler := newViewerHandler(root)
	document, _ := authoring.Read(root, "guide.md", false)
	valid, _ := json.Marshal(authoring.SaveRequest{Path: document.Path, Text: document.Text + "\nDraft\n", ExpectedRevision: document.Revision})
	for _, tc := range []struct {
		name, method, route, body, origin, contentType string
		status                                         int
	}{
		{"missing-origin", "POST", "/api/document", string(valid), "", "application/json", 403},
		{"foreign-origin", "POST", "/api/document", string(valid), "http://elsewhere.test", "application/json", 403},
		{"wrong-scheme", "POST", "/api/document", string(valid), "https://localhost", "application/json", 403},
		{"invalid-origin", "POST", "/api/document", string(valid), "http://localhost/path", "application/json", 403},
		{"wrong-content-type", "POST", "/api/document", string(valid), "http://localhost", "text/plain", 415},
		{"missing-text", "POST", "/api/document", `{"path":"guide.md","expectedRevision":"` + document.Revision + `"}`, "http://localhost", "application/json", 400},
		{"duplicate-field", "POST", "/api/document", `{"path":"guide.md","path":"index.md","text":"x","expectedRevision":"` + document.Revision + `"}`, "http://localhost", "application/json", 400},
		{"extra-field", "POST", "/api/document", strings.TrimSuffix(string(valid), "}") + `,"force":true}`, "http://localhost", "application/json", 400},
		{"trailing-json", "POST", "/api/document", string(valid) + `{}`, "http://localhost", "application/json", 400},
		{"oversize-envelope", "POST", "/api/document", strings.Repeat(" ", viewerDocumentMaxRequestBytes+1), "http://localhost", "application/json", 413},
		{"oversize-document", "POST", "/api/document", `{"path":"guide.md","text":"` + strings.Repeat("a", authoring.MaxDocumentBytes+1) + `","expectedRevision":"` + document.Revision + `"}`, "http://localhost", "application/json", 413},
		{"method", "DELETE", "/api/document", "", "http://localhost", "application/json", 405},
		{"traversal", "GET", "/api/document?path=../guide.md", "", "", "", 404},
		{"missing", "GET", "/api/document?path=missing.md", "", "", "", 404},
		{"invalid-markdown", "POST", "/api/document", `{"path":"guide.md","text":"# No frontmatter","expectedRevision":"` + document.Revision + `"}`, "http://localhost", "application/json", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := viewerDocumentRequest(t, handler, tc.method, tc.route, tc.body, tc.origin, tc.contentType)
			if response.Code != tc.status || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
			if string(content) != document.Text {
				t.Fatal("rejected request changed the document")
			}
		})
	}
}

func TestViewerDocumentAllowsJSONEscapingAtTextLimit(t *testing.T) {
	root := viewerDocumentFixture(t)
	document, _ := authoring.Read(root, "guide.md", false)
	// '<' becomes six bytes in encoding/json. The document remains exactly
	// 1 MiB even though its JSON envelope is much larger than 1 MiB.
	text := document.Text + strings.Repeat("<", authoring.MaxDocumentBytes-len(document.Text))
	body, _ := json.Marshal(authoring.SaveRequest{Path: document.Path, Text: text, ExpectedRevision: document.Revision})
	response := viewerDocumentRequest(t, newViewerHandler(root), "POST", "/api/document", string(body), "http://localhost", "application/json")
	if response.Code != http.StatusOK {
		t.Fatalf("escaped save status=%d body=%s", response.Code, response.Body.String())
	}
	content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
	if string(content) != text {
		t.Fatal("escaped content did not round-trip")
	}
}

func TestViewerDocumentReadOnlyCapability(t *testing.T) {
	root := viewerDocumentFixture(t)
	for _, tc := range []struct {
		route   string
		handler http.Handler
	}{
		{"/api/document", newViewerHandlerWithOptions(root, viewerOptions{ReadOnly: true})},
		{"/kb/docs/api/document", newRegistryViewerHandler([]okf.RegistryEntry{{Name: "docs", Path: root, Access: "read"}})},
		{"/docs/api/document", newRegistryViewerHandler([]okf.RegistryEntry{{Name: "docs", Path: root, Access: "read"}})},
	} {
		read := viewerDocumentRequest(t, tc.handler, "GET", tc.route+"?path=guide.md", "", "", "")
		var document authoring.Document
		if read.Code != 200 || json.Unmarshal(read.Body.Bytes(), &document) != nil || document.Writable {
			t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
		}
		body, _ := json.Marshal(authoring.SaveRequest{Path: document.Path, Text: document.Text, ExpectedRevision: document.Revision})
		saved := viewerDocumentRequest(t, tc.handler, "POST", tc.route, string(body), "http://localhost", "application/json")
		if saved.Code != 403 {
			t.Fatalf("read-only save status=%d body=%s", saved.Code, saved.Body.String())
		}
	}
}
