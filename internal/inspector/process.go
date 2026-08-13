// Package inspector resolves ports to processes and enriches them with
// context: working directory, git branch, container and process tree.
package inspector

import (
	"sort"
	"strings"
	"time"
)

// Process is the enriched representation of a system process holding a port.
type Process struct {
	PID        int        // process identifier
	PPID       int        // parent process identifier
	Name       string     // process name (comm on Linux, kp_comm on macOS)
	Command    string     // full command line (argv joined by spaces)
	Argv       []string   // exact argv (Linux); nil when not recoverable
	User       string     // owning user name
	Port       int        // listening port
	Protocol   string     // "tcp" or "udp"
	LocalAddr  string     // e.g. "127.0.0.1" or "::"
	MemoryMB   float64    // resident set size in MB
	CPUPercent float64    // CPU usage percentage (approximate)
	CWD        string     // working directory
	Project    string     // project name (git repo root, or cwd basename)
	GitBranch  string     // current git branch, if cwd is inside a repo
	GitDirty   bool       // working tree has uncommitted changes
	Container  string     // container name if the process runs inside one
	Children   []*Process // direct child processes
}

// IsRunning checks whether the process still exists.
func (p *Process) IsRunning() bool {
	return isAlive(p.PID)
}

// CommandTokens returns the exact argv when available (Linux), otherwise a
// best-effort whitespace reconstruction.
func (p *Process) CommandTokens() []string {
	if len(p.Argv) > 0 {
		return p.Argv
	}
	if p.Command == "" {
		return nil
	}
	if toks := parseCommandTokens(p.Command); len(toks) > 0 {
		return toks
	}
	return strings.Fields(p.Command)
}

// SortByPort orders a slice of processes by port.
func SortByPort(ps []*Process) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Port == ps[j].Port {
			return ps[i].PID < ps[j].PID
		}
		return ps[i].Port < ps[j].Port
	})
}

// FormatDuration is a small helper for consistent durations in the UI.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}
