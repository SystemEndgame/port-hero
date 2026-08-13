//go:build linux

package ancestry

import "fmt"

func procCgroupPath(pid int) string {
	return fmt.Sprintf("/proc/%d/cgroup", pid)
}
