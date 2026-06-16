package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type batchEntry struct {
	Name    string
	Edition ping.Edition
	Host    string
	Port    int
}

type batchRunResult struct {
	Entry   batchEntry
	Result  ping.Result
	Details ping.ExecuteDetails
	Err     error
}

type batchProgress struct {
	total     int
	completed atomic.Int64
	current   atomic.Value
}

func (a *App) executeBatch() error {
	path, err := askBatchPath()
	if err != nil {
		return err
	}
	defaultEdition, err := a.askEdition()
	if err != nil {
		return err
	}
	entries, parseErrors, err := loadBatchEntries(path, defaultEdition)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return renderTextPageAndWait("Batch check", "No valid entries found.")
	}

	var runResults []batchRunResult
	progress := &batchProgress{total: len(entries)}
	resultText, err := withControlledSpinner("Batch check", func(frame int, control *spinnerControl) string {
		return progress.Render(frame, control)
	}, 120*time.Millisecond, func(control *spinnerControl) (string, error) {
		runResults = a.runBatchEntries(control, entries, progress)
		return formatBatchResults("Batch check", runResults, parseErrors, control.IsCancelled()), nil
	})
	if err != nil {
		return err
	}

	exportText := formatBatchResults("Batch check", runResults, parseErrors, false)
	records := batchExportRecords("batch", runResults)
	if a.settings.SaveResults {
		path, err := a.saveExport("Batch check", exportText, records)
		if err != nil {
			resultText = appendWarningText(resultText, "Result export failed", err)
		} else {
			resultText += fmt.Sprintf("\nSaved result: %s", path)
		}
	}
	return renderTextPageAndWait("Batch check", resultText)
}

func askBatchPath() (string, error) {
	var errMsg string
	for {
		path, err := promptInput("Batch file", "Path to a text file. Lines: edition,host,port or host:port.", errMsg)
		if err != nil {
			return "", err
		}
		path = strings.TrimSpace(path)
		if path == "" {
			errMsg = "Path cannot be empty"
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			errMsg = friendlyErrorMessage(err)
			continue
		}
		if info.IsDir() {
			errMsg = "Path points to a directory"
			continue
		}
		return path, nil
	}
}

func loadBatchEntries(path string, defaultEdition ping.Edition) ([]batchEntry, []string, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(data), "\n")
	entries := make([]batchEntry, 0, len(lines))
	parseErrors := make([]string, 0)
	for i, line := range lines {
		entry, ok, err := parseBatchLine(line, defaultEdition)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: %v", i+1, err))
			continue
		}
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, parseErrors, nil
}

func parseBatchLine(line string, defaultEdition ping.Edition) (batchEntry, bool, error) {
	line = strings.TrimSpace(stripInlineComment(line))
	if line == "" {
		return batchEntry{}, false, nil
	}
	var parts []string
	if strings.Contains(line, ",") {
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				parts = append(parts, part)
			}
		}
	} else {
		parts = strings.Fields(line)
	}
	if len(parts) == 0 {
		return batchEntry{}, false, nil
	}

	edition := defaultEdition
	if parsed, ok := parseEdition(parts[0]); ok {
		edition = parsed
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return batchEntry{}, false, fmt.Errorf("missing host")
	}

	host := parts[0]
	port := ping.DefaultPort(edition)
	hostOnly, hostPort, hasPort := splitHostPortLoose(host)
	if hasPort {
		host = hostOnly
		port = hostPort
	}
	if len(parts) > 1 {
		parsed, err := ping.ParsePort(parts[1])
		if err != nil {
			return batchEntry{}, false, err
		}
		if parsed != 0 {
			port = parsed
		}
	}
	if strings.TrimSpace(host) == "" {
		return batchEntry{}, false, fmt.Errorf("missing host")
	}
	return batchEntry{
		Name:    host,
		Edition: edition,
		Host:    host,
		Port:    port,
	}, true, nil
}

func stripInlineComment(line string) string {
	if index := strings.Index(line, "#"); index >= 0 {
		return line[:index]
	}
	return line
}

func parseEdition(value string) (ping.Edition, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bedrock", "be":
		return ping.EditionBedrock, true
	case "java", "je":
		return ping.EditionJava, true
	default:
		return "", false
	}
}

func splitHostPortLoose(value string) (string, int, bool) {
	value = strings.TrimSpace(value)
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return value, 0, false
	}
	if strings.Count(value, ":") > 1 && !strings.Contains(value, "]") {
		return value, 0, false
	}
	port, err := ping.ParsePort(value[index+1:])
	if err != nil || port == 0 {
		return value, 0, false
	}
	host := strings.Trim(value[:index], "[]")
	return host, port, true
}

func (a *App) runBatchEntries(control *spinnerControl, entries []batchEntry, progress *batchProgress) []batchRunResult {
	results := make([]batchRunResult, len(entries))
	concurrency := resolveLookupConcurrency(a.settings.LookupConcurrency, len(entries))
	if concurrency <= 0 {
		concurrency = 1
	}

	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for index := range jobs {
			entry := entries[index]
			if progress != nil {
				progress.current.Store(fmt.Sprintf("%s:%d", entry.Host, entry.Port))
			}
			if waitWhilePaused(control) != nil {
				results[index] = batchRunResult{Entry: entry, Err: context.Canceled}
				if progress != nil {
					progress.completed.Add(1)
				}
				continue
			}
			result, details, err := ping.Execute(control.Context(), ping.ExecuteConfig{
				Edition:    entry.Edition,
				Host:       entry.Host,
				Port:       entry.Port,
				Timeout:    a.settings.RequestTimeout(),
				RetryCount: a.settings.RetryCount,
				RetryDelay: a.settings.RetryDelay(),
				EnableSRV:  a.settings.EnableSRV,
				IPMode:     a.settings.IPMode,
			})
			results[index] = batchRunResult{
				Entry:   entry,
				Result:  result,
				Details: details,
				Err:     err,
			}
			if progress != nil {
				progress.completed.Add(1)
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range entries {
		select {
		case <-control.Context().Done():
			results[i] = batchRunResult{Entry: entries[i], Err: context.Canceled}
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	for i := range results {
		if results[i].Entry.Host == "" {
			results[i] = batchRunResult{Entry: entries[i], Err: context.Canceled}
		}
	}
	return results
}

func waitWhilePaused(control *spinnerControl) error {
	for control.IsPaused() {
		select {
		case <-control.Context().Done():
			return control.Context().Err()
		case <-time.After(120 * time.Millisecond):
		}
	}
	return nil
}

func formatBatchResults(title string, results []batchRunResult, parseErrors []string, canceled bool) string {
	var builder strings.Builder
	success := 0
	for _, result := range results {
		if result.Err == nil {
			success++
		}
	}
	builder.WriteString("Summary\n")
	builder.WriteString(fmt.Sprintf("- Targets: %d\n", len(results)))
	builder.WriteString(fmt.Sprintf("- Online: %d\n", success))
	builder.WriteString(fmt.Sprintf("- Failed: %d\n", len(results)-success))
	if len(parseErrors) > 0 {
		builder.WriteString(fmt.Sprintf("- Skipped lines: %d\n", len(parseErrors)))
	}
	if canceled {
		builder.WriteString("- Status: canceled\n")
	}
	builder.WriteString("\n")

	builder.WriteString("Results\n")
	for _, result := range results {
		entry := result.Entry
		if result.Err != nil {
			builder.WriteString(fmt.Sprintf("[ERR] %s %s:%d - %s\n", entry.Edition, entry.Host, entry.Port, result.Err))
			continue
		}
		builder.WriteString(fmt.Sprintf("[OK]  %s %s:%d - %s\n", entry.Edition, entry.Host, entry.Port, compactResultStatus(result.Result)))
	}
	if len(parseErrors) > 0 {
		builder.WriteString("\nSkipped\n")
		for _, parseErr := range parseErrors {
			builder.WriteString(fmt.Sprintf("- %s\n", parseErr))
		}
	}
	_ = title
	return strings.TrimRight(builder.String(), "\n")
}

func compactResultStatus(result ping.Result) string {
	switch value := result.(type) {
	case ping.BedrockPong:
		return fmt.Sprintf("%s players %s/%s", value.GameVersion, value.CurrentPlayers, value.MaxPlayers)
	case ping.JavaStatus:
		return fmt.Sprintf("%s players %d/%d latency %dms", value.VersionName, value.CurrentPlayers, value.MaxPlayers, value.LatencyMillis)
	case nil:
		return "no response"
	default:
		return result.String()
	}
}

func batchExportRecords(mode string, results []batchRunResult) []exportRecord {
	records := make([]exportRecord, 0, len(results))
	for _, result := range results {
		records = append(records, newExportRecord(mode, result.Entry.Edition, result.Entry.Host, result.Entry.Port, result.Result, result.Details, nil, result.Err))
	}
	return records
}

func parsePortList(value string) ([]int, error) {
	parts := splitList(value)
	ports := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		values, err := parsePortToken(part)
		if err != nil {
			return nil, err
		}
		for _, port := range values {
			if _, ok := seen[port]; ok {
				continue
			}
			seen[port] = struct{}{}
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports provided")
	}
	return ports, nil
}

func parsePortToken(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, "-") {
		if strings.Count(value, "-") != 1 {
			return nil, fmt.Errorf("invalid port range")
		}
		startText, endText, _ := strings.Cut(value, "-")
		start, err := ping.ParsePort(startText)
		if err != nil || start == 0 {
			return nil, fmt.Errorf("invalid range start")
		}
		end, err := ping.ParsePort(endText)
		if err != nil || end == 0 {
			return nil, fmt.Errorf("invalid range end")
		}
		if start > end {
			return nil, fmt.Errorf("range start must be before range end")
		}
		ports := make([]int, 0, end-start+1)
		for port := start; port <= end; port++ {
			ports = append(ports, port)
		}
		return ports, nil
	}
	port, err := ping.ParsePort(value)
	if err != nil {
		return nil, err
	}
	if port == 0 {
		return nil, nil
	}
	return []int{port}, nil
}

func portListText(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}

func (p *batchProgress) Render(frame int, control *spinnerControl) string {
	status := "running"
	if control.IsPaused() {
		status = "paused"
	}
	if control.IsCancelled() {
		status = "aborting"
	}
	completed := 0
	if p != nil {
		completed = int(p.completed.Load())
	}
	total := 0
	if p != nil {
		total = p.total
	}
	percent := 0.0
	if total > 0 {
		percent = (float64(completed) / float64(total)) * 100
	}
	current := "preparing"
	if p != nil {
		if value, ok := p.current.Load().(string); ok && value != "" {
			current = value
		}
	}
	progressLine := fmt.Sprintf("Progress: %s %d/%d (%.1f%%)", renderProgressBar(completed, total, frame, progressBarWidth()), completed, total, percent)
	if terminalWidth() < 54 {
		progressLine = fmt.Sprintf("Progress: %s %.1f%%", renderProgressBar(completed, total, frame, progressBarWidth()), percent)
	}
	return fmt.Sprintf("Status: %s\n%s\nTarget: %s", status, progressLine, current)
}
