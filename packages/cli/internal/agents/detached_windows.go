//go:build windows

package agents

import (
	"os"
	"os/exec"
	"syscall"
)

const createDetachedProcess = 0x00000008

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
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createDetachedProcess,
	}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return 0, err
	}
	return pid, nil
}
