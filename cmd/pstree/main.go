package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/msantos/goreap/process"
)

func main() {
	strategy := "any"

	switch len(os.Args) {
	case 3:
		strategy = os.Args[2]
	case 2:
	default:
		fmt.Fprintln(os.Stderr, "usage: <pid> [<strategy: any | ps | children>]")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ps, err := process.New(process.WithPid(pid), process.WithStrategy(strategy))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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
