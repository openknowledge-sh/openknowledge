//go:build windows

package agents

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type commandCancellation struct {
	mu  sync.Mutex
	job windows.Handle
}

func configureCommandCancellation(command *exec.Cmd) *commandCancellation {
	cancellation := &commandCancellation{}
	job, err := windows.CreateJobObject(nil, nil)
	if err == nil {
		limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err = windows.SetInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&limits)),
			uint32(unsafe.Sizeof(limits)),
		); err == nil {
			cancellation.job = job
		} else {
			_ = windows.CloseHandle(job)
		}
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	command.Cancel = func() error {
		cancellation.mu.Lock()
		defer cancellation.mu.Unlock()
		if cancellation.job != 0 {
			if err := windows.TerminateJobObject(cancellation.job, 1); err == nil {
				return nil
			}
		}
		return killCommandTree(command)
	}
	command.WaitDelay = 5 * time.Second
	return cancellation
}

func (cancellation *commandCancellation) attach(command *exec.Cmd) {
	cancellation.mu.Lock()
	defer cancellation.mu.Unlock()
	if cancellation.job == 0 || command.Process == nil {
		return
	}
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(command.Process.Pid),
	)
	if err == nil {
		err = windows.AssignProcessToJobObject(cancellation.job, process)
		_ = windows.CloseHandle(process)
	}
	if err != nil {
		_ = windows.CloseHandle(cancellation.job)
		cancellation.job = 0
	}
}

func (cancellation *commandCancellation) close() {
	cancellation.mu.Lock()
	defer cancellation.mu.Unlock()
	if cancellation.job != 0 {
		_ = windows.CloseHandle(cancellation.job)
		cancellation.job = 0
	}
}

func killCommandTree(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
	if err := killer.Run(); err == nil {
		return nil
	}
	err := command.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return os.ErrProcessDone
	}
	return err
}

func forceCommandCancellation(command *exec.Cmd) error {
	if command == nil {
		return os.ErrProcessDone
	}
	return killCommandTree(command)
}
