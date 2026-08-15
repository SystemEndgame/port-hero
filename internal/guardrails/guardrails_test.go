package guardrails

import (
	"testing"

	"github.com/SystemEndgame/port-hero/internal/config"
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

func TestConfigureWhitelistPort(t *testing.T) {
	Configure(&config.Config{Whitelist: config.Whitelist{Ports: []int{5432}}})
	defer Configure(nil)

	active, _ := Check(proc(1234, "postgres", currentUser(), 5432), false)
	for _, v := range active {
		if v.Code == "PROTECTED_PORT" {
			t.Error("whitelisted port 5432 must not raise PROTECTED_PORT")
		}
	}

	// A protected port that is NOT whitelisted must still warn.
	active, _ = Check(proc(1234, "postgres", currentUser(), 3306), false)
	found := false
	for _, v := range active {
		if v.Code == "PROTECTED_PORT" {
			found = true
		}
	}
	if !found {
		t.Error("non-whitelisted protected port must still raise PROTECTED_PORT")
	}
}

func TestConfigureWhitelistProcess(t *testing.T) {
	Configure(&config.Config{Whitelist: config.Whitelist{Processes: []string{"myworker"}}})
	defer Configure(nil)

	// Low PID warning suppressed for whitelisted process name.
	active, _ := Check(proc(7, "myworker", currentUser(), 3000), false)
	for _, v := range active {
		if v.Code == "LOW_PID" {
			t.Error("whitelisted process must not raise LOW_PID")
		}
	}

	// Non-whitelisted low PID still warns.
	active, _ = Check(proc(7, "node", currentUser(), 3000), false)
	found := false
	for _, v := range active {
		if v.Code == "LOW_PID" {
			found = true
		}
	}
	if !found {
		t.Error("non-whitelisted low PID must still raise LOW_PID")
	}
}

func TestConfigureExtraProtectedDaemon(t *testing.T) {
	Configure(&config.Config{Protection: config.Protection{Daemons: []string{"mycriticald"}}})
	defer Configure(nil)

	active, _ := Check(proc(1234, "mycriticald", "root", 3000), true)
	if !HasCritical(active) {
		t.Error("user-configured protected daemon must be critical even with --force")
	}
}
