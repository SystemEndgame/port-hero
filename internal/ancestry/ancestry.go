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

// Resolution describes how reliably the ancestry chain was resolved.
type Resolution string

const (
	ResolutionResolved    Resolution = "resolved"     // full chain to PID 1, no gaps
	ResolutionUnreachable Resolution = "unreachable"  // chain broken (parent exited, PID reused)
	ResolutionUnknown    Resolution = "unknown"      // target process no longer exists
	ResolutionOrphan     Resolution = "orphan"       // target alive but parent/session gone
)

// OrphanInfo describes orphan detection details.
type OrphanInfo struct {
	IsOrphan   bool   `json:"is_orphan"`
	OrphanType string `json:"orphan_type,omitempty"` // "accidental" or "deliberate"
	Detail     string `json:"detail,omitempty"`
}

// Chain is the full causal explanation for one process.
type Chain struct {
	TargetPID  int    `json:"target_pid"`
	TargetName string `json:"target_name"`
	Nodes      []Node `json:"causality_chain"`
	// Resolution indicates how reliably the chain was resolved.
	Resolution Resolution `json:"resolution"`
	// Source is the primary system responsible for the target (best effort).
	Source string `json:"source,omitempty"`
	// Service is the service/unit name (systemd unit, launchd label…).
	Service string `json:"service,omitempty"`
	// Container is set when the process runs inside a container.
	Container string `json:"container,omitempty"`
	// Session describes the controlling session when detectable.
	Session string `json:"session,omitempty"`
	// Orphan contains orphan detection details.
	Orphan *OrphanInfo `json:"orphan,omitempty"`
	// Warnings collects non-blocking observations.
	Warnings []string `json:"warnings,omitempty"`
}

// Build walks the parent chain of pid up to the root process and enriches
// every link with supervisor/source detection. It also detects PID reuse,
// orphan processes, and sets the resolution field.
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

	chain := &Chain{TargetPID: pid, Resolution: ResolutionUnknown}
	target, targetExists := byPID[pid]
	if !targetExists {
		// Target no longer exists — we cannot build a chain, but we still
		// include a node for the target PID so callers can see what was asked.
		chain.Nodes = append(chain.Nodes, Node{PID: pid})
		chain.Warnings = append(chain.Warnings, "target process no longer exists")
		return chain, nil
	}
	chain.TargetName = target.Name

	// Collect the chain: target first, then parents until PID 1 or loop.
	// Along the way, validate that each parent's start_time is before the
	// child's start_time to detect PID reuse.
	seen := map[int]bool{}
	var ids []int
	var pidReuse bool
	for cur := pid; cur > 0 && !seen[cur]; cur = parentOf(byPID, cur) {
		seen[cur] = true
		ids = append(ids, cur)

		// PID reuse detection: verify parent started before child.
		if len(ids) >= 2 {
			child := ids[len(ids)-2]
			parent := cur
			if cp, ok := byPID[child]; ok {
				if pp, ok := byPID[parent]; ok {
					if pp.StartTime > 0 && cp.StartTime > 0 && pp.StartTime >= cp.StartTime {
						pidReuse = true
						chain.Warnings = append(chain.Warnings,
							fmt.Sprintf("possible PID reuse at pid %d: parent started after child", parent))
						break
					}
				}
			}
		}

		if cur == 1 {
			break
		}
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

	// Determine resolution.
	if pidReuse {
		chain.Resolution = ResolutionUnreachable
	} else if len(ids) > 0 && ids[len(ids)-1] == 1 {
		chain.Resolution = ResolutionResolved
	} else {
		chain.Resolution = ResolutionUnreachable
	}

	chain.Container = inspector.ProcessContainer(pid)
	chain.Source = detectSource(chain.Nodes)
	chain.Service = detectService(pid)
	chain.Session = detectSession(chain.Nodes)
	chain.Orphan = detectOrphan(chain, byPID)
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

// detectOrphan checks whether the target process is an orphan (parent/session
// leader no longer exists). It distinguishes accidental orphans (started from
// a shell session that ended) from deliberate ones (nohup, setsid, tmux).
func detectOrphan(c *Chain, byPID map[int]*inspector.Process) *OrphanInfo {
	if len(c.Nodes) < 2 {
		return nil
	}

	// The target's parent is the second node (index 1).
	parent := c.Nodes[1]
	parentAlive := false
	if _, ok := byPID[parent.PID]; ok {
		parentAlive = true
	}

	if parentAlive {
		return nil
	}

	// Parent is gone — this is an orphan. Determine if accidental or deliberate.
	info := &OrphanInfo{IsOrphan: true}

	// Check for deliberate orphan indicators in the target's command/argv.
	target := c.Nodes[0]
	isDeliberate := false
	cmd := strings.ToLower(target.Command)

	if strings.Contains(cmd, "nohup") || strings.Contains(cmd, "setsid") {
		isDeliberate = true
		info.Detail = "started with " + target.Command
	}
	if strings.Contains(cmd, "tmux") || strings.Contains(cmd, "screen") {
		isDeliberate = true
		info.Detail = "running inside " + target.Name + " session"
	}

	if isDeliberate {
		info.OrphanType = "deliberate"
	} else {
		info.OrphanType = "accidental"
		info.Detail = fmt.Sprintf("parent process (pid %d) no longer exists", parent.PID)
	}

	return info
}

func buildWarnings(c *Chain) []string {
	var w []string
	if c.Container != "" {
		w = append(w, "running inside container: "+c.Container)
	}
	if c.Source == "" || c.Source == "unknown" {
		w = append(w, "could not determine how this process was started")
	}
	if c.Resolution == ResolutionUnreachable {
		w = append(w, "ancestry chain is incomplete — possible PID reuse or missing parent")
	}
	if c.Orphan != nil && c.Orphan.IsOrphan && c.Orphan.OrphanType == "accidental" {
		w = append(w, "accidental orphan: "+c.Orphan.Detail)
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

	// Dev process managers (npm, yarn, pm2…) run under "node" and are only
	// detectable from the parent's command line. A parent like `npm run dev`
	// will respawn the target after it is killed, so warn about it.
	if parent, ok := byPID[pid]; ok && parent.PPID > 0 {
		if full, err := inspector.GetProcessNoCPU(parent.PPID); err == nil {
			if m := devProcessManager(full.Name, full.Command); m != "" {
				return true, m, m + " parent may respawn the process after it is killed"
			}
		}
	}

	return respawnSupervisorAncestor(byPID, pid)
}

// respawnSupervisorAncestor walks the parent chain looking for a supervisor
// known to respawn its children after they exit.
func respawnSupervisorAncestor(byPID map[int]*inspector.Process, pid int) (managed bool, manager, detail string) {
	seen := map[int]bool{}
	for cur := pid; cur > 0 && !seen[cur]; cur = parentOf(byPID, cur) {
		seen[cur] = true
		if p, ok := byPID[cur]; ok {
			if label, isSuper := supervisorTable[p.Name]; isSuper && isRespawnManager(label) {
				return true, label, "ancestor " + label + " may restart the process"
			}
		}
		if cur == 1 {
			break
		}
	}
	return false, "", ""
}

// isRespawnManager reports whether a supervisor label is known to respawn
// its children after they exit.
func isRespawnManager(label string) bool {
	switch label {
	case "launchd", "systemd", "npm", "yarn", "pnpm", "pm2", "nodemon", "forever", "supervisor":
		return true
	}
	return false
}

// devProcessManager detects npm/yarn/pnpm/pm2/nodemon/forever from a process
// name or command line. These run under "node", so the argv is the signal.
func devProcessManager(name, command string) string {
	if label, ok := supervisorTable[name]; ok {
		switch label {
		case "npm", "yarn", "pnpm", "pm2", "nodemon", "forever", "supervisor":
			return label
		}
	}
	low := strings.ToLower(command)
	switch {
	case strings.Contains(low, "npm-cli"):
		return "npm"
	case strings.Contains(low, "yarn"):
		return "yarn"
	case strings.Contains(low, "pnpm"):
		return "pnpm"
	case strings.Contains(low, "pm2"):
		return "pm2"
	case strings.Contains(low, "nodemon"):
		return "nodemon"
	case strings.Contains(low, "forever"):
		return "forever"
	}
	return ""
}
