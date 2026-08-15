package inspector

import (
	"errors"
	"strings"
)

// ErrPortFree is returned when nothing is listening on the requested port.
var ErrPortFree = errors.New("no process is listening on this port")

// FindByPort returns the process(es) listening on the given TCP port.
// On the rare case that multiple processes bind the same port (SO_REUSEPORT),
// all of them are returned. Processes are enriched with cwd/git/container.
func FindByPort(port int) ([]*Process, error) {
	if port < 1 || port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}
	procs, err := platformFindByPort(port)
	if err != nil {
		return nil, err
	}
	for _, p := range procs {
		Enrich(p)
	}
	SortByPort(procs)
	return procs, nil
}

// FindAll returns every TCP process currently in LISTEN state, enriched.
// Ports can be restricted with maxPorts (0 = unlimited).
func FindAll() ([]*Process, error) {
	procs, err := platformFindAll()
	if err != nil {
		return nil, err
	}
	for _, p := range procs {
		Enrich(p)
	}
	SortByPort(procs)
	return procs, nil
}

// GetProcess returns full enriched information about a single PID,
// including a live CPU sample.
func GetProcess(pid int) (*Process, error) {
	p, err := platformGetProcess(pid)
	if err != nil {
		return nil, err
	}
	Enrich(p)
	return p, nil
}

// GetProcessNoCPU returns full enriched information about a single PID
// without blocking on a CPU sample. Use it in bulk/loop paths (causality
// chains, name search) where per-PID CPU sampling would serialize 250 ms
// sleeps.
func GetProcessNoCPU(pid int) (*Process, error) {
	p, err := platformGetProcessNoCPU(pid)
	if err != nil {
		return nil, err
	}
	Enrich(p)
	return p, nil
}

// VerifyProcess returns the raw platform identity of a PID (name, user,
// start time) without enrichment or CPU sampling. It is used by the killer
// to re-confirm a process is still the same one before signalling, closing
// the PID-reuse / owner-change race window.
func VerifyProcess(pid int) (*Process, error) {
	return platformGetProcessNoCPU(pid)
}

// FindByName returns processes whose name (comm) contains the given
// substring, enriched, limited to maxMatches (0 = unlimited). Matching is
// case-insensitive and against the short process name. CPU usage is sampled
// once for the whole result set via platformBatchCPU.
func FindByName(name string, maxMatches int) ([]*Process, error) {
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	all, err := platformAllProcesses()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(name)
	var out []*Process
	for _, p := range all {
		if !strings.Contains(strings.ToLower(p.Name), needle) {
			continue
		}
		full, err := GetProcessNoCPU(p.PID)
		if err != nil {
			continue
		}
		out = append(out, full)
		if maxMatches > 0 && len(out) >= maxMatches {
			break
		}
	}
	platformBatchCPU(out)
	SortByPort(out)
	return out, nil
}

// AllProcesses returns a lightweight snapshot of every process on the system
// (PID/PPID/name only). Used to build the process tree for orphan prevention.
func AllProcesses() ([]*Process, error) {
	return platformAllProcesses()
}

// IsAlive reports whether a process with the given PID currently exists.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return platformIsAlive(pid)
}

// ProcessContainer reports the container label for a PID, if any
// (e.g. "redis-dev (Docker)").
func ProcessContainer(pid int) string {
	if pid <= 0 {
		return ""
	}
	return containerInfo(pid)
}

// Enrich adds shared context (project, cwd, git branch, container) to a
// process.
func Enrich(p *Process) {
	if p == nil || p.PID <= 0 {
		return
	}
	if p.CWD == "" {
		p.CWD = platformCWD(p.PID)
	}
	if p.Project == "" && p.CWD != "" {
		p.Project = ProjectName(p.CWD)
	}
	if p.GitBranch == "" && p.CWD != "" {
		branch, dirty := gitInfo(p.CWD)
		p.GitBranch, p.GitDirty = branch, dirty
	}
	if p.Container == "" {
		p.Container = containerInfo(p.PID)
	}
}
