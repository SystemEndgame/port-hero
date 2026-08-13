//go:build !windows

package killer

import (
	"os"
	"syscall"
)

// send delivers a signal to a PID.
func send(pid int, s Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	var sig syscall.Signal
	switch s {
	case SignalTerm:
		sig = syscall.SIGTERM
	case SignalKill:
		sig = syscall.SIGKILL
	default:
		sig = syscall.SIGTERM
	}
	return proc.Signal(sig)
}
