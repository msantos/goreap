package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/msantos/goreap/process"

	"golang.org/x/sys/unix"
)

var version = "0.7.0"

type stateT struct {
	argv          []string
	sig           syscall.Signal
	disableSetuid bool
	wait          bool
	verbose       bool
	deadline      time.Duration
	delay         time.Duration
	ps            *process.Ps
	sigChan       chan os.Signal
}

const (
	maxInt64 = 1<<63 - 1
)

func args() *stateT {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, `%s v%s
Usage: %s [options] <command> <...>

Options:
`, path.Base(os.Args[0]), version, os.Args[0])
		flag.PrintDefaults()
	}

	sig := flag.Int("signal", int(syscall.SIGTERM),
		"signal sent to supervised processes")
	disableSetuid := flag.Bool("disable-setuid", false,
		"disallow setuid (unkillable) subprocesses")
	wait := flag.Bool("wait", false, "wait for subprocesses to exit")
	deadline := flag.Duration(
		"deadline",
		60*time.Second,
		"send SIGKILL if processes running after deadline (0 to disable)",
	)
	delay := flag.Duration(
		"delay",
		1*time.Second,
		"delay between signals (0 to disable)",
	)
	verbose := flag.Bool("verbose", false, "debug output")

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	ps, err := process.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(111)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	return &stateT{
		argv:          flag.Args(),
		sig:           syscall.Signal(*sig),
		disableSetuid: *disableSetuid,
		wait:          *wait,
		deadline:      *deadline,
		delay:         *delay,
		verbose:       *verbose,
		ps:            ps,
		sigChan:       sigChan,
	}
}

func main() {
	state := args()
	if state.disableSetuid {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			fmt.Fprintf(os.Stderr, "prctl(PR_SET_NO_NEW_PRIVS): %s\n", err)
			os.Exit(111)
		}
	}
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "prctl(PR_SET_CHILD_SUBREAPER): %s\n", err)
		os.Exit(111)
	}
	exitStatus := state.execv(state.argv[0], state.argv[1:], os.Environ())
	if err := state.reap(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitStatus)
}

func kill(pid int, sig syscall.Signal) {
	err := syscall.Kill(pid, sig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
}

func (state *stateT) signalWith(sig syscall.Signal) {
	pids, err := state.ps.Children()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	for _, pid := range pids {
		if state.verbose {
			fmt.Fprintf(os.Stderr, "%d: kill %d %d\n", state.ps.Pid, sig, pid)
		}

		kill(pid, sig)
	}
}

func (state *stateT) reap() error {
	go func() {
		deadline := state.deadline
		if deadline <= 0 {
			deadline = time.Duration(maxInt64)
		}

		t := time.NewTimer(deadline)
		tick := time.NewTicker(state.delay)

		sig := state.sig

		if !state.wait {
			state.signalWith(sig)
		}

		for {
			select {
			case <-t.C:
				sig = syscall.SIGKILL
			case sig := <-state.sigChan:
				switch sig.(syscall.Signal) {
				case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
				default:
					state.signalWith(sig.(syscall.Signal))
				}
			case <-tick.C:
				if !state.wait {
					state.signalWith(sig)
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

func (state *stateT) execv(command string, args []string, env []string) int {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		case sig := <-state.sigChan:
			switch sig.(syscall.Signal) {
			case syscall.SIGCHLD, syscall.SIGIO, syscall.SIGPIPE, syscall.SIGURG:
			default:
				state.signalWith(sig.(syscall.Signal))
			}
		case err := <-waitCh:
			if err == nil {
				return 0
			}

			if !errors.As(err, &exitError) {
				fmt.Fprintln(os.Stderr, err)
				return 111
			}

			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				fmt.Fprintln(os.Stderr, err)
				return 111
			}

			if waitStatus.Signaled() {
				return 128 + int(waitStatus.Signal())
			}

			return waitStatus.ExitStatus()
		}
	}
}
