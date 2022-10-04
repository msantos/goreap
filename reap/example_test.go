package reap_test

import (
	"fmt"

	"github.com/msantos/goreap/reap"
)

func ExampleReap_Exec() {
	r := reap.New()

	status, err := r.Exec([]string{"env", "-i"}, []string{"FOO=bar"})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("exit status:", status)
	// Output: exit status: 0
}
