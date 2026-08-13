//go:build linux

package locks

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/golive-ly/port-hero/internal/inspector"
)

// linuxLockRow is one parsed /proc/locks line.
type linuxLockRow struct {
	lockType string // POSIX | FLOCK | OFDLCK
	advisory bool
	mode     string // READ | WRITE
	pid      int
	inode    uint64
	start    int64
	end      int64 // -1 = EOF
}

func parseProcLocks() ([]linuxLockRow, error) {
	f, err := os.Open("/proc/locks")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []linuxLockRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// Format:
		// 1: POSIX  ADVISORY  WRITE 1234 08:01:123 0 1073741825 1073742335
		// 1: FLOCK  ADVISORY  WRITE 5678 08:01:456 0 0 0
		if len(fields) < 8 {
			continue
		}
		row := linuxLockRow{lockType: fields[1], advisory: fields[2] == "ADVISORY", mode: fields[3]}
		row.pid, _ = strconv.Atoi(fields[4])
		inodePart := strings.TrimPrefix(fields[5], "08:")
		// fields[5] is "major:minor:inode"; inode is after the last colon.
		if idx := strings.LastIndex(fields[5], ":"); idx >= 0 {
			inodePart = fields[5][idx+1:]
		}
		row.inode, _ = strconv.ParseUint(inodePart, 10, 64)
		if len(fields) >= 9 {
			row.start, _ = strconv.ParseInt(fields[7], 10, 64)
			end, _ := strconv.ParseInt(fields[8], 10, 64)
			row.end = end
			if fields[8] == "EOF" || end == 0 && fields[7] == "0" {
				row.end = -1
			}
		}
		rows = append(rows, row)
	}
	return rows, sc.Err()
}

// platformByFile matches locks against the inode of the given path.
func platformByFile(path string) ([]Lock, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	target := inodeOf(fi)
	rows, err := parseProcLocks()
	if err != nil {
		return nil, err
	}
	var out []Lock
	for _, r := range rows {
		if r.inode != target {
			continue
		}
		out = append(out, rowToLock(r, path))
	}
	return out, nil
}

// platformAll returns all locks, resolving inode→path via /proc/*/fd.
func platformAll() ([]Lock, error) {
	rows, err := parseProcLocks()
	if err != nil {
		return nil, err
	}
	pathByInode := inodeToPaths()
	var out []Lock
	for _, r := range rows {
		p := pathByInode[r.inode]
		out = append(out, rowToLock(r, p))
	}
	return out, nil
}

func rowToLock(r linuxLockRow, path string) Lock {
	l := Lock{
		PID:   r.pid,
		File:  path,
		Type:  r.lockType,
		Mode:  r.mode,
		Start: r.start,
		End:   r.end,
	}
	if r.pid > 0 {
		if p, err := inspector.GetProcess(r.pid); err == nil {
			l.Name = p.Name
			l.Command = p.Command
		}
	}
	return l
}

// inodeToPaths builds a best-effort inode→path map by scanning fd symlinks.
func inodeToPaths() map[uint64]string {
	out := map[uint64]string{}
	proc, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	for _, d := range proc {
		if !d.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		fds, err := os.ReadDir(filepath.Join("/proc", d.Name(), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join("/proc", d.Name(), "fd", fd.Name()))
			if err != nil || strings.HasPrefix(link, "socket:") || strings.HasPrefix(link, "pipe:") {
				continue
			}
			fi, err := os.Stat(link)
			if err != nil {
				continue
			}
			out[inodeOf(fi)] = link
		}
	}
	return out
}

// inodeOf extracts the inode number from a FileInfo.
func inodeOf(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return st.Ino
	}
	return 0
}
