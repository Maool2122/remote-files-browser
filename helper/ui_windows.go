//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// The Windows build has no console window of its own (it is built with -H=windowsgui), so a double-click shows a message
// box instead, and nothing flashes up when Chrome starts the helper.

func showMessage(title, text string, isError bool) {
	flags := uint32(windows.MB_OK | windows.MB_ICONINFORMATION)
	if isError {
		flags = windows.MB_OK | windows.MB_ICONERROR
	}
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, m, t, flags)
}

// attachConsole lets the commands print into the terminal they were typed in. It says whether what is printed reaches
// anyone: true when the output is redirected already (to a file or a pipe) or the terminal was joined, false when the
// program was started without any terminal (the Run window, a shortcut, the Explorer): the result then has to be shown
// in a message box, or the person would see nothing at all.
func attachConsole() bool {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != windows.InvalidHandle {
		return true
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0)
	if ok, _, _ := proc.Call(attachParentProcess); ok == 0 {
		return false
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	os.Stdout = out
	os.Stderr = out
	return true
}
