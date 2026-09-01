package ping

import (
	"encoding/binary"
	"testing"
)

func TestParsePongIncludesCompleteBedrockAdvertisement(t *testing.T) {
	advertisement := "MCPE;Main §aMOTD;827;1.21.111;12;50;13253860892328930865;Survival world;Survival;1;19132;19133;proxy-extra;"
	packet := make([]byte, 35+len(advertisement))
	packet[0] = 0x1c
	binary.BigEndian.PutUint64(packet[1:9], 123456789)
	binary.BigEndian.PutUint64(packet[9:17], 987654321)
	copy(packet[17:33], magic)
	binary.BigEndian.PutUint16(packet[33:35], uint16(len(advertisement)))
	copy(packet[35:], advertisement)

	pong, err := parsePong(packet)
	if err != nil {
		t.Fatalf("parsePong returned an error: %v", err)
	}

	if pong.GameID != "MCPE" || pong.GameVersion != "1.21.111" || pong.ProtocolVersion != "827" {
		t.Fatalf("unexpected edition/version fields: %+v", pong)
	}
	if pong.CleanMOTD != "Main MOTD" || pong.SubMOTD != "Survival world" {
		t.Fatalf("unexpected MOTD fields: %+v", pong)
	}
	if pong.ServerID != "13253860892328930865" || pong.ServerGUID != 987654321 {
		t.Fatalf("unexpected server identities: %+v", pong)
	}
	if pong.GameMode != "Survival" || pong.GameModeNumeric != "1" {
		t.Fatalf("unexpected game mode: %+v", pong)
	}
	if pong.AdvertisedIPv4Port != 19132 || pong.AdvertisedIPv6Port != 19133 {
		t.Fatalf("unexpected advertised ports: %+v", pong)
	}
	if pong.ResponseBytes != len(packet) {
		t.Fatalf("response bytes = %d, want %d", pong.ResponseBytes, len(packet))
	}
	if len(pong.ExtraFields) != 1 || pong.ExtraFields[0] != "proxy-extra" {
		t.Fatalf("unexpected extra fields: %v", pong.ExtraFields)
	}
}

func TestParseAdvertisedPortRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "0", "65536", "not-a-port"} {
		if got := parseAdvertisedPort(value); got != 0 {
			t.Fatalf("parseAdvertisedPort(%q) = %d, want 0", value, got)
		}
	}
}

func TestParsePongRejectsInvalidRakNetMagic(t *testing.T) {
	packet := make([]byte, 35)
	packet[0] = 0x1c

	if _, err := parsePong(packet); err == nil {
		t.Fatal("expected invalid RakNet magic to be rejected")
	}
}
