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
	Mode                string            `json:"mode"`
	Edition             string            `json:"edition"`
	Host                string            `json:"host"`
	Port                int               `json:"port"`
	Success             bool              `json:"success"`
	Error               string            `json:"error,omitempty"`
	MOTD                string            `json:"motd,omitempty"`
	CleanMOTD           string            `json:"clean_motd,omitempty"`
	SubMOTD             string            `json:"sub_motd,omitempty"`
	Version             string            `json:"version,omitempty"`
	Protocol            string            `json:"protocol,omitempty"`
	PlayersOnline       int               `json:"players_online,omitempty"`
	PlayersMax          int               `json:"players_max,omitempty"`
	PlayerSample        []ping.JavaPlayer `json:"player_sample,omitempty"`
	LatencyMillis       int64             `json:"latency_ms,omitempty"`
	ServerID            string            `json:"server_id,omitempty"`
	ServerGUID          uint64            `json:"server_guid,omitempty"`
	GameMode            string            `json:"game_mode,omitempty"`
	GameModeNumeric     string            `json:"game_mode_numeric,omitempty"`
	AdvertisedIPv4Port  int               `json:"advertised_ipv4_port,omitempty"`
	AdvertisedIPv6Port  int               `json:"advertised_ipv6_port,omitempty"`
	EnforcesSecureChat  *bool             `json:"enforces_secure_chat,omitempty"`
	PreviewsChat        *bool             `json:"previews_chat,omitempty"`
	PreventsChatReports *bool             `json:"prevents_chat_reports,omitempty"`
	ModLoader           string            `json:"mod_loader,omitempty"`
	Mods                []ping.JavaMod    `json:"mods,omitempty"`
	ModChannels         int               `json:"mod_channels,omitempty"`
	ModDataTruncated    bool              `json:"mod_data_truncated,omitempty"`
	IconType            string            `json:"icon_type,omitempty"`
	IconWidth           int               `json:"icon_width,omitempty"`
	IconHeight          int               `json:"icon_height,omitempty"`
	ResponseBytes       int               `json:"response_bytes,omitempty"`
	ExtraFields         []string          `json:"extra_fields,omitempty"`
	SelectedIP          string            `json:"selected_ip,omitempty"`
	ResolvedIPs         []string          `json:"resolved_ips,omitempty"`
	SRVUsed             bool              `json:"srv_used,omitempty"`
	SRVHost             string            `json:"srv_host,omitempty"`
	SRVPort             int               `json:"srv_port,omitempty"`
	AddURL              string            `json:"add_url,omitempty"`
	ConnectURL          string            `json:"connect_url,omitempty"`
	JavaIconSavedTo     string            `json:"java_icon_saved_to,omitempty"`
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
		"player_sample",
		"latency_ms",
		"sub_motd",
		"server_id",
		"server_guid",
		"game_mode",
		"game_mode_numeric",
		"advertised_ipv4_port",
		"advertised_ipv6_port",
		"enforces_secure_chat",
		"previews_chat",
		"prevents_chat_reports",
		"mod_loader",
		"mods",
		"mod_channels",
		"mod_data_truncated",
		"icon_type",
		"icon_width",
		"icon_height",
		"response_bytes",
		"extra_fields",
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
			formatExportPlayers(record.PlayerSample),
			int64String(record.LatencyMillis),
			record.SubMOTD,
			record.ServerID,
			uint64String(record.ServerGUID),
			record.GameMode,
			record.GameModeNumeric,
			intString(record.AdvertisedIPv4Port),
			intString(record.AdvertisedIPv6Port),
			boolPointerString(record.EnforcesSecureChat),
			boolPointerString(record.PreviewsChat),
			boolPointerString(record.PreventsChatReports),
			record.ModLoader,
			formatExportMods(record.Mods),
			intString(record.ModChannels),
			strconv.FormatBool(record.ModDataTruncated),
			record.IconType,
			intString(record.IconWidth),
			intString(record.IconHeight),
			intString(record.ResponseBytes),
			strings.Join(record.ExtraFields, ";"),
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

func uint64String(value uint64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(value, 10)
}

func boolPointerString(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

func formatExportPlayers(players []ping.JavaPlayer) string {
	values := make([]string, 0, len(players))
	for _, player := range players {
		value := player.Name
		if player.ID != "" {
			value += " (" + player.ID + ")"
		}
		values = append(values, value)
	}
	return strings.Join(values, ";")
}

func formatExportMods(mods []ping.JavaMod) string {
	values := make([]string, 0, len(mods))
	for _, mod := range mods {
		value := mod.ID
		if mod.Version != "" {
			value += "@" + mod.Version
		}
		values = append(values, value)
	}
	return strings.Join(values, ";")
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
		record.SubMOTD = value.SubMOTD
		record.Version = value.GameVersion
		record.Protocol = value.ProtocolVersion
		record.PlayersOnline = parseCount(value.CurrentPlayers)
		record.PlayersMax = parseCount(value.MaxPlayers)
		record.LatencyMillis = value.LatencyMillis
		record.ServerID = value.ServerID
		record.ServerGUID = value.ServerGUID
		record.GameMode = value.GameMode
		record.GameModeNumeric = value.GameModeNumeric
		record.AdvertisedIPv4Port = value.AdvertisedIPv4Port
		record.AdvertisedIPv6Port = value.AdvertisedIPv6Port
		record.ResponseBytes = value.ResponseBytes
		record.ExtraFields = append([]string(nil), value.ExtraFields...)
	case ping.JavaStatus:
		record.MOTD = value.MOTD
		record.CleanMOTD = value.CleanMOTD
		record.Version = value.VersionName
		record.Protocol = strconv.Itoa(value.ProtocolVersion)
		record.PlayersOnline = value.CurrentPlayers
		record.PlayersMax = value.MaxPlayers
		record.PlayerSample = append([]ping.JavaPlayer(nil), value.PlayerSample...)
		record.LatencyMillis = value.LatencyMillis
		record.EnforcesSecureChat = value.EnforcesSecureChat
		record.PreviewsChat = value.PreviewsChat
		record.PreventsChatReports = value.PreventsChatReports
		record.ModLoader = value.ModLoader
		record.Mods = append([]ping.JavaMod(nil), value.Mods...)
		record.ModChannels = value.ModChannels
		record.ModDataTruncated = value.ModDataTruncated
		record.IconType = value.IconType
		record.IconWidth = value.IconWidth
		record.IconHeight = value.IconHeight
		record.ResponseBytes = value.StatusJSONBytes
		record.ExtraFields = append([]string(nil), value.ExtraFields...)
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
