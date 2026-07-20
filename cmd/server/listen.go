package main

import (
	"context"
	"net"
	"syscall"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
)

// listenWithRetry retries listening with exponential backoff and SO_REUSEADDR,
// useful during hot-reload when the previous process may not have released the port yet.
func listenWithRetry(addr string, maxRetries int, baseDelay time.Duration) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// Platform-specific setsockopt handled in sockopt_*.go.
				_ = setReuseAddr(fd)
			})
		},
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		listener, err := lc.Listen(context.Background(), "tcp", addr)
		if err == nil {
			return listener, nil
		}
		lastErr = err
		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<uint(i))
			if delay > 3 * time.Second {
				delay = 3 * time.Second
			}
			logger.Warnf(context.Background(), "Port %s in use, retrying in %v... (%d/%d)", addr, delay, i+1, maxRetries)
			time.Sleep(delay)
		}
	}
	return nil, lastErr
}
