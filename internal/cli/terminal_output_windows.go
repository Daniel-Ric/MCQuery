//go:build windows

package cli

import "golang.org/x/sys/windows"

func virtualTerminalOutputMode(mode uint32) uint32 {
	return mode | windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
}

func enableVirtualTerminalOutput() func() {
	handle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || handle == 0 || handle == windows.InvalidHandle {
		return func() {}
	}

	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		// Pseudo terminals such as JetBrains handle VT sequences themselves and
		// often expose stdout as a pipe instead of a native console handle.
		return func() {}
	}

	mode := virtualTerminalOutputMode(original)
	if mode == original {
		return func() {}
	}
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		return func() {}
	}

	return func() {
		_ = windows.SetConsoleMode(handle, original)
	}
}
