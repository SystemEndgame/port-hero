//go:build linux

package locks

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseProcLocks(f *testing.F) {
	seed := "1: POSIX  ADVISORY  WRITE 1234 08:01:123 0 1073741825 1073742335\n" +
		"2: FLOCK  ADVISORY  WRITE 5678 08:01:456 0 0 0\n" +
		"3: OFDLCK ADVISORY  READ  -1 08:02:789 0 0 9223372036854775807\n"
	f.Add(seed)
	f.Add("")
	f.Add("garbage line\n")
	f.Fuzz(func(t *testing.T, data string) {
		path := filepath.Join(t.TempDir(), "locks")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Skip()
		}
		rows, err := parseProcLocksPath(path)
		if err != nil {
			return
		}
		for _, r := range rows {
			if r.pid < -1 {
				t.Fatalf("invalid pid parsed: %d", r.pid)
			}
		}
	})
}
