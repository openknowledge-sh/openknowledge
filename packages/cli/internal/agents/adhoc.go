package agents

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IsolatedWorkspace describes an ad-hoc worktree created for a human-driven
// agent session. The worktree is intentionally retained after the session so
// its uncommitted changes can be inspected or continued.
type IsolatedWorkspace struct {
	RepoRoot string
	Branch   string
	Worktree string
	WorkDir  string
}

// PrepareIsolatedWorkspace creates a branch and worktree at the repository's
// current HEAD while preserving the caller's relative directory within it.
func PrepareIsolatedWorkspace(target string) (IsolatedWorkspace, error) {
	if strings.TrimSpace(target) == "" {
		target = "."
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return IsolatedWorkspace{}, err
	}
	info, err := os.Stat(absoluteTarget)
	if err != nil {
		return IsolatedWorkspace{}, err
	}
	if !info.IsDir() {
		return IsolatedWorkspace{}, fmt.Errorf("agent path is not a directory: %s", absoluteTarget)
	}
	absoluteTarget, err = canonicalPath(absoluteTarget)
	if err != nil {
		return IsolatedWorkspace{}, err
	}
	repoRoot, err := gitOutput(absoluteTarget, "rev-parse", "--show-toplevel")
	if err != nil {
		return IsolatedWorkspace{}, fmt.Errorf("--isolate requires a Git repository: %w", err)
	}
	repoRoot, err = canonicalPath(repoRoot)
	if err != nil {
		return IsolatedWorkspace{}, err
	}
	relative, err := filepath.Rel(repoRoot, absoluteTarget)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return IsolatedWorkspace{}, fmt.Errorf("agent path must be inside its Git repository")
	}

	stateRoot, err := JobsStateDirectory()
	if err != nil {
		return IsolatedWorkspace{}, err
	}
	if pathInside(repoRoot, stateRoot) {
		return IsolatedWorkspace{}, fmt.Errorf("jobs state directory must be outside the Git repository: %s", stateRoot)
	}
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return IsolatedWorkspace{}, err
	}
	id := time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(random)
	branch := "agent/" + id
	worktree := filepath.Join(stateRoot, repositoryStateName(repoRoot), "interactive-worktrees", id)
	if err := os.MkdirAll(filepath.Dir(worktree), 0700); err != nil {
		return IsolatedWorkspace{}, err
	}
	command := exec.Command("git", "worktree", "add", "-b", branch, worktree, "HEAD")
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return IsolatedWorkspace{}, fmt.Errorf("create isolated agent worktree: %w\n%s", err, output)
	}
	return IsolatedWorkspace{
		RepoRoot: repoRoot,
		Branch:   branch,
		Worktree: worktree,
		WorkDir:  filepath.Join(worktree, relative),
	}, nil
}
