package process

import (
	"os"
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
// default path is set to /proc/[Pid]/task/[Pid]/children.
type ProcChildren struct {
	*Ps
	procChildrenPath string
}

// Return the list of subprocesses for a PID by reading /proc/children.
func (ps *ProcChildren) Children() ([]int, error) {
	b, err := os.ReadFile(ps.procChildrenPath)
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
