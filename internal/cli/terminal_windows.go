//go:build windows

package cli

import (
	"syscall"
)

type terminalState struct {
	mode uint32
}

func makeRaw(fd int) (*terminalState, error) {
	handle := syscall.Handle(fd)
	var original uint32
	if err := syscall.GetConsoleMode(handle, &original); err != nil {
		return nil, err
	}

	raw := original
	raw &^= syscall.ENABLE_ECHO_INPUT
	raw &^= syscall.ENABLE_LINE_INPUT
	raw &^= syscall.ENABLE_PROCESSED_INPUT
	raw |= syscall.ENABLE_VIRTUAL_TERMINAL_INPUT

	if err := syscall.SetConsoleMode(handle, raw); err != nil {
		return nil, err
	}

	return &terminalState{mode: original}, nil
}

func restore(fd int, state *terminalState) {
	if state == nil {
		return
	}
	handle := syscall.Handle(fd)
	_ = syscall.SetConsoleMode(handle, state.mode)
}
