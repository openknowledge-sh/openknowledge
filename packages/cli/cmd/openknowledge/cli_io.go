package main

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
)

type cliWriterOverride struct {
	writer io.Writer
}

var activeStderrOverride atomic.Pointer[cliWriterOverride]
var cliRunMutex sync.Mutex

func stderrOutput() io.Writer {
	if override := activeStderrOverride.Load(); override != nil {
		return override.writer
	}
	return os.Stderr
}

func withStderrOutput(output io.Writer, run func() int) int {
	previous := activeStderrOverride.Swap(&cliWriterOverride{writer: output})
	defer activeStderrOverride.Store(previous)
	return run()
}
