package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const (
	runControlFileName   = "control.json"
	runControlLockName   = "control.lock"
	runRequestFileName   = "control-request.json"
	runControlPollPeriod = 100 * time.Millisecond
)

var (
	ErrRunNotActive = errors.New("agent run is not active")
	ErrRunOrphaned  = errors.New("agent run has no live supervisor")
	ErrRunStopWait  = errors.New("timed out waiting for agent run to stop")
	errRunStopped   = errors.New("agent run stopped by user")
	errRunKilled    = errors.New("agent run killed by user")
)

type RunControlRecord struct {
	SchemaVersion string    `json:"schemaVersion"`
	RunID         string    `json:"run_id"`
	State         string    `json:"state"`
	Phase         string    `json:"phase,omitempty"`
	SupervisorPID int       `json:"supervisor_pid"`
	CommandPID    int       `json:"command_pid,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type runControlRequest struct {
	SchemaVersion string    `json:"schemaVersion"`
	Action        string    `json:"action"`
	RequestedAt   time.Time `json:"requested_at"`
}

type runController struct {
	mu       sync.Mutex
	record   RunControlRecord
	runDir   string
	lock     *flock.Flock
	current  *exec.Cmd
	action   string
	cancel   context.CancelCauseFunc
	done     chan struct{}
	doneOnce sync.Once
}

func newRunController(parent context.Context, plan RunPlan) (*runController, context.Context, error) {
	if parent == nil {
		parent = context.Background()
	}
	lockPath := filepath.Join(plan.RunDir, runControlLockName)
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return nil, nil, fmt.Errorf("acquire run control lock: %w", err)
	}
	if err := os.Chmod(lockPath, privateArtifactMode); err != nil {
		_ = lock.Unlock()
		return nil, nil, fmt.Errorf("secure run control lock: %w", err)
	}
	ctx, cancel := context.WithCancelCause(parent)
	now := time.Now()
	controller := &runController{
		record: RunControlRecord{
			SchemaVersion: "1",
			RunID:         plan.RunID,
			State:         "running",
			SupervisorPID: os.Getpid(),
			StartedAt:     now,
			UpdatedAt:     now,
		},
		runDir: plan.RunDir,
		lock:   lock,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	if err := controller.writeRecord(); err != nil {
		_ = lock.Unlock()
		return nil, nil, err
	}
	go controller.watchRequests()
	return controller, ctx, nil
}

func (controller *runController) watchRequests() {
	ticker := time.NewTicker(runControlPollPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-controller.done:
			return
		case <-ticker.C:
			requestPath := filepath.Join(controller.runDir, runRequestFileName)
			content, err := os.ReadFile(requestPath)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				continue
			}
			var request runControlRequest
			if err := json.Unmarshal(content, &request); err != nil || request.SchemaVersion != "1" {
				_ = os.Remove(requestPath)
				continue
			}
			_ = os.Remove(requestPath)
			if request.Action == "stop" || request.Action == "kill" {
				controller.request(request.Action)
			}
		}
	}
}

func (controller *runController) request(action string) {
	controller.mu.Lock()
	if controller.action == "kill" || controller.action == action {
		controller.mu.Unlock()
		return
	}
	// A force request may escalate an earlier graceful stop request.
	controller.action = action
	if action == "kill" {
		controller.record.State = "killing"
	} else {
		controller.record.State = "stopping"
	}
	controller.record.UpdatedAt = time.Now()
	current := controller.current
	controller.mu.Unlock()
	_ = controller.writeRecord()
	if action == "kill" && current != nil {
		_ = forceCommandCancellation(current)
	}
	if action == "kill" {
		controller.cancel(errRunKilled)
	} else {
		controller.cancel(errRunStopped)
	}
}

func (controller *runController) setCommand(command *exec.Cmd, phase string) error {
	controller.mu.Lock()
	controller.current = command
	controller.record.Phase = phase
	controller.record.CommandPID = 0
	if command != nil && command.Process != nil {
		controller.record.CommandPID = command.Process.Pid
	}
	controller.record.UpdatedAt = time.Now()
	controller.mu.Unlock()
	return controller.writeRecord()
}

func (controller *runController) clearCommand() error {
	controller.mu.Lock()
	controller.current = nil
	controller.record.CommandPID = 0
	controller.record.UpdatedAt = time.Now()
	controller.mu.Unlock()
	return controller.writeRecord()
}

func (controller *runController) Action() string {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.action
}

func (controller *runController) Close(finalState string) error {
	controller.doneOnce.Do(func() { close(controller.done) })
	controller.mu.Lock()
	controller.record.State = finalState
	controller.record.Phase = ""
	controller.record.CommandPID = 0
	controller.record.UpdatedAt = time.Now()
	controller.current = nil
	controller.mu.Unlock()
	writeErr := controller.writeRecord()
	unlockErr := controller.lock.Unlock()
	if writeErr != nil {
		return writeErr
	}
	return unlockErr
}

func (controller *runController) writeRecord() error {
	controller.mu.Lock()
	record := controller.record
	controller.mu.Unlock()
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateArtifactAtomic(filepath.Join(controller.runDir, runControlFileName), append(content, '\n'))
}

func RequestRunAction(runDir string, action string, wait time.Duration) error {
	if action != "stop" && action != "kill" {
		return fmt.Errorf("unsupported run action %q", action)
	}
	active, err := runControlLockHeld(runDir)
	if err != nil {
		return err
	}
	if !active {
		return ErrRunNotActive
	}
	request := runControlRequest{SchemaVersion: "1", Action: action, RequestedAt: time.Now()}
	content, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return err
	}
	if err := writePrivateArtifactAtomic(filepath.Join(runDir, runRequestFileName), append(content, '\n')); err != nil {
		return err
	}
	if wait <= 0 {
		return nil
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		time.Sleep(runControlPollPeriod)
		active, err := runControlLockHeld(runDir)
		if err != nil {
			return err
		}
		if !active {
			return nil
		}
	}
	return ErrRunStopWait
}

func runControlLockHeld(runDir string) (bool, error) {
	lockPath := filepath.Join(runDir, runControlLockName)
	if _, err := os.Stat(lockPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	lock := flock.New(lockPath)
	acquired, err := lock.TryLock()
	if err != nil {
		return false, err
	}
	if !acquired {
		return true, nil
	}
	if err := lock.Unlock(); err != nil {
		return false, err
	}
	return false, nil
}

func readRunControl(runDir string) (RunControlRecord, error) {
	content, err := os.ReadFile(filepath.Join(runDir, runControlFileName))
	if err != nil {
		return RunControlRecord{}, err
	}
	var record RunControlRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return RunControlRecord{}, err
	}
	return record, nil
}
