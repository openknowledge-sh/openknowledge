//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package agents

import (
	"os/exec"
	"time"
)

func configureCommandCancellation(command *exec.Cmd) {
	command.WaitDelay = 5 * time.Second
}
