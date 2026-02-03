package ping

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
)

func resolveToIPv4(ctx context.Context, host string) (string, error) {
	ip := net.ParseIP(host)
	if ip != nil && ip.To4() != nil {
		return host, nil
	}

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
