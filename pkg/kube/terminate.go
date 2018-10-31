package kube

import (
	"fmt"
	"syscall"
	"time"
)

// Terminate stops process if such exists with either SIGTERM
// or with SIGKIILL when force is true. It checks for process termination
// and in case signal was ignored by the process it returns an error.
func Terminate(pid int, force bool) error {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	err := syscall.Kill(pid, sig)
	if err != nil {
		return fmt.Errorf("could not signal: %v", err)
	}

	var attempt int
	err = syscall.Kill(pid, 0)
	for err == nil && attempt < 10 {
		time.Sleep(time.Millisecond)
		err = syscall.Kill(pid, 0)
		attempt++
	}
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("signaling failed: %v", err)
	}
	if attempt == 10 {
		return fmt.Errorf("signal ignored: %v", err)
	}
	return nil
}
