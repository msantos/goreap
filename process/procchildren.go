package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcChildren sets the configuration for generating a process snapshot
// by reading the procfs(5) children file:
//
//	A space-separated list of child tasks of this task.  Each child task
//	is represented by its TID.
//
// The kernel must be compiled with CONFIG_PROC_CHILDREN enabled.
type ProcChildren struct {
	*Ps
}

// Children returns the list of subprocesses for a PID by reading
// /proc/self/task/*/children.
//
// If CONFIG_PROC_CHILDREN is not enabled, the error is set to
// ErrNotExist.
func (ps *ProcChildren) Children() ([]int, error) {
	if !exists(ps.procfs, ps.pid) {
		return nil, ErrSearch
	}

	pids := make([]int, 0)

	paths, err := filepath.Glob(
		fmt.Sprintf("%s/self/task/*/children", ps.procfs),
	)
	if err != nil {
		return pids, err
	}
	if len(paths) == 0 {
		return pids, ErrNotExist
	}

	for _, v := range paths {
		pid, err := ps.readChildren(v)
		if err != nil {
			return pids, err
		}
		pids = append(pids, pid...)
	}

	return pids, nil
}

func (ps *ProcChildren) readChildren(path string) ([]int, error) {
	b, err := os.ReadFile(path)
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
