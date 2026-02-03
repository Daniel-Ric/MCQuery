package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <host> [port]")
		fmt.Println("Example: go run main.go play.example.com 19132")
		os.Exit(1)
	}

	host := os.Args[1]
	port := 19132
	if len(os.Args) >= 3 {
		p, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("Invalid port:", os.Args[2])
			os.Exit(1)
		}
		port = p
	}

	// Timeout: 3 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

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
