//go:build windows

package main

import "syscall"

// setReuseAddr enables SO_REUSEADDR for a TCP listener file descriptor.
// On Windows, syscall.SetsockoptInt expects a syscall.Handle.
func setReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
