package process

// Ps contains the state for a process when scanning /proc.
type Ps struct {
	pid      int
	procfs   string
	snapshot string
}

// Pid retrieves the process identifier.
func (ps *Ps) Pid() int {
	return ps.pid
}

// Children returns a snapshot of the list of subprocesses for a PID by
// walking /proc.
func (ps *Ps) Children() ([]int, error) {
	p, err := Processes(ps.procfs)
	if err != nil {
		return nil, err
	}
	return descendents(p, ps.pid), nil
}

func descendents(pids []PID, pid int) []int {
	children := make(map[int]struct{})
	walk(pids, pid, children)
	cld := make([]int, 0, len(children))
	for p := range children {
		cld = append(cld, p)
	}
	return cld
}

func subprocs(pids []PID, pid int) (cld []PID) {
	for _, p := range pids {
		if p.PPid != pid {
			continue
		}
		cld = append(cld, p)
	}
	return cld
}

func walk(pids []PID, pid int, children map[int]struct{}) {
	for _, p := range subprocs(pids, pid) {
		if _, ok := children[p.Pid]; ok {
			continue
		}
		children[p.Pid] = struct{}{}
		walk(pids, p.Pid, children)
	}
}
