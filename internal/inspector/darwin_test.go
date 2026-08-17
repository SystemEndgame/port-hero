//go:build darwin

package inspector

import (
	"os"
	"testing"
)

func TestParseLsofName(t *testing.T) {
	cases := map[string]struct{ addr, proto string }{
		"localhost:3000":  {"localhost:3000", "tcp"},
		"127.0.0.1:8080":  {"127.0.0.1:8080", "tcp"},
		"[::1]:3000":      {"[::1]:3000", "tcp"},
		"*:3000":          {"*:3000", "tcp"},
		"*:3000 (LISTEN)": {"*:3000", "tcp"},
		"":                {"", "tcp"},
	}
	for in, want := range cases {
		addr, proto := parseLsofName(in)
		if addr != want.addr || proto != want.proto {
			t.Errorf("parseLsofName(%q) = (%q, %q), want (%q, %q)", in, addr, proto, want.addr, want.proto)
		}
	}
}

func TestDarwinCommandLine(t *testing.T) {
	argv, ok := darwinCommandLine(os.Getpid())
	if !ok {
		t.Fatal("darwinCommandLine should succeed on our own process")
	}
	if len(argv) == 0 || argv[0] == "" {
		t.Fatalf("expected non-empty argv, got %q", argv)
	}
	// The exact argv must preserve argument boundaries: the test binary's
	// own command line ends with "-test.v" style flags as separate tokens.
	t.Logf("argv[0]=%q argv=%q", argv[0], argv)
}

func TestDarwinEnrichNative(t *testing.T) {
	p := &Process{PID: os.Getpid(), Name: "stale", Command: "stale command"}
	darwinEnrichNative(p)
	if p.Command == "stale command" {
		t.Error("expected exact argv to replace the ps-derived command")
	}
	if len(p.Argv) == 0 {
		t.Error("expected exact argv to be populated")
	}
	if p.Name == "stale" {
		t.Error("expected name to be derived from the executable path")
	}
	// Start time must be populated on the current macOS layout.
	if p.StartTime == 0 {
		t.Error("expected start time from PROC_PIDTBSDINFO")
	}
}

func FuzzParseLsofName(f *testing.F) {
	f.Add("localhost:3000")
	f.Add("[::1]:3000 (LISTEN)")
	f.Add("*:3000")
	f.Add("")
	f.Fuzz(func(t *testing.T, name string) {
		// Must never panic.
		parseLsofName(name)
	})
}
