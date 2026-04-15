//go:build windows

package main

import "os"

func openDevTTY() (*os.File, error) {
	return nil, os.ErrNotExist
}
