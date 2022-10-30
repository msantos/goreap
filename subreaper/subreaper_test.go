package subreaper_test

import (
	"fmt"

	"github.com/msantos/goreap/subreaper"
)

func init() {
	ExampleSet()
}

func ExampleSet() {
	if err := subreaper.Set(); err != nil {
		fmt.Println("failed to set subreaper:", err)
	}
}

func ExampleGet() {
	fmt.Println(subreaper.Get())
	// Output:
	// true
}
