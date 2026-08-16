//go:build darwin

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
	case SignalCont:
		sig = syscall.SIGCONT
	case SignalTerm:
		sig = syscall.SIGTERM
	case SignalKill:
		sig = syscall.SIGKILL
	default:
		sig = syscall.SIGTERM
	}
	return proc.Signal(sig)
}

// wake resumes a stopped process so a subsequent SIGTERM can be handled.
func wake(pid int) error {
	return send(pid, SignalCont)
}
