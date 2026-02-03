package ping

import (
	"fmt"
	"regexp"
)

type Edition string

const (
	EditionBedrock Edition = "bedrock"
	EditionJava    Edition = "java"
)

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

var mcFormatRE = regexp.MustCompile(`(?i)\x{00A7}[0-9A-FK-OR]`)

func stripMCFormatting(s string) string {
	return mcFormatRE.ReplaceAllString(s, "")
}

func (p BedrockPong) String() string {
	return fmt.Sprintf(
		"Edition: Bedrock\nGameID: %s\nMOTD: %s\nCleanMOTD: %s\nProtocolVersion: %s\nGameVersion: %s\nPlayers: %s/%s",
		p.GameID,
		p.MOTD,
		p.CleanMOTD,
		p.ProtocolVersion,
		p.GameVersion,
		p.CurrentPlayers,
		p.MaxPlayers,
	)
}

func (s JavaStatus) String() string {
	return fmt.Sprintf(
		"Edition: Java\nMOTD: %s\nCleanMOTD: %s\nVersion: %s\nProtocol: %d\nPlayers: %d/%d\nLatency(ms): %d",
		s.MOTD,
		s.CleanMOTD,
		s.VersionName,
		s.ProtocolVersion,
		s.CurrentPlayers,
		s.MaxPlayers,
		s.LatencyMillis,
	)
}
