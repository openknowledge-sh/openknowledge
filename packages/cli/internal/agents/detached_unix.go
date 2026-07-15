//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package agents

import (
	"os"
	"os/exec"
	"syscall"
)

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
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}
