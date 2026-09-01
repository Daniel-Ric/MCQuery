package cli

import (
	"strings"
	"testing"

	"UWP-TCP-Con/internal/ping"
)

func TestFormatBedrockSummaryIncludesExtendedPongData(t *testing.T) {
	value := ping.BedrockPong{
		GameID:             "MCPE",
		MOTD:               "Main",
		CleanMOTD:          "Main",
		SubMOTD:            "World",
		GameVersion:        "1.21.111",
		ProtocolVersion:    "827",
		CurrentPlayers:     "3",
		MaxPlayers:         "20",
		ServerID:           "123456789",
		ServerGUID:         987654321,
		GameMode:           "Survival",
		GameModeNumeric:    "1",
		AdvertisedIPv4Port: 19132,
		AdvertisedIPv6Port: 19133,
		LatencyMillis:      14,
	}

	text := formatBedrockSummary(value, resultFormatOptions{})
	for _, expected := range []string{"World / second MOTD: World", "Game mode: Survival", "RakNet GUID: 987654321", "Advertised IPv6 port: 19133", "Latency: 14 ms"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Bedrock summary is missing %q in %q", expected, text)
		}
	}
}

func TestFormatJavaSummaryIncludesStatusExtensions(t *testing.T) {
	secure := true
	preview := false
	value := ping.JavaStatus{
		VersionName:        "1.20.1 Forge",
		ProtocolVersion:    763,
		CurrentPlayers:     1,
		MaxPlayers:         20,
		PlayerSample:       []ping.JavaPlayer{{Name: "Alex", ID: "uuid"}},
		MOTD:               "Modded",
		CleanMOTD:          "Modded",
		LatencyMillis:      22,
		EnforcesSecureChat: &secure,
		PreviewsChat:       &preview,
		ModLoader:          "Forge/FML network 3",
		Mods:               []ping.JavaMod{{ID: "example", Version: "1.4.2"}},
	}

	text := formatJavaSummary(value, resultFormatOptions{})
	for _, expected := range []string{"Alex (uuid)", "Enforces secure chat: yes", "Chat previews: no", "Loader: Forge/FML network 3", "example 1.4.2"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Java summary is missing %q in %q", expected, text)
		}
	}
}

func TestLookupResultsHaveClearServerDividers(t *testing.T) {
	result := ping.LookupResult{
		Matches: []ping.LookupMatch{
			{Host: "one.example.com", Port: 19132, Result: ping.BedrockPong{GameVersion: "1.21", LatencyMillis: 10}},
			{Host: "two.example.com", Port: 19133, Result: ping.BedrockPong{GameVersion: "1.21", LatencyMillis: 12}},
			{Host: "three.example.com", Port: 19134, Result: ping.BedrockPong{GameVersion: "1.21", LatencyMillis: 14}},
		},
		Attempts:  3,
		Completed: 3,
	}

	text := formatLookupResult(result, nil, lookupMetrics{}, resultFormatOptions{})
	if got, want := strings.Count(text, resultEntryDivider), len(result.Matches)-1; got != want {
		t.Fatalf("divider count = %d, want %d in %q", got, want, text)
	}
	for _, expected := range []string{"Match 1 of 3", "Match 2 of 3", "Match 3 of 3"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("lookup results are missing %q in %q", expected, text)
		}
	}
}

func TestBatchResultsHaveClearServerDividers(t *testing.T) {
	results := []batchRunResult{
		{Entry: batchEntry{Edition: ping.EditionBedrock, Host: "one.example.com", Port: 19132}, Result: ping.BedrockPong{GameVersion: "1.21", LatencyMillis: 10}},
		{Entry: batchEntry{Edition: ping.EditionJava, Host: "two.example.com", Port: 25565}, Result: ping.JavaStatus{VersionName: "1.21", LatencyMillis: 12}},
	}

	text := formatBatchResults("Batch check", results, nil, false)
	if got, want := strings.Count(text, resultEntryDivider), 1; got != want {
		t.Fatalf("divider count = %d, want %d in %q", got, want, text)
	}
}
