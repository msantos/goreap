package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/msantos/goreap/reap"
)

var version = "0.8.0"

type stateT struct {
	argv          []string
	sig           int
	disableSetuid bool
	wait          bool
	verbose       bool
	deadline      time.Duration
	delay         time.Duration
}

func args() *stateT {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, `%s v%s
Usage: %s [options] <command> <...>

Options:
`, path.Base(os.Args[0]), version, os.Args[0])
		flag.PrintDefaults()
	}

	sig := flag.Int("signal", 15, "signal sent to supervised processes")
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

	return &stateT{
		argv:          flag.Args(),
		sig:           *sig,
		disableSetuid: *disableSetuid,
		wait:          *wait,
		deadline:      *deadline,
		delay:         *delay,
		verbose:       *verbose,
	}
}

func main() {
	state := args()

	r, err := reap.New(
		reap.SetDeadline(state.deadline),
		reap.SetDelay(state.delay),
		reap.SetDisableSetuid(state.disableSetuid),
		reap.SetSignal(state.sig),
		reap.SetWait(state.wait),
		reap.SetLog(func(err error) {
			if state.verbose {
				fmt.Println(err)
			}
		}),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(111)
	}

	status, err := r.Exec(state.argv, os.Environ())
	if err != nil {
		fmt.Printf("%s: %s\n", state.argv[0], err)
	}

	os.Exit(status)
}
