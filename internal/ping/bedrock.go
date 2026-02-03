package ping

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
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
		return BedrockPong{}, fmt.Errorf("pong zu kurz: %d bytes", len(buf))
	}

	if buf[0] != 0x1c {
		return BedrockPong{}, fmt.Errorf("unerwartete packet id: 0x%02x", buf[0])
	}

	nameLen := int(binary.BigEndian.Uint16(buf[33:35]))
	if 35+nameLen > len(buf) {
		return BedrockPong{}, fmt.Errorf("ungültige advertise länge: %d (buf=%d)", nameLen, len(buf))
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

func buildUnconnectedPing() ([]byte, error) {
	buf := make([]byte, 1+8+len(magic)+8)

	buf[0] = 0x01
	binary.BigEndian.PutUint64(buf[1:9], uint64(time.Now().UnixMilli()))
	copy(buf[9:9+len(magic)], magic)
	binary.BigEndian.PutUint64(buf[25:33], 0)

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
			return BedrockPong{}, fmt.Errorf("timeout beim ping von %s:%d", host, port)
		}
		return BedrockPong{}, err
	}

	return parsePong(buf[:n])
}
