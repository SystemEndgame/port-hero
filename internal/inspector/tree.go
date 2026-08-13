package inspector

// Snapshot is an in-memory index of all processes, used to compute the
// process tree so we can kill whole trees (orphan prevention).
type Snapshot struct {
	byPID map[int]*Process
}

// NewSnapshot builds an index from a process listing.
func NewSnapshot(procs []*Process) *Snapshot {
	s := &Snapshot{byPID: make(map[int]*Process, len(procs))}
	for _, p := range procs {
		if p != nil && p.PID > 0 {
			s.byPID[p.PID] = p
		}
	}
	return s
}

// Lookup returns the process entry for a PID, if present.
func (s *Snapshot) Lookup(pid int) (*Process, bool) {
	p, ok := s.byPID[pid]
	return p, ok
}

// Children returns the direct child processes of pid.
func (s *Snapshot) Children(pid int) []*Process {
	var out []*Process
	for _, p := range s.byPID {
		if p.PPID == pid {
			out = append(out, p)
		}
	}
	return out
}

// Descendants returns every process in the subtree rooted at pid
// (children, grandchildren, ...), in breadth-first order.
// The root itself is NOT included.
func (s *Snapshot) Descendants(pid int) []*Process {
	out := []*Process{}
	queue := s.Children(pid)
	seen := map[int]bool{pid: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur.PID] {
			continue
		}
		seen[cur.PID] = true
		out = append(out, cur)
		queue = append(queue, s.Children(cur.PID)...)
	}
	return out
}

// TreeOrder returns the kill order for the whole tree: deepest descendants
// first, root last (post-order). This guarantees children are stopped before
// their parent, avoiding orphans.
func (s *Snapshot) TreeOrder(root int) []int {
	var order []int
	visited := map[int]bool{}
	var visit func(pid int)
	visit = func(pid int) {
		if visited[pid] {
			return
		}
		visited[pid] = true
		for _, c := range s.Children(pid) {
			visit(c.PID)
		}
		order = append(order, pid)
	}
	visit(root)
	return order
}
