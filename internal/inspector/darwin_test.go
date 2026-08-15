//go:build darwin

package inspector

import "testing"

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
