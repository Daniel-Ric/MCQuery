//go:build windows

package cli

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getNumberOfConsoleInputEvents = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetNumberOfConsoleInputEvents")

func terminalSupportsKeyPolling() bool {
	return true
}

func readPendingByte(fd int) (byte, bool, error) {
	handle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return 0, false, err
	}
	var events uint32
	ok, _, _ := getNumberOfConsoleInputEvents.Call(uintptr(handle), uintptr(unsafe.Pointer(&events)))
	if ok == 0 {
		return 0, false, nil
	}
	if events == 0 {
		return 0, false, nil
	}

	var buf [1]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return 0, false, err
	}
	if n == 0 {
		return 0, false, nil
	}
	return buf[0], true, nil
}
