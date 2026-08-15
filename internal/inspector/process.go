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
	PID        int        `json:"pid"`                   // process identifier
	PPID       int        `json:"ppid"`                  // parent process identifier
	Name       string     `json:"name"`                  // process name (comm on Linux, kp_comm on macOS)
	Command    string     `json:"command,omitempty"`     // full command line (argv joined by spaces)
	Argv       []string   `json:"argv,omitempty"`        // exact argv (Linux); nil when not recoverable
	User       string     `json:"user,omitempty"`        // owning user name
	Port       int        `json:"port,omitempty"`        // listening port
	Protocol   string     `json:"protocol,omitempty"`    // "tcp" or "udp"
	LocalAddr  string     `json:"local_addr,omitempty"`  // e.g. "127.0.0.1" or "::"
	MemoryMB   float64    `json:"memory_mb,omitempty"`   // resident set size in MB
	CPUPercent float64    `json:"cpu_percent,omitempty"` // CPU usage percentage (approximate)
	CWD        string     `json:"cwd,omitempty"`         // working directory
	Project    string     `json:"project,omitempty"`     // project name (git repo root, or cwd basename)
	GitBranch  string     `json:"git_branch,omitempty"`  // current git branch, if cwd is inside a repo
	GitDirty   bool       `json:"git_dirty,omitempty"`   // working tree has uncommitted changes
	Container  string     `json:"container,omitempty"`   // container name if the process runs inside one
	Children   []*Process `json:"children,omitempty"`    // direct child processes
	// StartTime is the process start time used to detect PID reuse. Units are
	// platform-specific: jiffies since boot on Linux (proc(5) field 22), and
	// zero on platforms where it cannot be cheaply recovered.
	StartTime uint64 `json:"start_time,omitempty"`
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
