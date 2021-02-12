package main

import (
	"errors"
	"flag"
	"fmt"
	"goreap/ps"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var version = "0.2.1"

type stateT struct {
	argv          []string
	signal        syscall.Signal
	disableSetuid bool
	wait          bool
	verbose       bool
}

func args() *stateT {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, `%s v%s
Usage: %s [options] <command> <...>

Options:
`, path.Base(os.Args[0]), version, os.Args[0])
		flag.PrintDefaults()
	}

	signal := flag.Int("signal", int(syscall.SIGTERM),
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

	return &stateT{
		argv:          flag.Args(),
		signal:        syscall.Signal(*signal),
		disableSetuid: *disableSetuid,
		wait:          *wait,
		verbose:       *verbose,
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
	exitStatus := execv(state.argv[0], state.argv[1:], os.Environ())
	if err := state.reap(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitStatus)
}

func (state *stateT) kill(pid int) {
	err := syscall.Kill(pid, state.signal)
	switch {
	case err == nil:
	case errors.Is(err, syscall.ESRCH):
	default:
		fmt.Fprintln(os.Stderr, err)
	}
}

func (state *stateT) pskill(self int) error {
	pids, err := ps.Processes()
	if err != nil {
		return err
	}

	for _, p := range pids {
		if p.PPid != self {
			continue
		}
		state.kill(p.Pid)
	}

	return nil
}

func (state *stateT) prockill(children string) error {
	b, err := ioutil.ReadFile(children)
	if err != nil {
		return err
	}

	pids := strings.Split(string(b), " ")
	for _, s := range pids {
		if s == "" {
			continue
		}
		pid, err := strconv.Atoi(s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		state.kill(pid)
	}

	return nil
}

func (state *stateT) reap() error {
	self := os.Getpid()
	children := fmt.Sprintf("/proc/%d/task/%d/children", self, self)
	useProc := true
	if _, err := os.Stat(children); err != nil {
		useProc = false
	}

	waitOpt := syscall.WNOHANG
	if state.wait {
		waitOpt = 0
	}

	for {
	waitpid:
		for {
			pid, err := syscall.Wait4(-1, nil, waitOpt, nil)
			switch {
			case pid == 0:
				// no subprocess has exited
				break waitpid
			case err == nil:
			case errors.Is(err, syscall.EINTR):
				continue
			case errors.Is(err, syscall.ECHILD):
				return nil
			default:
				return err
			}
		}

		if state.wait {
			continue
		}

		if !useProc {
			if err := state.pskill(self); err != nil {
				return err
			}
			continue
		}

		if err := state.prockill(children); err != nil {
			return err
		}
	}
}

func execv(command string, args []string, env []string) int {
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
			_ = cmd.Process.Signal(sig)
		case err := <-waitCh:
			var waitStatus syscall.WaitStatus
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				waitStatus = exitError.Sys().(syscall.WaitStatus)
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
