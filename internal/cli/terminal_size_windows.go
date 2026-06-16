//go:build windows

package cli

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

var getConsoleScreenBufferInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")

func readTerminalSize() (int, int, bool) {
	handle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return 0, 0, false
	}
	var info consoleScreenBufferInfo
	ok, _, _ := getConsoleScreenBufferInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0, 0, false
	}
	width := int(info.window.right-info.window.left) + 1
	height := int(info.window.bottom-info.window.top) + 1
	return width, height, width > 0 && height > 0
}
