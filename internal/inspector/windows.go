//go:build windows

package inspector

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// platformFindByPort resolves a port via `netstat -ano`.
func platformFindByPort(port int) ([]*Process, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %w", err)
	}
	pids := map[int]bool{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// Format: proto local foreign state PID
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		_, portStr, err := splitWindowsAddr(fields[1])
		if err != nil {
			continue
		}
		p, _ := strconv.Atoi(portStr)
		if p != port {
			continue
		}
		pid, _ := strconv.Atoi(fields[4])
		if pid > 0 {
			pids[pid] = true
		}
	}
	if len(pids) == 0 {
		return nil, ErrPortFree
	}
	procs := make([]*Process, 0, len(pids))
	for pid := range pids {
		p, err := platformGetProcess(pid)
		if err != nil {
			continue
		}
		p.Port = port
		p.Protocol = "tcp"
		procs = append(procs, p)
	}
	return procs, nil
}

// platformFindAll: every TCP LISTENING process.
func platformFindAll() ([]*Process, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %w", err)
	}
	seen := map[int]bool{}
	var procs []*Process
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		addr, portStr, err := splitWindowsAddr(fields[1])
		if err != nil {
			continue
		}
		port, _ := strconv.Atoi(portStr)
		pid, _ := strconv.Atoi(fields[4])
		if pid <= 0 || seen[pid] {
			continue
		}
		p, err := platformGetProcess(pid)
		if err != nil {
			continue
		}
		p.Port = port
		p.LocalAddr = addr
		seen[pid] = true
		procs = append(procs, p)
	}
	return procs, nil
}

// platformGetProcess resolves details via tasklist + PowerShell.
func platformGetProcess(pid int) (*Process, error) {
	if pid <= 0 {
		return nil, errors.New("invalid pid")
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil || len(out) == 0 {
		return nil, fmt.Errorf("process %d not found", pid)
	}
	// CSV: "name","pid","session","session#","mem"
	fields := parseCSV(string(out))
	if len(fields) < 2 {
		return nil, fmt.Errorf("process %d not found", pid)
	}
	p := &Process{PID: pid, Protocol: "tcp", Name: fields[0]}
	if len(fields) >= 5 {
		if mem, err := strconv.ParseFloat(strings.ReplaceAll(fields[4], ",", ""), 64); err == nil {
			p.MemoryMB = mem / 1024.0 // tasklist reports KB
		}
	}
	p.User = windowsProcessUser(pid)
	p.Command = windowsCommandLine(pid)
	p.PPID = windowsParentPID(pid)
	if p.Command == "" {
		p.Command = p.Name
	}
	return p, nil
}

// platformAllProcesses: PID/PPID via tasklist /v is too slow; use PowerShell
// one-shot instead.
func platformAllProcesses() ([]*Process, error) {
	ps := `Get-CimInstance Win32_Process | ForEach-Object { "{0} {1} {2}" -f $_.ProcessId, $_.ParentProcessId, $_.Name }`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return nil, fmt.Errorf("powershell failed: %w", err)
	}
	var procs []*Process
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		procs = append(procs, &Process{PID: pid, PPID: ppid, Name: fields[2]})
	}
	return procs, nil
}

// platformCWD via PowerShell (Win32_Process ExecutablePath is not cwd; we
// approximate with the containing directory of the executable).
func platformCWD(pid int) string {
	ps := fmt.Sprintf(`(Get-Process -Id %d).Path`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return ""
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return ""
	}
	// Walk up to a git root by checking for .git in parent dirs is done by
	// the caller's gitInfo via resolveGitDir on this cwd value.
	return path
}

func windowsCommandLine(pid int) string {
	ps := fmt.Sprintf(`(Get-CimInstance Win32_Process -Filter "ProcessId=%d").CommandLine`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func windowsParentPID(pid int) int {
	ps := fmt.Sprintf(`(Get-CimInstance Win32_Process -Filter "ProcessId=%d").ParentProcessId`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return v
}

func windowsProcessUser(pid int) string {
	ps := fmt.Sprintf(`(Get-CimInstance Win32_Process -Filter "ProcessId=%d").GetOwner().User`, pid)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// splitWindowsAddr splits "0.0.0.0:3000" or "[::]:3000".
func splitWindowsAddr(addr string) (host, port string, err error) {
	if strings.HasPrefix(addr, "[") {
		end := strings.Index(addr, "]")
		if end < 0 {
			return "", "", errors.New("bad addr")
		}
		host = addr[:end+1]
		port = strings.TrimPrefix(addr[end+1:], ":")
		return host, port, nil
	}
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return "", "", errors.New("bad addr")
	}
	return addr[:idx], addr[idx+1:], nil
}

// parseCSV parses a single tasklist CSV row into fields.
func parseCSV(s string) []string {
	line := strings.TrimSpace(s)
	line = strings.TrimPrefix(line, "\"")
	line = strings.TrimSuffix(line, "\"\r\n")
	line = strings.TrimSuffix(line, "\"\n")
	var fields []string
	for _, part := range strings.Split(line, "\",\"") {
		fields = append(fields, strings.Trim(part, "\""))
	}
	return fields
}

// platformIsAlive via tasklist.
func platformIsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(fmt.Sprintf(" %d ", pid)))
}

// platformParseCommand: naive whitespace split is acceptable on Windows.
func platformParseCommand(cmd string) []string { return nil }
