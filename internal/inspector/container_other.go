//go:build !linux

package inspector

// containerInfo on non-Linux platforms. macOS Docker Desktop runs inside a
// VM, so host processes are never inside a container; Windows lacks the
// cgroup interface. Both cases return "" (not containerised).
func containerInfo(_ int) string {
	return ""
}
