package reap_test

import (
	"fmt"

	"github.com/msantos/goreap/reap"
)

func ExampleReap_Supervise() {
	r := reap.New()

	status, err := r.Supervise([]string{"env", "-i"}, []string{"FOO=bar"})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("exit status:", status)
	// Output: exit status: 0
}
