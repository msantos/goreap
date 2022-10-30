//go:build !linux
// +build !linux

// Package subreaper sets the process as the init for descendant
// processes.
package subreaper

import (
	"golang.org/x/sys/unix"
)

// Set is disabled on this platform.
func Set() error {
	return unix.ENOSYS
}

// Get always returns false on this platform.
func Get() bool {
	return false
}
