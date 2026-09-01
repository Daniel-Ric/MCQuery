package ping

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var magic = mustHex("00ffff00fefefefefdfdfdfd12345678")

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func parsePong(buf []byte) (BedrockPong, error) {
	if len(buf) < 35 {
		return BedrockPong{}, fmt.Errorf("pong too short: %d bytes", len(buf))
	}

	if buf[0] != 0x1c {
		return BedrockPong{}, fmt.Errorf("unexpected packet id: 0x%02x", buf[0])
	}
	if !bytes.Equal(buf[17:33], magic) {
		return BedrockPong{}, fmt.Errorf("invalid RakNet offline message magic")
	}

	nameLen := int(binary.BigEndian.Uint16(buf[33:35]))
	if 35+nameLen > len(buf) {
		return BedrockPong{}, fmt.Errorf("invalid advertise length: %d (buf=%d)", nameLen, len(buf))
	}

	advertise := string(buf[35 : 35+nameLen])
	parts := strings.Split(strings.TrimRight(advertise, ";"), ";")

	get := func(i int) string {
		if i >= 0 && i < len(parts) {
			return parts[i]
		}
		return ""
	}

	motd := get(1)
	subMOTD := get(7)
	extraFields := []string(nil)
	if len(parts) > 12 {
		extraFields = append(extraFields, parts[12:]...)
	}

	return BedrockPong{
		GameID:             get(0),
		MOTD:               motd,
		CleanMOTD:          stripMCFormatting(motd),
		ProtocolVersion:    get(2),
		GameVersion:        get(3),
		CurrentPlayers:     get(4),
		MaxPlayers:         get(5),
		ServerID:           get(6),
		ServerGUID:         binary.BigEndian.Uint64(buf[9:17]),
		SubMOTD:            subMOTD,
		CleanSubMOTD:       stripMCFormatting(subMOTD),
		GameMode:           get(8),
		GameModeNumeric:    get(9),
		AdvertisedIPv4Port: parseAdvertisedPort(get(10)),
		AdvertisedIPv6Port: parseAdvertisedPort(get(11)),
		LatencyMillis:      -1,
		ResponseBytes:      len(buf),
		ExtraFields:        extraFields,
		RawAdvertisement:   advertise,
	}, nil
}

func parseAdvertisedPort(value string) int {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}

func buildUnconnectedPing() ([]byte, error) {
	buf := make([]byte, 1+8+len(magic)+8)

	buf[0] = 0x01
	binary.BigEndian.PutUint64(buf[1:9], uint64(time.Now().UnixMilli()))
	copy(buf[9:9+len(magic)], magic)
	binary.BigEndian.PutUint64(buf[25:33], 0)

	return buf, nil
}

func PingBedrock(ctx context.Context, ip net.IP, host string, port int) (BedrockPong, error) {
	network := "udp6"
	if ip.To4() != nil {
		network = "udp4"
	}

	raddr := &net.UDPAddr{IP: ip, Port: port}
	conn, err := net.DialUDP(network, nil, raddr)
	if err != nil {
		return BedrockPong{}, err
	}
	defer conn.Close()

	pingPacket, err := buildUnconnectedPing()
	if err != nil {
		return BedrockPong{}, err
	}

	stop := make(chan struct{})
	defer close(stop)

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

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

	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetReadDeadline(deadline)
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return BedrockPong{}, fmt.Errorf("timeout while pinging %s:%d", host, port)
		}
		return BedrockPong{}, err
	}

	pong, err := parsePong(buf[:n])
	if err != nil {
		return BedrockPong{}, err
	}
	sentMillis := int64(binary.BigEndian.Uint64(buf[1:9]))
	latency := time.Now().UnixMilli() - sentMillis
	if latency >= 0 && latency <= int64((24*time.Hour)/time.Millisecond) {
		pong.LatencyMillis = latency
	}
	return pong, nil
}
