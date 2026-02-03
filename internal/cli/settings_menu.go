package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
)

func (a *App) manageSettings() error {
	for {
		options := []string{
			fmt.Sprintf("Request timeout (seconds): %d", a.settings.RequestTimeoutSeconds),
			fmt.Sprintf("Retry count: %d", a.settings.RetryCount),
			fmt.Sprintf("Retry delay (ms): %d", a.settings.RetryDelayMillis),
			fmt.Sprintf("Enable Java SRV: %t", a.settings.EnableSRV),
			fmt.Sprintf("IP mode: %s", a.settings.IPMode),
			fmt.Sprintf("Lookup concurrency: %d", a.settings.LookupConcurrency),
			fmt.Sprintf("Lookup rate limit (req/s): %d", a.settings.LookupRateLimit),
			fmt.Sprintf("Verbose output: %t", a.settings.Verbose),
			fmt.Sprintf("Save results: %t", a.settings.SaveResults),
			fmt.Sprintf("Results path: %s", a.settings.ResultsPath),
			"Reset settings",
			"Back",
		}
		index, err := selectOption("Settings", options)
		if err != nil {
			return err
		}
		switch index {
		case 0:
			value, err := askIntValue("Request timeout (seconds)", a.settings.RequestTimeoutSeconds)
			if err != nil {
				return err
			}
			a.settings.RequestTimeoutSeconds = value
		case 1:
			value, err := askIntValue("Retry count", a.settings.RetryCount)
			if err != nil {
				return err
			}
			a.settings.RetryCount = value
		case 2:
			value, err := askIntValue("Retry delay (ms)", a.settings.RetryDelayMillis)
			if err != nil {
				return err
			}
			a.settings.RetryDelayMillis = value
		case 3:
			value, err := askBoolValue("Enable Java SRV", a.settings.EnableSRV)
			if err != nil {
				return err
			}
			a.settings.EnableSRV = value
		case 4:
			value, err := askIPMode(a.settings.IPMode)
			if err != nil {
				return err
			}
			a.settings.IPMode = value
		case 5:
			value, err := askIntValue("Lookup concurrency (0 = auto)", a.settings.LookupConcurrency)
			if err != nil {
				return err
			}
			a.settings.LookupConcurrency = value
		case 6:
			value, err := askIntValue("Lookup rate limit (req/s)", a.settings.LookupRateLimit)
			if err != nil {
				return err
			}
			a.settings.LookupRateLimit = value
		case 7:
			value, err := askBoolValue("Verbose output", a.settings.Verbose)
			if err != nil {
				return err
			}
			a.settings.Verbose = value
		case 8:
			value, err := askBoolValue("Save results", a.settings.SaveResults)
			if err != nil {
				return err
			}
			a.settings.SaveResults = value
		case 9:
			value, err := askTextValue("Results path", a.settings.ResultsPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(value) == "" {
				value = defaultResultsPath()
			}
			a.settings.ResultsPath = value
		case 10:
			a.settings = defaultSettings()
		default:
			return nil
		}
		if err := a.settings.Validate(); err != nil {
			return err
		}
		if err := saveSettings(a.settings); err != nil {
			return err
		}
	}
}

func askIntValue(label string, current int) (int, error) {
	for {
		value, err := promptInput(label, fmt.Sprintf("Current: %d", current), "")
		if err != nil {
			return 0, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return current, nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		return parsed, nil
	}
}

func askTextValue(label, current string) (string, error) {
	value, err := promptInput(label, fmt.Sprintf("Current: %s", current), "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return current, nil
	}
	return value, nil
}

func askBoolValue(label string, current bool) (bool, error) {
	options := []string{"Disabled", "Enabled"}
	index, err := selectOption(label, options)
	if err != nil {
		return false, err
	}
	return index == 1, nil
}

func askIPMode(current ping.IPMode) (ping.IPMode, error) {
	options := []string{"Auto", "IPv4", "IPv6"}
	index, err := selectOption("IP mode", options)
	if err != nil {
		return current, err
	}
	switch index {
	case 1:
		return ping.IPModeIPv4, nil
	case 2:
		return ping.IPModeIPv6, nil
	default:
		return ping.IPModeAuto, nil
	}
}

func ensureResultsPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = defaultResultsPath()
	}
	hasSeparator := strings.HasSuffix(trimmed, string(os.PathSeparator))
	clean := filepath.Clean(trimmed)
	info, err := os.Stat(clean)
	if err == nil && info.IsDir() {
		return filepath.Join(clean, fmt.Sprintf("result-%s.txt", timeStamp())), nil
	}
	if err != nil && os.IsNotExist(err) {
		if hasSeparator {
			return filepath.Join(clean, fmt.Sprintf("result-%s.txt", timeStamp())), nil
		}
	}
	return clean, nil
}

func timeStamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
