// Package ancestry answers "why is this running?" — it builds the causal
// chain of parent processes from a target PID up to the root (PID 1), then
// identifies the supervisor, service and session that keep it alive.
package ancestry

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// Node is one link in the causal chain.
type Node struct {
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid,omitempty"`
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	User    string `json:"user,omitempty"`
	// Supervisor is set when this process is a known process manager,
	// service manager or session leader (systemd, launchd, pm2, tmux…).
	Supervisor string `json:"supervisor,omitempty"`
}

// Chain is the full causal explanation for one process.
type Chain struct {
	TargetPID  int    `json:"target_pid"`
	TargetName string `json:"target_name"`
	Nodes      []Node `json:"causality_chain"`
	// Source is the primary system responsible for the target (best effort).
	Source string `json:"source,omitempty"`
	// Service is the service/unit name (systemd unit, launchd label…).
	Service string `json:"service,omitempty"`
	// Container is set when the process runs inside a container.
	Container string `json:"container,omitempty"`
	// Session describes the controlling session when detectable.
	Session string `json:"session,omitempty"`
	// Warnings collects non-blocking observations.
	Warnings []string `json:"warnings,omitempty"`
}

// Build walks the parent chain of pid up to the root process and enriches
// every link with supervisor/source detection.
func Build(pid int) (*Chain, error) {
	procs, err := inspector.AllProcesses()
	if err != nil {
		return nil, err
	}
	byPID := map[int]*inspector.Process{}
	for _, p := range procs {
		if p != nil && p.PID > 0 {
			byPID[p.PID] = p
		}
	}

	// Collect the chain: target first, then parents until PID 1 or loop.
	seen := map[int]bool{}
	var ids []int
	for cur := pid; cur > 0 && !seen[cur]; cur = parentOf(byPID, cur) {
		seen[cur] = true
		ids = append(ids, cur)
		if cur == 1 {
			break
		}
	}

	chain := &Chain{TargetPID: pid}
	if p, ok := byPID[pid]; ok {
		chain.TargetName = p.Name
	}

	// Fetch enriched info for up to the first 24 ancestors (bounded).
	// GetProcessNoCPU avoids per-ancestor 250 ms CPU samples on Linux.
	limit := len(ids)
	if limit > 24 {
		limit = 24
	}
	for _, id := range ids[:limit] {
		node := Node{PID: id}
		if p, ok := byPID[id]; ok {
			node.PPID = p.PPID
			node.Name = p.Name
		}
		if full, err := inspector.GetProcessNoCPU(id); err == nil {
			node.Command = full.Command
			node.User = full.User
			if node.Name == "" {
				node.Name = full.Name
			}
		}
		node.Supervisor = classifySupervisor(node.Name, node.Command)
		chain.Nodes = append(chain.Nodes, node)
	}

	chain.Container = inspector.ProcessContainer(pid)
	chain.Source = detectSource(chain.Nodes)
	chain.Service = detectService(pid)
	chain.Session = detectSession(chain.Nodes)
	chain.Warnings = buildWarnings(chain)
	return chain, nil
}

func parentOf(byPID map[int]*inspector.Process, pid int) int {
	if p, ok := byPID[pid]; ok {
		return p.PPID
	}
	return 0
}

// Text renders the classic causality chain, root first:
//
//	systemd (pid 1) → pm2 (pid 5034) → node (pid 14233)
func (c *Chain) Text() string {
	parts := make([]string, 0, len(c.Nodes))
	// Nodes are stored target-first; render root-first.
	for i := len(c.Nodes) - 1; i >= 0; i-- {
		n := c.Nodes[i]
		parts = append(parts, fmt.Sprintf("%s (pid %d)", shortName(n.Name), n.PID))
	}
	return strings.Join(parts, " → ")
}

// Tree renders the chain as a tree, root at the top, target at the bottom:
//
//	launchd (pid 1)
//	└─ pm2 (pid 5034)  [pm2]
//	   └─ node (pid 14233)
func (c *Chain) Tree() string {
	var b strings.Builder
	total := len(c.Nodes)
	for i := total - 1; i >= 0; i-- {
		n := c.Nodes[i]
		depth := total - 1 - i
		prefix := ""
		if depth == 0 {
			prefix = ""
		} else {
			prefix = strings.Repeat("   ", depth-1) + "└─ "
		}
		label := shortName(n.Name)
		if label == "" {
			label = "?"
		}
		line := prefix + label + " (pid " + strconv.Itoa(n.PID) + ")"
		if n.Supervisor != "" {
			line += "  [" + n.Supervisor + "]"
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// shortName reduces an executable path to its base name for display.
func shortName(name string) string {
	if name == "" {
		return ""
	}
	base := name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		base = name[idx+1:]
	}
	// Strip common macOS wrappers for readability.
	base = strings.TrimSuffix(base, ".app")
	return base
}

// detectSource picks the nearest known supervisor in the chain; if none is
// found, it falls back to describing the immediate parent.
func detectSource(nodes []Node) string {
	for _, n := range nodes[1:] { // skip the target itself
		if n.Supervisor != "" {
			return n.Supervisor
		}
	}
	// No supervisor found — the immediate parent is the source.
	if len(nodes) >= 2 {
		parent := nodes[1]
		return fmt.Sprintf("%s (pid %d)", shortName(parent.Name), parent.PID)
	}
	return "unknown"
}

func buildWarnings(c *Chain) []string {
	var w []string
	if c.Container != "" {
		w = append(w, "running inside container: "+c.Container)
	}
	if c.Source == "" || c.Source == "unknown" {
		w = append(w, "could not determine how this process was started")
	}
	return w
}

// AutoRestart reports whether a process is managed by a supervisor that is
// likely to respawn it after it is killed: a launchd label on macOS, a
// systemd unit on Linux, or an ancestor that is itself launchd/systemd.
//
// It is deliberately conservative (best effort, never blocks) — a false
// positive just means an extra warning before a kill.
func AutoRestart(pid int) (managed bool, manager, detail string) {
	procs, err := inspector.AllProcesses()
	if err != nil {
		return false, "", ""
	}
	byPID := map[int]*inspector.Process{}
	for _, p := range procs {
		if p != nil && p.PID > 0 {
			byPID[p.PID] = p
		}
	}

	// The strongest signal: the target itself is a managed service.
	if unit := detectService(pid); unit != "" {
		// On Linux the service unit path implies systemd management; on
		// macOS a launchd label implies KeepAlive may be set.
		return true, platformManagerName(), unit
	}

	// Weaker signal: a launchd/systemd ancestor in the parent chain.
	seen := map[int]bool{}
	for cur := pid; cur > 0 && !seen[cur]; cur = parentOf(byPID, cur) {
		seen[cur] = true
		if p, ok := byPID[cur]; ok {
			if label, isSuper := supervisorTable[p.Name]; isSuper {
				switch label {
				case "launchd":
					return true, "launchd", "ancestor launchd may keep the process alive"
				case "systemd":
					return true, "systemd", "ancestor systemd may restart the unit"
				}
			}
		}
		if cur == 1 {
			break
		}
	}
	return false, "", ""
}
