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

// Procfs is the default mount point for procfs filesystems. The default
// mountpoint can be changed by setting the PROC environment variable.
const Procfs = "/proc"

type Process interface {
	Pid() int
	Children() ([]int, error)
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

type Opt struct {
	procfs   string
	pid      int
	snapshot string
}

type Option func(*Opt)

// New sets the default configuration state for the process.
func New(opts ...Option) Process {
	o := &Opt{
		pid:    os.Getpid(),
		procfs: getenv("PROC", Procfs),
	}

	for _, opt := range opts {
		opt(o)
	}

	ps := &Ps{
		pid:    o.pid,
		procfs: o.procfs,
	}

	if o.snapshot == "ps" {
		return ps
	}

	if err := procChildrenExists(o.procfs, o.pid); err != nil {
		if o.snapshot == "" {
			return ps
		}
	}

	return &ProcChildren{Ps: ps}
}

// WithPid sets the process ID.
func WithPid(pid int) Option {
	return func(o *Opt) {
		o.pid = pid
	}
}

// WithPid sets the location of the procfs mount point.
func WithProcfs(procfs string) Option {
	return func(o *Opt) {
		path, err := filepath.Abs(procfs)
		if err != nil {
			return
		}
		if err := isProcMounted(path); err != nil {
			return
		}
		o.procfs = path
	}
}

// WithSnapshot sets the method for discovering subprocesses:
//
//  * ps: scan a snapshot of the system process table
//  * children: read /proc/[PID]/task/*/children
func WithSnapshot(snapshot string) Option {
	return func(o *Opt) {
		if snapshot == "ps" || snapshot == "children" {
			o.snapshot = snapshot
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
		return fs.ErrNotExist
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
		return PID{}, fs.ErrInvalid
	}

	bracket := strings.LastIndexByte(stat, ')')
	if bracket == -1 {
		return PID{}, fs.ErrInvalid
	}

	var state byte
	var ppid int

	if n, err := fmt.Sscanf(stat[bracket+1:], " %c %d", &state, &ppid); err != nil || n != 2 {
		return PID{}, fs.ErrInvalid
	}
	return PID{Pid: pid, PPid: ppid}, nil
}

// Processes returns a snapshot of the sysytem process table by walking
// through /proc.
func Processes(procfs string) (p []PID, err error) {
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
