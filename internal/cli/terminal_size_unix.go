//go:build !windows

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

type terminalWindowSize struct {
	rows    uint16
	columns uint16
	xPixels uint16
	yPixels uint16
}

func readTerminalSize() (int, int, bool) {
	var size terminalWindowSize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, 0, false
	}
	width := int(size.columns)
	height := int(size.rows)
	return width, height, width > 0 && height > 0
}
