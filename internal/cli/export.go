package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
	"UWP-TCP-Con/internal/web"
)

const (
	exportFormatText = "text"
	exportFormatJSON = "json"
	exportFormatCSV  = "csv"
)

type exportPayload struct {
	Title     string         `json:"title"`
	CreatedAt string         `json:"created_at"`
	Records   []exportRecord `json:"records"`
}

type exportRecord struct {
	Mode            string   `json:"mode"`
	Edition         string   `json:"edition"`
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	Success         bool     `json:"success"`
	Error           string   `json:"error,omitempty"`
	MOTD            string   `json:"motd,omitempty"`
	CleanMOTD       string   `json:"clean_motd,omitempty"`
	Version         string   `json:"version,omitempty"`
	Protocol        string   `json:"protocol,omitempty"`
	PlayersOnline   int      `json:"players_online,omitempty"`
	PlayersMax      int      `json:"players_max,omitempty"`
	LatencyMillis   int64    `json:"latency_ms,omitempty"`
	SelectedIP      string   `json:"selected_ip,omitempty"`
	ResolvedIPs     []string `json:"resolved_ips,omitempty"`
	SRVUsed         bool     `json:"srv_used,omitempty"`
	SRVHost         string   `json:"srv_host,omitempty"`
	SRVPort         int      `json:"srv_port,omitempty"`
	AddURL          string   `json:"add_url,omitempty"`
	ConnectURL      string   `json:"connect_url,omitempty"`
	JavaIconSavedTo string   `json:"java_icon_saved_to,omitempty"`
}

func isValidExportFormat(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case exportFormatText, exportFormatJSON, exportFormatCSV:
		return true
	default:
		return false
	}
}

func normalizeExportFormat(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if isValidExportFormat(value) {
		return value
	}
	return exportFormatText
}

func exportExtension(format string) string {
	switch normalizeExportFormat(format) {
	case exportFormatJSON:
		return "json"
	case exportFormatCSV:
		return "csv"
	default:
		return "txt"
	}
}

func (a *App) saveExport(title, textContent string, records []exportRecord) (string, error) {
	format := normalizeExportFormat(a.settings.ExportFormat)
	path, err := a.exportPath(exportExtension(format))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	switch format {
	case exportFormatJSON:
		payload := exportPayload{
			Title:     title,
			CreatedAt: time.Now().Format(time.RFC3339),
			Records:   records,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return "", err
		}
	case exportFormatCSV:
		file, err := os.Create(path)
		if err != nil {
			return "", err
		}
		writer := csv.NewWriter(file)
		if err := writeCSVExport(writer, records); err != nil {
			_ = file.Close()
			return "", err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("%s\n", title))
		builder.WriteString(fmt.Sprintf("Saved at: %s\n", time.Now().Format(time.RFC3339)))
		builder.WriteString("\n")
		builder.WriteString(textContent)
		builder.WriteString("\n")
		if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
			return "", err
		}
	}

	return path, nil
}

func writeCSVExport(writer *csv.Writer, records []exportRecord) error {
	header := []string{
		"mode",
		"edition",
		"host",
		"port",
		"success",
		"error",
		"motd",
		"clean_motd",
		"version",
		"protocol",
		"players_online",
		"players_max",
		"latency_ms",
		"selected_ip",
		"resolved_ips",
		"srv_used",
		"srv_host",
		"srv_port",
		"add_url",
		"connect_url",
		"java_icon_saved_to",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, record := range records {
		row := []string{
			record.Mode,
			record.Edition,
			record.Host,
			strconv.Itoa(record.Port),
			strconv.FormatBool(record.Success),
			record.Error,
			record.MOTD,
			record.CleanMOTD,
			record.Version,
			record.Protocol,
			intString(record.PlayersOnline),
			intString(record.PlayersMax),
			int64String(record.LatencyMillis),
			record.SelectedIP,
			strings.Join(record.ResolvedIPs, ";"),
			strconv.FormatBool(record.SRVUsed),
			record.SRVHost,
			intString(record.SRVPort),
			record.AddURL,
			record.ConnectURL,
			record.JavaIconSavedTo,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func intString(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func int64String(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func (a *App) exportPath(ext string) (string, error) {
	trimmed := strings.TrimSpace(a.settings.ResultsPath)
	if trimmed == "" {
		trimmed = defaultResultsPath()
	}
	hasSeparator := strings.HasSuffix(trimmed, string(os.PathSeparator))
	clean := filepath.Clean(trimmed)
	info, err := os.Stat(clean)
	if err == nil && info.IsDir() {
		return filepath.Join(clean, fmt.Sprintf("result-%s.%s", timeStamp(), ext)), nil
	}
	if err != nil && os.IsNotExist(err) && (hasSeparator || filepath.Ext(clean) == "") {
		return filepath.Join(clean, fmt.Sprintf("result-%s.%s", timeStamp(), ext)), nil
	}
	if filepath.Ext(clean) == "" {
		return clean + "." + ext, nil
	}
	return clean, nil
}

func newExportRecord(mode string, edition ping.Edition, host string, port int, result ping.Result, details ping.ExecuteDetails, link *web.LookupLinkURLs, runErr error) exportRecord {
	record := exportRecord{
		Mode:        mode,
		Edition:     string(edition),
		Host:        host,
		Port:        port,
		Success:     runErr == nil,
		SelectedIP:  details.SelectedIP,
		ResolvedIPs: append([]string(nil), details.ResolvedIPs...),
		SRVUsed:     details.SRVUsed,
		SRVHost:     details.SRVHost,
		SRVPort:     details.SRVPort,
	}
	if runErr != nil {
		record.Error = runErr.Error()
		return record
	}
	if link != nil {
		record.AddURL = link.AddURL
		record.ConnectURL = link.ConnectURL
	}
	switch value := result.(type) {
	case ping.BedrockPong:
		record.MOTD = value.MOTD
		record.CleanMOTD = value.CleanMOTD
		record.Version = value.GameVersion
		record.Protocol = value.ProtocolVersion
		record.PlayersOnline = parseCount(value.CurrentPlayers)
		record.PlayersMax = parseCount(value.MaxPlayers)
	case ping.JavaStatus:
		record.MOTD = value.MOTD
		record.CleanMOTD = value.CleanMOTD
		record.Version = value.VersionName
		record.Protocol = strconv.Itoa(value.ProtocolVersion)
		record.PlayersOnline = value.CurrentPlayers
		record.PlayersMax = value.MaxPlayers
		record.LatencyMillis = value.LatencyMillis
	}
	return record
}

func parseCount(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func (a *App) saveJavaIcon(host string, status ping.JavaStatus) (string, error) {
	if len(status.IconPNG) == 0 {
		return "", nil
	}
	basePath, err := a.exportPath("txt")
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(basePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("server-icon-%s-%s.png", safeFileName(host), timeStamp()))
	return path, os.WriteFile(path, status.IconPNG, 0o644)
}

func safeFileName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('-')
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "server"
	}
	return result
}
