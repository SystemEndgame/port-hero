//go:build linux

package inspector

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeProcIPIPv4(t *testing.T) {
	cases := map[string]string{
		"0100007F": "127.0.0.1",
		"00000000": "0.0.0.0",
		"0100000A": "10.0.0.1",
		"0A000001": "1.0.0.10",
	}
	for in, want := range cases {
		if got := decodeProcIP(in, "/proc/net/tcp"); got != want {
			t.Errorf("decodeProcIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeProcIPIPv6(t *testing.T) {
	cases := map[string]string{
		// ::1 — four little-endian 32-bit words.
		"00000000000000000000000001000000": "[0:0:0:0:0:0:0:1]",
		// :: — the unspecified address.
		"00000000000000000000000000000000": "[0:0:0:0:0:0:0:0]",
		// ::ffff:127.0.0.1 — rendered as dotted quad.
		"0000000000000000FFFF00000100007F": "127.0.0.1",
		// ::ffff:10.0.0.1
		"0000000000000000FFFF00000100000A": "10.0.0.1",
		// 2001:db8::1 — word0=0x20010db8 (LE "b80d0120"), word3=0x1 (LE "01000000").
		"b80d0120000000000000000001000000": "[2001:db8:0:0:0:0:0:1]",
	}
	for in, want := range cases {
		if got := decodeProcIP(in, "/proc/net/tcp6"); got != want {
			t.Errorf("decodeProcIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeProcIPShortIPv6(t *testing.T) {
	// Malformed/short tcp6 values must degrade to the unspecified address.
	if got := decodeProcIP("", "/proc/net/tcp6"); got != "[::]" {
		t.Errorf("empty tcp6 = %q, want [::]", got)
	}
	if got := decodeProcIP("1F90", "/proc/net/tcp6"); got != "[::]" {
		t.Errorf("short tcp6 = %q, want [::]", got)
	}
}

func TestProcStartTimeCurrentProcess(t *testing.T) {
	v := procStartTime(fmt.Sprintf("/proc/%d/stat", os.Getpid()))
	if v == 0 {
		t.Error("start time of the current process should be non-zero")
	}
}

func TestProcStartTimeMissingProcess(t *testing.T) {
	if v := procStartTime("/proc/999999999/stat"); v != 0 {
		t.Errorf("missing process start time = %d, want 0", v)
	}
}

func FuzzDecodeIPv6(f *testing.F) {
	f.Add("00000000000000000000000001000000")
	f.Add("0000000000000000FFFF00000100007F")
	f.Add("")
	f.Add("garbage-not-hex")
	f.Fuzz(func(t *testing.T, hex string) {
		// Must never panic, regardless of input.
		decodeProcIP(hex, "/proc/net/tcp6")
		decodeProcIP(hex, "/proc/net/tcp")
	})
}

func FuzzParseProcNet(f *testing.F) {
	seed := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0\n"
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data string) {
		path := filepath.Join(t.TempDir(), "tcp")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Skip()
		}
		_, _ = parseProcNet(path)
	})
}
