package okf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildContentReviewFullUsesExactDefaultRulesAndConcerns(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\ntype: Index\n---\n\n# Wiki\n\nRead [Guide](guide.md).\n")
	writeRuleTestFile(t, wiki, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")

	review, err := BuildContentReview(ContentReviewOptions{
		Wiki:  wiki,
		Scope: ContentReviewScopeFull,
		Now:   time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(review.Identity.RuleIDs, ",") != "project,writing" {
		t.Fatalf("unexpected rule IDs: %#v", review.Identity.RuleIDs)
	}
	if len(review.Identity.ConcernIDs) != len(ContentHealthConcerns()) || len(review.Identity.Paths) != 2 {
		t.Fatalf("unexpected review identity: %#v", review.Identity)
	}
	if len(review.Identity.BundleSHA256) != 64 || len(review.Identity.RulesSHA256) != 64 || len(review.Identity.ReviewID) != 24 {
		t.Fatalf("missing deterministic identity: %#v", review.Identity)
	}
	for _, expected := range []string{
		"Open Knowledge Content Review",
		"Review created: `2026-08-11T12:00:00Z`",
		"Review scope: `full`",
		"Deterministic validation:",
		"duplication-conflicts",
		"maintenance-priority",
		"## project",
		"## writing",
		"findings as advisory",
	} {
		if !strings.Contains(review.Prompt, expected) {
			t.Fatalf("expected prompt to include %q:\n%s", expected, review.Prompt)
		}
	}
}

func TestBuildContentReviewChangedAddsDirectDependenciesAndUntrackedPages(t *testing.T) {
	repo := t.TempDir()
	runContentReviewGit(t, repo, "init")
	runContentReviewGit(t, repo, "config", "core.autocrlf", "true")
	runContentReviewGit(t, repo, "config", "user.name", "Test")
	runContentReviewGit(t, repo, "config", "user.email", "test@example.com")
	wiki := filepath.Join(repo, "Wiki")
	writeRuleTestFile(t, wiki, "index.md", "---\ntype: Index\n---\n\n# Wiki\n\nRead [Guide](guide.md).\n")
	writeRuleTestFile(t, wiki, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")
	runContentReviewGit(t, repo, "add", "Wiki")
	runContentReviewGit(t, repo, "commit", "-m", "initial")

	writeRuleTestFile(t, wiki, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n\nChanged.\n")
	writeRuleTestFile(t, wiki, "new.md", "---\ntype: Note\n---\n\n# New\n")
	review, err := BuildContentReview(ContentReviewOptions{Wiki: wiki, Scope: ContentReviewScopeChanged})
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]string{}
	for _, item := range review.Identity.Paths {
		statuses[item.Path] = item.Status
		if item.Status != "deleted" && len(item.SHA256) != 64 {
			t.Fatalf("missing path digest: %#v", item)
		}
	}
	if statuses["guide.md"] != "changed" || statuses["new.md"] != "changed" || statuses["index.md"] != "dependency" {
		t.Fatalf("unexpected changed review scope: %#v", statuses)
	}
	if len(review.Identity.GitHead) != 40 || len(review.Identity.BaseSHA) != 40 || review.Identity.Base != "HEAD" {
		t.Fatalf("unexpected Git identity: %#v", review.Identity)
	}
	if !strings.Contains(review.Prompt, "Comparison base commit: `"+review.Identity.BaseSHA+"`") {
		t.Fatalf("prompt does not bind the comparison base:\n%s", review.Prompt)
	}
}

func TestBuildContentReviewChangedRecordsDeletedPagesAndIncomingDependencies(t *testing.T) {
	repo := t.TempDir()
	runContentReviewGit(t, repo, "init")
	runContentReviewGit(t, repo, "config", "core.autocrlf", "false")
	runContentReviewGit(t, repo, "config", "user.name", "Test")
	runContentReviewGit(t, repo, "config", "user.email", "test@example.com")
	wiki := filepath.Join(repo, "Wiki")
	writeRuleTestFile(t, wiki, "index.md", "---\ntype: Index\n---\n\n# Wiki\n\nRead [Old](old.md).\n")
	writeRuleTestFile(t, wiki, "old.md", "---\ntype: Note\n---\n\n# Old\n")
	runContentReviewGit(t, repo, "add", "Wiki")
	runContentReviewGit(t, repo, "commit", "-m", "initial")
	if err := os.Remove(filepath.Join(wiki, "old.md")); err != nil {
		t.Fatal(err)
	}

	review, err := BuildContentReview(ContentReviewOptions{Wiki: wiki, Scope: ContentReviewScopeChanged})
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]ContentReviewPathIdentity{}
	for _, item := range review.Identity.Paths {
		statuses[item.Path] = item
	}
	if statuses["old.md"].Status != "deleted" || statuses["old.md"].SHA256 != "" || statuses["index.md"].Status != "dependency" {
		t.Fatalf("unexpected deleted review scope: %#v", statuses)
	}
}

func TestContentReviewIDIsPortableAcrossWikiLocations(t *testing.T) {
	firstWiki := t.TempDir()
	secondWiki := t.TempDir()
	content := "---\ntype: Index\n---\n\n# Wiki\n"
	writeRuleTestFile(t, firstWiki, "index.md", content)
	writeRuleTestFile(t, secondWiki, "index.md", content)
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	first, err := BuildContentReview(ContentReviewOptions{Wiki: firstWiki, Scope: ContentReviewScopeFull, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildContentReview(ContentReviewOptions{Wiki: secondWiki, Scope: ContentReviewScopeFull, Now: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity.ReviewID != second.Identity.ReviewID || first.Identity.BundleSHA256 != second.Identity.BundleSHA256 {
		t.Fatalf("review identity must not depend on local path or review time: first=%#v second=%#v", first.Identity, second.Identity)
	}
}

func TestBuildContentReviewRejectsInvalidScopeConcernAndChangedReviewOutsideGit(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\ntype: Index\n---\n\n# Wiki\n")
	for _, options := range []ContentReviewOptions{
		{Wiki: wiki, Scope: "partial"},
		{Wiki: wiki, Scope: ContentReviewScopeFull, Concerns: []string{"unknown"}},
		{Wiki: wiki, Scope: ContentReviewScopeFull, Base: "main"},
		{Wiki: wiki, Scope: ContentReviewScopeChanged},
	} {
		if _, err := BuildContentReview(options); err == nil {
			t.Fatalf("expected content review options to fail: %#v", options)
		}
	}
}

func TestContentReviewIdentityTracksRuleOrder(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\ntype: Index\n---\n\n# Wiki\n")
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	first, err := BuildContentReview(ContentReviewOptions{Wiki: wiki, Scope: ContentReviewScopeFull, Rules: []string{"project", "writing"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildContentReview(ContentReviewOptions{Wiki: wiki, Scope: ContentReviewScopeFull, Rules: []string{"writing", "project"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity.RulesSHA256 == second.Identity.RulesSHA256 || first.Identity.ReviewID == second.Identity.ReviewID {
		t.Fatal("review identity must track ordered rule instructions")
	}
}

func runContentReviewGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
