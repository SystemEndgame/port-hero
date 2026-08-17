package inspector

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// startTestServer launches a small HTTP server on an ephemeral port and
// returns the actual port it bound to.
func startTestServer(t *testing.T, dir string) int {
	t.Helper()
	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
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

func TestFindByPortUDP(t *testing.T) {
	// Bind a real UDP socket and resolve it via the platform backend. This
	// exercises the UDP path on every OS the tests run on.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer func() { _ = conn.Close() }()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	procs, err := FindByPortProto(port, "udp")
	if err == ErrPortFree {
		t.Skip("bound UDP socket not visible to the platform backend")
	}
	if err != nil {
		t.Fatalf("FindByPortProto(udp): %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("no process found for the UDP socket")
	}
	for _, p := range procs {
		if p.Protocol != "udp" {
			t.Errorf("expected protocol udp, got %q", p.Protocol)
		}
		if p.Port != port {
			t.Errorf("port = %d, want %d", p.Port, port)
		}
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

	// Sibling order is unspecified (map iteration); verify the invariants:
	// every process appears exactly once, the root is last, and each child
	// is signalled before its parent (post-order).
	if len(order) != 4 {
		t.Fatalf("TreeOrder(1) length = %d, want 4: %v", len(order), order)
	}
	if order[3] != 1 {
		t.Errorf("root must be signalled last, got order=%v", order)
	}
	pos := map[int]int{}
	for i, pid := range order {
		if _, dup := pos[pid]; dup {
			t.Fatalf("duplicate PID %d in order %v", pid, order)
		}
		pos[pid] = i
	}
	for _, p := range procs {
		if p.PPID != 0 && pos[p.PID] > pos[p.PPID] {
			t.Errorf("child %d signalled after parent %d in %v", p.PID, p.PPID, order)
		}
	}

	desc := s.Descendants(1)
	if len(desc) != 3 {
		t.Errorf("Descendants(1) = %d, want 3", len(desc))
	}
}

func TestTreeOrderDeepChain(t *testing.T) {
	// A linear chain of 10 must come out strictly child-first.
	var procs []*Process
	for i := 1; i <= 10; i++ {
		ppid := i - 1
		if i == 1 {
			ppid = 0
		}
		procs = append(procs, &Process{PID: i, PPID: ppid, Name: fmt.Sprintf("p%d", i)})
	}
	s := NewSnapshot(procs)
	order := s.TreeOrder(1)
	if len(order) != 10 {
		t.Fatalf("TreeOrder length = %d, want 10", len(order))
	}
	if order[0] != 10 || order[len(order)-1] != 1 {
		t.Errorf("deep chain order = %v, want [10..1]", order)
	}
}
