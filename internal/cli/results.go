package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
)

func formatDirectResult(result ping.Result, details ping.ExecuteDetails, verbose bool) string {
	if !verbose {
		return result.String()
	}
	var builder strings.Builder
	builder.WriteString(result.String())
	builder.WriteString("\n")
	builder.WriteString("Debug\n")
	builder.WriteString(fmt.Sprintf("Requested: %s:%d\n", details.RequestedHost, details.RequestedPort))
	if details.DialHost != "" {
		builder.WriteString(fmt.Sprintf("Dial target: %s:%d\n", details.DialHost, details.DialPort))
	}
	if details.SelectedIP != "" {
		builder.WriteString(fmt.Sprintf("Selected IP: %s\n", details.SelectedIP))
	}
	if len(details.ResolvedIPs) > 0 {
		builder.WriteString(fmt.Sprintf("Resolved IPs: %s\n", strings.Join(details.ResolvedIPs, ", ")))
	}
	if details.SRVUsed {
		builder.WriteString(fmt.Sprintf("SRV: %s:%d\n", details.SRVHost, details.SRVPort))
	} else if details.SRVError != "" {
		builder.WriteString(fmt.Sprintf("SRV error: %s\n", details.SRVError))
	}
	if details.Attempts > 0 {
		builder.WriteString(fmt.Sprintf("Attempts: %d\n", details.Attempts))
	}
	if details.LastError != "" {
		builder.WriteString(fmt.Sprintf("Last error: %s\n", details.LastError))
	}
	return builder.String()
}

func (a *App) saveResult(title, content string) error {
	path, err := ensureResultsPath(a.settings.ResultsPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s\n", title))
	builder.WriteString(fmt.Sprintf("Saved at: %s\n", time.Now().Format(time.RFC3339)))
	builder.WriteString("\n")
	builder.WriteString(content)
	builder.WriteString("\n")
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}
