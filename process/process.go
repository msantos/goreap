package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Default mount point for procfs filesystems. The default mountpoint
// can be changed by setting the PROC environment variable:
//
//	export PROC=/tmp/proc
const Procfs = "/proc"

type Process interface {
	Pid() int
	Children() ([]int, error)
}

// Contents of /proc/stat for a process.
//
// Pid is the process ID.
//
// PPid is the parent process ID.
type PID struct {
	Pid  int
	PPid int
}

var (
	// ErrProcNotMounted is returned if /proc is not mounted or is
	// not a procfs filesystem.
	ErrProcNotMounted = errors.New("procfs not mounted")

	// ErrParseFailProcStat is returned if /proc/[PID]/stat is
	// malformed.
	ErrParseFailProcStat = errors.New("unable to parse stat")

	ErrUnsupportedProcStrategy = errors.New("unsupported proc strategy")
)

func getenv(s, def string) string {
	v := os.Getenv(s)
	if v == "" {
		return def
	}
	return v
}

type Opt struct {
	Procfs   string
	Pid      int
	Strategy string
}

type Option func(*Opt)

// Create the default configuration state for the process.
// Returns an error if /proc is not mounted or is not a procfs filesystem.
func New(opts ...Option) (Process, error) {
	v := getenv("PROC", Procfs)

	o := &Opt{
		Pid:    os.Getpid(),
		Procfs: v,
	}

	for _, opt := range opts {
		opt(o)
	}

	procfs, err := procfsExists(o.Procfs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", o.Procfs, err)
	}

	ps := &Ps{
		pid:    o.Pid,
		procfs: procfs,
	}

	err = procChildrenExists(procfs, o.Pid)

	switch o.Strategy {
	case "children":
		return &ProcChildren{Ps: ps}, err
	case "ps":
		return ps, nil
	case "", "any":
		if err == nil {
			return &ProcChildren{Ps: ps}, nil
		}
		return ps, nil
	}

	return nil, ErrUnsupportedProcStrategy
}

func WithPid(pid int) Option {
	return func(o *Opt) {
		o.Pid = pid
	}
}

func WithProcfs(procfs string) Option {
	return func(o *Opt) {
		o.Procfs = procfs
	}
}

func WithStrategy(strategy string) Option {
	return func(o *Opt) {
		o.Strategy = strategy
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

func procfsExists(path string) (string, error) {
	procfs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if err := isProcMounted(procfs); err != nil {
		return "", fmt.Errorf("%s: %w", procfs, err)
	}
	return procfs, nil
}

func isProcMounted(procfs string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(procfs, &buf); err != nil {
		return err
	}
	if buf.Type != unix.PROC_SUPER_MAGIC {
		return ErrProcNotMounted
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
		return PID{}, ErrParseFailProcStat
	}

	bracket := strings.LastIndexByte(stat, ')')
	if bracket == -1 {
		return PID{}, ErrParseFailProcStat
	}

	var state byte
	var ppid int

	if n, err := fmt.Sscanf(stat[bracket+1:], " %c %d", &state, &ppid); err != nil || n != 2 {
		return PID{}, ErrParseFailProcStat
	}
	return PID{Pid: pid, PPid: ppid}, nil
}

// Scan the process table in /proc.
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
