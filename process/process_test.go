package process_test

import (
	"errors"
	"os"
	"testing"

	"github.com/msantos/goreap/process"
)

func TestNew(t *testing.T) {
	ps, err := process.New()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	if pid := os.Getpid(); pid != ps.Pid() {
		t.Errorf("pid = %d, want %d", ps.Pid(), pid)
		return
	}
}

func TestNewWithProcfs(t *testing.T) {
	procfs := "/bin"
	if err := os.Setenv("PROC", procfs); err != nil {
		t.Errorf("%v", err)
		return
	}
	_, err := process.New()
	if err == nil {
		t.Errorf("non-existent procfs %s", procfs)
		return
	}
	if !errors.Is(err, process.ErrProcNotMounted) {
		t.Errorf("procfs error = %v, want %v", err, process.ErrProcNotMounted)
		return
	}
	if err := os.Unsetenv("PROC"); err != nil {
		t.Errorf("%v", err)
		return
	}
}

func TestReadProcList(t *testing.T) {
	ps, err := process.New(process.WithPid(1), process.WithStrategy("ps"))
	if err != nil {
		t.Errorf("%v", err)
		return
	}
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
