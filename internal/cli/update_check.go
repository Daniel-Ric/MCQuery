package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	appVersion        = "0.2.0"
	updateReleaseURL  = "https://api.github.com/repos/Daniel-Ric/MCQuery/releases/latest"
	updateTagsURL     = "https://api.github.com/repos/Daniel-Ric/MCQuery/tags"
	updateRequestTime = 5 * time.Second
)

type updateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	LatestURL       string
	Source          string
	UpdateAvailable bool
}

func (a *App) executeUpdateCheck() error {
	resultText, err := withSpinner("Update check", func(frame int) string {
		_ = frame
		return "Checking GitHub"
	}, 120*time.Millisecond, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), updateRequestTime)
		defer cancel()
		info, err := checkForUpdates(ctx)
		if err != nil {
			return "", err
		}
		return formatUpdateInfo(info), nil
	})
	if err != nil {
		return err
	}
	return renderTextPageAndWait("Update check", resultText)
}

func (a *App) showStartupUpdateNotice() error {
	resultText, err := withSpinner("Update check", func(frame int) string {
		_ = frame
		return "Checking GitHub"
	}, 120*time.Millisecond, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), updateRequestTime)
		defer cancel()
		info, err := checkForUpdates(ctx)
		if err != nil || !info.UpdateAvailable {
			return "", err
		}
		return formatUpdateInfo(info), nil
	})
	if err != nil || strings.TrimSpace(resultText) == "" {
		return err
	}
	renderTextPage("Update available", resultText)
	return waitForEnter()
}

func checkForUpdates(ctx context.Context) (updateInfo, error) {
	latest, url, source, err := fetchLatestRelease(ctx)
	if err != nil {
		latest, url, source, err = fetchLatestTag(ctx)
		if err != nil {
			return updateInfo{}, err
		}
	}
	return updateInfo{
		CurrentVersion:  appVersion,
		LatestVersion:   latest,
		LatestURL:       url,
		Source:          source,
		UpdateAvailable: compareVersions(latest, appVersion) > 0,
	}, nil
}

func fetchLatestRelease(ctx context.Context) (string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateReleaseURL, nil)
	if err != nil {
		return "", "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("release response: %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		URL     string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", "", err
	}
	version := firstNonEmpty(payload.TagName, payload.Name)
	if strings.TrimSpace(version) == "" {
		return "", "", "", fmt.Errorf("release did not include a version")
	}
	return cleanVersion(version), payload.URL, "release", nil
}

func fetchLatestTag(ctx context.Context) (string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateTagsURL, nil)
	if err != nil {
		return "", "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("tags response: %s", resp.Status)
	}
	var payload []struct {
		Name string `json:"name"`
		URL  string `json:"zipball_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", "", err
	}
	if len(payload) == 0 || strings.TrimSpace(payload[0].Name) == "" {
		return "", "", "", fmt.Errorf("no tags found")
	}
	return cleanVersion(payload[0].Name), payload[0].URL, "tag", nil
}

func formatUpdateInfo(info updateInfo) string {
	var builder strings.Builder
	builder.WriteString("Update\n")
	builder.WriteString(fmt.Sprintf("Current version: %s\n", info.CurrentVersion))
	builder.WriteString(fmt.Sprintf("Latest version: %s\n", info.LatestVersion))
	builder.WriteString(fmt.Sprintf("Source: %s\n", info.Source))
	if info.LatestURL != "" {
		builder.WriteString(fmt.Sprintf("URL: %s\n", info.LatestURL))
	}
	if info.UpdateAvailable {
		builder.WriteString("Status: update available")
	} else {
		builder.WriteString("Status: up to date")
	}
	return builder.String()
}

func compareVersions(left, right string) int {
	leftParts := versionParts(cleanVersion(left))
	rightParts := versionParts(cleanVersion(right))
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for len(leftParts) < maxLen {
		leftParts = append(leftParts, 0)
	}
	for len(rightParts) < maxLen {
		rightParts = append(rightParts, 0)
	}
	for i := 0; i < maxLen; i++ {
		if leftParts[i] > rightParts[i] {
			return 1
		}
		if leftParts[i] < rightParts[i] {
			return -1
		}
	}
	return 0
}

func versionParts(value string) []int {
	value = strings.TrimSpace(value)
	if value == "" {
		return []int{0}
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '+'
	})
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		digits := leadingDigits(field)
		if digits == "" {
			parts = append(parts, 0)
			continue
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, parsed)
	}
	if len(parts) == 0 {
		return []int{0}
	}
	return parts
}

func cleanVersion(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "version")
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return strings.TrimSpace(value)
}

func leadingDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
