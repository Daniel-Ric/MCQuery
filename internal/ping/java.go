package ping

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func PingJava(ctx context.Context, host string, port int) (JavaStatus, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return JavaStatus{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	}

	if err := writeHandshake(conn, host, port); err != nil {
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
		return JavaStatus{}, fmt.Errorf("unerwartete status packet id: %d", packetID)
	}

	statusJSON, err := readString(respReader)
	if err != nil {
		return JavaStatus{}, err
	}

	status, err := parseJavaStatus([]byte(statusJSON))
	if err != nil {
		return JavaStatus{}, err
	}

	latency, err := pingJavaLatency(conn)
	if err != nil {
		return JavaStatus{}, err
	}
	status.LatencyMillis = latency

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
		return 0, fmt.Errorf("unerwartete pong packet id: %d", packetID)
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
		} `json:"players"`
		Description any `json:"description"`
	}

	var raw rawStatus
	if err := json.Unmarshal(data, &raw); err != nil {
		return JavaStatus{}, err
	}

	motd := extractJavaDescription(raw.Description)
	return JavaStatus{
		VersionName:     raw.Version.Name,
		ProtocolVersion: raw.Version.Protocol,
		CurrentPlayers:  raw.Players.Online,
		MaxPlayers:      raw.Players.Max,
		MOTD:            motd,
		CleanMOTD:       stripMCFormatting(motd),
	}, nil
}

func extractJavaDescription(desc any) string {
	switch v := desc.(type) {
	case string:
		return v
	case map[string]any:
		var builder strings.Builder
		appendDescriptionText(&builder, v)
		return builder.String()
	default:
		return ""
	}
}

func appendDescriptionText(builder *strings.Builder, desc map[string]any) {
	if text, ok := desc["text"].(string); ok {
		builder.WriteString(text)
	}
	if extra, ok := desc["extra"].([]any); ok {
		for _, item := range extra {
			switch itemValue := item.(type) {
			case string:
				builder.WriteString(itemValue)
			case map[string]any:
				appendDescriptionText(builder, itemValue)
			}
		}
	}
}
