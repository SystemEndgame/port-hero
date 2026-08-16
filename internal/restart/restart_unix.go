//go:build !windows

package restart

import (
	"os/exec"
	"syscall"
)

// buildCommand prepares the detached respawn on Unix.
func buildCommand(argv []string, dir string) *exec.Cmd {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	// New session: the respawned process is not killed when the TUI exits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

// shellCommand renders a config start string for execution via the shell.
func shellCommand(cmd string) []string {
	return []string{"/bin/sh", "-c", cmd}
}

// detach: with Setsid already applied, we only need to Release so the child
// is not reaped (and killed) with the parent.
func detach(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}
