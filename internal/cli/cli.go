package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type App struct {
	inputTimeout time.Duration
}

func NewApp() *App {
	return &App{inputTimeout: 3 * time.Second}
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
	Host    string
	Port    int
	Edition ping.Edition
}

func (a *App) collectConfig() (Config, error) {
	edition, err := a.askEdition()
	if err != nil {
		return Config{}, err
	}

	host, err := a.askHost()
	if err != nil {
		return Config{}, err
	}

	port, err := a.askPort(edition)
	if err != nil {
		return Config{}, err
	}

	return Config{Host: host, Port: port, Edition: edition}, nil
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

func (a *App) askAgain() (bool, error) {
	index, err := selectOption("Nächster Schritt", []string{"Neue Abfrage", "Beenden"})
	if err != nil {
		return false, err
	}
	return index == 0, nil
}
