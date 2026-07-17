package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

func acquireConcurrency(plan RunPlan) (func() error, bool, error) {
	if plan.Concurrency.Key == "" {
		return func() error { return nil }, true, nil
	}
	if plan.Concurrency.Policy != "skip" {
		return nil, false, fmt.Errorf("unsupported concurrency policy %q", plan.Concurrency.Policy)
	}
	stateRoot, err := JobsStateDirectory()
	if err != nil {
		return nil, false, err
	}
	lockRoot := filepath.Join(stateRoot, "concurrency")
	if err := os.MkdirAll(lockRoot, privateRunDirMode); err != nil {
		return nil, false, fmt.Errorf("create agent concurrency directory: %w", err)
	}
	if err := os.Chmod(lockRoot, privateRunDirMode); err != nil {
		return nil, false, fmt.Errorf("secure agent concurrency directory: %w", err)
	}
	digest := sha256.Sum256([]byte(plan.Concurrency.Key))
	lockPath := filepath.Join(lockRoot, hex.EncodeToString(digest[:])+".lock")
	lock := flock.New(lockPath)
	acquired, err := lock.TryLock()
	if err != nil {
		return nil, false, fmt.Errorf("acquire concurrency key %q: %w", plan.Concurrency.Key, err)
	}
	if !acquired {
		return nil, false, nil
	}
	if err := os.Chmod(lockPath, privateArtifactMode); err != nil {
		_ = lock.Unlock()
		return nil, false, fmt.Errorf("secure concurrency key %q: %w", plan.Concurrency.Key, err)
	}
	return lock.Unlock, true, nil
}
