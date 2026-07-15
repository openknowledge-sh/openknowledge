//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package agents

import (
	"os"
	"os/exec"
	"time"
)

func configureCommandCancellation(command *exec.Cmd) {
	command.WaitDelay = 5 * time.Second
}

func forceCommandCancellation(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}
