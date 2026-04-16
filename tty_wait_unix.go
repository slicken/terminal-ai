//go:build !windows

package main

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// ttyReadAfterPoll waits up to d for more bytes on the tty, then reads into buf.
// On timeout with no input, returns (0, os.ErrDeadlineExceeded).
// SetReadDeadline+Read is unreliable on some PTY/tty setups; poll(2) is not.
func ttyReadAfterPoll(tty interface {
	Fd() uintptr
	Read([]byte) (int, error)
}, buf []byte, d time.Duration) (n int, err error) {
	fd := int(tty.Fd())
	ms := int(d / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	fds := []unix.PollFd{{
		Fd:     int32(fd),
		Events: unix.POLLIN,
	}}
	for {
		nfd, err := unix.Poll(fds, ms)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		if nfd == 0 {
			return 0, os.ErrDeadlineExceeded
		}
		re := fds[0].Revents
		if re&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			return tty.Read(buf)
		}
		return 0, os.ErrDeadlineExceeded
	}
}
