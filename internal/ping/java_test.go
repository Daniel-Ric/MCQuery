package ping

import "testing"

func TestParseJavaStatusDescription(t *testing.T) {
	payload := []byte(`{"version":{"name":"1.20.4","protocol":765},"players":{"max":20,"online":5},"description":{"text":"Hello ","extra":["World",{"text":"!"}]}}`)
	status, err := parseJavaStatus(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.MOTD != "Hello World!" {
		t.Fatalf("unexpected motd: %s", status.MOTD)
	}
	if status.VersionName != "1.20.4" {
		t.Fatalf("unexpected version name: %s", status.VersionName)
	}
	if status.ProtocolVersion != 765 {
		t.Fatalf("unexpected protocol: %d", status.ProtocolVersion)
	}
	if status.CurrentPlayers != 5 || status.MaxPlayers != 20 {
		t.Fatalf("unexpected players: %d/%d", status.CurrentPlayers, status.MaxPlayers)
	}
}
