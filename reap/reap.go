package reap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/msantos/goreap/process"

	"golang.org/x/sys/unix"
)

const (
	maxInt64 = 1<<63 - 1
)

type Reap struct {
	sig           syscall.Signal
	disableSetuid bool
	wait          bool
	deadline      time.Duration
	delay         time.Duration
	log           func(error)

	ps    process.Process
	sigch chan os.Signal
}

type ReapOption func(*Reap)

func SetDeadline(t time.Duration) ReapOption {
	return func(r *Reap) {
		r.deadline = t
	}
}

func SetDelay(t time.Duration) ReapOption {
	return func(r *Reap) {
		r.delay = t
	}
}

func SetDisableSetuid(b bool) ReapOption {
	return func(r *Reap) {
		r.disableSetuid = b
	}
}

func SetLog(f func(error)) ReapOption {
	return func(r *Reap) {
		r.log = f
	}
}

func SetSignal(sig int) ReapOption {
	return func(r *Reap) {
		r.sig = syscall.Signal(sig)
	}
}

func SetWait(b bool) ReapOption {
	return func(r *Reap) {
		r.wait = b
	}
}

func New(opts ...ReapOption) (*Reap, error) {
	r := &Reap{}

	ps, err := process.New()
	if err != nil {
		return r, err
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch)

	r.ps = ps
	r.sigch = sigch

	for _, opt := range opts {
		opt(r)
	}

	if r.log == nil {
		r.log = func(error) {}
	}

	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return r, fmt.Errorf("prctl(PR_SET_CHILD_SUBREAPER): %w", err)
	}

	return r, nil
}

func (r *Reap) setNoNewPrivs() error {
	if !r.disableSetuid {
		return nil
	}

	return unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
}

func (r *Reap) Exec(argv []string, env []string) (int, error) {
	if err := r.setNoNewPrivs(); err != nil {
		return 111, fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", err)
	}

	exitStatus := r.execv(argv[0], argv[1:], env)
	if err := r.reap(); err != nil {
		return 111, err
	}

	return exitStatus, nil
}

func (r *Reap) kill(pid int, sig syscall.Signal) {
	err := syscall.Kill(pid, sig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return
	}
	r.log(err)
}

func (r *Reap) signalWith(sig syscall.Signal) {
	pids, err := r.ps.Children()
	if err != nil {
		r.log(err)
		return
	}

	for _, pid := range pids {
		r.log(fmt.Errorf("%d: kill %d %d", r.ps.Pid(), sig, pid))
		r.kill(pid, sig)
	}
}

func (r *Reap) reap() error {
	go func() {
		deadline := r.deadline
		if deadline <= 0 {
			deadline = time.Duration(maxInt64)
		}

		t := time.NewTimer(deadline)
		tick := time.NewTicker(r.delay)

		sig := r.sig

		if !r.wait {
			r.signalWith(sig)
		}

		for {
			select {
			case <-t.C:
				sig = syscall.SIGKILL
			case sig := <-r.sigch:
				switch sig.(syscall.Signal) {
				case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
				default:
					r.signalWith(sig.(syscall.Signal))
				}
			case <-tick.C:
				if !r.wait {
					r.signalWith(sig)
				}
			}
		}
	}()

	for {
		_, err := syscall.Wait4(-1, nil, 0, nil)
		switch {
		case err == nil, errors.Is(err, syscall.EINTR):
		case errors.Is(err, syscall.ECHILD):
			return nil
		default:
			return err
		}
	}
}

func (r *Reap) execv(command string, args []string, env []string) int {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Start(); err != nil {
		r.log(err)
		return 127
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	var exitError *exec.ExitError

	for {
		select {
		case sig := <-r.sigch:
			switch sig.(syscall.Signal) {
			case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
			default:
				r.signalWith(sig.(syscall.Signal))
			}
		case err := <-waitCh:
			if err == nil {
				return 0
			}

			if !errors.As(err, &exitError) {
				r.log(err)
				return 111
			}

			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				r.log(err)
				return 111
			}

			if waitStatus.Signaled() {
				return 128 + int(waitStatus.Signal())
			}

			return waitStatus.ExitStatus()
		}
	}
}
