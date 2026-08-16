//go:build darwin

package inspector

import (
	"encoding/binary"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// darwinCommandLine returns the exact argv of a PID via the KERN_PROCARGS2
// sysctl. Unlike ps output, this preserves argument boundaries and quotes, so
// --restart can reconstruct the command faithfully.
func darwinCommandLine(pid int) ([]string, bool) {
	data, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(data) < 8 {
		return nil, false
	}
	// Layout: int32 argc, NUL-terminated executable path, then argc
	// NUL-terminated argv strings, then the environment.
	argc := int(binary.LittleEndian.Uint32(data[0:4]))
	rest := data[4:]
	if i := strings.IndexByte(string(rest), 0); i >= 0 {
		rest = rest[i+1:]
	} else {
		return nil, false
	}
	argv := make([]string, 0, argc)
	for len(rest) > 0 && len(argv) < argc {
		j := strings.IndexByte(string(rest), 0)
		if j < 0 {
			break
		}
		if j > 0 {
			argv = append(argv, string(rest[:j]))
		}
		rest = rest[j+1:]
	}
	if len(argv) == 0 {
		return nil, false
	}
	return argv, true
}

// darwinEnrichArgv replaces the ps-derived command with the exact argv from
// KERN_PROCARGS2, deriving the process name from the executable path.
func darwinEnrichArgv(p *Process) {
	if p == nil || p.PID <= 0 {
		return
	}
	argv, ok := darwinCommandLine(p.PID)
	if !ok {
		return
	}
	p.Argv = argv
	p.Command = strings.Join(argv, " ")
	if argv[0] != "" {
		if base := filepath.Base(argv[0]); base != "" && base != "." && base != "/" {
			p.Name = base
		}
	}
}
