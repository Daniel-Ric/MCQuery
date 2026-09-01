//go:build windows

package cli

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestVirtualTerminalOutputModeEnablesANSIProcessing(t *testing.T) {
	mode := virtualTerminalOutputMode(0)
	for _, required := range []uint32{
		windows.ENABLE_PROCESSED_OUTPUT,
		windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING,
	} {
		if mode&required == 0 {
			t.Fatalf("console output mode %#x is missing required flag %#x", mode, required)
		}
	}
}
