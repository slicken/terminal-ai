//go:build windows

package main

import (
	"time"
)

// ttyReadAfterPoll waits up to d for more bytes (Windows: read deadline).
func ttyReadAfterPoll(tty interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}, buf []byte, d time.Duration) (n int, err error) {
	if err := tty.SetReadDeadline(time.Now().Add(d)); err != nil {
		return 0, err
	}
	defer func() { _ = tty.SetReadDeadline(time.Time{}) }()
	return tty.Read(buf)
}
