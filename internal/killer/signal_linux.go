//go:build linux

package killer

import (
	"errors"

	"golang.org/x/sys/unix"
)

// send delivers a signal to a PID through a pidfd (Linux ≥ 5.3). Signalling
// via the file descriptor is atomic with respect to PID reuse: if the
// original process has exited, the kernel returns ESRCH instead of letting
// the signal reach whatever process now owns the PID.
func send(pid int, s Signal) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(fd) }()

	var sig unix.Signal
	switch s {
	case SignalKill:
		sig = unix.SIGKILL
	case SignalCont:
		sig = unix.SIGCONT
	default:
		sig = unix.SIGTERM
	}
	return unix.PidfdSendSignal(fd, sig, nil, 0)
}

// wake resumes a stopped process so a subsequent SIGTERM can be handled.
// Running processes are unaffected (SIGCONT is a no-op for them).
func wake(pid int) error {
	return send(pid, SignalCont)
}
