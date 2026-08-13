//go:build !linux && !darwin

package ancestry

// detectService is unsupported on this platform.
func detectService(pid int) string { return "" }
