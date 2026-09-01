package ping

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

func PingJava(ctx context.Context, dialHost string, handshakeHost string, port int, skipLatency bool) (JavaStatus, error) {
	addr := net.JoinHostPort(dialHost, strconv.Itoa(port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return JavaStatus{}, err
	}
	defer func() {
		_ = conn.Close()
	}()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	}

	if err := writeHandshake(conn, handshakeHost, port); err != nil {
		return JavaStatus{}, err
	}
	if err := writeStatusRequest(conn); err != nil {
		return JavaStatus{}, err
	}

	respPayload, err := readPacket(conn)
	if err != nil {
		return JavaStatus{}, err
	}

	respReader := bytes.NewReader(respPayload)
	packetID, err := readVarInt(respReader)
	if err != nil {
		return JavaStatus{}, err
	}
	if packetID != 0x00 {
		return JavaStatus{}, fmt.Errorf("unexpected status packet id: %d", packetID)
	}

	statusJSON, err := readString(respReader)
	if err != nil {
		return JavaStatus{}, err
	}

	status, err := parseJavaStatus([]byte(statusJSON))
	if err != nil {
		return JavaStatus{}, err
	}

	if !skipLatency {
		latency, err := pingJavaLatency(conn)
		if err != nil {
			return JavaStatus{}, err
		}
		status.LatencyMillis = latency
	}

	return status, nil
}

func writeHandshake(w io.Writer, host string, port int) error {
	const protocolVersion = 754
	payload := &bytes.Buffer{}
	writeVarInt(payload, 0x00)
	writeVarInt(payload, protocolVersion)
	writeString(payload, host)
	if err := binary.Write(payload, binary.BigEndian, uint16(port)); err != nil {
		return err
	}
	writeVarInt(payload, 0x01)

	return writePacket(w, payload.Bytes())
}

func writeStatusRequest(w io.Writer) error {
	payload := &bytes.Buffer{}
	writeVarInt(payload, 0x00)
	return writePacket(w, payload.Bytes())
}

func pingJavaLatency(conn net.Conn) (int64, error) {
	payload := &bytes.Buffer{}
	writeVarInt(payload, 0x01)
	now := time.Now().UnixMilli()
	if err := binary.Write(payload, binary.BigEndian, uint64(now)); err != nil {
		return 0, err
	}
	if err := writePacket(conn, payload.Bytes()); err != nil {
		return 0, err
	}

	respPayload, err := readPacket(conn)
	if err != nil {
		return 0, err
	}
	respReader := bytes.NewReader(respPayload)
	packetID, err := readVarInt(respReader)
	if err != nil {
		return 0, err
	}
	if packetID != 0x01 {
		return 0, fmt.Errorf("unexpected pong packet id: %d", packetID)
	}
	var sent uint64
	if err := binary.Read(respReader, binary.BigEndian, &sent); err != nil {
		return 0, err
	}
	return time.Now().UnixMilli() - int64(sent), nil
}

func parseJavaStatus(data []byte) (JavaStatus, error) {
	type rawStatus struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
			Sample []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"sample"`
		} `json:"players"`
		Description         any    `json:"description"`
		Favicon             string `json:"favicon"`
		EnforcesSecureChat  *bool  `json:"enforcesSecureChat"`
		PreviewsChat        *bool  `json:"previewsChat"`
		PreventsChatReports *bool  `json:"preventsChatReports"`
		ModInfo             struct {
			Type    string `json:"type"`
			ModList []struct {
				ID      string `json:"modid"`
				Version string `json:"version"`
			} `json:"modList"`
		} `json:"modinfo"`
		ForgeData *struct {
			Channels []json.RawMessage `json:"channels"`
			Mods     []struct {
				ID       string `json:"modId"`
				LegacyID string `json:"modid"`
				Marker   string `json:"modmarker"`
				Version  string `json:"version"`
			} `json:"mods"`
			Truncated         bool `json:"truncated"`
			FMLNetworkVersion int  `json:"fmlNetworkVersion"`
		} `json:"forgeData"`
	}

	var raw rawStatus
	if err := json.Unmarshal(data, &raw); err != nil {
		return JavaStatus{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return JavaStatus{}, err
	}

	motd := extractJavaDescription(raw.Description)
	iconType, iconPNG, iconWidth, iconHeight := parseJavaFavicon(raw.Favicon)
	players := make([]JavaPlayer, 0, len(raw.Players.Sample))
	for _, player := range raw.Players.Sample {
		players = append(players, JavaPlayer{Name: player.Name, ID: player.ID})
	}
	mods := make([]JavaMod, 0, len(raw.ModInfo.ModList))
	for _, mod := range raw.ModInfo.ModList {
		mods = append(mods, JavaMod{ID: mod.ID, Version: mod.Version})
	}
	modLoader := strings.TrimSpace(raw.ModInfo.Type)
	modChannels := 0
	modDataTruncated := false
	if raw.ForgeData != nil {
		modLoader = "Forge/FML"
		if raw.ForgeData.FMLNetworkVersion > 0 {
			modLoader = fmt.Sprintf("Forge/FML network %d", raw.ForgeData.FMLNetworkVersion)
		}
		for _, mod := range raw.ForgeData.Mods {
			id := strings.TrimSpace(mod.ID)
			if id == "" {
				id = strings.TrimSpace(mod.LegacyID)
			}
			version := strings.TrimSpace(mod.Version)
			if version == "" {
				version = strings.TrimSpace(mod.Marker)
			}
			if id != "" {
				mods = append(mods, JavaMod{ID: id, Version: version})
			}
		}
		modChannels = len(raw.ForgeData.Channels)
		modDataTruncated = raw.ForgeData.Truncated
	}
	extraFields := javaStatusExtraFields(fields)
	return JavaStatus{
		VersionName:         raw.Version.Name,
		ProtocolVersion:     raw.Version.Protocol,
		CurrentPlayers:      raw.Players.Online,
		MaxPlayers:          raw.Players.Max,
		PlayerSample:        players,
		MOTD:                motd,
		CleanMOTD:           stripMCFormatting(motd),
		IconPNG:             iconPNG,
		IconType:            iconType,
		IconWidth:           iconWidth,
		IconHeight:          iconHeight,
		EnforcesSecureChat:  raw.EnforcesSecureChat,
		PreviewsChat:        raw.PreviewsChat,
		PreventsChatReports: raw.PreventsChatReports,
		ModLoader:           modLoader,
		Mods:                mods,
		ModChannels:         modChannels,
		ModDataTruncated:    modDataTruncated,
		StatusJSONBytes:     len(data),
		ExtraFields:         extraFields,
	}, nil
}

func javaStatusExtraFields(fields map[string]json.RawMessage) []string {
	known := map[string]struct{}{
		"version": {}, "players": {}, "description": {}, "favicon": {},
		"enforcesSecureChat": {}, "previewsChat": {}, "preventsChatReports": {},
		"modinfo": {}, "forgeData": {},
	}
	extra := make([]string, 0)
	for field := range fields {
		if _, ok := known[field]; !ok {
			extra = append(extra, field)
		}
	}
	sort.Strings(extra)
	return extra
}

func extractJavaDescription(desc any) string {
	switch v := desc.(type) {
	case string:
		return v
	case map[string]any:
		var builder strings.Builder
		appendDescriptionText(&builder, v)
		return builder.String()
	case []any:
		var builder strings.Builder
		for _, item := range v {
			appendJavaDescriptionValue(&builder, item)
		}
		return builder.String()
	default:
		return ""
	}
}

func appendDescriptionText(builder *strings.Builder, desc map[string]any) {
	appendJavaFormatting(builder, desc)
	if text, ok := desc["text"].(string); ok {
		builder.WriteString(text)
	} else if fallback, ok := desc["fallback"].(string); ok {
		builder.WriteString(fallback)
	} else if translate, ok := desc["translate"].(string); ok {
		builder.WriteString(translate)
	}
	if with, ok := desc["with"].([]any); ok {
		for _, item := range with {
			appendJavaDescriptionValue(builder, item)
		}
	}
	if extra, ok := desc["extra"].([]any); ok {
		for _, item := range extra {
			appendJavaDescriptionValue(builder, item)
		}
	}
}

func appendJavaDescriptionValue(builder *strings.Builder, value any) {
	switch item := value.(type) {
	case string:
		builder.WriteString(item)
	case map[string]any:
		appendDescriptionText(builder, item)
	case []any:
		for _, nested := range item {
			appendJavaDescriptionValue(builder, nested)
		}
	}
}

func appendJavaFormatting(builder *strings.Builder, desc map[string]any) {
	if colorName, ok := desc["color"].(string); ok {
		if code := javaColorCode(colorName); code != 0 {
			builder.WriteRune('\u00A7')
			builder.WriteRune(code)
		}
	}
	if value, ok := desc["bold"].(bool); ok && value {
		builder.WriteString("\u00A7l")
	}
	if value, ok := desc["italic"].(bool); ok && value {
		builder.WriteString("\u00A7o")
	}
	if value, ok := desc["underlined"].(bool); ok && value {
		builder.WriteString("\u00A7n")
	}
	if value, ok := desc["strikethrough"].(bool); ok && value {
		builder.WriteString("\u00A7m")
	}
}

func javaColorCode(name string) rune {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "black":
		return '0'
	case "dark_blue":
		return '1'
	case "dark_green":
		return '2'
	case "dark_aqua":
		return '3'
	case "dark_red":
		return '4'
	case "dark_purple":
		return '5'
	case "gold":
		return '6'
	case "gray":
		return '7'
	case "dark_gray":
		return '8'
	case "blue":
		return '9'
	case "green":
		return 'a'
	case "aqua":
		return 'b'
	case "red":
		return 'c'
	case "light_purple":
		return 'd'
	case "yellow":
		return 'e'
	case "white":
		return 'f'
	default:
		return 0
	}
}

func parseJavaFavicon(value string) (string, []byte, int, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil, 0, 0
	}
	const marker = ";base64,"
	parts := strings.SplitN(value, marker, 2)
	if len(parts) != 2 {
		return "", nil, 0, 0
	}
	iconType := strings.TrimPrefix(parts[0], "data:")
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, 0, 0
	}
	width, height := 0, 0
	if strings.EqualFold(iconType, "image/png") {
		if config, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
			width = config.Width
			height = config.Height
		}
	}
	return iconType, data, width, height
}
