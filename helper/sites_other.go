//go:build !windows

package main

// WinSCP exists only on Windows, so there is no registry to read anywhere else.
func readWinSCPRegistry(string) ([]winscpSession, bool, error) {
	return nil, false, nil
}
