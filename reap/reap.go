package reap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
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

	sigch chan os.Signal

	process.Process
}

type Option func(*Reap)

func WithDeadline(t time.Duration) Option {
	return func(r *Reap) {
		r.deadline = t
	}
}

func WithDelay(t time.Duration) Option {
	return func(r *Reap) {
		r.delay = t
	}
}

func WithDisableSetuid(b bool) Option {
	return func(r *Reap) {
		r.disableSetuid = b
	}
}

func WithLog(f func(error)) Option {
	return func(r *Reap) {
		r.log = f
	}
}

func WithSignal(sig int) Option {
	return func(r *Reap) {
		r.sig = syscall.Signal(sig)
	}
}

func WithWait(b bool) Option {
	return func(r *Reap) {
		r.wait = b
	}
}

func New(opts ...Option) (*Reap, error) {
	r := &Reap{}

	ps, err := process.New()
	if err != nil {
		return r, err
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch)

	r.sig = syscall.Signal(15)
	r.delay = time.Duration(1) * time.Second
	r.deadline = time.Duration(60) * time.Second
	r.Process = ps
	r.sigch = sigch

	for _, opt := range opts {
		opt(r)
	}

	if r.log == nil {
		r.log = func(error) {}
	}

	if r.deadline == 0 {
		r.deadline = time.Duration(maxInt64)
	}

	if r.delay == 0 {
		r.delay = time.Duration(1)
	}

	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return r, fmt.Errorf("prctl(PR_SET_CHILD_SUBREAPER): %w", err)
	}

	return r, nil
}

func (r *Reap) Exec(argv []string, env []string) (int, error) {
	if r.disableSetuid {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return 111, fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", err)
		}
	}

	exitStatus, err := r.execv(argv[0], argv[1:], env)
	if err != nil {
		return exitStatus, err
	}

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
	pids, err := r.Children()
	if err != nil {
		r.log(err)
		return
	}

	for _, pid := range pids {
		r.log(fmt.Errorf("%d: kill %d %d", r.Pid(), sig, pid))
		r.kill(pid, sig)
	}
}

func (r *Reap) reap() error {
	exitch := make(chan struct{})
	defer close(exitch)

	go func() {
		t := time.NewTimer(r.deadline)
		tick := time.NewTicker(r.delay)

		sig := r.sig

		if !r.wait {
			r.signalWith(sig)
		}

		for {
			select {
			case <-exitch:
				return
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

func (r *Reap) execv(command string, args []string, env []string) (int, error) {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Start(); err != nil {
		return 127, err
	}

	waitch := make(chan error, 1)
	go func() {
		waitch <- cmd.Wait()
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
		case err := <-waitch:
			if err == nil {
				return 0, nil
			}

			if !errors.As(err, &exitError) {
				return 111, err
			}

			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return 111, err
			}

			if waitStatus.Signaled() {
				return 128 + int(waitStatus.Signal()), nil
			}

			return waitStatus.ExitStatus(), nil
		}
	}
}
