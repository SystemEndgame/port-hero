//go:build !linux && !darwin

package ancestry

// detectService is unsupported on this platform.
func detectService(pid int) string { return "" }

// platformManagerName returns the primary service manager name.
func platformManagerName() string { return "unknown" }
