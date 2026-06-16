//go:build !windows

package cli

func terminalSupportsKeyPolling() bool {
	return false
}

func readPendingByte(fd int) (byte, bool, error) {
	return 0, false, nil
}
