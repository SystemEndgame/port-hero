package guardrails

import (
	"testing"

	"github.com/SystemEndgame/port-hero/internal/inspector"
)

func proc(pid int, name, user string, port int) *inspector.Process {
	return &inspector.Process{PID: pid, Name: name, User: user, Port: port}
}

func TestCheckPIDOne(t *testing.T) {
	active, _ := Check(proc(1, "launchd", "root", 3000), false)
	if !HasCritical(active) {
		t.Error("PID 1 must be critical")
	}
}

func TestCheckKernelThread(t *testing.T) {
	active, _ := Check(proc(42, "[kworker/0:1]", "root", 3000), false)
	if !HasCritical(active) {
		t.Error("kernel thread must be critical")
	}
}

func TestCheckProtectedProcess(t *testing.T) {
	active, _ := Check(proc(999, "sshd", "root", 22), false)
	if !HasCritical(active) {
		t.Error("sshd must be critical")
	}
}

func TestCheckSelfKill(t *testing.T) {
	active, _ := Check(proc(Self(), "port-hero", "me", 3000), true)
	if !HasCritical(active) {
		t.Error("self-kill must be critical even with force")
	}
}

func TestCheckProtectedPort(t *testing.T) {
	active, _ := Check(proc(1234, "node", currentUser(), 53), false)
	if HasCritical(active) {
		t.Fatal("protected port should be a warning, not critical")
	}
	hasWarn := false
	for _, v := range active {
		if v.Code == "PROTECTED_PORT" {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Error("port 53 must raise PROTECTED_PORT warning")
	}

	// With force, the warning is bypassed.
	active, _ = Check(proc(1234, "node", currentUser(), 53), true)
	for _, v := range active {
		if v.Code == "PROTECTED_PORT" {
			t.Error("force must bypass protected-port warning")
		}
	}
}

func TestCheckForeignProcess(t *testing.T) {
	// Running as a non-root user against another user's process.
	if isRoot() {
		t.Skip("running as root; ownership check is skipped")
	}
	active, _ := Check(proc(1234, "node", "someone-else", 3000), true)
	if !HasCritical(active) {
		t.Error("foreign process must be critical")
	}
}

func TestCheckNormalProcess(t *testing.T) {
	active, _ := Check(proc(1234, "node", currentUser(), 3000), false)
	if HasCritical(active) {
		t.Errorf("normal process should pass: %v", active)
	}
	for _, v := range active {
		t.Errorf("unexpected violation: %v", v)
	}
}

func TestIsKernelThread(t *testing.T) {
	if !IsKernelThread("[kworker]") {
		t.Error("[kworker] should be a kernel thread")
	}
	if IsKernelThread("node") {
		t.Error("node is not a kernel thread")
	}
}
