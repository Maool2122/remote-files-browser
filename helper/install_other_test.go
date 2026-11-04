//go:build !windows

package main

import "testing"

// useTestRegistry sends the registration files into a folder that only this test uses.
func useTestRegistry(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home+"/.config")
}
