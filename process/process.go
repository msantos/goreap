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
//    export PROC=/tmp/proc
//
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

	// ErrParseFailProcStat is returned if /proc/<pid>/stat is
	// malformed.
	ErrParseFailProcStat = errors.New("unable to parse stat")
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

type ProcessOption func(*Opt)

// Create the default configuration state for the process.
// Returns an error if /proc is not mounted or is not a procfs filesystem.
func New(opts ...ProcessOption) (Process, error) {
	v := getenv("PROC", Procfs)

	o := &Opt{
		Pid:    os.Getpid(),
		Procfs: v,
	}

	for _, opt := range opts {
		opt(o)
	}

	procfs, err := procfsPath(o.Procfs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", o.Procfs, err)
	}

	switch o.Strategy {
	case "children":
		return useProcChildren(procfs, o.Pid)
	case "ps":
		return useProcPs(procfs, o.Pid)
	case "", "any":
		ps, err := useProcChildren(procfs, o.Pid)
		if err == nil {
			return ps, nil
		}
		return useProcPs(procfs, o.Pid)
	}

	return nil, fmt.Errorf("unknown proc strategy")
}

func useProcChildren(procfs string, pid int) (Process, error) {
	path, err := procChildrenPath(procfs, pid)
	if err != nil {
		return nil, err
	}

	return &ProcChildren{
		pid:              pid,
		procfs:           procfs,
		procChildrenPath: path,
	}, nil
}

func useProcPs(procfs string, pid int) (Process, error) {
	return &Ps{
		pid:    pid,
		procfs: procfs,
	}, nil
}

func SetPid(pid int) ProcessOption {
	return func(o *Opt) {
		o.Pid = pid
	}
}

func SetProcfs(procfs string) ProcessOption {
	return func(o *Opt) {
		o.Procfs = procfs
	}
}

func SetStrategy(strategy string) ProcessOption {
	return func(o *Opt) {
		o.Strategy = strategy
	}
}

func procChildrenPath(procfs string, pid int) (string, error) {
	children := fmt.Sprintf(
		"%s/%d/task/%d/children",
		procfs,
		pid,
		pid,
	)
	if _, err := os.Stat(children); err != nil {
		return "", err
	}
	return children, nil
}

func procfsPath(path string) (string, error) {
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

func readProcStat(name string) (pid, ppid int, err error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return 0, 0, err
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

	if n, err := fmt.Sscanf(stat, "%d ", &pid); err != nil || n != 1 {
		return 0, 0, ErrParseFailProcStat
	}

	bracket := strings.LastIndexByte(stat, ')')
	if bracket == -1 {
		return 0, 0, ErrParseFailProcStat
	}

	var state byte
	if n, err := fmt.Sscanf(stat[bracket+1:], " %c %d", &state, &ppid); err != nil || n != 2 {
		return 0, 0, ErrParseFailProcStat
	}
	return pid, ppid, nil
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
		pid, ppid, err := readProcStat(stat)
		if err != nil {
			continue
		}
		p = append(p, PID{Pid: pid, PPid: ppid})
	}
	return p, err
}
