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

func TestParseJavaStatusFormattingAndFavicon(t *testing.T) {
	payload := []byte(`{"version":{"name":"1.20.4","protocol":765},"players":{"max":20,"online":5},"description":{"text":"Hello ","color":"red","extra":[{"text":"World","bold":true}]},"favicon":"data:image/png;base64,AQID"}`)
	status, err := parseJavaStatus(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.MOTD != "§cHello §lWorld" {
		t.Fatalf("unexpected motd: %q", status.MOTD)
	}
	if status.CleanMOTD != "Hello World" {
		t.Fatalf("unexpected clean motd: %q", status.CleanMOTD)
	}
	if status.IconType != "image/png" {
		t.Fatalf("unexpected icon type: %s", status.IconType)
	}
	if len(status.IconPNG) != 3 {
		t.Fatalf("expected decoded favicon bytes")
	}
}

func TestParseJavaStatusIncludesPlayersSecurityAndForgeData(t *testing.T) {
	payload := []byte(`{
		"version":{"name":"1.20.1 Forge","protocol":763},
		"players":{"max":100,"online":2,"sample":[{"name":"Alex","id":"00000000-0000-0000-0000-000000000001"}]},
		"description":{"text":"Modded"},
		"enforcesSecureChat":true,
		"previewsChat":false,
		"preventsChatReports":true,
		"forgeData":{"channels":[{"res":"forge:handshake"}],"mods":[{"modId":"example","modmarker":"1.4.2"}],"truncated":true,"fmlNetworkVersion":3},
		"customProxyField":"visible"
	}`)

	status, err := parseJavaStatus(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status.PlayerSample) != 1 || status.PlayerSample[0].Name != "Alex" {
		t.Fatalf("unexpected player sample: %+v", status.PlayerSample)
	}
	if status.EnforcesSecureChat == nil || !*status.EnforcesSecureChat {
		t.Fatalf("expected secure chat flag, got %v", status.EnforcesSecureChat)
	}
	if status.PreviewsChat == nil || *status.PreviewsChat {
		t.Fatalf("expected disabled chat previews, got %v", status.PreviewsChat)
	}
	if status.PreventsChatReports == nil || !*status.PreventsChatReports {
		t.Fatalf("expected prevents-chat-reports flag, got %v", status.PreventsChatReports)
	}
	if status.ModLoader != "Forge/FML network 3" || len(status.Mods) != 1 || status.Mods[0].ID != "example" {
		t.Fatalf("unexpected Forge metadata: loader=%q mods=%+v", status.ModLoader, status.Mods)
	}
	if status.ModChannels != 1 || !status.ModDataTruncated {
		t.Fatalf("unexpected Forge channel/truncation data: %+v", status)
	}
	if len(status.ExtraFields) != 1 || status.ExtraFields[0] != "customProxyField" {
		t.Fatalf("unexpected extra status fields: %v", status.ExtraFields)
	}
	if status.StatusJSONBytes != len(payload) {
		t.Fatalf("status JSON bytes = %d, want %d", status.StatusJSONBytes, len(payload))
	}
}

func TestParseJavaStatusReadsFaviconDimensions(t *testing.T) {
	const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	payload := []byte(`{"version":{"name":"1.21","protocol":767},"players":{"max":20,"online":0},"description":"Ready","favicon":"data:image/png;base64,` + onePixelPNG + `"}`)

	status, err := parseJavaStatus(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.IconWidth != 1 || status.IconHeight != 1 {
		t.Fatalf("icon dimensions = %dx%d, want 1x1", status.IconWidth, status.IconHeight)
	}
}
