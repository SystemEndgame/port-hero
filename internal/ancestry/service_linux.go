//go:build linux

package ancestry

import (
	"bufio"
	"os"
	"strings"
)

// detectService identifies the systemd unit (or container) that manages a
// process, by parsing its cgroup membership. Returns "" when the process is
// not managed by a systemd service.
func detectService(pid int) string {
	f, err := os.Open(procCgroupPath(pid))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// cgroup v2: "0::/system.slice/nginx.service"
		if !strings.Contains(line, ".service") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		path := parts[2]
		for _, seg := range strings.Split(path, "/") {
			if strings.HasSuffix(seg, ".service") {
				return seg
			}
		}
	}
	return ""
}

// platformManagerName returns the primary service manager name.
func platformManagerName() string { return "systemd" }
