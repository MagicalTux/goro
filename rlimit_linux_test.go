//go:build linux

package main_test

import "syscall"

// setMemoryLimit sets RLIMIT_AS (virtual memory limit) to prevent OOM-killing
// the host when a test causes runaway memory allocation.
func setMemoryLimit(bytes uint64) {
	var rLimit syscall.Rlimit
	rLimit.Cur = bytes
	rLimit.Max = bytes
	_ = syscall.Setrlimit(syscall.RLIMIT_AS, &rLimit)
}
