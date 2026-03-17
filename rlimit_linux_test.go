//go:build linux

package main_test

import (
	"fmt"
	"os"
	"syscall"
)

// setMemoryLimit sets RLIMIT_AS (virtual memory limit) to prevent OOM-killing
// the host when a test causes runaway memory allocation.
func setMemoryLimit(bytes uint64) {
	var rLimit syscall.Rlimit
	rLimit.Cur = bytes
	rLimit.Max = bytes
	err := syscall.Setrlimit(syscall.RLIMIT_AS, &rLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to set RLIMIT_AS: %v\n", err)
	}
	// Verify it took effect
	var check syscall.Rlimit
	syscall.Getrlimit(syscall.RLIMIT_AS, &check)
	if check.Cur != bytes {
		fmt.Fprintf(os.Stderr, "Warning: RLIMIT_AS is %d, wanted %d\n", check.Cur, bytes)
	}
}
