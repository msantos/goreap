package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/msantos/goreap/process"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

var version = "0.4.0"

type stateT struct {
	argv          []string
	sig           syscall.Signal
	disableSetuid bool
	wait          bool
	verbose       bool
	ps            *process.Ps
}

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

	return &stateT{
		argv:          flag.Args(),
		sig:           syscall.Signal(*sig),
		disableSetuid: *disableSetuid,
		wait:          *wait,
		verbose:       *verbose,
		ps:            ps,
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
	switch {
	case err == nil:
	case errors.Is(err, syscall.ESRCH):
	default:
		fmt.Fprintln(os.Stderr, err)
	}
}

func (state *stateT) signal() {
	state.signalWith(state.sig)
}

func (state *stateT) signalWith(sig syscall.Signal) {
	pids, err := state.ps.Children()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	for _, pid := range pids {
		if state.verbose {
			fmt.Fprintf(os.Stderr, "%d: kill %s %d\n",
				state.ps.Pid, unix.SignalName(sig), pid)
		}

		kill(pid, sig)
	}
}

func (state *stateT) reap() error {
	if !state.wait {
		go func() {
			for {
				state.signal()
			}
		}()
	}

	for {
		pid, err := syscall.Wait4(-1, nil, 0, nil)
		switch {
		case pid == 0:
		case err == nil:
		case errors.Is(err, syscall.EINTR):
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
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	for {
		select {
		case sig := <-sigChan:
			switch sig.(syscall.Signal) {
			case syscall.SIGCHLD:
			case syscall.SIGIO:
			case syscall.SIGPIPE:
			case syscall.SIGURG:
			default:
				state.signalWith(sig.(syscall.Signal))
			}
		case err := <-waitCh:
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				waitStatus := exitError.Sys().(syscall.WaitStatus)
				if waitStatus.Signaled() {
					return 128 + int(waitStatus.Signal())
				}
				return waitStatus.ExitStatus()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 111
			}
			return 0
		}
	}
}
