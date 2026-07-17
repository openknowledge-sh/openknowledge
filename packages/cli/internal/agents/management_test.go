package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNextScheduledAtReportsEligibleSlot(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 7, 30, 0, time.UTC)
	tests := []struct {
		name string
		job  Job
		want time.Time
		ok   bool
	}{
		{
			name: "every",
			job:  Job{Enabled: true, Schedule: ScheduleSpec{Every: "15m", Timezone: "UTC"}},
			want: time.Date(2026, 7, 15, 10, 15, 0, 0, time.UTC),
			ok:   true,
		},
		{
			name: "cron",
			job:  Job{Enabled: true, Schedule: ScheduleSpec{Cron: "30 10 * * *", Timezone: "UTC"}},
			want: time.Date(2026, 7, 15, 10, 30, 0, 0, time.UTC),
			ok:   true,
		},
		{name: "manual", job: Job{Enabled: true}},
		{name: "disabled", job: Job{Enabled: false, Schedule: ScheduleSpec{Every: "15m"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok, err := NextScheduledAt(test.job, now)
			if err != nil {
				t.Fatal(err)
			}
			if ok != test.ok || (ok && !got.Equal(test.want)) {
				t.Fatalf("NextScheduledAt() = %s, %t; want %s, %t", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestListRunsSortsHistoryAndMarksOrphanedRecords(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "state"))
	runsDir, err := RepositoryRunDirectory(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runsDir, privateRunDirMode); err != nil {
		t.Fatal(err)
	}
	write := func(id, jobID, status string, started time.Time) {
		t.Helper()
		runDir := filepath.Join(runsDir, id)
		if err := os.Mkdir(runDir, privateRunDirMode); err != nil {
			t.Fatal(err)
		}
		record := RunRecord{
			SchemaVersion: "1",
			RunID:         id,
			JobID:         jobID,
			Status:        status,
			ScheduledAt:   started,
			StartedAt:     started,
			Plan:          RunPlan{Branch: "jobs/" + jobID, Worktree: filepath.Join(repo, id)},
		}
		if status != "running" {
			record.FinishedAt = started.Add(time.Minute)
		}
		if err := writeRunRecord(runDir, record); err != nil {
			t.Fatal(err)
		}
	}
	older := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	write("aaaaaaaaaaaaaaaaaaaaaaaa", "docs", "succeeded", older)
	write("bbbbbbbbbbbbbbbbbbbbbbbb", "review", "running", newer)

	runs, issues, _, err := ListRuns(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || len(runs) != 2 {
		t.Fatalf("unexpected list result: runs=%#v issues=%#v", runs, issues)
	}
	if runs[0].RunID != "bbbbbbbbbbbbbbbbbbbbbbbb" || runs[0].Status != "orphaned" || runs[1].Status != "succeeded" {
		t.Fatalf("unexpected order or effective status: %#v", runs)
	}
}

func TestRunSupervisorHandlesStopAndKillRequests(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command uses POSIX shell syntax")
	}
	for _, action := range []string{"stop", "kill"} {
		t.Run(action, func(t *testing.T) {
			repo := t.TempDir()
			runTestGit(t, repo, "init")
			jobPath := filepath.Join(repo, "job.md")
			jobContent := `---
id: managed-run
agent:
  command: sh
  args: ["-c", "trap 'exit 0' TERM; while :; do sleep 1; done"]
workspace: {repo: ".", base: HEAD}
concurrency: {key: managed-run}
---
Wait until managed.
`
			if err := os.WriteFile(jobPath, []byte(jobContent), 0644); err != nil {
				t.Fatal(err)
			}
			runTestGit(t, repo, "add", "job.md")
			runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
			t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "state"))
			job, err := ParseJobFile(jobPath)
			if err != nil {
				t.Fatal(err)
			}
			scheduledAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			plan, err := BuildRunPlan(job, scheduledAt, "")
			if err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				record RunRecord
				err    error
			}
			result := make(chan outcome, 1)
			go func() {
				record, err := RunJob(job, RunOptions{ScheduledAt: scheduledAt})
				result <- outcome{record: record, err: err}
			}()
			deadline := time.Now().Add(10 * time.Second)
			for {
				control, err := readRunControl(plan.RunDir)
				if err == nil && control.CommandPID > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("agent command did not become observable: %v", err)
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := RequestRunAction(plan.RunDir, action, 10*time.Second); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-result:
				wantStatus := "cancelled"
				wantError := errRunStopped
				if action == "kill" {
					wantStatus = "killed"
					wantError = errRunKilled
				}
				if got.record.Status != wantStatus || !errors.Is(got.err, wantError) {
					t.Fatalf("unexpected %s result: status=%s err=%v", action, got.record.Status, got.err)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("RunJob did not return after %s", action)
			}
			summary, err := GetRunSummary(repo, plan.RunID)
			if err != nil || summary.Status != map[string]string{"stop": "cancelled", "kill": "killed"}[action] {
				t.Fatalf("unexpected terminal summary: %#v err=%v", summary, err)
			}
		})
	}
}
