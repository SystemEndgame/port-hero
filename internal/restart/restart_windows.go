//go:build windows

package restart

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// buildCommand prepares the detached respawn on Windows.
func buildCommand(argv []string, dir string) *exec.Cmd {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	return cmd
}

// shellCommand renders a config start string for execution via cmd.exe.
func shellCommand(cmd string) []string {
	return []string{"cmd", "/C", cmd}
}

// detach releases the handle so the child outlives the parent.
func detach(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}
