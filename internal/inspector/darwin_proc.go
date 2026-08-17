//go:build darwin

package inspector

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The libproc syscalls were consolidated into a single `proc_info` trap
// (SYS_proc_info = 336) whose first argument selects the call variant.
const (
	sysProcInfo     = 336
	callPidinfo     = 2
	procPIDTBSDINFO = 3
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

// darwinEnrichNative adds the exact argv (KERN_PROCARGS2) and the start time
// (PROC_PIDTBSDINFO) to a process, replacing the ps-derived command.
func darwinEnrichNative(p *Process) {
	if p == nil || p.PID <= 0 {
		return
	}
	if argv, ok := darwinCommandLine(p.PID); ok {
		p.Argv = argv
		p.Command = strings.Join(argv, " ")
		if argv[0] != "" {
			if base := filepath.Base(argv[0]); base != "" && base != "." && base != "/" {
				p.Name = base
			}
		}
	}
	p.StartTime = darwinStartTime(p.PID)
}

// darwinStartTime returns the process start time in microseconds since the
// epoch, or 0 when the layout of proc_bsdinfo differs from the version we
// know (the struct grew across macOS releases). StartTime is used to detect
// PID reuse; a zero value just disables that specific check.
func darwinStartTime(pid int) uint64 {
	buf := make([]byte, 512)
	n, _, err := syscall.Syscall6(sysProcInfo, callPidinfo, uintptr(pid), procPIDTBSDINFO, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if err != 0 || int(n) < 136 {
		return 0
	}
	if int(n) != 136 {
		return 0
	}
	sec := binary.LittleEndian.Uint64(buf[120:128])
	if sec == 0 {
		return 0
	}
	usec := binary.LittleEndian.Uint64(buf[128:136])
	return sec*1_000_000 + usec
}
