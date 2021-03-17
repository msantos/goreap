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
	}
	if pid := os.Getpid(); pid != ps.Pid {
		t.Errorf("pid = %d, want %d", ps.Pid, pid)
	}
}

func TestNewWithProcfs(t *testing.T) {
	procfs := "/bin"
	if err := os.Setenv("PROC", procfs); err != nil {
		t.Errorf("%v", err)
	}
	_, err := process.New()
	if err == nil {
		t.Errorf("non-existent procfs %s", procfs)
	}
	if !errors.Is(err, process.ErrProcNotMounted) {
		t.Errorf("procfs error = %v, want %v", err, process.ErrProcNotMounted)
	}
}
