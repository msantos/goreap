// Package subreaper sets the process as the init for descendant
// processes.
package subreaper

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	P_PID = 0

	PROC_REAP_ACQUIRE = 2 // reaping enable
	PROC_REAP_RELEASE = 3 // reaping disable
	PROC_REAP_STATUS  = 4 // reaping status
	PROC_REAP_GETPIDS = 5 // get descendants
	PROC_REAP_KILL    = 6 // kill descendants

	REAPER_STATUS_OWNED    = 0x00000001 // process has acquired reaper status
	REAPER_STATUS_REALINIT = 0x00000002 // process is the root of the reaper tree
)

// Set configures the process as a subreaper.
func Set() error {
	_, _, errno := syscall.Syscall6(
		unix.SYS_PROCCTL,  // trap
		P_PID,             // idtype
		0,                 // id
		PROC_REAP_ACQUIRE, // cmd
		0,                 // data
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Get indicates whether the current process is the init process
// for descendant processes.
func Get() bool {
	status, err := Status()
	return err == nil && (status.Flags&REAPER_STATUS_OWNED != 0)
}

type ReapStatus struct {
	Flags       uint
	Children    uint
	Descendants uint
	Reaper      int
	Pid         int
	pad0        [15]uint
}

func Status() (*ReapStatus, error) {
	status := &ReapStatus{}

	_, _, errno := syscall.Syscall6(
		unix.SYS_PROCCTL,                // trap
		P_PID,                           // idtype
		0,                               // id
		PROC_REAP_STATUS,                // cmd
		uintptr(unsafe.Pointer(status)), // data
		0,
		0,
	)

	if errno != 0 {
		return status, errno
	}
	return status, nil
}
