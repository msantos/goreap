// Package reap configures the go process as a process supervisor. A
// process supervisor is the init process for subprocesses and
// terminates all subprocesses when the foreground process exits.
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
	err   error

	process.Process
}

type Option func(*Reap)

// WithDeadline sets a timeout for subprocesses to exit after the
// foreground process exits. When the deadline is reached, subprocesses
// are signaled with SIGKILL.
func WithDeadline(t time.Duration) Option {
	return func(r *Reap) {
		if t == 0 {
			r.deadline = time.Duration(maxInt64)
			return
		}
		r.deadline = t
	}
}

// WithDelay waits the specified duration before resending signals
// after the foreground process exits.
func WithDelay(t time.Duration) Option {
	return func(r *Reap) {
		if t == 0 {
			r.delay = time.Duration(1)
			return
		}
		r.delay = t
	}
}

// WithDisableSetuid disallows unkillable setuid subprocesses.
func WithDisableSetuid(b bool) Option {
	return func(r *Reap) {
		r.disableSetuid = b
	}
}

// WithLog specifies a function for logging.
func WithLog(f func(error)) Option {
	return func(r *Reap) {
		if f == nil {
			r.log = func(error) {}
			return
		}
		r.log = f
	}
}

// WithSignal sets the signal sent to subprocesses after the foreground
// process exits.
func WithSignal(sig int) Option {
	return func(r *Reap) {
		r.sig = syscall.Signal(sig)
	}
}

// WithWait disables signalling subprocesses.
func WithWait(b bool) Option {
	return func(r *Reap) {
		r.wait = b
	}
}

// New sets the current process to act as a process supervisor.
func New(opts ...Option) *Reap {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch)

	r := &Reap{
		Process:  process.New(),
		delay:    time.Duration(1) * time.Second,
		deadline: time.Duration(60) * time.Second,
		log:      func(error) {},
		sig:      syscall.Signal(15),
		sigch:    sigch,
	}

	for _, opt := range opts {
		opt(r)
	}

	r.err = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)

	return r
}

// Supervise creates a subprocess, terminating all subprocesses when
// the foreground process exits.
func (r *Reap) Supervise(argv []string, env []string) (int, error) {
	status, err := r.Exec(argv, env)

	if err := r.Reap(); err != nil {
		return 111, err
	}

	return status, err
}

// Exec forks and executes a subprocess.
func (r *Reap) Exec(argv []string, env []string) (int, error) {
	if r.err != nil {
		return 111, r.err
	}

	if r.disableSetuid {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return 111, fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", err)
		}
	}

	return r.execv(argv[0], argv[1:], env)
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

func (r *Reap) reaper(exitch <-chan struct{}) {
	t := time.NewTimer(r.deadline)
	tick := time.NewTicker(r.delay)

	signal := func(sig syscall.Signal) {
		if r.wait {
			return
		}
		r.signalWith(r.sig)
	}

	signal(r.sig)

	for {
		select {
		case <-exitch:
			return
		case <-t.C:
			r.sig = syscall.SIGKILL
		case sig := <-r.sigch:
			switch sig {
			case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
			default:
				r.signalWith(sig.(syscall.Signal))
			}
		case <-tick.C:
			signal(r.sig)
		}
	}
}

// Reap delivers a signal to all descendants of this process.
func (r *Reap) Reap() error {
	exitch := make(chan struct{})
	defer close(exitch)

	go r.reaper(exitch)

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

	return r.waitpid(waitch)
}

func (r *Reap) waitpid(waitch <-chan error) (int, error) {
	var exitError *exec.ExitError

	for {
		select {
		case sig := <-r.sigch:
			switch sig {
			case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
			default:
				r.signalWith(sig.(syscall.Signal))
			}
		case err := <-waitch:
			if err == nil {
				return 0, nil
			}

			if !errors.As(err, &exitError) {
				return 128, err
			}

			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return 128, err
			}

			if waitStatus.Signaled() {
				return 128 + int(waitStatus.Signal()), nil
			}

			return waitStatus.ExitStatus(), nil
		}
	}
}
