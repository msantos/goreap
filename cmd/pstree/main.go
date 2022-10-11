package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/msantos/goreap/process"
)

func main() {
	snapshot := "any"

	switch len(os.Args) {
	case 3:
		snapshot = os.Args[2]
	case 2:
	default:
		fmt.Fprintln(os.Stderr, "usage: <pid> [<snapshot: %s | %s>]",
			process.SnapshotPs, process.SnapshotChildren,
		)
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ps := process.New(
		process.WithPid(pid),
		process.WithSnapshot(process.SnapshotStrategy(snapshot)),
	)

	children, err := ps.Children()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(children) == 0 {
		os.Exit(0)
	}

	fmt.Println(pid)
	for _, cld := range children {
		fmt.Printf("|-%v\n", cld)
	}
}
