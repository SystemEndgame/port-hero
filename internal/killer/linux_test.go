//go:build linux

package killer

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// TestKillAbortsOnPIDReuse verifies that the start-time check catches a PID
// that has been recycled between inspection and kill. Linux populates
// StartTime from proc(5) field 22, so a fabricated mismatching start time
// must abort the kill before any signal is sent.
func TestKillAbortsOnPIDReuse(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("kill", "-9", strconv.Itoa(cmd.Process.Pid)).Run()
	})

	p := &inspector.Process{
		PID:       cmd.Process.Pid,
		Name:      "sleep",
		User:      me(),
		Port:      0,
		StartTime: 123456789, // definitely wrong for a fresh process
	}
	if _, err := Kill(p, Options{Tree: true}); err == nil {
		t.Fatal("expected kill to abort when start time differs (PID reuse)")
	}

	// The process must still be alive — nothing was signalled.
	if !inspector.IsAlive(cmd.Process.Pid) {
		t.Fatal("process was killed despite the identity check aborting")
	}
}
