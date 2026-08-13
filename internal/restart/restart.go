// Package restart implements "Kill & Restart": after a graceful kill, the
// original command is respawned detached (survives the TUI), from its
// original working directory, with output captured to a log file.
package restart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// Result reports a restart outcome.
type Result struct {
	NewPID int
	Log    string // path to the captured log file
	Error  error
}

// Restart respawns the command of p after it has been killed.
// dir is the working directory for the new process (p.CWD preferred).
// Returns the new PID and the log file path.
func Restart(p *inspector.Process, dir string) Result {
	if p == nil {
		return Result{Error: fmt.Errorf("no process to restart")}
	}
	argv := p.CommandTokens()
	if len(argv) == 0 {
		return Result{Error: fmt.Errorf("command line could not be reconstructed")}
	}
	if dir == "" {
		dir = p.CWD
	}
	if dir == "" {
		dir = "."
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		// Fall back to a writable location.
		dir = "."
	}

	logFile := logPath(p)
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		logFile = ""
		f = nil
	}

	cmd := buildCommand(argv, dir)
	if f != nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return Result{Log: logFile, Error: fmt.Errorf("restart failed: %w", err)}
	}

	// Capture the PID before detaching: Process.Release() clears Pid.
	newPID := cmd.Process.Pid

	// Detach so the new process survives our exit.
	detach(cmd)
	if f != nil {
		_ = f.Close()
	}
	return Result{NewPID: newPID, Log: logFile}
}

// logPath builds a log location like ~/.port-hero/restarts/port-3000-1712.log.
func logPath(p *inspector.Process) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	dir := filepath.Join(home, ".port-hero", "restarts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	base := fmt.Sprintf("port-%d-%d.log", p.Port, time.Now().Unix())
	return filepath.Join(dir, base)
}

// ShellCommand renders a human-friendly command string for a process.
func ShellCommand(p *inspector.Process) string {
	argv := p.CommandTokens()
	if len(argv) == 0 {
		return ""
	}
	return strings.Join(argv, " ")
}
