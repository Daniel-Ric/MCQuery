//go:build !windows

package cli

func enableVirtualTerminalOutput() func() {
	return func() {}
}
