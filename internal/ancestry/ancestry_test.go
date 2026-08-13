package ancestry

import (
	"os"
	"strings"
	"testing"
)

func TestShortName(t *testing.T) {
	cases := map[string]string{
		"/Applications/Postgres.app/Contents/MacOS/postgres": "postgres",
		"/sbin/launchd":            "launchd",
		"node":                     "node",
		"/usr/lib/systemd/systemd": "systemd",
		"":                         "",
	}
	for in, want := range cases {
		if got := shortName(in); got != want {
			t.Errorf("shortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifySupervisor(t *testing.T) {
	cases := map[string]string{
		"systemd": "systemd",
		"launchd": "launchd",
		"sshd":    "ssh",
		"tmux":    "tmux",
		"cron":    "cron",
		"dockerd": "docker",
		"node":    "", // plain node is not a supervisor
	}
	for name, want := range cases {
		if got := classifySupervisor(name, ""); got != want {
			t.Errorf("classifySupervisor(%q) = %q, want %q", name, got, want)
		}
	}
	// pm2 detection via argv.
	if got := classifySupervisor("node", "/usr/local/bin/node /usr/bin/pm2 start server.js"); got != "pm2" {
		t.Errorf("pm2 argv detection failed: %q", got)
	}
}

func TestBuildSelf(t *testing.T) {
	chain, err := Build(os.Getpid())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if chain.TargetPID != os.Getpid() {
		t.Errorf("TargetPID = %d, want %d", chain.TargetPID, os.Getpid())
	}
	if len(chain.Nodes) < 2 {
		t.Error("expected at least target + parent in chain")
	}
	// Chain must reach PID 1 eventually.
	foundRoot := false
	for _, n := range chain.Nodes {
		if n.PID == 1 {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Error("chain should reach PID 1 (launchd/systemd)")
	}
	text := chain.Text()
	if !strings.Contains(text, "pid") {
		t.Errorf("Text() missing pid labels: %q", text)
	}
}

func TestBuildInvalidPID(t *testing.T) {
	chain, err := Build(999999999)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// A non-existent PID yields a chain containing just that PID.
	if len(chain.Nodes) == 0 {
		t.Error("expected at least one node")
	}
}
