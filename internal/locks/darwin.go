//go:build darwin

package locks

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// platformByFile derives the processes holding a file open (and thus any
// lock on it) via `lsof` (preinstalled on macOS). We parse the default
// table output, which is stable across macOS versions.
func platformByFile(path string) ([]Lock, error) {
	out, err := exec.Command("lsof", "-nP", "--", path).Output()
	if err != nil {
		// lsof exits 1 when nothing holds the file — not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof failed: %w", err)
	}
	return parseLsofTable(string(out), path)
}

// platformAll: macOS has no central lock table; lsof can list every open
// file but that is impractical to enumerate system-wide. The per-file query
// is the supported path (matching witr's macOS behaviour).
func platformAll() ([]Lock, error) {
	return nil, fmt.Errorf("system-wide lock listing is not supported on macOS; use `port --file <path>` for a specific file")
}

// parseLsofTable parses the default lsof table:
//
//	COMMAND   PID   USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
//	Python   66195 george   3w   REG   1,17      0  123 /private/tmp/x.txt
//
// NAME may contain spaces, so it is the remainder after column 8.
func parseLsofTable(out, path string) ([]Lock, error) {
	var locks []Lock
	sc := bufio.NewScanner(strings.NewReader(out))
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue // header
		}
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			continue
		}
		l := Lock{
			PID:   pid,
			Name:  fields[0],
			File:  path,
			Mode:  lockModeFromFD(fields[3]),
			Type:  "FLOCK",
			Start: 0,
			End:   -1,
		}
		if len(fields) > 8 {
			l.File = strings.Join(fields[8:], " ")
		}
		locks = append(locks, l)
	}
	// Enrich with command lines.
	for i := range locks {
		if p, err := inspector.GetProcess(locks[i].PID); err == nil {
			if locks[i].Name == "" {
				locks[i].Name = p.Name
			}
			locks[i].Command = p.Command
		}
	}
	return locks, nil
}

// lockModeFromFD decodes the FD column ("3w", "4r", "5u") into a mode.
func lockModeFromFD(fd string) string {
	if strings.Contains(fd, "r") {
		return "READ"
	}
	if strings.Contains(fd, "w") {
		return "WRITE"
	}
	if strings.Contains(fd, "u") {
		return "READWRITE"
	}
	return "UNKNOWN"
}
