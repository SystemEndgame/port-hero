//go:build windows

package killer

import (
	"os"
	"syscall"
)

// send on Windows: TerminateProcess. SIGTERM is approximated with the same
// termination because Windows has no graceful POSIX signal; the tree-first
// ordering and guardrails still apply.
func send(pid int, s Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM) // maps to TerminateProcess in os
}
