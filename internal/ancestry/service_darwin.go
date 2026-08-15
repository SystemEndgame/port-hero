//go:build darwin

package ancestry

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/SystemEndgame/port-hero/internal/cache"
)

// launchdCache maps PIDs to launchd labels. The 30s TTL lets labels follow
// short-lived processes while bounding memory.
var launchdCache = cache.New[string](30 * time.Second)

// detectService identifies the launchd label managing a PID, best effort.
// Uses `launchctl procinfo <pid>` with a short timeout and caching.
func detectService(pid int) string {
	if pid <= 1 {
		return ""
	}
	key := strconv.Itoa(pid)
	if label, ok := launchdCache.Get(key); ok {
		return label
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

	launchdCache.Set(key, label)
	return label
}

// platformManagerName returns the primary service manager name.
func platformManagerName() string { return "launchd" }
