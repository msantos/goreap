package process_test

import (
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
	ps := process.New(process.WithPid(1), process.WithSnapshot("ps"))
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
