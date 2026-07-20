//go:build !windows

package main

import "syscall"

// setReuseAddr enables SO_REUSEADDR for a TCP listener file descriptor.
// On Unix-like systems, syscall.SetsockoptInt expects an int fd.
func setReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
