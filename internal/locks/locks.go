// Package locks detects which processes hold file locks — the silent
// killer of deployments and builds.
//
// Linux reads /proc/locks (pure Go). macOS derives holders via lsof.
package locks

// Lock describes one file lock held by a process.
type Lock struct {
	PID     int    `json:"pid"`
	Name    string `json:"name,omitempty"`
	File    string `json:"file,omitempty"`
	Type    string `json:"type"` // POSIX | FLOCK | OFDLCK
	Mode    string `json:"mode"` // READ | WRITE
	Command string `json:"command,omitempty"`
	Start   int64  `json:"start,omitempty"`
	End     int64  `json:"end,omitempty"` // -1 means "to EOF"
}

// ByFile returns every lock currently held on the given path.
func ByFile(path string) ([]Lock, error) {
	return platformByFile(path)
}

// All returns every advisory lock on the system.
func All() ([]Lock, error) {
	return platformAll()
}
