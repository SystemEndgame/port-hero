//go:build windows

package inspector

import (
	"os"
	"syscall"
)

// osFindProcess on Windows.
func osFindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// probeSignal: signal 0 is unsupported on Windows; use a no-op marker.
func probeSignal() syscall.Signal {
	return syscall.Signal(0xdead)
}
