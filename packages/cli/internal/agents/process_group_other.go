//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package agents

import (
	"os"
	"os/exec"
	"time"
)

type commandCancellation struct{}

func configureCommandCancellation(command *exec.Cmd) commandCancellation {
	command.WaitDelay = 5 * time.Second
	return commandCancellation{}
}

func (commandCancellation) attach(*exec.Cmd) {}

func (commandCancellation) close() {
}

func forceCommandCancellation(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}
