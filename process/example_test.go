package process_test

import (
	"fmt"

	"github.com/msantos/goreap/process"
)

func ExamplePs_Children() {
	ps := process.New(process.WithPid(1))
	pids, err := ps.Children()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(pids)
}

func ExamplePs_Snapshot() {
	ps := process.New()
	pids, err := ps.Snapshot()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(pids)
}
