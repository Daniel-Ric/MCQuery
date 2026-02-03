package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type Settings struct {
	RequestTimeoutSeconds int         `json:"request_timeout_seconds"`
	RetryCount            int         `json:"retry_count"`
	RetryDelayMillis      int         `json:"retry_delay_millis"`
	EnableSRV             bool        `json:"enable_srv"`
	IPMode                ping.IPMode `json:"ip_mode"`
	LookupConcurrency     int         `json:"lookup_concurrency"`
	LookupRateLimit       int         `json:"lookup_rate_limit"`
	Verbose               bool        `json:"verbose"`
	SaveResults           bool        `json:"save_results"`
	ResultsPath           string      `json:"results_path"`
}

func defaultSettings() Settings {
	return Settings{
		RequestTimeoutSeconds: 3,
		RetryCount:            0,
		RetryDelayMillis:      200,
		EnableSRV:             true,
		IPMode:                ping.IPModeAuto,
		LookupConcurrency:     0,
		LookupRateLimit:       0,
		Verbose:               false,
		SaveResults:           false,
		ResultsPath:           defaultResultsPath(),
	}
}

func loadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return defaultSettings(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return defaultSettings(), err
	}
	settings := defaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultSettings(), err
	}
	return settings, nil
}

func saveSettings(settings Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcquery", "settings.json"), nil
}

func defaultResultsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "mcquery-results"
	}
	return filepath.Join(dir, "mcquery", "results")
}

func (s Settings) RequestTimeout() time.Duration {
	if s.RequestTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(s.RequestTimeoutSeconds) * time.Second
}

func (s Settings) RetryDelay() time.Duration {
	if s.RetryDelayMillis <= 0 {
		return 0
	}
	return time.Duration(s.RetryDelayMillis) * time.Millisecond
}

func (s Settings) Validate() error {
	if s.RequestTimeoutSeconds < 0 {
		return fmt.Errorf("request timeout cannot be negative")
	}
	if s.RetryCount < 0 {
		return fmt.Errorf("retry count cannot be negative")
	}
	if s.RetryDelayMillis < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}
	if s.LookupConcurrency < 0 {
		return fmt.Errorf("lookup concurrency cannot be negative")
	}
	if s.LookupRateLimit < 0 {
		return fmt.Errorf("lookup rate limit cannot be negative")
	}
	if s.IPMode != ping.IPModeAuto && s.IPMode != ping.IPModeIPv4 && s.IPMode != ping.IPModeIPv6 {
		return fmt.Errorf("invalid IP mode")
	}
	return nil
}
