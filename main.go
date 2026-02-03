package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Special thanks to https://minecraft.wiki/w/RakNet
// RakNet "Unconnected Ping" magic:
var magic = mustHex("00ffff00fefefefefdfdfdfd12345678")

type BedrockPong struct {
	GameID          string
	MOTD            string
	ProtocolVersion string
	GameVersion     string
	CurrentPlayers  string
	MaxPlayers      string
	CleanMOTD       string
}

type JavaStatus struct {
	VersionName     string
	ProtocolVersion int
	CurrentPlayers  int
	MaxPlayers      int
	MOTD            string
	CleanMOTD       string
	LatencyMillis   int64
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// parsePong matches the offsets used in your JS script:
// nameLength at offset 33, advertise string starts at offset 35.
func parsePong(buf []byte) (BedrockPong, error) {
	// Minimum size to safely read uint16 at 33
	if len(buf) < 35 {
		return BedrockPong{}, fmt.Errorf("pong too short: %d bytes", len(buf))
	}

	// Optional: verify packet ID (0x1c is Unconnected Pong)
	// Not strictly required, but helpful.
	if buf[0] != 0x1c {
		return BedrockPong{}, fmt.Errorf("unexpected packet id: 0x%02x", buf[0])
	}

	nameLen := int(binary.BigEndian.Uint16(buf[33:35]))
	if 35+nameLen > len(buf) {
		return BedrockPong{}, fmt.Errorf("invalid advertise length: %d (buf=%d)", nameLen, len(buf))
	}

	advertise := string(buf[35 : 35+nameLen])
	parts := strings.Split(advertise, ";")

	get := func(i int) string {
		if i >= 0 && i < len(parts) {
			return parts[i]
		}
		return ""
	}

	motd := get(1)
	clean := stripMCFormatting(motd)

	return BedrockPong{
		GameID:          get(0),
		MOTD:            motd,
		ProtocolVersion: get(2),
		GameVersion:     get(3),
		CurrentPlayers:  get(4),
		MaxPlayers:      get(5),
		CleanMOTD:       clean,
	}, nil
}

// Removes "§" formatting codes (matches your JS regex intent)
var mcFormatRE = regexp.MustCompile(`(?i)\x{00A7}[0-9A-FK-OR]`)

func stripMCFormatting(s string) string {
	return mcFormatRE.ReplaceAllString(s, "")
}

func resolveToIPv4(ctx context.Context, host string) (string, error) {
	// If it's already an IPv4 string, keep it.
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() != nil {
		return host, nil
	}

	// DNS lookup and pick the first IPv4.
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if a.IP.To4() != nil {
			return a.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found for %s", host)
}

func buildUnconnectedPing() ([]byte, error) {
	// JS builds: 1 + 8 + magic(16) + 8
	// [0]=0x01, [1..8]=timestamp, [9..24]=magic, [25..32]=client GUID
	buf := make([]byte, 1+8+len(magic)+8)

	buf[0] = 0x01
	binary.BigEndian.PutUint64(buf[1:9], uint64(time.Now().UnixMilli()))
	copy(buf[9:9+len(magic)], magic)
	binary.BigEndian.PutUint64(buf[25:33], 0) // client GUID = 0

	return buf, nil
}

func PingBedrock(ctx context.Context, host string, port int) (BedrockPong, error) {
	ipv4, err := resolveToIPv4(ctx, host)
	if err != nil {
		return BedrockPong{}, err
	}

	raddr := &net.UDPAddr{IP: net.ParseIP(ipv4), Port: port}
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return BedrockPong{}, err
	}
	defer conn.Close()

	pingPacket, err := buildUnconnectedPing()
	if err != nil {
		return BedrockPong{}, err
	}

	// Sender goroutine: send every 50ms until we stop.
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		// send immediately once
		_, _ = conn.Write(pingPacket)

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, _ = conn.Write(pingPacket)
			}
		}
	}()

	// Read with context deadline/timeout
	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetReadDeadline(deadline)
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		// Normalize timeout errors
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return BedrockPong{}, fmt.Errorf("timeout pinging %s:%d", host, port)
		}
		return BedrockPong{}, err
	}

	return parsePong(buf[:n])
}

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
	writeVarInt(payload, 0x00)                                                    // Packet ID
	writeVarInt(payload, protocolVersion)                                         // Protocol version
	writeString(payload, host)                                                    // Server address
	if err := binary.Write(payload, binary.BigEndian, uint16(port)); err != nil { // Server port
		return err
	}
	writeVarInt(payload, 0x01) // Next state: status

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

func writePacket(w io.Writer, payload []byte) error {
	header := &bytes.Buffer{}
	writeVarInt(header, len(payload))
	if _, err := w.Write(header.Bytes()); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readPacket(r io.Reader) ([]byte, error) {
	length, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeVarInt(w io.Writer, value int) {
	for {
		if (value & ^0x7F) == 0 {
			_, _ = w.Write([]byte{byte(value)})
			return
		}
		_, _ = w.Write([]byte{byte(value&0x7F | 0x80)})
		value >>= 7
	}
}

func readVarInt(r io.Reader) (int, error) {
	var numRead int
	var result int
	for {
		if numRead > 5 {
			return 0, fmt.Errorf("varint too long")
		}
		var buf [1]byte
		if _, err := r.Read(buf[:]); err != nil {
			return 0, err
		}
		value := int(buf[0] & 0x7F)
		result |= value << (7 * numRead)

		numRead++
		if (buf[0] & 0x80) == 0 {
			break
		}
	}
	return result, nil
}

func readString(r io.Reader) (string, error) {
	length, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", fmt.Errorf("invalid string length: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func writeString(w io.Writer, value string) {
	writeVarInt(w, len(value))
	_, _ = w.Write([]byte(value))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <host> [port] [edition]")
		fmt.Println("Example (Bedrock): go run main.go play.example.com 19132 bedrock")
		fmt.Println("Example (Java): go run main.go play.example.com 25565 java")
		os.Exit(1)
	}

	host := os.Args[1]
	edition := "bedrock"
	port := 19132

	if len(os.Args) >= 3 {
		if os.Args[2] == "java" || os.Args[2] == "bedrock" {
			edition = os.Args[2]
		} else {
			p, err := strconv.Atoi(os.Args[2])
			if err != nil {
				fmt.Println("Invalid port:", os.Args[2])
				os.Exit(1)
			}
			port = p
		}
	}

	if len(os.Args) >= 4 {
		if os.Args[3] == "java" || os.Args[3] == "bedrock" {
			edition = os.Args[3]
		} else {
			fmt.Println("Invalid edition:", os.Args[3])
			os.Exit(1)
		}
	}

	if edition == "java" && port == 19132 {
		port = 25565
	}

	// Timeout: 3 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if edition == "java" {
		status, err := PingJava(ctx, host, port)
		if err != nil {
			fmt.Println("Ping failed:", err)
			os.Exit(1)
		}

		fmt.Println("MOTD:", status.MOTD)
		fmt.Println("CleanMOTD:", status.CleanMOTD)
		fmt.Println("Version:", status.VersionName)
		fmt.Println("Protocol:", status.ProtocolVersion)
		fmt.Println("Players:", fmt.Sprintf("%d/%d", status.CurrentPlayers, status.MaxPlayers))
		fmt.Println("Latency(ms):", status.LatencyMillis)
		return
	}

	pong, err := PingBedrock(ctx, host, port)
	if err != nil {
		fmt.Println("Ping failed:", err)
		os.Exit(1)
	}

	fmt.Println("GameID:", pong.GameID)
	fmt.Println("MOTD:", pong.MOTD)
	fmt.Println("CleanMOTD:", pong.CleanMOTD)
	fmt.Println("ProtocolVersion:", pong.ProtocolVersion)
	fmt.Println("GameVersion:", pong.GameVersion)
	fmt.Println("Players:", pong.CurrentPlayers+"/"+pong.MaxPlayers)
}
