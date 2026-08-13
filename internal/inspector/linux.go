//go:build linux

package inspector

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// /proc/net parsing — pure Go, zero external dependencies.
// ---------------------------------------------------------------------------

// procNetLine is one parsed row of /proc/net/tcp{,6}.
type procNetLine struct {
	inode uint64
	port  int
	state string
	addr  string
}

// listenState is the TCP state code for LISTEN in /proc/net/tcp.
const listenState = "0A"

// platformFindByPort resolves a port via /proc/net/tcp + inode→PID scan.
func platformFindByPort(port int) ([]*Process, error) {
	lines, err := procNetListeners()
	if err != nil {
		return nil, err
	}
	var wanted []procNetLine
	for _, l := range lines {
		if l.port == port {
			wanted = append(wanted, l)
		}
	}
	if len(wanted) == 0 {
		return nil, ErrPortFree
	}
	pids := inodeToPIDs()
	procs := make([]*Process, 0, len(wanted))
	for _, l := range wanted {
		pid, ok := pids[l.inode]
		if !ok {
			continue
		}
		p, err := platformGetProcess(pid)
		if err != nil {
			continue
		}
		p.Port = port
		p.Protocol = "tcp"
		p.LocalAddr = l.addr
		procs = append(procs, p)
	}
	if len(procs) == 0 {
		return nil, ErrPortFree
	}
	return procs, nil
}

// platformFindAll returns every TCP LISTEN process.
func platformFindAll() ([]*Process, error) {
	lines, err := procNetListeners()
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	pids := inodeToPIDs()

	seen := map[int]bool{}
	procs := make([]*Process, 0, len(lines))
	for _, l := range lines {
		pid, ok := pids[l.inode]
		if !ok || seen[pid] {
			continue
		}
		seen[pid] = true
		p, err := platformGetProcess(pid)
		if err != nil {
			continue
		}
		p.Port = l.port
		p.Protocol = "tcp"
		p.LocalAddr = l.addr
		procs = append(procs, p)
	}
	return procs, nil
}

// platformGetProcess reads full info from /proc.
func platformGetProcess(pid int) (*Process, error) {
	if pid <= 0 {
		return nil, errors.New("invalid pid")
	}
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	if _, err := os.Stat(statPath); err != nil {
		return nil, fmt.Errorf("process %d not found", pid)
	}

	p := &Process{PID: pid, Protocol: "tcp"}
	p.PPID, p.Name = procStatBasic(statPath)
	p.Argv = procCmdlineArgs(pid)
	p.Command = strings.Join(p.Argv, " ")
	p.User = procUser(pid)
	p.MemoryMB, _ = procRSSMB(pid)
	p.CPUPercent = procCPUPercent(pid)

	if p.Name == "" && p.Command == "" {
		return nil, fmt.Errorf("process %d has no readable metadata", pid)
	}
	// Kernel threads have no cmdline; comm is inside "[...]".
	if p.Command == "" && !strings.HasPrefix(p.Name, "[") {
		p.Command = p.Name
	}
	return p, nil
}

// platformAllProcesses: full PID/PPID/name snapshot from /proc.
func platformAllProcesses() ([]*Process, error) {
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	procs := make([]*Process, 0, len(dirs))
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		stat := filepath.Join("/proc", d.Name(), "stat")
		ppid, name := procStatBasic(stat)
		if name == "" {
			continue
		}
		procs = append(procs, &Process{PID: pid, PPID: ppid, Name: name})
	}
	return procs, nil
}

// platformCWD reads the working directory symlink.
func platformCWD(pid int) string {
	link := fmt.Sprintf("/proc/%d/cwd", pid)
	dst, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	return dst
}

// procNetListeners parses /proc/net/tcp and /proc/net/tcp6 for LISTEN rows.
func procNetListeners() ([]procNetLine, error) {
	var out []procNetLine
	for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		lines, err := parseProcNet(file)
		if err != nil {
			if os.IsNotExist(err) {
				continue // tcp6 may be absent on some kernels
			}
			return nil, err
		}
		out = append(out, lines...)
	}
	return out, nil
}

func parseProcNet(path string) ([]procNetLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []procNetLine
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first { // header line
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		if fields[3] != listenState {
			continue
		}
		local := fields[1] // hex IP:hex port
		idx := strings.LastIndex(local, ":")
		if idx < 0 {
			continue
		}
		portHex := local[idx+1:]
		ipHex := local[:idx]
		port64, err := strconv.ParseUint(portHex, 16, 32)
		if err != nil {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, procNetLine{
			inode: inode,
			port:  int(port64),
			addr:  decodeProcIP(ipHex, path),
		})
	}
	return out, sc.Err()
}

// decodeProcIP converts the little-endian hex IP from /proc/net into
// a human-readable address.
func decodeProcIP(hex, path string) string {
	if strings.HasSuffix(path, "tcp6") {
		// 32 hex chars, groups of 4, reversed, separated by ":".
		trimmed := strings.TrimLeft(hex, "0")
		if len(hex) == 32 && trimmed != "" {
			var groups []string
			for i := 24; i >= 0; i -= 4 {
				groups = append(groups, hex[i:i+4])
			}
			return "[" + strings.Join(groups, ":") + "]"
		}
		if trimmed == "" {
			return "[::]"
		}
	}
	// IPv4: 8 hex chars, little-endian.
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		v, _ := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		b[3-i] = byte(v)
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

// inodeToPIDs maps socket inodes to owning PIDs by scanning /proc/*/fd.
func inodeToPIDs() map[uint64]int {
	result := map[uint64]int{}
	proc, err := os.ReadDir("/proc")
	if err != nil {
		return result
	}
	for _, d := range proc {
		if !d.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(filepath.Join("/proc", d.Name(), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join("/proc", d.Name(), "fd", fd.Name()))
			if err != nil {
				continue
			}
			// Link format: "socket:[123456]"
			if !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]"), 10, 64)
			if err != nil {
				continue
			}
			result[inode] = pid
		}
	}
	return result
}

// procStatBasic parses pid, ppid and comm from /proc/PID/stat.
func procStatBasic(path string) (ppid int, name string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}
	s := string(data)
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 0 || close < 0 || close <= open {
		return 0, ""
	}
	name = s[open+1 : close]
	rest := strings.Fields(s[close+1:])
	if len(rest) < 2 {
		return 0, name
	}
	ppid, _ = strconv.Atoi(rest[1]) // state(0), ppid(1), pgrp(2)...
	return ppid, name
}

// procCmdlineArgs reads the exact argv, NUL-separated.
func procCmdlineArgs(pid int) []string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return nil
	}
	parts := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	return parts
}

// procUser resolves the owner UID via /proc/PID/status.
func procUser(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return ParseUIDToName(fields[1])
			}
		}
	}
	return ""
}

// procRSSMB returns the resident set size in MB.
func procRSSMB(pid int) (float64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return 0, err
				}
				return kb / 1024.0, nil
			}
		}
	}
	return 0, nil
}

// procCPUPercent computes a live CPU % using a two-sample delta over 250ms.
func procCPUPercent(pid int) float64 {
	t1, s1 := procTicks(pid)
	if t1 <= 0 {
		return 0
	}
	time.Sleep(250 * time.Millisecond)
	t2, s2 := procTicks(pid)
	if t2 <= 0 || t2 == t1 {
		return 0
	}
	procDelta := float64(s2 - s1)
	totalDelta := float64(t2 - t1)
	if totalDelta <= 0 {
		return 0
	}
	pct := (procDelta / totalDelta) * float64(runtime.NumCPU()) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// procTicks returns (systemTotalTicks, processTicks).
func procTicks(pid int) (int64, int64) {
	total := int64(0)
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		first := strings.SplitN(string(data), "\n", 2)[0]
		fields := strings.Fields(first)
		for _, f := range fields[1:] {
			if v, err := strconv.ParseInt(f, 10, 64); err == nil {
				total += v
			}
		}
	}
	proc := int64(0)
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		s := string(data)
		open := strings.Index(s, "(")
		close := strings.LastIndex(s, ")")
		if open >= 0 && close > open {
			fields := strings.Fields(s[close+1:])
			// utime = fields[11], stime = fields[12]
			for _, idx := range []int{11, 12} {
				if len(fields) > idx {
					if v, err := strconv.ParseInt(fields[idx], 10, 64); err == nil {
						proc += v
					}
				}
			}
		}
	}
	return total, proc
}

// platformIsAlive: /proc existence + zombie check. Zombies keep their
// /proc entry, so we inspect the state character from stat(5).
func platformIsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	s := string(data)
	close := strings.LastIndex(s, ")")
	if close < 0 || close+2 >= len(s) {
		return false
	}
	// Format: "pid (comm) STATE ..." — state is the first field after ")".
	return s[close+2] != 'Z'
}

// platformParseCommand: on Linux the argv is stored verbatim and joined
// with spaces; tokens are reconstructed from the original NUL-joined data
// when available (see procCmdline). The fields split is a good fallback.
func platformParseCommand(cmd string) []string { return nil }
