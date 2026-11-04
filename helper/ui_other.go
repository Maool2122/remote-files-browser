//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showMessage(title, text string, isError bool) {
	out := os.Stdout
	if isError {
		out = os.Stderr
	}
	fmt.Fprintf(out, "%s\n\n%s\n", title, text)
}

// attachConsole: macOS and Linux programs always have a terminal or a pipe to print to.
func attachConsole() bool { return true }
