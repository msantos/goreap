package process

type SnapshotStrategy string

const (
	SnapshotAny      SnapshotStrategy = ""
	SnapshotPs       SnapshotStrategy = "ps"
	SnapshotChildren SnapshotStrategy = "children"
)

// Ps contains the state for a process when scanning /proc.
type Ps struct {
	pid      int
	procfs   string
	snapshot SnapshotStrategy
}

// Pid retrieves the process identifier.
func (ps *Ps) Pid() int {
	return ps.pid
}

// Snapshot returns a snapshot of the system process table.
func (ps *Ps) Snapshot() ([]PID, error) {
	return Snapshot(ps.procfs)
}

// Children returns a snapshot of the list of subprocesses for a PID by
// walking /proc.
func (ps *Ps) Children() ([]int, error) {
	if !exists(ps.procfs, ps.pid) {
		return nil, ErrSearch
	}

	p, err := ps.Snapshot()
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
