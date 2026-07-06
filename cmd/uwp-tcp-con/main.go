package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"UWP-TCP-Con/internal/cli"
	"UWP-TCP-Con/internal/ping"
	"UWP-TCP-Con/internal/web"
)

func main() {
	apiMode := flag.Bool("api", false, "start status API server")
	apiConfig := flag.String("api-config", "", "status API config file")
	apiAddr := flag.String("api-addr", "127.0.0.1:8080", "status API address")
	apiTimeoutMS := flag.Int("api-timeout-ms", 1000, "status API request timeout in milliseconds")
	apiConcurrency := flag.Int("api-concurrency", 32, "status API query concurrency")
	flag.Parse()

	if *apiMode || strings.TrimSpace(*apiConfig) != "" {
		config := web.StatusAPIConfig{
			Addr:        *apiAddr,
			Timeout:     time.Duration(*apiTimeoutMS) * time.Millisecond,
			Concurrency: *apiConcurrency,
			EnableSRV:   true,
			IPMode:      ping.IPModeAuto,
		}
		if strings.TrimSpace(*apiConfig) != "" {
			loaded, err := loadStatusAPIConfig(*apiConfig, config)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			config = loaded
		}
		server := web.NewStatusAPIServer(config)
		fmt.Fprintf(os.Stdout, "MCQuery API listening on http://%s\n", config.Addr)
		if err := server.ListenAndServe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	app := cli.NewApp()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type statusAPIConfigFile struct {
	Addr        string      `json:"addr"`
	TimeoutMS   *int        `json:"timeout_ms"`
	Concurrency *int        `json:"concurrency"`
	EnableSRV   *bool       `json:"enable_srv"`
	IPMode      ping.IPMode `json:"ip_mode"`
}

func loadStatusAPIConfig(path string, fallback web.StatusAPIConfig) (web.StatusAPIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback, fmt.Errorf("load api config: %w", err)
	}
	var fileConfig statusAPIConfigFile
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fallback, fmt.Errorf("load api config: %w", err)
	}
	if strings.TrimSpace(fileConfig.Addr) != "" {
		fallback.Addr = strings.TrimSpace(fileConfig.Addr)
	}
	if fileConfig.TimeoutMS != nil {
		if *fileConfig.TimeoutMS < 0 {
			return fallback, fmt.Errorf("load api config: timeout_ms cannot be negative")
		}
		fallback.Timeout = time.Duration(*fileConfig.TimeoutMS) * time.Millisecond
	}
	if fileConfig.Concurrency != nil {
		if *fileConfig.Concurrency < 0 {
			return fallback, fmt.Errorf("load api config: concurrency cannot be negative")
		}
		fallback.Concurrency = *fileConfig.Concurrency
	}
	if fileConfig.EnableSRV != nil {
		fallback.EnableSRV = *fileConfig.EnableSRV
	}
	if fileConfig.IPMode != "" {
		switch fileConfig.IPMode {
		case ping.IPModeAuto, ping.IPModeIPv4, ping.IPModeIPv6:
			fallback.IPMode = fileConfig.IPMode
		default:
			return fallback, fmt.Errorf("load api config: ip_mode must be auto, ipv4, or ipv6")
		}
	}
	return fallback, nil
}
