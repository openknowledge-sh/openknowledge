package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestViewerLiveReloadRevisionTracksVisibleFileOperations(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	initial, err := viewerLiveReloadRootRevision(root)
	if err != nil {
		t.Fatal(err)
	}

	writeViewerFile(t, root, "notes/new.md", "# New\n")
	added := viewerLiveReloadTestRevision(t, root)
	if added == initial {
		t.Fatal("adding a Markdown file must change the live reload revision")
	}

	path := filepath.Join(root, "notes", "new.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	modified := viewerLiveReloadTestRevision(t, root)
	if modified == added {
		t.Fatal("same-size and same-mtime content changes must change the revision")
	}

	renamedPath := filepath.Join(root, "notes", "renamed.md")
	if err := os.Rename(path, renamedPath); err != nil {
		t.Fatal(err)
	}
	renamed := viewerLiveReloadTestRevision(t, root)
	if renamed == modified {
		t.Fatal("renaming a file must change the revision")
	}

	if err := os.Remove(renamedPath); err != nil {
		t.Fatal(err)
	}
	deleted := viewerLiveReloadTestRevision(t, root)
	if deleted == renamed || deleted != initial {
		t.Fatalf("deleting the added file should restore the initial revision: initial=%s deleted=%s", initial, deleted)
	}

	writeViewerFile(t, root, ".secret.md", "secret\n")
	writeViewerFile(t, root, "notes/draft.md~", "temporary\n")
	if ignored := viewerLiveReloadTestRevision(t, root); ignored != deleted {
		t.Fatal("private and temporary files must not change the live reload revision")
	}
}

func TestViewerLiveReloadReusesUnchangedFileDigests(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "assets/large.svg", strings.Repeat("x", 1024*1024))
	roots := []viewerLiveReloadRoot{{Alias: "docs", Root: root}}

	initial, _, files, filesRead, err := viewerLiveReloadRevisionCached(roots, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filesRead != 2 {
		t.Fatalf("initial revision should read two files, got %d", filesRead)
	}
	unchanged, _, files, filesRead, err := viewerLiveReloadRevisionCached(roots, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged != initial || filesRead != 0 {
		t.Fatalf("unchanged revision should reuse file digests: initial=%s unchanged=%s reads=%d", initial, unchanged, filesRead)
	}

	path := filepath.Join(root, "index.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Else\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, _, _, filesRead, err := viewerLiveReloadRevisionCached(roots, files, map[string]bool{path: true})
	if err != nil {
		t.Fatal(err)
	}
	if changed == unchanged || filesRead != 1 {
		t.Fatalf("dirty same-metadata file should refresh one digest: unchanged=%s changed=%s reads=%d", unchanged, changed, filesRead)
	}
}

func TestViewerLiveReloadPublishesOneDebouncedRevision(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	hub, err := newViewerLiveReload(context.Background(), func() ([]viewerLiveReloadRoot, error) {
		return []viewerLiveReloadRoot{{Alias: "docs", Root: root}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	subscription, initial, ok := hub.subscribe()
	if !ok {
		t.Fatal("expected live reload subscription")
	}
	defer hub.unsubscribe(subscription)

	writeViewerFile(t, root, "notes/change.md", "one\n")
	writeViewerFile(t, root, "notes/change.md", "two\n")
	writeViewerFile(t, root, "notes/change.md", "three\n")
	event := viewerLiveReloadTestEvent(t, subscription.events)
	if event.Revision == "" || event.Revision == initial.Revision || len(event.KnowledgeBases) != 1 || event.KnowledgeBases[0] != "docs" {
		t.Fatalf("unexpected live reload event: %#v", event)
	}
	select {
	case duplicate := <-subscription.events:
		t.Fatalf("one save burst must publish one revision, got %#v", duplicate)
	case <-time.After(2 * viewerLiveReloadDebounce):
	}
}

func TestViewerLiveReloadSSEAndSecurity(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	hub, err := newViewerLiveReload(context.Background(), func() ([]viewerLiveReloadRoot, error) {
		return []viewerLiveReloadRoot{{Alias: "docs", Root: root}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	secure := secureViewerHandler(viewerLiveReloadHandler(http.NotFoundHandler(), hub), "0123456789abcdef")
	unauthorized := httptest.NewRecorder()
	secure.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/viewer-events", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("live reload endpoint must use viewer authentication, got %d", unauthorized.Code)
	}

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/viewer-events", nil).WithContext(ctx)
	stream := newViewerLiveReloadTestResponse()
	done := make(chan struct{})
	go func() {
		hub.serveEvents(stream, request)
		close(done)
	}()
	select {
	case <-stream.flushed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initial SSE flush")
	}
	if !strings.HasPrefix(stream.Header().Get("Content-Type"), "text/event-stream") || stream.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected SSE headers: %v", stream.Header())
	}
	ready := stream.String()
	if !strings.Contains(ready, "event: ready") || !strings.Contains(ready, `"revision":"`) || strings.Contains(ready, root) {
		t.Fatalf("unexpected or unsafe ready event: %q", ready)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SSE handler did not stop after request cancellation")
	}
}

func TestLocalViewerIncludesLiveReloadClientButStaticExportDoesNot(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	writeViewerFile(t, root, "index.md", "# Home\n")
	page := getViewerBody(t, newViewerHandler(root), "/file/index.md")
	if !strings.Contains(page, viewerLiveReloadScriptAsset) {
		t.Fatalf("local viewer page did not load live reload client:\n%s", page)
	}
	runtime := getViewerBody(t, newViewerHandler(root), "/"+viewerLiveReloadScriptAsset)
	for _, expected := range []string{"EventSource", "/api/viewer-events", "OpenKnowledgeViewerLiveReload"} {
		if !strings.Contains(runtime, expected) {
			t.Fatalf("live reload client missing %q", expected)
		}
	}

	out := t.TempDir()
	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	staticPage := readViewerExportFile(t, out, "index.html")
	if strings.Contains(staticPage, viewerLiveReloadScriptAsset) {
		t.Fatal("static HTML export must not load the live reload client")
	}
	if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(viewerLiveReloadScriptAsset))); !os.IsNotExist(err) {
		t.Fatalf("static HTML export must not contain a live reload asset, err=%v", err)
	}
}

func viewerLiveReloadTestRevision(t *testing.T, root string) string {
	t.Helper()
	revision, err := viewerLiveReloadRootRevision(root)
	if err != nil {
		t.Fatal(err)
	}
	return revision
}

func viewerLiveReloadTestEvent(t *testing.T, events <-chan viewerLiveReloadEvent) viewerLiveReloadEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for live reload event")
		return viewerLiveReloadEvent{}
	}
}

type viewerLiveReloadTestResponse struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	flushed chan struct{}
}

func newViewerLiveReloadTestResponse() *viewerLiveReloadTestResponse {
	return &viewerLiveReloadTestResponse{header: make(http.Header), flushed: make(chan struct{}, 8)}
}

func (response *viewerLiveReloadTestResponse) Header() http.Header {
	return response.header
}

func (response *viewerLiveReloadTestResponse) WriteHeader(_ int) {}

func (response *viewerLiveReloadTestResponse) Write(content []byte) (int, error) {
	response.mu.Lock()
	defer response.mu.Unlock()
	return response.body.Write(content)
}

func (response *viewerLiveReloadTestResponse) Flush() {
	select {
	case response.flushed <- struct{}{}:
	default:
	}
}

func (response *viewerLiveReloadTestResponse) String() string {
	response.mu.Lock()
	defer response.mu.Unlock()
	return response.body.String()
}
