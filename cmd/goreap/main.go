package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/msantos/goreap/reap"
)

var version = "0.10.0"

func usage() {
	fmt.Fprintf(os.Stderr, `%s v%s
Usage: %s [options] <command> <...>

Options:
`, path.Base(os.Args[0]), version, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = func() { usage() }

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
	showVersion := flag.Bool("version", false, "display version and exit")
	verbose := flag.Bool("verbose", false, "debug output")

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	r := reap.New(
		reap.WithDeadline(*deadline),
		reap.WithDelay(*delay),
		reap.WithDisableSetuid(*disableSetuid),
		reap.WithSignal(*sig),
		reap.WithWait(*wait),
		reap.WithLog(func(err error) {
			if *verbose {
				fmt.Println(err)
			}
		}),
	)

	status, err := r.Exec(flag.Args(), os.Environ())
	if err != nil {
		fmt.Printf("%s: %s\n", flag.Arg(0), err)
	}

	os.Exit(status)
}
