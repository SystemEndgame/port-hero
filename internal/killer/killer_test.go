package killer

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/SystemEndgame/port-hero/internal/guardrails"
	"github.com/SystemEndgame/port-hero/internal/inspector"
)

func me() string { return guardrails.CurrentUser() }

func spawn(t *testing.T, argv ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %v: %v", argv, err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return cmd
}

func TestKillGraceful(t *testing.T) {
	cmd := spawn(t, "sleep", "30")
	p := &inspector.Process{PID: cmd.Process.Pid, Name: "sleep", User: me(), Port: 0}

	res, err := Kill(p, Options{GracePeriod: 500 * time.Millisecond, Tree: true})
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if !res.Graceful {
		t.Errorf("expected graceful termination, got: %v", res.Summary())
	}
	if !res.AllGone {
		t.Errorf("expected all gone, got: %v", res.Summary())
	}
}

func TestKillTreeNoOrphans(t *testing.T) {
	// shell spawns two grandchildren (sleep 34 & sleep 35) and waits.
	shell := spawn(t, "sh", "-c", "sleep 34 & sleep 35 & wait")

	time.Sleep(300 * time.Millisecond) // let children start

	p := &inspector.Process{PID: shell.Process.Pid, Name: "sh", User: me(), Port: 0}
	res, err := Kill(p, Options{GracePeriod: 400 * time.Millisecond, Tree: true})
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if !res.AllGone {
		t.Errorf("expected whole tree gone, got: %v", res.Summary())
	}
	if len(res.Terminated) < 1 {
		t.Errorf("expected root terminated, got: %v", res.Summary())
	}

	// Root must be dead.
	for i := 0; i < 30; i++ {
		if !inspector.IsAlive(shell.Process.Pid) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if inspector.IsAlive(shell.Process.Pid) {
		t.Error("root process still alive after tree kill")
	}
}

func TestKillBlockedPIDOne(t *testing.T) {
	p := &inspector.Process{PID: 1, Name: "launchd", User: "root", Port: 1}
	_, err := Kill(p, Options{Tree: true})
	if err == nil {
		t.Fatal("expected guardrail block for PID 1")
	}
}

func TestKillBlockedKernelThread(t *testing.T) {
	p := &inspector.Process{PID: 42, Name: "[kworker/0:1]", User: "root", Port: 1}
	_, err := Kill(p, Options{Tree: true, Force: true})
	if err == nil {
		t.Fatal("expected guardrail block for kernel thread even with force")
	}
}

func TestKillBlockedSelf(t *testing.T) {
	p := &inspector.Process{PID: os.Getpid(), Name: "killer.test", User: me(), Port: 1}
	_, err := Kill(p, Options{Tree: true, Force: true})
	if err == nil {
		t.Fatal("expected guardrail block for self-kill even with force")
	}
}

func TestReverifyOK(t *testing.T) {
	cmd := spawn(t, "sleep", "30")
	p := &inspector.Process{PID: cmd.Process.Pid, Name: "sleep", User: me(), Port: 0}
	if err := reverify(p); err != nil {
		t.Fatalf("reverify failed for live own process: %v", err)
	}
}

func TestReverifyGoneProcess(t *testing.T) {
	// A PID that no longer exists must be rejected by reverify.
	p := &inspector.Process{PID: 999999999, Name: "ghost", User: me(), Port: 0}
	if err := reverify(p); err == nil {
		t.Fatal("expected reverify to reject a non-existent process")
	}
}

func TestReverifyOwnerChange(t *testing.T) {
	cmd := spawn(t, "sleep", "30")
	p := &inspector.Process{PID: cmd.Process.Pid, Name: "sleep", User: "someone-else", Port: 0}
	if err := reverify(p); err == nil {
		t.Fatal("expected reverify to reject an owner change")
	}
}

func TestKillDryRunDoesNotSignal(t *testing.T) {
	cmd := spawn(t, "sleep", "30")
	p := &inspector.Process{PID: cmd.Process.Pid, Name: "sleep", User: me(), Port: 0}
	res, err := Kill(p, Options{Tree: true, DryRun: true})
	if err != nil {
		t.Fatalf("Kill(dry-run): %v", err)
	}
	if !res.Graceful || !res.AllGone {
		t.Errorf("dry run should report graceful+allgone: %v", res.Summary())
	}
	if !inspector.IsAlive(cmd.Process.Pid) {
		t.Fatal("dry run must not terminate the process")
	}
}
