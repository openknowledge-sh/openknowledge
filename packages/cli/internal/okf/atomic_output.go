package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteDirectoryAtomically builds a complete sibling generation and publishes
// it only after the callback succeeds. Existing output is moved aside during
// the final same-filesystem rename and restored if the new generation cannot
// take its place.
func WriteDirectoryAtomically(out string, build func(staging string) error) (string, error) {
	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(absoluteOut)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(parent, ".openknowledge-output-*")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(staging, 0755); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	defer os.RemoveAll(staging)

	if err := build(staging); err != nil {
		return "", err
	}
	if err := replaceOutputDirectory(staging, absoluteOut); err != nil {
		return "", err
	}
	return absoluteOut, nil
}

func ValidateHTMLOutputBoundary(root string, out string) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if insideRoot(absoluteOut, absoluteRoot) {
		return fmt.Errorf("HTML output directory must not contain the source bundle: %s", absoluteOut)
	}
	return nil
}

func replaceOutputDirectory(staging string, target string) error {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return os.Rename(staging, target)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("HTML output path exists and is not a directory: %s", target)
	}

	backup, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-previous-output-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		restoreErr := os.Rename(backup, target)
		return errors.Join(err, restoreErr)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("published new HTML output but could not delete previous generation %s: %w", backup, err)
	}
	return nil
}
