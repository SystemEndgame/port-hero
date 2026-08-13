//go:build darwin

package ancestry

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	launchdCacheMu sync.Mutex
	launchdCache   = map[int]string{}
)

// detectService identifies the launchd label managing a PID, best effort.
// Uses `launchctl procinfo <pid>` with a short timeout and caching.
func detectService(pid int) string {
	launchdCacheMu.Lock()
	if label, ok := launchdCache[pid]; ok {
		launchdCacheMu.Unlock()
		return label
	}
	launchdCacheMu.Unlock()

	if pid <= 1 {
		return ""
	}
	ctx, cancel := newTimeoutContext(700 * time.Millisecond)
	defer cancel()

	out, err := exec.CommandContext(ctx, "launchctl", "procinfo", strconv.Itoa(pid)).Output()
	label := ""
	if err == nil {
		// Output contains "program = /path/to/exe" and often the label in
		// "state = running" blocks; the label appears as a top-level key.
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.Contains(trimmed, " = ") {
				continue
			}
			// A bare line at the start is typically the service label.
			if !strings.HasPrefix(trimmed, "0x") && !strings.ContainsAny(trimmed, " \t") {
				label = trimmed
				break
			}
		}
	}

	launchdCacheMu.Lock()
	launchdCache[pid] = label
	launchdCacheMu.Unlock()
	return label
}
