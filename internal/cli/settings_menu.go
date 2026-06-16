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
			fmt.Sprintf("Timeout: %d s", a.settings.RequestTimeoutSeconds),
			fmt.Sprintf("Retries: %d", a.settings.RetryCount),
			fmt.Sprintf("Retry delay: %d ms", a.settings.RetryDelayMillis),
			fmt.Sprintf("Java SRV lookup: %s", boolText(a.settings.EnableSRV)),
			fmt.Sprintf("IP mode: %s", a.settings.IPMode),
			fmt.Sprintf("Lookup workers: %s", lookupWorkerSettingText(a.settings.LookupConcurrency)),
			fmt.Sprintf("Lookup rate cap: %s", settingRateLimitText(a.settings.LookupRateLimit)),
			fmt.Sprintf("Verbose output: %s", boolText(a.settings.Verbose)),
			fmt.Sprintf("Colored MOTD: %s", boolText(a.settings.ColorMOTD)),
			fmt.Sprintf("Save results: %s", boolText(a.settings.SaveResults)),
			fmt.Sprintf("Export format: %s", a.settings.ExportFormat),
			fmt.Sprintf("Save Java icons: %s", boolText(a.settings.SaveJavaIcons)),
			fmt.Sprintf("Results path: %s", a.settings.ResultsPath),
			fmt.Sprintf("Check for updates: %s", boolText(a.settings.CheckForUpdates)),
			"Lookup presets: Subdomains and endings",
			"Reset settings: Restore defaults",
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
			value, err := askBoolValue("Colored MOTD", a.settings.ColorMOTD)
			if err != nil {
				return err
			}
			a.settings.ColorMOTD = value
		case 9:
			value, err := askBoolValue("Save results", a.settings.SaveResults)
			if err != nil {
				return err
			}
			a.settings.SaveResults = value
		case 10:
			value, err := askExportFormat(a.settings.ExportFormat)
			if err != nil {
				return err
			}
			a.settings.ExportFormat = value
		case 11:
			value, err := askBoolValue("Save Java icons", a.settings.SaveJavaIcons)
			if err != nil {
				return err
			}
			a.settings.SaveJavaIcons = value
		case 12:
			value, err := askTextValue("Results path", a.settings.ResultsPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(value) == "" {
				value = defaultResultsPath()
			}
			a.settings.ResultsPath = value
		case 13:
			value, err := askBoolValue("Check for updates", a.settings.CheckForUpdates)
			if err != nil {
				return err
			}
			a.settings.CheckForUpdates = value
		case 14:
			if err := a.manageLookupPresets(); err != nil {
				return err
			}
			continue
		case 15:
			ok, err := askConfirm("Reset all settings?")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
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

func boolText(value bool) string {
	if value {
		return "Enabled"
	}
	return "Disabled"
}

func lookupWorkerSettingText(value int) string {
	if value <= 0 {
		return fmt.Sprintf("Auto (%d)", ping.AutoLookupConcurrencyTarget())
	}
	return strconv.Itoa(value)
}

func settingRateLimitText(value int) string {
	if value <= 0 {
		return "Uncapped"
	}
	return fmt.Sprintf("%d req/s", value)
}

func askExportFormat(current string) (string, error) {
	options := []string{"text", "json", "csv"}
	initial := 0
	for i, option := range options {
		if option == normalizeExportFormat(current) {
			initial = i
			break
		}
	}
	index, err := selectOptionWithInitial("Export format", options, initial)
	if err != nil {
		return current, err
	}
	return options[index], nil
}

func askIntValue(label string, current int) (int, error) {
	var errMsg string
	for {
		value, err := promptInput(label, fmt.Sprintf("Current: %d. Leave empty to keep it.", current), errMsg)
		if err != nil {
			return 0, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return current, nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			errMsg = "Enter a whole number"
			continue
		}
		if parsed < 0 {
			errMsg = "Value cannot be negative"
			continue
		}
		return parsed, nil
	}
}

func askTextValue(label, current string) (string, error) {
	value, err := promptInput(label, fmt.Sprintf("Current: %s. Leave empty to keep it.", current), "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return current, nil
	}
	return value, nil
}

func askBoolValue(label string, current bool) (bool, error) {
	options := []string{"Enabled", "Disabled"}
	initial := 1
	if current {
		initial = 0
	}
	index, err := selectOptionWithInitial(label, options, initial)
	if err != nil {
		return false, err
	}
	return index == 0, nil
}

func askIPMode(current ping.IPMode) (ping.IPMode, error) {
	options := []string{"Auto", "IPv4", "IPv6"}
	initial := 0
	switch current {
	case ping.IPModeIPv4:
		initial = 1
	case ping.IPModeIPv6:
		initial = 2
	}
	index, err := selectOptionWithInitial("IP mode", options, initial)
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
