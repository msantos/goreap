// Package subreaper sets the process as the init for descendant
// processes.
package subreaper

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// Set configures the process as a subreaper.
func Set() error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
}

// Get indicates whether the current process is the init process
// for descendant processes.
func Get() bool {
	var arg2 int

	err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER,
		uintptr(unsafe.Pointer(&arg2)), 0, 0, 0)

	return err == nil && arg2 == 1
}
