package reap_test

import (
	"fmt"

	"github.com/msantos/goreap/reap"
)

func ExampleExec() {
	r, err := reap.New()
	if err != nil {
		fmt.Println(err)
		return
	}

	status, err := r.Exec([]string{"env", "-i"}, []string{"FOO=bar"})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("exit status:", status)
	// Output: exit status: 0
}
