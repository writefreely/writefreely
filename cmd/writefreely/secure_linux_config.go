//go:build !windows
// +build !windows

package main

func checkWindowsACL(path string) error {
    // No-op on non-Windows systems
    return nil
}
