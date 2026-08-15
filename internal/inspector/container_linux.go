//go:build linux

package inspector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SystemEndgame/port-hero/internal/cache"
)

// containerNames resolves short container IDs to friendly names. The 30s TTL
// lets freshly created containers appear quickly while bounding memory.
var containerNames = cache.New[string](30 * time.Second)

// containerInfo detects whether a PID runs inside a container and returns
// a human label like "redis-dev (Docker)" or "abc123 (Docker)".
func containerInfo(pid int) string {
	cgroup := readCgroup(pid)
	if cgroup == "" {
		return ""
	}
	id := containerIDFromCgroup(cgroup)
	if id == "" {
		// Inside a non-standard container (e.g. k8s with opaque ids) —
		// the path itself is evidence we are containerised.
		if strings.Contains(cgroup, "docker") || strings.Contains(cgroup, "containerd") {
			return "(container)"
		}
		return ""
	}
	name := containerName(id)
	if name != "" {
		return name + " (Docker)"
	}
	return id + " (Docker)"
}

// readCgroup returns the (first) cgroup path of the process.
func readCgroup(pid int) string {
	f, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// Format: "hierarchy-id:controller-list:cgroup-path"
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		path := parts[2]
		// The unified cgroup (v2) is identified by "0::".
		if parts[0] == "0" && parts[1] == "" {
			return path
		}
		// Fall back to any path that mentions a container runtime.
		if strings.Contains(path, "docker") || strings.Contains(path, "containerd") || strings.Contains(path, "kubepods") {
			return path
		}
	}
	return ""
}

// containerIDFromCgroup extracts the 64-hex container id from a cgroup path.
func containerIDFromCgroup(path string) string {
	const hexLen = 64
	for _, field := range strings.Split(path, "/") {
		if len(field) == hexLen && isHex(field) {
			return field
		}
	}
	// Short 12-char ids appear in some runtimes (e.g. docker exec paths).
	if i := strings.Index(path, "docker-"); i >= 0 {
		rest := path[i+len("docker-"):]
		if j := strings.Index(rest, "."); j > 0 {
			rest = rest[:j]
		}
		if len(rest) >= 12 && isHex(rest) {
			return rest[:12]
		}
	}
	return ""
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

// containerName resolves a short name for a container id via the docker CLI.
// Best effort only — silently returns "" on failure or timeout.
func containerName(id string) string {
	short := id
	if len(short) > 12 {
		short = short[:12]
	}
	if n, ok := containerNames.Get(short); ok {
		return n
	}

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker",
		"inspect", "--format", "{{.Name}}", short).Output()
	name := ""
	if err == nil {
		name = strings.Trim(strings.TrimSpace(string(out)), "/")
	}

	containerNames.Set(short, name)
	return name
}
