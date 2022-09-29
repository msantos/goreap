package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Configuration state for the process using /proc/children.
//
// Contains the path to the procfs(5) children file:
//
//	A space-separated list of child tasks of this task.  Each child task
//	is represented by its TID.
//
// If the kernel was compiled with CONFIG_PROC_CHILDREN enabled, the
// default path is set to /proc/self/task/*/children.
type ProcChildren struct {
	*Ps
}

// Return the list of subprocesses for a PID by reading /proc/children.
func (ps *ProcChildren) Children() ([]int, error) {
	pids := make([]int, 0)

	paths, err := filepath.Glob(
		fmt.Sprintf("%s/self/task/*/children", ps.procfs),
	)
	if err != nil {
		return pids, err
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
