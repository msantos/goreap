package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// Configuration state for the process.
//
// The Pid field defaults to the PID of the current process.
//
// ProcChildren contains the path to the procfs(5) children file:
//
//    A space-separated list of child tasks of this task.  Each child task
//    is represented by its TID.
//
// If the kernel was compiled with CONFIG_PROC_CHILDREN enabled, the
// default path is set to /proc/[Pid]/task/[Pid]/children.
//
// If CONFIG_PROC_CHILDREN is not supported, the value is set to an
// empty string.
type Ps struct {
	Pid          int
	ProcChildren string
	procfs       string
}

// Contents of /proc/stat for a process.
//
// Pid is the process ID.
//
// PPid is the parent process ID.
type Process struct {
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

// Create the default configuration state for the process.
// Returns an error if /proc is not mounted or is not a procfs filesystem.
func New() (*Ps, error) {
	v := getenv("PROC", Procfs)
	procfs, err := procfsPath(v)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", v, err)
	}

	pid := os.Getpid()
	procChildren, _ := procChildrenPath(pid, procfs)

	return &Ps{
		Pid:          pid,
		procfs:       procfs,
		ProcChildren: procChildren,
	}, nil
}

// Get the current procfs path.
func (ps *Ps) GetProcfsPath() string {
	return ps.procfs
}

// Set the current procfs path, returning an error if the path is not
// a procfs filesystem.
func (ps *Ps) SetProcfsPath(path string) error {
	procfs, err := procfsPath(path)
	if err != nil {
		return err
	}
	ps.procfs = procfs
	return nil
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

// Retrieve the process table.
func (ps *Ps) Processes() (p []Process, err error) {
	matches, err := filepath.Glob(
		fmt.Sprintf("%s/[0-9]*/stat", ps.procfs),
	)
	if err != nil {
		return p, err
	}
	for _, stat := range matches {
		pid, ppid, err := readProcStat(stat)
		if err != nil {
			continue
		}
		p = append(p, Process{Pid: pid, PPid: ppid})
	}
	return p, err
}

// Return the list of subprocesses for a PID.
func (ps *Ps) Children() ([]int, error) {
	if ps.ProcChildren != "" {
		return ps.ReadProcChildren()
	}
	return ps.ReadProcList()
}

// Return the list of subprocesses for a PID by traversing /proc.
func (ps *Ps) ReadProcList() ([]int, error) {
	p, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return descendents(p, ps.Pid), nil
}

func descendents(pids []Process, pid int) []int {
	children := make(map[int]struct{})
	walk(pids, pid, children)
	cld := make([]int, len(children))
	i := 0
	for p := range children {
		cld[i] = p
		i++
	}
	return cld
}

func subprocs(pids []Process, pid int) (cld []Process) {
	for _, p := range pids {
		if p.PPid != pid {
			continue
		}
		cld = append(cld, p)
	}
	return cld
}

func walk(pids []Process, pid int, children map[int]struct{}) {
	for _, p := range subprocs(pids, pid) {
		if _, ok := children[p.Pid]; ok {
			continue
		}
		children[p.Pid] = struct{}{}
		walk(pids, p.Pid, children)
	}
}

// Return the list of subprocesses for a PID by reading /proc/children.
func (ps *Ps) ReadProcChildren() ([]int, error) {
	b, err := os.ReadFile(ps.ProcChildren)
	if err != nil {
		return nil, err
	}

	pids := strings.Fields(string(b))
	children := make([]int, len(pids))
	for i, s := range pids {
		pid, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		children[i] = pid
	}

	return children, nil
}

func procChildrenPath(pid int, procfs string) (string, error) {
	children := fmt.Sprintf(
		"%s/%d/task/%d/children",
		procfs,
		pid,
		pid,
	)
	_, err := os.Stat(children)
	if err != nil {
		return "", err
	}
	return children, nil
}
