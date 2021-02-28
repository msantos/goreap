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

const Procfs = "/proc"

type Ps struct {
	Pid          int
	ProcChildren string
	procfs       string
}

type Option func(*Ps)

type Process struct {
	Pid  int
	PPid int
}

var (
	ErrProcNotMounted    = errors.New("procfs not mounted")
	ErrParseFailProcStat = errors.New("unable to parse stat")
)

func getenv(s, def string) string {
	v := os.Getenv(s)
	if v == "" {
		return def
	}
	return v
}

func New() (*Ps, error) {
	ps := &Ps{
		procfs: getenv("PROC", Procfs),
	}

	if err := procMounted(ps.procfs); err != nil {
		return nil, fmt.Errorf("%s: %w", ps.procfs, err)
	}

	ps.Pid = os.Getpid()
	ps.ProcChildren, _ = procChildrenPath(ps.Pid, ps.procfs)

	return ps, nil
}

func (ps *Ps) Procfs() string {
	return ps.procfs
}

func procMounted(procfs string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(procfs, &buf); err != nil {
		return err
	}
	if buf.Type != unix.PROC_SUPER_MAGIC {
		return ErrProcNotMounted
	}

	return nil
}

func parseStat(name string) (pid, ppid int, err error) {
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

func Processes(procfs string) (p []Process, err error) {
	matches, err := filepath.Glob(
		fmt.Sprintf("%s/[0-9]*/stat", procfs),
	)
	if err != nil {
		return p, err
	}
	for _, stat := range matches {
		pid, ppid, err := parseStat(stat)
		if err != nil {
			continue
		}
		p = append(p, Process{Pid: pid, PPid: ppid})
	}
	return p, err
}

func (ps *Ps) Processes() (p []Process, err error) {
	return Processes(ps.procfs)
}

func (ps *Ps) Children() ([]int, error) {
	if ps.ProcChildren != "" {
		return ps.ReadProcChildren()
	}
	return ps.ReadProcList()
}

func (ps *Ps) ReadProcListPid(pid int) ([]int, error) {
	p, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return Descendents(p, pid), nil
}

func (ps *Ps) ReadProcList() ([]int, error) {
	return ps.ReadProcListPid(ps.Pid)
}

func Children(pids []Process, pid int) (cld []Process) {
	for _, p := range pids {
		if p.PPid != pid {
			continue
		}
		cld = append(cld, p)
	}
	return cld
}

func Descendents(pids []Process, pid int) []int {
	seen := make(map[int]struct{})
	cld := make([]Process, 0)
	cld = walk(pids, pid, seen, cld)
	children := make([]int, len(cld))
	for i, p := range cld {
		children[i] = p.Pid
	}
	return children
}

func walk(pids []Process, pid int, seen map[int]struct{}, cld []Process) []Process {
	for _, p := range Children(pids, pid) {
		if _, ok := seen[p.Pid]; ok {
			continue
		}
		seen[p.Pid] = struct{}{}
		cld = append(cld, p)
		cld = walk(pids, p.Pid, seen, cld)
	}
	return cld
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

func (ps *Ps) ProcChildrenPath(pid int) (string, error) {
	return procChildrenPath(pid, ps.procfs)
}

func (ps *Ps) ReadProcChildrenPath(path string) ([]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	pids := strings.Fields(string(b))
	children := make([]int, len(pids))
	for i, s := range pids {
		pid, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		children[i] = pid
	}

	return children, nil
}

func (ps *Ps) ReadProcChildren() ([]int, error) {
	return ps.ReadProcChildrenPath(ps.ProcChildren)
}
