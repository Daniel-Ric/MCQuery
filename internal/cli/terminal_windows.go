//go:build windows

package cli

import (
	"golang.org/x/sys/windows"
)

type terminalState struct {
	mode uint32
}

func makeRaw(fd int) (*terminalState, error) {
	handle := windows.Handle(fd)
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
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

	return &terminalState{mode: original}, nil
}

func restore(fd int, state *terminalState) {
	if state == nil {
		return
	}
	handle := windows.Handle(fd)
	_ = windows.SetConsoleMode(handle, state.mode)
}
