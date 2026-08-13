package inspector

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// startTestServer launches a small HTTP server on an ephemeral port and
// returns the actual port it bound to.
func startTestServer(t *testing.T, dir string) int {
	t.Helper()
	if dir != "" {
		if err := exec.Command("mkdir", "-p", dir).Run(); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"))
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return port
}

func TestFindByPort(t *testing.T) {
	dir := t.TempDir()
	// Put the listener inside a git repo to exercise branch detection.
	if err := exec.Command("git", "init", "-q", dir).Run(); err == nil {
		_ = exec.Command("git", "-C", dir, "checkout", "-q", "-b", "feature/test").Run()
		_ = exec.Command("git", "-C", dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init").Run()
	}
	port := startTestServer(t, dir)

	deadline := time.Now().Add(5 * time.Second)
	var procs []*Process
	for time.Now().Before(deadline) {
		var err error
		procs, err = FindByPort(port)
		if err == ErrPortFree {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			t.Fatalf("FindByPort: %v", err)
		}
		break
	}
	if len(procs) == 0 {
		t.Fatal("no process found")
	}
	p := procs[0]
	t.Logf("found: PID=%d name=%q cmd=%q user=%q cwd=%q branch=%q dirty=%v mem=%.1fMB cpu=%.1f%%",
		p.PID, p.Name, p.Command, p.User, p.CWD, p.GitBranch, p.GitDirty, p.MemoryMB, p.CPUPercent)

	if p.PID <= 0 {
		t.Error("invalid PID")
	}
	if p.Name == "" {
		t.Error("missing process name")
	}
	if p.Project == "" {
		t.Log("warning: project name empty")
	}
}

func TestFindAll(t *testing.T) {
	procs, err := FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	_ = startTestServer(t, "")
	for _, p := range procs {
		if p.Port < 1 {
			t.Errorf("invalid port %d", p.Port)
		}
	}
	t.Logf("found %d listening processes", len(procs))
}

func TestTreeOrder(t *testing.T) {
	// Simulate: A(1) -> B(2) -> C(3), plus A(1) -> D(4)
	procs := []*Process{
		{PID: 1, PPID: 0, Name: "root"},
		{PID: 2, PPID: 1, Name: "child"},
		{PID: 3, PPID: 2, Name: "grandchild"},
		{PID: 4, PPID: 1, Name: "sibling"},
	}
	s := NewSnapshot(procs)
	order := s.TreeOrder(1)
	want := []int{3, 2, 4, 1}
	if strings.Join(intsToStrings(order), ",") != strings.Join(intsToStrings(want), ",") {
		t.Errorf("TreeOrder(1) = %v, want %v", order, want)
	}
	desc := s.Descendants(1)
	if len(desc) != 3 {
		t.Errorf("Descendants(1) = %d, want 3", len(desc))
	}
}

func intsToStrings(in []int) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fmt.Sprint(v)
	}
	return out
}
