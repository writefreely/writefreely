//go:build windows
// +build windows

package main

import (
	"fmt"
	stdlog "log"
	"github.com/capnspacehook/go-acl"
	"github.com/gorilla/mux"
)

func checkWindowsACL(path string) error {
	if runtime.GOOS == "windows" {
		if err := acl.Chmod(path, 0600); err != nil {
			return fmt.Errorf("failed to set ACL on %s: %w", path, err)
		}
	}
	return nil
}