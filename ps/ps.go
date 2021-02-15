package ps

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type Process struct {
	Pid  int
	PPid int
}

var errParseFailProcStat = errors.New("unable to parse stat")

func parseStat(name string) (pid, ppid int, err error) {
	b, err := ioutil.ReadFile(name)
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
		return 0, 0, errParseFailProcStat
	}

	bracket := strings.LastIndexByte(stat, ')')
	if bracket == -1 {
		return 0, 0, errParseFailProcStat
	}

	var state byte
	if n, err := fmt.Sscanf(stat[bracket+1:], " %c %d", &state, &ppid); err != nil || n != 2 {
		return 0, 0, errParseFailProcStat
	}
	return pid, ppid, nil
}

func Processes() (p []Process, err error) {
	matches, err := filepath.Glob("/proc/[0-9]*/stat")
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

func Children(pids []Process, pid int) (cld []Process) {
	for _, p := range pids {
		if p.PPid != pid {
			continue
		}
		cld = append(cld, p)
	}
	return cld
}

func Descendents(pids []Process, pid int) (cld []Process) {
	seen := make(map[int]struct{})
	return walk(pids, pid, seen, cld)
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
