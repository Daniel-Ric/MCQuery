package web

import (
	"net/http/httptest"
	"testing"

	"UWP-TCP-Con/internal/ping"
)

func TestParseQueryTargetsFromServers(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/status?server=bedrock,play.example.com,19132,main&server=java,mc.example.com,25565", nil)
	targets := parseQueryTargets(request)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[0].Edition != ping.EditionBedrock {
		t.Fatalf("expected bedrock edition, got %s", targets[0].Edition)
	}
	if targets[0].Host != "play.example.com" {
		t.Fatalf("expected first host to match, got %s", targets[0].Host)
	}
	if targets[0].Port != 19132 {
		t.Fatalf("expected first port to match, got %d", targets[0].Port)
	}
	if targets[0].Name != "main" {
		t.Fatalf("expected first name to match, got %s", targets[0].Name)
	}
	if targets[1].Edition != ping.EditionJava {
		t.Fatalf("expected java edition, got %s", targets[1].Edition)
	}
}

func TestNormalizeTargetDefaultsPort(t *testing.T) {
	target := StatusServerTarget{
		Edition: " Bedrock ",
		Host:    " play.example.com ",
	}

	if err := normalizeTarget(&target); err != nil {
		t.Fatalf("expected target to normalize: %v", err)
	}
	if target.Edition != ping.EditionBedrock {
		t.Fatalf("expected normalized edition, got %s", target.Edition)
	}
	if target.Host != "play.example.com" {
		t.Fatalf("expected normalized host, got %s", target.Host)
	}
	if target.Port != 19132 {
		t.Fatalf("expected default bedrock port, got %d", target.Port)
	}
}

func TestParseStatusPlayers(t *testing.T) {
	players := parseStatusPlayers("42", "100")
	if players == nil {
		t.Fatal("expected players to parse")
	}
	if players.Current != 42 || players.Max != 100 {
		t.Fatalf("expected parsed player counts, got %d/%d", players.Current, players.Max)
	}
}
