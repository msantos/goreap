package reap_test

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/msantos/goreap/process"
	"github.com/msantos/goreap/reap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

var (
	errNotSubreaper  = errors.New("process is not subreaper")
	errSetuidEnabled = errors.New("process does not disable setuid")
	errReapFailed    = errors.New("process failed to reap subprocesses")
)

func TestNew(t *testing.T) {
	_ = reap.New(
		reap.WithLog(func(err error) {
			t.Log(err)
		}),
	)

	g := new(errgroup.Group)
	n := runtime.NumCPU() * 2

	for i := n; i > 0; i-- {
		g.Go(func() error {
			var v uintptr
			if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&v)), 0, 0, 0); err != nil {
				return err
			}
			if int(v) != 0 {
				return nil
			}
			return fmt.Errorf("%d: %w", int(v), errNotSubreaper)
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("%v", err)
	}
}

func TestDisableSetuid(t *testing.T) {
	r := reap.New(
		reap.WithDisableSetuid(true),
		reap.WithLog(func(err error) {
			t.Log(err)
		}),
	)

	g := new(errgroup.Group)
	n := runtime.NumCPU() * 2

	for i := n; i > 0; i-- {
		g.Go(func() error {
			status, err := r.Exec([]string{"sh", "-c", "sudo -h 2>/dev/null"}, os.Environ())
			if err != nil {
				if errors.Is(err, syscall.ECHILD) {
					return nil
				}
				return err
			}
			if status == 0 {
				return fmt.Errorf("sudo: %d: %w", status, errSetuidEnabled)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("%v", err)
	}
}

func exec(r *reap.Reap, cmd []string, n int) error {
	g := new(errgroup.Group)

	for i := n; i > 0; i-- {
		g.Go(func() error {
			_, err := r.Exec(cmd, os.Environ())
			if err != nil {
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		if !errors.Is(err, syscall.ECHILD) {
			return err
		}
	}

	ps := process.New()

	children, err := ps.Children()
	if err != nil {
		return err
	}

	if len(children) != 0 {
		return errReapFailed
	}

	return nil
}

func TestExec(t *testing.T) {
	r := reap.New(
		reap.WithLog(func(err error) {
			t.Log(err)
		}),
	)

	cmd := []string{
		"bash", "-c",
		"(exec -a goreaptest-exec sleep 120) & (exec -a goreaptest-exec sleep 120) & (exec -a goreaptest-exec sleep 120) &",
	}

	if err := exec(r, cmd, 3); err != nil {
		t.Errorf("%v", err)
	}
}

func TestExecDeadline(t *testing.T) {
	r := reap.New(
		reap.WithSignal(15),
		reap.WithDeadline(time.Duration(1)*time.Second),
		reap.WithLog(func(err error) {
			t.Log(err)
		}),
	)

	cmd := []string{
		"bash", "-c",
		"trap '' TERM; (exec -a goreaptest-deadline sleep 120) & (exec -a goreaptest-deadline sleep 120) & (exec -a goreaptest-deadline sleep 120) &",
	}

	if err := exec(r, cmd, 3); err != nil {
		t.Errorf("%v", err)
	}
}
