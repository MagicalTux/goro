//go:build !linux

package main_test

// setMemoryLimit is a no-op on non-Linux platforms.
func setMemoryLimit(_ uint64) {}
