package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RunIssue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type RunSummary struct {
	RunID          string     `json:"run_id"`
	JobID          string     `json:"job_id"`
	Status         string     `json:"status"`
	RecordedStatus string     `json:"recorded_status"`
	ScheduledAt    time.Time  `json:"scheduled_at"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	Branch         string     `json:"branch"`
	Worktree       string     `json:"worktree"`
	RunRecord      string     `json:"run_record"`
	Phase          string     `json:"phase,omitempty"`
}

type JobStatus struct {
	ID             string       `json:"id"`
	Enabled        bool         `json:"enabled"`
	Path           string       `json:"path"`
	RepoRoot       string       `json:"repo_root"`
	Schedule       ScheduleSpec `json:"schedule"`
	NextEligibleAt *time.Time   `json:"next_eligible_at"`
	LastRun        *RunSummary  `json:"last_run"`
	ActiveRuns     []RunSummary `json:"active_runs"`
}

func ResolveJobRepoRoot(job Job) (string, error) {
	repo := job.Workspace.Repo
	if repo == "" {
		repo = "."
	}
	if !filepath.IsAbs(repo) && job.Path != "" {
		repo = filepath.Join(filepath.Dir(job.Path), repo)
	}
	absolute, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	root, err := gitOutput(absolute, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve git repository: %w", err)
	}
	return canonicalPath(root)
}

func ResolveRepoRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := gitOutput(absolute, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve git repository: %w", err)
	}
	return canonicalPath(root)
}

func RepositoryRunDirectory(repoRoot string) (string, error) {
	root, err := ResolveRepoRoot(repoRoot)
	if err != nil {
		return "", err
	}
	stateRoot, err := AgentStateDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateRoot, repositoryStateName(root), "runs"), nil
}

func RunDirectory(repoRoot string, runID string) (string, error) {
	if !validRunID.MatchString(runID) {
		return "", fmt.Errorf("run id must contain exactly 24 lowercase hexadecimal characters")
	}
	runsDir, err := RepositoryRunDirectory(repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(runsDir, runID), nil
}

func ListRuns(repoRoot string) ([]RunSummary, []RunIssue, string, error) {
	resolvedRoot, err := ResolveRepoRoot(repoRoot)
	if err != nil {
		return nil, nil, "", err
	}
	runsDir, err := RepositoryRunDirectory(resolvedRoot)
	if err != nil {
		return nil, nil, "", err
	}
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, os.ErrNotExist) {
		return []RunSummary{}, []RunIssue{}, resolvedRoot, nil
	}
	if err != nil {
		return nil, nil, "", err
	}
	runs := make([]RunSummary, 0, len(entries))
	issues := make([]RunIssue, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !validRunID.MatchString(entry.Name()) {
			continue
		}
		runDir := filepath.Join(runsDir, entry.Name())
		summary, err := summarizeRun(runDir)
		if err != nil {
			issues = append(issues, RunIssue{Path: filepath.Join(runDir, "run.json"), Error: err.Error()})
			continue
		}
		runs = append(runs, summary)
	}
	sort.Slice(runs, func(first int, second int) bool {
		if !runs[first].StartedAt.Equal(runs[second].StartedAt) {
			return runs[first].StartedAt.After(runs[second].StartedAt)
		}
		return runs[first].RunID < runs[second].RunID
	})
	sort.Slice(issues, func(first int, second int) bool { return issues[first].Path < issues[second].Path })
	return runs, issues, resolvedRoot, nil
}

func GetRunSummary(repoRoot string, runID string) (RunSummary, error) {
	runDir, err := RunDirectory(repoRoot, runID)
	if err != nil {
		return RunSummary{}, err
	}
	return summarizeRun(runDir)
}

func summarizeRun(runDir string) (RunSummary, error) {
	recordPath := filepath.Join(runDir, "run.json")
	content, err := os.ReadFile(recordPath)
	if err != nil {
		return RunSummary{}, err
	}
	var record RunRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return RunSummary{}, err
	}
	if record.RunID == "" || record.JobID == "" {
		return RunSummary{}, fmt.Errorf("run record is missing run_id or job_id")
	}
	status := record.Status
	phase := ""
	active, err := runControlLockHeld(runDir)
	if err != nil {
		return RunSummary{}, err
	}
	if active {
		status = "running"
		if control, err := readRunControl(runDir); err == nil {
			phase = control.Phase
			if control.State == "stopping" || control.State == "killing" {
				status = control.State
			}
		}
	} else if record.Status == "running" {
		status = "orphaned"
	}
	var finishedAt *time.Time
	if !record.FinishedAt.IsZero() {
		value := record.FinishedAt
		finishedAt = &value
	}
	return RunSummary{
		RunID:          record.RunID,
		JobID:          record.JobID,
		Status:         status,
		RecordedStatus: record.Status,
		ScheduledAt:    record.ScheduledAt,
		StartedAt:      record.StartedAt,
		FinishedAt:     finishedAt,
		Branch:         record.Plan.Branch,
		Worktree:       record.Plan.Worktree,
		RunRecord:      recordPath,
		Phase:          phase,
	}, nil
}

func BuildJobStatuses(jobs []Job, now time.Time) ([]JobStatus, []RunIssue) {
	statuses := make([]JobStatus, 0, len(jobs))
	issues := make([]RunIssue, 0)
	type cachedRuns struct {
		runs   []RunSummary
		issues []RunIssue
		err    error
	}
	runsByRepository := make(map[string]cachedRuns)
	for _, job := range jobs {
		status := JobStatus{
			ID:         job.ID,
			Enabled:    job.Enabled,
			Path:       job.Path,
			Schedule:   job.Schedule,
			ActiveRuns: []RunSummary{},
		}
		next, scheduled, err := NextScheduledAt(job, now)
		if err != nil {
			issues = append(issues, RunIssue{Path: job.Path, Error: err.Error()})
		} else if scheduled {
			status.NextEligibleAt = &next
		}
		repoRoot, err := ResolveJobRepoRoot(job)
		if err != nil {
			issues = append(issues, RunIssue{Path: job.Path, Error: err.Error()})
			statuses = append(statuses, status)
			continue
		}
		status.RepoRoot = repoRoot
		cached, present := runsByRepository[repoRoot]
		if !present {
			cached.runs, cached.issues, _, cached.err = ListRuns(repoRoot)
			runsByRepository[repoRoot] = cached
			issues = append(issues, cached.issues...)
		}
		if cached.err != nil {
			issues = append(issues, RunIssue{Path: job.Path, Error: cached.err.Error()})
			statuses = append(statuses, status)
			continue
		}
		for _, run := range cached.runs {
			if run.JobID != job.ID {
				continue
			}
			if status.LastRun == nil {
				copy := run
				status.LastRun = &copy
			}
			if run.Status == "running" || run.Status == "stopping" || run.Status == "killing" {
				status.ActiveRuns = append(status.ActiveRuns, run)
			}
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(first int, second int) bool {
		if statuses[first].ID != statuses[second].ID {
			return statuses[first].ID < statuses[second].ID
		}
		return statuses[first].Path < statuses[second].Path
	})
	return statuses, issues
}

func IsTerminalRunStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "verification_failed", "skipped", "cancelled", "killed":
		return true
	default:
		return false
	}
}
