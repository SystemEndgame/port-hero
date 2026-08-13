//go:build !windows

package inspector

import (
	"os"
	"syscall"
)

// osFindProcess is a thin wrapper so non-unix targets can share code.
func osFindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// probeSignal is signal 0 — used purely to test process existence.
func probeSignal() syscall.Signal {
	return syscall.Signal(0)
}
