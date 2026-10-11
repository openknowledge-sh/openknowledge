package authoring

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const initialText = "---\r\ntype: Guide\r\ntitle: Original\r\ncustom: keep-me\r\n---\r\n\r\n# Original\r\n"

func documentFixture(t *testing.T) string {
	t.Helper()
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte(initialText), 0640); err != nil {
		t.Fatal(err)
	}
	return root
}

func requireCode(t *testing.T, err error, code string) *Error {
	t.Helper()
	var problem *Error
	if !errors.As(err, &problem) || problem.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
	return problem
}

func TestSavePreservesRawMarkdownAndPermissions(t *testing.T) {
	root := documentFixture(t)
	document, err := Read(root, "guide.md", false)
	if err != nil || document.Text != initialText || !document.Writable || len(document.Revision) != 64 {
		t.Fatalf("read=%#v error=%v", document, err)
	}
	updated := strings.Replace(initialText, "# Original", "# Updated", 1)
	result, err := Save(context.Background(), root, SaveRequest{"guide.md", updated, document.Revision}, false)
	if err != nil || result.Revision == document.Revision || result.Text != updated {
		t.Fatalf("save=%#v error=%v", result, err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
	info, _ := os.Stat(filepath.Join(root, "guide.md"))
	if string(content) != updated || info.Mode().Perm() != 0640 {
		t.Fatalf("raw content or permissions changed: %q %v", content, info.Mode())
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".okn-edit-*"))
	if len(matches) != 0 {
		t.Fatalf("staging files leaked: %v", matches)
	}
}

func TestSaveRejectsInvalidDraftWithoutChangingDisk(t *testing.T) {
	root := documentFixture(t)
	document, _ := Read(root, "guide.md", false)
	for _, text := range []string{"# Missing type", "---\ntype: [broken\n---\n# Draft\n", string([]byte{'#', 0xff})} {
		_, err := Save(context.Background(), root, SaveRequest{"guide.md", text, document.Revision}, false)
		problem := requireCode(t, err, "validation")
		if len(problem.Diagnostics) == 0 {
			t.Fatal("missing actionable diagnostics")
		}
		content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
		if string(content) != initialText {
			t.Fatal("invalid draft changed disk")
		}
	}
}

func TestSaveConflictIncludesLatestDocument(t *testing.T) {
	root := documentFixture(t)
	document, _ := Read(root, "guide.md", false)
	external := strings.Replace(initialText, "# Original", "# External", 1)
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte(external), 0640); err != nil {
		t.Fatal(err)
	}
	_, err := Save(context.Background(), root, SaveRequest{"guide.md", initialText, document.Revision}, false)
	problem := requireCode(t, err, "conflict")
	if problem.Current == nil || problem.Current.Text != external || problem.Current.Revision == document.Revision {
		t.Fatalf("missing current conflict content: %#v", problem)
	}
}

func TestConcurrentSavesWithSameRevisionHaveOneWinner(t *testing.T) {
	root := documentFixture(t)
	document, _ := Read(root, "guide.md", false)
	errors := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	for _, heading := range []string{"# First", "# Second"} {
		go func(heading string) {
			ready.Done()
			<-start
			_, err := Save(context.Background(), root, SaveRequest{"guide.md", strings.Replace(initialText, "# Original", heading, 1), document.Revision}, false)
			errors <- err
		}(heading)
	}
	ready.Wait()
	close(start)
	first, second := <-errors, <-errors
	if first == nil && second == nil || first != nil && second != nil {
		t.Fatalf("expected one winner: %v, %v", first, second)
	}
	if first != nil {
		requireCode(t, first, "conflict")
	} else {
		requireCode(t, second, "conflict")
	}
}

func TestDocumentAccessBoundariesAndLimits(t *testing.T) {
	root := documentFixture(t)
	document, _ := Read(root, "guide.md", false)
	for _, directory := range []string{".git", ".GIT", ".GiT"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, directory, "hidden.md"), []byte(initialText), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, relative := range []string{"../guide.md", "/guide.md", "./guide.md", "sub/../guide.md", "guide.md/", " guide.md", "guide.md ", "missing.md", "guide.txt", "sub\\guide.md", ".git/hidden.md", ".GIT/hidden.md", ".GiT/hidden.md"} {
		_, err := Read(root, relative, false)
		requireCode(t, err, "not_found")
	}
	for _, relative := range []string{"linked.md", "linked-directory/guide.md"} {
		link, target := filepath.Join(root, "linked.md"), filepath.Join(root, "guide.md")
		if strings.Contains(relative, "/") {
			link, target = filepath.Join(root, "linked-directory"), root
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		_, err := Read(root, relative, false)
		requireCode(t, err, "not_found")
	}
	_, err := Save(context.Background(), root, SaveRequest{"guide.md", initialText, ""}, false)
	requireCode(t, err, "invalid_request")
	_, err = Save(context.Background(), root, SaveRequest{"guide.md", strings.Repeat("a", MaxDocumentBytes+1), document.Revision}, false)
	requireCode(t, err, "too_large")
	_, err = Save(context.Background(), root, SaveRequest{"guide.md", initialText, document.Revision}, true)
	requireCode(t, err, "read_only")
	if _, _, err := okf.ConnectRegistryEntry("knowledge", root, "read", true); err != nil {
		t.Fatal(err)
	}
	readonly, err := Read(root, "guide.md", false)
	if err != nil || readonly.Writable {
		t.Fatalf("registry access not enforced: %#v %v", readonly, err)
	}
	_, err = Save(context.Background(), root, SaveRequest{"guide.md", initialText, document.Revision}, false)
	requireCode(t, err, "read_only")
}

func TestCanceledSaveDoesNotChangeDisk(t *testing.T) {
	root := documentFixture(t)
	document, _ := Read(root, "guide.md", false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Save(ctx, root, SaveRequest{"guide.md", initialText + "change", document.Revision}, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wanted cancellation, got %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
	if string(content) != initialText {
		t.Fatal("canceled save changed disk")
	}
}
