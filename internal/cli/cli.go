package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"UWP-TCP-Con/internal/ping"
	"UWP-TCP-Con/internal/web"
)

type App struct {
	inputTimeout  time.Duration
	lookupTimeout time.Duration
	linkServer    *web.LinkServer
}

func NewApp() *App {
	return &App{
		inputTimeout: 3 * time.Second,
	}
}

func (a *App) Run() error {
	for {
		config, err := a.collectConfig()
		if err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			return err
		}

		if err := a.execute(config); err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			return err
		}

		again, err := a.askAgain()
		if err != nil {
			if errors.Is(err, errAborted) {
				return nil
			}
			return err
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

	direct, err := a.collectDirectConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{Mode: mode, Direct: direct}, nil
}

type Mode string

const (
	ModeDirect Mode = "direct"
	ModeLookup Mode = "lookup"
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
	Subdomains []string
	Endings    []string
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

	port, err := a.askPort(edition)
	if err != nil {
		return LookupConfig{}, err
	}

	return LookupConfig{
		Edition:    edition,
		BaseHost:   baseHost,
		Port:       port,
		Subdomains: subdomains,
		Endings:    endings,
	}, nil
}

func (a *App) askMode() (Mode, error) {
	index, err := selectOption("Start mode", []string{"UWP/TCP query", "IP lookup"})
	if err != nil {
		return "", err
	}
	if index == 1 {
		return ModeLookup, nil
	}
	return ModeDirect, nil
}

func (a *App) askEdition() (ping.Edition, error) {
	index, err := selectOption("Edition", []string{"Bedrock", "Java"})
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
	index, err := selectOption("Subdomains", []string{"Custom subdomains", "Subdomain pool", "Custom + pool"})
	if err != nil {
		return nil, err
	}
	if index == 1 {
		return subdomainPool, nil
	}
	if index == 2 {
		custom, err := a.askCustomSubdomains()
		if err != nil {
			return nil, err
		}
		return append(custom, subdomainPool...), nil
	}

	return a.askCustomSubdomains()
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
	index, err := selectOption("Domain endings", []string{"Custom endings", "Ending pool", "Custom + pool"})
	if err != nil {
		return nil, err
	}
	if index == 1 {
		endings, err := loadDomainEndings()
		if err != nil {
			return endings, nil
		}
		return endings, nil
	}
	custom, err := a.askCustomEndings()
	if err != nil {
		return nil, err
	}
	if index == 2 {
		endings, err := loadDomainEndings()
		if err != nil {
			return append(custom, endings...), nil
		}
		return append(custom, endings...), nil
	}
	return custom, nil
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

func (a *App) execute(config Config) error {
	switch config.Mode {
	case ModeLookup:
		return a.executeLookup(config.Lookup)
	default:
		return a.executeDirect(config.Direct)
	}
}

func (a *App) executeDirect(config DirectConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.inputTimeout)
	defer cancel()

	resultText, err := withSpinner("Query", func(frame int) string {
		_ = frame
		return "Querying server"
	}, 120*time.Millisecond, func() (string, error) {
		result, err := ping.Execute(ctx, config.Edition, config.Host, config.Port)
		if err != nil {
			return "", err
		}
		return result.String(), nil
	})
	if err != nil {
		return err
	}

	renderTextPage("Result", resultText)
	return nil
}

func (a *App) executeLookup(config LookupConfig) error {
	ctx := context.Background()

	var current atomic.Value
	current.Store(ping.LookupProgress{})

	resultText, err := withSpinner("IP lookup", func(frame int) string {
		progress := current.Load().(ping.LookupProgress)
		return formatLookupProgress(progress, frame)
	}, 120*time.Millisecond, func() (string, error) {
		result, lookupErr := ping.LookupDomains(ctx, ping.LookupConfig{
			Edition:       config.Edition,
			Port:          config.Port,
			BaseHost:      config.BaseHost,
			Subdomains:    config.Subdomains,
			DomainEndings: config.Endings,
			Concurrency:   0,
			Progress: func(progress ping.LookupProgress) {
				current.Store(progress)
			},
		})
		if lookupErr != nil {
			return "", lookupErr
		}
		var links []web.LookupLinkURLs
		if len(result.Matches) > 0 {
			if a.linkServer != nil {
				_ = a.linkServer.Close()
			}
			entries := make([]web.LookupLink, 0, len(result.Matches))
			for _, match := range result.Matches {
				entries = append(entries, web.LookupLink{
					Name: match.Host,
					Host: match.Host,
					Port: config.Port,
				})
			}
			server, serverErr := web.StartLookupLinkServer(entries, 15*time.Minute)
			if serverErr != nil {
				return "", serverErr
			}
			a.linkServer = server
			links = server.Links()
		}
		return formatLookupResult(result, links), nil
	})
	if err != nil {
		return err
	}

	renderTextPage("Result", resultText)
	return nil
}

func formatLookupProgress(progress ping.LookupProgress, frame int) string {
	if progress.Total == 0 {
		return "Checking domains"
	}
	subdomain := progress.Subdomain
	if subdomain == "" {
		subdomain = "(none)"
	}
	bar := buildProgressBar(progress.Completed, progress.Total, frame, 20)
	percent := (float64(progress.Completed) / float64(progress.Total)) * 100
	return fmt.Sprintf(
		"%s %d/%d (%.0f%%)\nSubdomain: %s\nEnding: %s\nHost: %s",
		bar,
		progress.Completed,
		progress.Total,
		percent,
		subdomain,
		progress.Ending,
		progress.Host,
	)
}

func buildProgressBar(completed, total, frame, width int) string {
	if total <= 0 || width <= 0 {
		return "[--------------------]"
	}
	if completed > total {
		completed = total
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := 0; i < width; i++ {
		bar[i] = '░'
	}
	for i := 0; i < filled; i++ {
		bar[i] = '█'
	}
	if completed < total && filled < width {
		animation := []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}
		bar[filled] = animation[frame%len(animation)]
	}
	return fmt.Sprintf("[%s]", string(bar))
}

func (a *App) askAgain() (bool, error) {
	index, err := selectOption("Next step", []string{"New query", "Exit"})
	if err != nil {
		return false, err
	}
	return index == 0, nil
}

func formatLookupResult(result ping.LookupResult, links []web.LookupLinkURLs) string {
	var builder strings.Builder
	builder.WriteString("Summary\n")
	builder.WriteString(fmt.Sprintf("• Checked combinations: %d/%d\n", result.Completed, result.Attempts))
	builder.WriteString(fmt.Sprintf("• Matches: %d\n", len(result.Matches)))
	builder.WriteString("\n")

	if len(result.Matches) == 0 {
		builder.WriteString("No matching servers found.")
		return builder.String()
	}

	for i, match := range result.Matches {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("Host: %s\n", match.Host))
		if i < len(links) {
			builder.WriteString(fmt.Sprintf("Add link (browser): %s\n", links[i].AddURL))
			builder.WriteString(fmt.Sprintf("Join link (browser): %s\n", links[i].ConnectURL))
		}
		builder.WriteString(match.Result.String())
		builder.WriteString("\n")
	}
	return builder.String()
}
