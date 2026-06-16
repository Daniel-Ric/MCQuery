package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"UWP-TCP-Con/internal/ping"
	"UWP-TCP-Con/internal/web"
)

type App struct {
	settings        Settings
	linkServer      *web.LinkServer
	startupWarnings []error
}

func NewApp() *App {
	settings, err := loadSettings()
	var warnings []error
	if err != nil {
		warnings = append(warnings, fmt.Errorf("settings could not be loaded: %w", err))
		settings = defaultSettings()
	} else if err := settings.Validate(); err != nil {
		warnings = append(warnings, fmt.Errorf("settings were invalid and have been reset: %w", err))
		settings = defaultSettings()
	}
	return &App{
		settings:        settings,
		startupWarnings: warnings,
	}
}

func (a *App) Run() (err error) {
	defer a.recoverPanic(&err)

	if !a.showStartupWarnings() {
		return nil
	}
	if a.settings.CheckForUpdates {
		_ = a.showStartupUpdateNotice()
	}
	for {
		config, err := a.collectConfig()
		if err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			if !a.reportError("Setup failed", err) {
				return nil
			}
			continue
		}

		if err := a.execute(config); err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			if !a.reportError(actionErrorTitle(config.Mode), err) {
				return nil
			}
			continue
		}

		again, err := a.askAgain()
		if err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			if !a.reportError("Navigation failed", err) {
				return nil
			}
			continue
		}
		if !again {
			return nil
		}
	}
}

type Config struct {
	Mode   Mode
	Direct DirectConfig
	Lookup LookupConfig
}

func (a *App) collectConfig() (Config, error) {
	mode, err := a.askMode()
	if err != nil {
		return Config{}, err
	}

	if mode == ModeLookup {
		lookup, err := a.collectLookupConfig()
		if err != nil {
			return Config{}, err
		}
		return Config{Mode: mode, Lookup: lookup}, nil
	}
	if mode == ModeSettings || mode == ModeFavorites || mode == ModeBatch || mode == ModePortScan || mode == ModeUpdate || mode == ModeExit {
		return Config{Mode: mode}, nil
	}

	direct, err := a.collectDirectConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{Mode: mode, Direct: direct}, nil
}

type Mode string

const (
	ModeDirect    Mode = "direct"
	ModeLookup    Mode = "lookup"
	ModeFavorites Mode = "favorites"
	ModeBatch     Mode = "batch"
	ModePortScan  Mode = "port_scan"
	ModeSettings  Mode = "settings"
	ModeUpdate    Mode = "update"
	ModeExit      Mode = "exit"
)

type DirectConfig struct {
	Host    string
	Port    int
	Edition ping.Edition
}

type LookupConfig struct {
	Edition    ping.Edition
	BaseHost   string
	Port       int
	Ports      []int
	Subdomains []string
	Endings    []string
	Sort       lookupSort
	Filter     lookupFilter
}

func (a *App) collectDirectConfig() (DirectConfig, error) {
	edition, err := a.askEdition()
	if err != nil {
		return DirectConfig{}, err
	}

	host, err := a.askHost()
	if err != nil {
		return DirectConfig{}, err
	}

	port, err := a.askPort(edition)
	if err != nil {
		return DirectConfig{}, err
	}

	return DirectConfig{Host: host, Port: port, Edition: edition}, nil
}

func (a *App) collectLookupConfig() (LookupConfig, error) {
	edition, err := a.askEdition()
	if err != nil {
		return LookupConfig{}, err
	}

	subdomains, err := a.askSubdomainChoice()
	if err != nil {
		return LookupConfig{}, err
	}

	baseHost, err := a.askBaseHost()
	if err != nil {
		return LookupConfig{}, err
	}

	endings, err := a.askDomainEndings()
	if err != nil {
		return LookupConfig{}, err
	}

	port, ports, err := a.askLookupPorts(edition)
	if err != nil {
		return LookupConfig{}, err
	}
	sortMode, err := a.askLookupSort()
	if err != nil {
		return LookupConfig{}, err
	}
	filterMode, err := a.askLookupFilter()
	if err != nil {
		return LookupConfig{}, err
	}

	return LookupConfig{
		Edition:    edition,
		BaseHost:   baseHost,
		Port:       port,
		Ports:      ports,
		Subdomains: subdomains,
		Endings:    endings,
		Sort:       sortMode,
		Filter:     filterMode,
	}, nil
}

func (a *App) askMode() (Mode, error) {
	index, err := selectOption("MCQuery", []string{
		"Direct query: Check one server",
		"Favorites: Saved server profiles",
		"Batch check: Run a target file",
		"Port scan: Probe common ports",
		"IP/domain lookup: Sweep domains and subdomains",
		"Settings: Network, output and presets",
		"Update check: Compare with GitHub",
		"Exit",
	})
	if err != nil {
		return "", err
	}
	switch index {
	case 1:
		return ModeFavorites, nil
	case 2:
		return ModeBatch, nil
	case 3:
		return ModePortScan, nil
	case 4:
		return ModeLookup, nil
	case 5:
		return ModeSettings, nil
	case 6:
		return ModeUpdate, nil
	case 7:
		return ModeExit, nil
	default:
		return ModeDirect, nil
	}
}

func (a *App) askEdition() (ping.Edition, error) {
	index, err := selectOption("Edition", []string{
		"Bedrock: UDP server list ping",
		"Java: TCP status ping",
	})
	if err != nil {
		return "", err
	}
	if index == 1 {
		return ping.EditionJava, nil
	}
	return ping.EditionBedrock, nil
}

func (a *App) askHost() (string, error) {
	var errMsg string
	for {
		value, err := promptInput("Server host", "e.g. play.example.com", errMsg)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) == "" {
			errMsg = "Host cannot be empty"
			continue
		}
		return value, nil
	}
}

func (a *App) askBaseHost() (string, error) {
	var errMsg string
	for {
		value, err := promptInput("IP/domain without ending", "e.g. example", errMsg)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			errMsg = "Value cannot be empty"
			continue
		}
		return value, nil
	}
}

func (a *App) askSubdomainChoice() ([]string, error) {
	presets, _ := loadLookupPresets()
	presetSubdomains := presets.Subdomains
	index, err := selectOption("Subdomains", []string{
		"Custom: Enter subdomains now",
		"Built-in pool: Common server names",
		"Saved presets: Your stored names",
		"Custom + pool: Manual plus built-in",
		"Custom + presets: Manual plus saved",
		"Pool + presets: Built-in plus saved",
		"Everything: Custom, built-in and saved",
	})
	if err != nil {
		return nil, err
	}
	switch index {
	case 1:
		return subdomainPool, nil
	case 2:
		if len(presetSubdomains) == 0 {
			return []string{""}, nil
		}
		return presetSubdomains, nil
	case 3:
		custom, err := a.askCustomSubdomains()
		if err != nil {
			return nil, err
		}
		return append(custom, subdomainPool...), nil
	case 4:
		custom, err := a.askCustomSubdomains()
		if err != nil {
			return nil, err
		}
		return append(custom, presetSubdomains...), nil
	case 5:
		return append(subdomainPool, presetSubdomains...), nil
	case 6:
		custom, err := a.askCustomSubdomains()
		if err != nil {
			return nil, err
		}
		values := append(custom, subdomainPool...)
		return append(values, presetSubdomains...), nil
	default:
		return a.askCustomSubdomains()
	}
}

func (a *App) askCustomSubdomains() ([]string, error) {
	var errMsg string
	for {
		value, err := promptInput("Subdomains (optional, comma-separated)", "e.g. play, mc (leave empty for none)", errMsg)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return []string{""}, nil
		}
		list := splitList(value)
		if len(list) == 0 {
			errMsg = "Subdomain cannot be empty"
			continue
		}
		return list, nil
	}
}

func (a *App) askDomainEndings() ([]string, error) {
	presets, _ := loadLookupPresets()
	presetEndings := presets.Endings
	index, err := selectOption("Domain endings", []string{
		"Custom: Enter endings now",
		"Built-in pool: Common TLDs",
		"Saved presets: Your stored endings",
		"Custom + pool: Manual plus built-in",
		"Custom + presets: Manual plus saved",
		"Pool + presets: Built-in plus saved",
		"Everything: Custom, built-in and saved",
	})
	if err != nil {
		return nil, err
	}
	switch index {
	case 1:
		endings, err := loadDomainEndings()
		if err != nil {
			return endings, nil
		}
		return endings, nil
	case 2:
		return presetEndings, nil
	case 3:
		custom, err := a.askCustomEndings()
		if err != nil {
			return nil, err
		}
		endings, err := loadDomainEndings()
		if err != nil {
			return append(custom, endings...), nil
		}
		return append(custom, endings...), nil
	case 4:
		custom, err := a.askCustomEndings()
		if err != nil {
			return nil, err
		}
		return append(custom, presetEndings...), nil
	case 5:
		endings, err := loadDomainEndings()
		if err != nil {
			return append(endings, presetEndings...), nil
		}
		return append(endings, presetEndings...), nil
	case 6:
		custom, err := a.askCustomEndings()
		if err != nil {
			return nil, err
		}
		endings, err := loadDomainEndings()
		if err != nil {
			values := append(custom, endings...)
			return append(values, presetEndings...), nil
		}
		values := append(custom, endings...)
		return append(values, presetEndings...), nil
	default:
		return a.askCustomEndings()
	}
}

func (a *App) askCustomEndings() ([]string, error) {
	var errMsg string
	for {
		value, err := promptInput("Domain endings (comma-separated)", "e.g. com, net", errMsg)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			errMsg = "Ending cannot be empty"
			continue
		}
		rawList := splitList(value)
		list := make([]string, 0, len(rawList))
		for _, entry := range rawList {
			normalized := normalizeEnding(entry)
			if normalized == "" {
				continue
			}
			list = append(list, normalized)
		}
		if len(list) == 0 {
			errMsg = "Ending cannot be empty"
			continue
		}
		return list, nil
	}
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	list := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		list = append(list, part)
	}
	return list
}

func (a *App) askPort(edition ping.Edition) (int, error) {
	defaultPort := ping.DefaultPort(edition)
	var errMsg string
	for {
		value, err := promptInput(fmt.Sprintf("Port (%d)", defaultPort), "Leave empty for the default port", errMsg)
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return defaultPort, nil
		}
		port, err := ping.ParsePort(value)
		if err != nil {
			errMsg = err.Error()
			continue
		}
		if port == 0 {
			errMsg = "Port cannot be empty"
			continue
		}
		return port, nil
	}
}

func (a *App) askLookupPorts(edition ping.Edition) (int, []int, error) {
	if edition != ping.EditionBedrock {
		port, err := a.askPort(edition)
		return port, nil, err
	}
	index, err := selectOption("Lookup ports", []string{
		"Single port: Default or custom",
		"Custom range: Set start and end",
		"Full UDP range: 1-65535, slow",
	})
	if err != nil {
		return 0, nil, err
	}
	if index == 0 {
		port, err := a.askPort(edition)
		return port, nil, err
	}
	if index == 1 {
		ports, err := a.askLookupPortRange(edition)
		return ping.DefaultPort(edition), ports, err
	}

	confirmed, err := askConfirm("Scan all UDP ports? This can take a long time.")
	if err != nil {
		return 0, nil, err
	}
	if !confirmed {
		port, err := a.askPort(edition)
		return port, nil, err
	}
	ports := fullPortRange()
	return ping.DefaultPort(edition), ports, nil
}

func (a *App) askLookupPortRange(edition ping.Edition) ([]int, error) {
	defaultPort := ping.DefaultPort(edition)
	var errMsg string
	for {
		value, err := promptInput("Lookup port range", fmt.Sprintf("Use start-end, e.g. %d-%d. Commas also work.", defaultPort, defaultPort+100), errMsg)
		if err != nil {
			return nil, err
		}
		ports, err := parsePortList(value)
		if err != nil {
			errMsg = err.Error()
			continue
		}
		if len(ports) < 2 {
			errMsg = "Enter at least two ports or use Single port"
			continue
		}
		if len(ports) > 10000 {
			confirmed, err := askConfirm(fmt.Sprintf("Scan %d ports per host?", len(ports)))
			if err != nil {
				return nil, err
			}
			if !confirmed {
				errMsg = "Enter a smaller range"
				continue
			}
		}
		return ports, nil
	}
}

func fullPortRange() []int {
	ports := make([]int, 65535)
	for i := range ports {
		ports[i] = i + 1
	}
	return ports
}

func (a *App) execute(config Config) error {
	switch config.Mode {
	case ModeLookup:
		return a.executeLookup(config.Lookup)
	case ModeFavorites:
		return a.manageFavorites()
	case ModeBatch:
		return a.executeBatch()
	case ModePortScan:
		return a.executePortScan()
	case ModeSettings:
		return a.manageSettings()
	case ModeUpdate:
		return a.executeUpdateCheck()
	case ModeExit:
		return errAborted
	default:
		return a.executeDirect(config.Direct)
	}
}

func (a *App) executeDirect(config DirectConfig) error {
	resultText, err := withSpinner("Query", func(frame int) string {
		_ = frame
		return "Querying server"
	}, 120*time.Millisecond, func() (string, error) {
		result, details, err := ping.Execute(context.Background(), ping.ExecuteConfig{
			Edition:    config.Edition,
			Host:       config.Host,
			Port:       config.Port,
			Timeout:    a.settings.RequestTimeout(),
			RetryCount: a.settings.RetryCount,
			RetryDelay: a.settings.RetryDelay(),
			EnableSRV:  a.settings.EnableSRV,
			IPMode:     a.settings.IPMode,
		})
		if err != nil {
			return "", err
		}
		var link *web.LookupLinkURLs
		var linkErr error
		if config.Edition == ping.EditionBedrock {
			links, err := a.startBedrockLinks([]web.LookupLink{{
				Name: config.Host,
				Host: config.Host,
				Port: config.Port,
			}})
			if err != nil {
				linkErr = err
			} else if len(links) > 0 {
				link = &links[0]
			}
		}

		displayOptions := resultFormatOptions{Verbose: a.settings.Verbose, ColorMOTD: a.settings.ColorMOTD}
		exportOptions := resultFormatOptions{Verbose: a.settings.Verbose, ColorMOTD: false}
		displayText := formatDirectResult(result, details, displayOptions)
		exportText := formatDirectResult(result, details, exportOptions)
		if link != nil {
			displayText = appendLinkText(displayText, *link)
			exportText = appendLinkText(exportText, *link)
		} else if linkErr != nil {
			displayText = appendWarningText(displayText, "Bedrock browser links unavailable", linkErr)
		}

		record := newExportRecord("direct", config.Edition, config.Host, config.Port, result, details, link, nil)
		if a.settings.SaveResults && a.settings.SaveJavaIcons {
			if status, ok := result.(ping.JavaStatus); ok && len(status.IconPNG) > 0 {
				path, err := a.saveJavaIcon(config.Host, status)
				if err != nil {
					displayText = appendWarningText(displayText, "Server icon could not be saved", err)
				} else if path != "" {
					record.JavaIconSavedTo = path
					displayText += fmt.Sprintf("\nServer icon saved: %s", path)
					exportText += fmt.Sprintf("\nServer icon saved: %s", path)
				}
			}
		}
		if a.settings.SaveResults {
			path, err := a.saveExport("Direct query", exportText, []exportRecord{record})
			if err != nil {
				displayText = appendWarningText(displayText, "Result export failed", err)
			} else {
				displayText += fmt.Sprintf("\nSaved result: %s", path)
			}
		}
		return displayText, nil
	})
	if err != nil {
		return err
	}

	return renderTextPageAndWait("Result", resultText)
}

func (a *App) executeLookup(config LookupConfig) error {
	progressView := newLookupProgressView(a.settings, config)
	startedAt := time.Now()

	resultText, err := withControlledSpinner("IP lookup", func(frame int, control *spinnerControl) string {
		status := progressView.Render(frame)
		if control.IsPaused() {
			status = "Status: paused\n" + status
		}
		if control.IsCancelled() {
			status = "Status: aborting\n" + status
		}
		return status
	}, 120*time.Millisecond, func(control *spinnerControl) (string, error) {
		result, lookupErr := ping.LookupDomains(control.Context(), ping.LookupConfig{
			Edition:       config.Edition,
			Port:          config.Port,
			Ports:         config.Ports,
			BaseHost:      config.BaseHost,
			Subdomains:    config.Subdomains,
			DomainEndings: config.Endings,
			Concurrency:   a.settings.LookupConcurrency,
			RateLimit:     a.settings.LookupRateLimit,
			Options: ping.ExecuteOptions{
				Timeout:    a.settings.RequestTimeout(),
				RetryCount: a.settings.RetryCount,
				RetryDelay: a.settings.RetryDelay(),
				EnableSRV:  a.settings.EnableSRV,
				IPMode:     a.settings.IPMode,
			},
			Progress: func(progress ping.LookupProgress) {
				progressView.Observe(progress)
			},
			Paused: control.IsPaused,
		})
		if lookupErr != nil && !errors.Is(lookupErr, context.Canceled) {
			return "", lookupErr
		}
		result.Matches = applyLookupView(result.Matches, config.Sort, config.Filter)
		var links []web.LookupLinkURLs
		var linkErr error
		if config.Edition == ping.EditionBedrock && len(result.Matches) > 0 {
			entries := make([]web.LookupLink, 0, len(result.Matches))
			for _, match := range result.Matches {
				entries = append(entries, web.LookupLink{
					Name: match.Host,
					Host: match.Host,
					Port: match.Port,
				})
			}
			var err error
			links, err = a.startBedrockLinks(entries)
			if err != nil {
				linkErr = err
			}
		}
		metrics := lookupMetrics{
			BaseHost:      config.BaseHost,
			Subdomains:    countLookupSubdomains(config.Subdomains),
			Endings:       countLookupEndings(config.Endings),
			Ports:         countLookupPorts(config),
			Duration:      time.Since(startedAt),
			AverageRate:   calculateLookupAverageRate(result.Completed, startedAt),
			Concurrency:   resolveLookupConcurrency(a.settings.LookupConcurrency, result.Attempts),
			RateLimit:     a.settings.LookupRateLimit,
			CompletionPct: calculateLookupCompletion(result.Completed, result.Attempts),
			Sort:          lookupSortLabel(config.Sort),
			Filter:        lookupFilterLabel(config.Filter),
			Canceled:      errors.Is(lookupErr, context.Canceled) || control.IsCancelled(),
		}
		displayOptions := resultFormatOptions{Verbose: a.settings.Verbose, ColorMOTD: a.settings.ColorMOTD}
		exportOptions := resultFormatOptions{Verbose: a.settings.Verbose, ColorMOTD: false}
		displayText := formatLookupResult(result, links, metrics, displayOptions)
		exportText := formatLookupResult(result, links, metrics, exportOptions)
		records := make([]exportRecord, 0, len(result.Matches))
		var iconErr error
		for i, match := range result.Matches {
			var link *web.LookupLinkURLs
			if i < len(links) {
				link = &links[i]
			}
			record := newExportRecord("lookup", config.Edition, match.Host, match.Port, match.Result, match.Detail, link, nil)
			if a.settings.SaveResults && a.settings.SaveJavaIcons {
				if status, ok := match.Result.(ping.JavaStatus); ok && len(status.IconPNG) > 0 {
					path, err := a.saveJavaIcon(match.Host, status)
					if err != nil {
						if iconErr == nil {
							iconErr = err
						}
					} else {
						record.JavaIconSavedTo = path
					}
				}
			}
			records = append(records, record)
		}
		if linkErr != nil {
			displayText = appendWarningText(displayText, "Bedrock browser links unavailable", linkErr)
		}
		if iconErr != nil {
			displayText = appendWarningText(displayText, "One or more server icons could not be saved", iconErr)
		}
		if a.settings.SaveResults {
			path, err := a.saveExport("Lookup", exportText, records)
			if err != nil {
				displayText = appendWarningText(displayText, "Result export failed", err)
			} else {
				displayText += fmt.Sprintf("\nSaved result: %s", path)
			}
		}
		return displayText, nil
	})
	if err != nil {
		return err
	}

	return renderTextPageAndWait("Result", resultText)
}

func (a *App) askAgain() (bool, error) {
	index, err := selectOption("Next step", []string{"Main menu", "Exit"})
	if err != nil {
		return false, err
	}
	return index == 0, nil
}

type lookupMetrics struct {
	BaseHost      string
	Subdomains    int
	Endings       int
	Ports         int
	Duration      time.Duration
	AverageRate   float64
	Concurrency   int
	RateLimit     int
	CompletionPct float64
	Sort          string
	Filter        string
	Canceled      bool
}

func countLookupSubdomains(values []string) int {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			value = "(none)"
		}
		seen[value] = struct{}{}
	}
	return len(seen)
}

func countLookupEndings(values []string) int {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		value = strings.TrimPrefix(value, ".")
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	return len(seen)
}

func countLookupPorts(config LookupConfig) int {
	if len(config.Ports) > 0 {
		return len(config.Ports)
	}
	if config.Port > 0 {
		return 1
	}
	return 0
}

func formatLookupPortCount(value int) string {
	if value == 65535 {
		return "65535 (full UDP range)"
	}
	return fmt.Sprintf("%d", value)
}

func formatLookupResult(result ping.LookupResult, links []web.LookupLinkURLs, metrics lookupMetrics, options resultFormatOptions) string {
	var builder strings.Builder
	builder.WriteString("Summary\n")
	builder.WriteString(fmt.Sprintf("- Base host: %s\n", metrics.BaseHost))
	builder.WriteString(fmt.Sprintf("- Subdomains: %d\n", metrics.Subdomains))
	builder.WriteString(fmt.Sprintf("- Domain endings: %d\n", metrics.Endings))
	builder.WriteString(fmt.Sprintf("- Ports: %s\n", formatLookupPortCount(metrics.Ports)))
	builder.WriteString(fmt.Sprintf("- Checked combinations: %d/%d\n", result.Completed, result.Attempts))
	builder.WriteString(fmt.Sprintf("- Completion: %.1f%%\n", metrics.CompletionPct))
	builder.WriteString(fmt.Sprintf("- Matches after filter: %d\n", len(result.Matches)))
	builder.WriteString(fmt.Sprintf("- Sort: %s\n", metrics.Sort))
	builder.WriteString(fmt.Sprintf("- Filter: %s\n", metrics.Filter))
	builder.WriteString(fmt.Sprintf("- Elapsed: %s\n", formatLookupDuration(metrics.Duration)))
	builder.WriteString(fmt.Sprintf("- Average throughput: %s\n", formatLookupRate(metrics.AverageRate)))
	builder.WriteString(fmt.Sprintf("- Pipeline: %d workers\n", metrics.Concurrency))
	builder.WriteString(fmt.Sprintf("- Rate cap: %s\n", formatLookupRateCap(metrics.RateLimit)))
	if metrics.Canceled {
		builder.WriteString("- Status: canceled\n")
	}
	builder.WriteString("\n")

	if len(result.Matches) == 0 {
		builder.WriteString("No matching servers found.")
		return builder.String()
	}

	builder.WriteString("Matches\n")
	for i, match := range result.Matches {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("Match %d\n", i+1))
		builder.WriteString(fmt.Sprintf("Host: %s\n", match.Host))
		builder.WriteString(fmt.Sprintf("Port: %d\n", match.Port))
		if i < len(links) {
			builder.WriteString(fmt.Sprintf("Add link (browser): %s\n", links[i].AddURL))
			builder.WriteString(fmt.Sprintf("Join link (browser): %s\n", links[i].ConnectURL))
		}
		builder.WriteString(formatDirectResult(match.Result, match.Detail, options))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (a *App) startBedrockLinks(entries []web.LookupLink) ([]web.LookupLinkURLs, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	if a.linkServer != nil {
		_ = a.linkServer.Close()
	}
	server, err := web.StartLookupLinkServer(entries, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	a.linkServer = server
	return server.Links(), nil
}

func appendLinkText(text string, link web.LookupLinkURLs) string {
	return fmt.Sprintf("%s\nAdd link (browser): %s\nJoin link (browser): %s", text, link.AddURL, link.ConnectURL)
}
