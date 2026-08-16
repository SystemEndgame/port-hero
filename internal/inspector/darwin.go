//go:build darwin

package inspector

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------------------
// Port → PIDs via lsof (preinstalled on macOS).
// ---------------------------------------------------------------------------

// platformFindByPort returns processes listening on a TCP port.
func platformFindByPort(port int) ([]*Process, error) {
	pids, err := darwinLsofPIDs(port)
	if err != nil {
		return nil, err
	}
	if len(pids) == 0 {
		return nil, ErrPortFree
	}
	procs, err := darwinProcessesForPIDs(pids)
	if err != nil {
		return nil, err
	}
	for _, p := range procs {
		p.Port = port
		p.Protocol = "tcp"
	}
	return procs, nil
}

// darwinProcessesForPIDs builds full Process structs for a set of PIDs
// using batched ps + lsof calls.
func darwinProcessesForPIDs(pids []int) ([]*Process, error) {
	stats, err := darwinPsStats(pids)
	if err != nil {
		return nil, err
	}
	cwds := darwinCwds(pids)
	procs := make([]*Process, 0, len(pids))
	for _, pid := range pids {
		s, ok := stats[pid]
		if !ok {
			continue
		}
		procs = append(procs, &Process{
			PID:        pid,
			PPID:       s.ppid,
			Name:       s.name,
			Command:    s.command,
			User:       s.user,
			MemoryMB:   s.rssMB,
			CPUPercent: s.cpu,
			CWD:        cwds[pid],
		})
	}
	return procs, nil
}

// platformFindAll returns all TCP LISTEN processes in one batched sweep.
func platformFindAll() ([]*Process, error) {
	entries, err := darwinLsofAllListeners()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// One ps call for every unique PID.
	pidSet := map[int]bool{}
	for _, e := range entries {
		pidSet[e.pid] = true
	}
	pids := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}
	stats, err := darwinPsStats(pids)
	if err != nil {
		return nil, err
	}

	// One lsof call for every unique cwd.
	cwds := darwinCwds(pids)

	procs := make([]*Process, 0, len(entries))
	for _, e := range entries {
		s, ok := stats[e.pid]
		if !ok {
			continue
		}
		p := &Process{
			PID:        e.pid,
			PPID:       s.ppid,
			Name:       s.name,
			Command:    s.command,
			User:       s.user, // ps gives the login name; lsof only the UID
			Port:       e.port,
			Protocol:   "tcp",
			LocalAddr:  e.localAddr,
			MemoryMB:   s.rssMB,
			CPUPercent: s.cpu,
			CWD:        cwds[e.pid],
		}
		procs = append(procs, p)
	}
	return procs, nil
}

// platformGetProcess returns enriched info about a single PID.
func platformGetProcess(pid int) (*Process, error) {
	return platformGetProcessNoCPU(pid)
}

// platformGetProcessNoCPU on darwin is the same as platformGetProcess:
// CPU usage comes from a single batched ps call, so there is no per-process
// blocking sample to skip.
func platformGetProcessNoCPU(pid int) (*Process, error) {
	if pid <= 0 {
		return nil, errors.New("invalid pid")
	}
	stats, err := darwinPsStats([]int{pid})
	if err != nil {
		return nil, err
	}
	s, ok := stats[pid]
	if !ok {
		return nil, fmt.Errorf("process %d not found", pid)
	}
	p := &Process{
		PID:        pid,
		PPID:       s.ppid,
		Name:       s.name,
		Command:    s.command,
		User:       s.user,
		MemoryMB:   s.rssMB,
		CPUPercent: s.cpu,
		CWD:        darwinCwds([]int{pid})[pid],
	}
	return p, nil
}

// platformBatchCPU is a no-op on darwin: CPU percentages are already
// populated by the batched ps invocation used in darwinPsStats.
func platformBatchCPU(_ []*Process) {}

// platformAllProcesses returns the lightweight PID/PPID/name snapshot.
func platformAllProcesses() ([]*Process, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}
	var procs []*Process
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		procs = append(procs, &Process{
			PID:  pid,
			PPID: ppid,
			Name: strings.Join(fields[2:], " "),
		})
	}
	return procs, nil
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

type lsofEntry struct {
	pid       int
	port      int
	protocol  string
	localAddr string
	user      string
	command   string
}

// darwinLsofPIDs returns PIDs listening on port.
func darwinLsofPIDs(port int) ([]int, error) {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		// lsof returns exit 1 when nothing matches.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof failed: %w", err)
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// darwinLsofAllListeners parses `lsof -F pcnu` records for all TCP listeners.
func darwinLsofAllListeners() ([]lsofEntry, error) {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcnu").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof failed: %w", err)
	}

	var entries []lsofEntry
	var cur *lsofEntry
	flush := func() {
		if cur != nil && cur.pid > 0 {
			entries = append(entries, *cur)
		}
		cur = nil
	}

	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 2 {
			continue
		}
		key, val := line[0], line[1:]
		switch key {
		case 'p':
			flush() // new record
			cur = &lsofEntry{}
			cur.pid, _ = strconv.Atoi(val)
		case 'c':
			if cur != nil {
				cur.command = val
			}
		case 'u':
			if cur != nil {
				cur.user = val
			}
		case 'n':
			applyNameField(cur, val)
		}
	}
	flush()
	return entries, nil
}

// applyNameField parses an lsof "n" (name) field into the current entry.
func applyNameField(cur *lsofEntry, val string) {
	if cur == nil {
		return
	}
	addr, proto := parseLsofName(val)
	cur.localAddr = addr
	cur.protocol = proto
	if _, portStr, err := net.SplitHostPort(addr); err == nil {
		if p, perr := strconv.Atoi(portStr); perr == nil {
			cur.port = p
		}
	}
}

// parseLsofName extracts "host:port" and protocol from an lsof name field.
// Examples: "localhost:3000", "127.0.0.1:8080", "[::1]:3000", "*:3000",
// "*:3000 (LISTEN)".
func parseLsofName(name string) (addr, proto string) {
	proto = "tcp"
	name = strings.TrimSpace(name)
	// Drop "(LISTEN)" suffix.
	if idx := strings.Index(name, " ("); idx > 0 {
		name = name[:idx]
	}
	// IPv6 bracket form.
	if strings.HasPrefix(name, "[") {
		if end := strings.Index(name, "]"); end > 0 {
			addr = name[:end+1] + name[end+1:]
			return addr, proto
		}
	}
	return name, proto
}

// psStat holds parsed `ps -o` fields.
type psStat struct {
	pid     int
	ppid    int
	user    string
	name    string
	command string
	rssMB   float64
	cpu     float64
}

// darwinPsStats batch-reads process stats with one ps invocation.
func darwinPsStats(pids []int) (map[int]psStat, error) {
	stats := map[int]psStat{}
	if len(pids) == 0 {
		return stats, nil
	}
	args := []string{"-p", intsToCSV(pids),
		"-o", "pid=,ppid=,user=,rss=,%cpu=,comm=,command=", "-ww"}
	out, err := exec.Command("ps", args...).Output()
	if err != nil {
		// If every process disappeared, ps exits non-zero.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return stats, nil
		}
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Fields: pid ppid user rss %cpu comm [command...]
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		rss, err3 := strconv.ParseFloat(fields[3], 64)
		cpu, err4 := strconv.ParseFloat(fields[4], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		command := strings.Join(fields[6:], " ")
		stats[pid] = psStat{
			pid:   pid,
			ppid:  ppid,
			user:  fields[2],
			rssMB: rss / 1024.0, // ps reports KB on macOS
			cpu:   cpu,
			// macOS truncates the `comm` column to 16 chars in batch output,
			// so derive the process name from the command's first token.
			name:    darwinCommName(command, fields[5]),
			command: command,
		}
	}
	return stats, nil
}

// darwinCommName returns the short process name from the full command line,
// falling back to the (possibly truncated) comm column. The basename of the
// executable keeps names like "launchd", "WindowServer" and "node" intact.
func darwinCommName(command, comm string) string {
	if command != "" {
		if tok := strings.Fields(command); len(tok) > 0 && tok[0] != "" {
			base := filepath.Base(tok[0])
			if base != "" && base != "." && base != "/" {
				return base
			}
		}
	}
	return comm
}

// darwinCwds batch-reads working directories with one lsof invocation.
func darwinCwds(pids []int) map[int]string {
	cwds := map[int]string{}
	if len(pids) == 0 {
		return cwds
	}
	args := []string{"-nP", "-a", "-p", intsToCSV(pids), "-d", "cwd", "-F", "pn"}
	out, err := exec.Command("lsof", args...).Output()
	if err != nil {
		return cwds
	}
	cur := 0
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			cur, _ = strconv.Atoi(line[1:])
		case 'n':
			if cur > 0 {
				cwds[cur] = line[1:]
			}
		}
	}
	return cwds
}

// platformCWD returns the working directory of a PID (via lsof).
func platformCWD(pid int) string {
	return darwinCwds([]int{pid})[pid]
}

func intsToCSV(pids []int) string {
	parts := make([]string, len(pids))
	for i, p := range pids {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

// platformIsAlive reports whether a PID exists and is not a zombie.
// Zombies respond to signal 0, so we additionally check the process state.
func platformIsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := osFindProcess(pid)
	if err != nil {
		return false
	}
	if proc.Signal(probeSignal()) != nil {
		return false
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "state=").Output()
	if err != nil {
		return false
	}
	state := strings.TrimSpace(string(out))
	if state == "" || strings.HasPrefix(state, "Z") {
		return false // zombie or gone
	}
	return true
}

// platformParseCommand: on darwin the argv is already joined; the naive
// whitespace split is fine for restart reconstruction in the common case.
func platformParseCommand(_ string) []string { return nil }

// osFindProcess is a thin wrapper so platformIsAlive can probe a PID.
func osFindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// probeSignal is signal 0 — used purely to test process existence.
func probeSignal() syscall.Signal {
	return syscall.Signal(0)
}
