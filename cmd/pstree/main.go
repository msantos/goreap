package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/msantos/goreap/process"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: <pid>")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ps, err := process.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ps.Pid = pid

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
