//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package agents

import (
	"os"
	"os/exec"
)

// StartDetachedProcess starts a best-effort detached supervisor on platforms
// without the Unix session or Windows process-group primitives used elsewhere.
func StartDetachedProcess(executable string, args []string, environment []string) (int, error) {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer null.Close()
	command := exec.Command(executable, args...)
	command.Env = environment
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}
