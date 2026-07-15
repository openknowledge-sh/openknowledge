//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTimedOutHostCommandKillsDescendantProcessGroup(t *testing.T) {
	runDir := t.TempDir()
	childPIDPath := filepath.Join(runDir, "child.pid")
	plan := RunPlan{
		RunDir:   runDir,
		Worktree: t.TempDir(),
		Sandbox:  SandboxSpec{Type: "host"},
	}
	command := Command{
		Command: fmt.Sprintf("sleep 30 & echo $! > %q; wait", childPIDPath),
		Shell:   true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	result := runPlanCommand(ctx, plan, command, "timeout", "", nil)
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || result.ExitCode == 0 {
		t.Fatalf("expected timed-out command result, ctx=%v result=%#v", ctx.Err(), result)
	}

	content, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil {
			t.Fatalf("inspect descendant process %d: %v", childPID, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant process %d survived command timeout", childPID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
