//go:build windows

package cli

import (
	"errors"

	"golang.org/x/sys/windows"
)

type terminalState struct {
	handle windows.Handle
	mode   uint32
}

func makeRaw(fd int) (*terminalState, error) {
	handle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return nil, err
	}
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		if errors.Is(err, windows.ERROR_INVALID_HANDLE) {
			return nil, nil
		}
		return nil, err
	}

	raw := original
	raw &^= windows.ENABLE_ECHO_INPUT
	raw &^= windows.ENABLE_LINE_INPUT
	raw &^= windows.ENABLE_PROCESSED_INPUT
	raw |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	if err := windows.SetConsoleMode(handle, raw); err != nil {
		return nil, err
	}

	return &terminalState{handle: handle, mode: original}, nil
}

func restore(fd int, state *terminalState) {
	if state == nil {
		return
	}
	_ = windows.SetConsoleMode(state.handle, state.mode)
}
