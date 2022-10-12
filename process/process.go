// Package process enumerates the process table for all processes or
// descendents of a process.
package process

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// Procfs is the default mount point for procfs filesystems. The default
	// mountpoint can be changed by setting the PROC environment variable.
	Procfs = "/proc"

	// No such process
	ErrSearch = unix.ESRCH
)

var (
	ErrInvalid  = fs.ErrInvalid  // "invalid argument"
	ErrNotExist = fs.ErrNotExist // "file does not exist"
)

type Process interface {
	Pid() int
	Children() ([]int, error)
	Snapshot() ([]PID, error)
}

// PID contains the contents of /proc/stat for a process.
type PID struct {
	// process ID
	Pid int
	// parent process ID
	PPid int
}

func getenv(s, def string) string {
	v := os.Getenv(s)
	if v == "" {
		return def
	}
	return v
}

type Option func(*Ps)

// New sets the default configuration state for the process.
func New(opts ...Option) Process {
	ps := &Ps{
		pid:    os.Getpid(),
		procfs: getenv("PROC", Procfs),
	}

	for _, opt := range opts {
		opt(ps)
	}

	if ps.snapshot == "ps" {
		return ps
	}

	if err := procChildrenExists(ps.procfs, ps.pid); err != nil {
		if ps.snapshot == "" {
			return ps
		}
	}

	return &ProcChildren{Ps: ps}
}

// WithPid sets the process ID.
func WithPid(pid int) Option {
	return func(ps *Ps) {
		ps.pid = pid
	}
}

// WithPid sets the location of the procfs mount point.
func WithProcfs(procfs string) Option {
	return func(ps *Ps) {
		path, err := filepath.Abs(procfs)
		if err != nil {
			return
		}
		if err := isProcMounted(path); err != nil {
			return
		}
		ps.procfs = path
	}
}

// WithSnapshot sets the method for discovering subprocesses.
func WithSnapshot(snapshot SnapshotStrategy) Option {
	return func(ps *Ps) {
		if snapshot == SnapshotPs || snapshot == SnapshotChildren {
			ps.snapshot = snapshot
		}
	}
}

func procChildrenExists(procfs string, pid int) error {
	children := fmt.Sprintf(
		"%s/self/task/%d/children",
		procfs,
		pid,
	)
	_, err := os.Stat(children)
	return err
}

func isProcMounted(procfs string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(procfs, &buf); err != nil {
		return err
	}
	if buf.Type != unix.PROC_SUPER_MAGIC {
		return ErrNotExist
	}
	return nil
}

func readProcStat(name string) (PID, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return PID{}, err
	}

	// <pid> (<comm>) <state> <ppid> ...
	// 21230 (cat) R 9985
	//
	// comm may contain spaces, brackets and newlines
	// 21230 (cat foo) R ...
	// 21230 (cat (foo) S) R ...
	// 21230 (cat (foo)
	// S) R ...
	stat := string(b)

	var pid int

	if n, err := fmt.Sscanf(stat, "%d ", &pid); err != nil || n != 1 {
		return PID{}, ErrInvalid
	}

	bracket := strings.LastIndexByte(stat, ')')
	if bracket == -1 {
		return PID{}, ErrInvalid
	}

	var state byte
	var ppid int

	if n, err := fmt.Sscanf(stat[bracket+1:], " %c %d", &state, &ppid); err != nil || n != 2 {
		return PID{}, ErrInvalid
	}
	return PID{Pid: pid, PPid: ppid}, nil
}

func exists(procfs string, pid int) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/%d", procfs, pid)); err != nil {
		return false
	}
	return true
}

// Snapshot returns a snapshot of the system process table by walking
// through /proc.
func Snapshot(procfs string) (p []PID, err error) {
	matches, err := filepath.Glob(
		fmt.Sprintf("%s/[0-9]*/stat", procfs),
	)
	if err != nil {
		return p, err
	}
	for _, stat := range matches {
		pid, err := readProcStat(stat)
		if err != nil {
			continue
		}
		p = append(p, pid)
	}
	return p, err
}
