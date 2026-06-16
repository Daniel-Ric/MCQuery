package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

func (a *App) recoverPanic(errp *error) {
	if recovered := recover(); recovered != nil {
		err := internalPanicError(recovered)
		_ = a.showErrorPage("Unexpected error", err)
		*errp = nil
	}
}

func internalPanicError(value any) error {
	return fmt.Errorf("unexpected internal error: %v", value)
}

func (a *App) showStartupWarnings() bool {
	for _, warning := range a.startupWarnings {
		if !a.reportError("Startup warning", warning) {
			return false
		}
	}
	return true
}

func (a *App) reportError(title string, err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, errAborted) {
		return false
	}
	if waitErr := a.showErrorPage(title, err); waitErr != nil {
		return false
	}
	return true
}

func (a *App) showErrorPage(title string, err error) error {
	renderTextPage(title, formatErrorPage(err))
	return waitForEnter()
}

func formatErrorPage(err error) string {
	var builder strings.Builder
	builder.WriteString("Summary\n")
	builder.WriteString("- Status: failed\n")
	builder.WriteString(fmt.Sprintf("- Error: %s\n", friendlyErrorMessage(err)))

	if detail := errorDetail(err); detail != "" {
		builder.WriteString("\nDetails\n")
		builder.WriteString(detail)
	}
	return builder.String()
}

func friendlyErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "Operation canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "Operation timed out"
	case os.IsNotExist(err):
		return "File or path was not found"
	case os.IsPermission(err):
		return "Permission denied"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return "Host could not be resolved"
		}
		if dnsErr.Timeout() {
			return "DNS lookup timed out"
		}
		return "DNS lookup failed"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "Network request timed out"
	}

	text := strings.TrimSpace(err.Error())
	if strings.Contains(strings.ToLower(text), "timeout") {
		return "Network request timed out"
	}
	if text == "" {
		return "Unknown error"
	}
	return text
}

func errorDetail(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	friendly := friendlyErrorMessage(err)
	if message == "" || message == friendly {
		return ""
	}
	return fmt.Sprintf("- Raw error: %s\n", message)
}

func appendWarningText(text, label string, err error) string {
	if err == nil {
		return text
	}
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n"
	}
	return fmt.Sprintf("%s[WARN] %s: %s", text, label, friendlyErrorMessage(err))
}

func actionErrorTitle(mode Mode) string {
	switch mode {
	case ModeLookup:
		return "Lookup failed"
	case ModeFavorites:
		return "Favorites failed"
	case ModeBatch:
		return "Batch check failed"
	case ModePortScan:
		return "Port scan failed"
	case ModeSettings:
		return "Settings failed"
	case ModeUpdate:
		return "Update check failed"
	default:
		return "Query failed"
	}
}
