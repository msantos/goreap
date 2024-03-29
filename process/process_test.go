package process_test

import (
	"errors"
	"os"
	"testing"

	"github.com/msantos/goreap/process"
)

func TestNew(t *testing.T) {
	ps := process.New()
	if pid := os.Getpid(); pid != ps.Pid() {
		t.Errorf("pid = %d, want %d", ps.Pid(), pid)
		return
	}
}

func TestNewWithProcfs(t *testing.T) {
	procfs := "/bin"
	ps := process.New(process.WithProcfs(procfs))
	_, err := ps.Children()
	if err != nil {
		t.Errorf("procfs failed %s", err)
		return
	}
}

func TestReadProcList(t *testing.T) {
	ps := process.New(
		process.WithPid(1),
		process.WithSnapshot(process.SnapshotPs),
	)
	pids, err := ps.Children()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	if len(pids) == 0 {
		t.Errorf("process table is empty = %v", ps)
		return
	}
}

func TestSnapshot(t *testing.T) {
	ps := process.New()
	pids, err := ps.Snapshot()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	if len(pids) == 0 {
		t.Errorf("process table is empty = %v", ps)
		return
	}
}

func TestErrSearch(t *testing.T) {
	pid := 123456
	ps := process.New(process.WithPid(pid))
	pids, err := ps.Children()
	if err == nil {
		t.Errorf("found: %d: %v", pid, pids)
		return
	}
	if !errors.Is(err, process.ErrSearch) {
		t.Errorf("%v", err)
		return
	}
}
