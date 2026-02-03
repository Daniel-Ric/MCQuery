package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type App struct {
	inputTimeout  time.Duration
	lookupTimeout time.Duration
}

func NewApp() *App {
	return &App{
		inputTimeout:  3 * time.Second,
		lookupTimeout: 12 * time.Second,
	}
}

func (a *App) Run() error {
	for {
		config, err := a.collectConfig()
		if err != nil {
			return err
		}

		if err := a.execute(config); err != nil {
			return err
		}

		again, err := a.askAgain()
		if err != nil {
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
	index, err := selectOption("Startmodus", []string{"UWP/TCP Abfrage", "IP Lookup"})
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
		value, err := promptInput("Server Host", "z.B. play.example.com", errMsg)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) == "" {
			errMsg = "Host darf nicht leer sein"
			continue
		}
		return value, nil
	}
}

func (a *App) askBaseHost() (string, error) {
	var errMsg string
	for {
		value, err := promptInput("IP/Domain ohne Endung", "z.B. example", errMsg)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			errMsg = "Wert darf nicht leer sein"
			continue
		}
		return value, nil
	}
}

func (a *App) askSubdomainChoice() ([]string, error) {
	index, err := selectOption("Subdomain", []string{"Eigene Subdomain", "Subdomain-Pool"})
	if err != nil {
		return nil, err
	}
	if index == 1 {
		return subdomainPool, nil
	}

	var errMsg string
	for {
		value, err := promptInput("Subdomain (optional)", "z.B. play (leer lassen für keine)", errMsg)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		return []string{value}, nil
	}
}

func (a *App) askDomainEndings() ([]string, error) {
	index, err := selectOption("Domain-Endung", []string{"Eigene Endung", "Endungs-Pool"})
	if err != nil {
		return nil, err
	}
	if index == 1 {
		return domainEndingPool, nil
	}
	var errMsg string
	for {
		value, err := promptInput("Domain-Endung", "z.B. com oder de", errMsg)
		if err != nil {
			return nil, err
		}
		value = normalizeEnding(value)
		if value == "" {
			errMsg = "Endung darf nicht leer sein"
			continue
		}
		return []string{value}, nil
	}
}

func (a *App) askPort(edition ping.Edition) (int, error) {
	defaultPort := ping.DefaultPort(edition)
	var errMsg string
	for {
		value, err := promptInput(fmt.Sprintf("Port (%d)", defaultPort), "Leer lassen für Standardport", errMsg)
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
			errMsg = "Port darf nicht leer sein"
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

	resultText, err := withSpinner("Abfrage", "Server wird abgefragt", 120*time.Millisecond, func() (string, error) {
		result, err := ping.Execute(ctx, config.Edition, config.Host, config.Port)
		if err != nil {
			return "", err
		}
		return result.String(), nil
	})
	if err != nil {
		return err
	}

	renderTextPage("Ergebnis", resultText)
	return nil
}

func (a *App) executeLookup(config LookupConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.lookupTimeout)
	defer cancel()

	resultText, err := withSpinner("IP Lookup", "Domains werden überprüft", 120*time.Millisecond, func() (string, error) {
		result, lookupErr := ping.LookupDomains(ctx, ping.LookupConfig{
			Edition:       config.Edition,
			Port:          config.Port,
			BaseHost:      config.BaseHost,
			Subdomains:    config.Subdomains,
			DomainEndings: config.Endings,
			Concurrency:   24,
		})
		if lookupErr != nil && !errors.Is(lookupErr, context.DeadlineExceeded) {
			return "", lookupErr
		}
		return formatLookupResult(result, errors.Is(lookupErr, context.DeadlineExceeded)), nil
	})
	if err != nil {
		return err
	}

	renderTextPage("Ergebnis", resultText)
	return nil
}

func (a *App) askAgain() (bool, error) {
	index, err := selectOption("Nächster Schritt", []string{"Neue Abfrage", "Beenden"})
	if err != nil {
		return false, err
	}
	return index == 0, nil
}

func formatLookupResult(result ping.LookupResult, timedOut bool) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Kombinationen geprüft: %d/%d\n", result.Completed, result.Attempts))
	builder.WriteString(fmt.Sprintf("Treffer: %d\n", len(result.Matches)))
	if timedOut {
		builder.WriteString("Hinweis: Zeitlimit erreicht, Ergebnisse können unvollständig sein.\n")
	}
	builder.WriteString("\n")

	if len(result.Matches) == 0 {
		builder.WriteString("Keine passenden Server gefunden.")
		return builder.String()
	}

	for i, match := range result.Matches {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("Host: %s\n", match.Host))
		builder.WriteString(match.Result.String())
		builder.WriteString("\n")
	}
	return builder.String()
}
