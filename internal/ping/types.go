package ping

import (
	"fmt"
	"regexp"
	"strings"
)

type Edition string

const (
	EditionBedrock Edition = "bedrock"
	EditionJava    Edition = "java"
)

type BedrockPong struct {
	GameID             string
	MOTD               string
	CleanMOTD          string
	ProtocolVersion    string
	GameVersion        string
	CurrentPlayers     string
	MaxPlayers         string
	ServerID           string
	ServerGUID         uint64
	SubMOTD            string
	CleanSubMOTD       string
	GameMode           string
	GameModeNumeric    string
	AdvertisedIPv4Port int
	AdvertisedIPv6Port int
	LatencyMillis      int64
	ResponseBytes      int
	ExtraFields        []string
	RawAdvertisement   string
}

type JavaPlayer struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

type JavaMod struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

type JavaStatus struct {
	VersionName         string
	ProtocolVersion     int
	CurrentPlayers      int
	MaxPlayers          int
	PlayerSample        []JavaPlayer
	MOTD                string
	CleanMOTD           string
	LatencyMillis       int64
	IconPNG             []byte
	IconType            string
	IconWidth           int
	IconHeight          int
	EnforcesSecureChat  *bool
	PreviewsChat        *bool
	PreventsChatReports *bool
	ModLoader           string
	Mods                []JavaMod
	ModChannels         int
	ModDataTruncated    bool
	StatusJSONBytes     int
	ExtraFields         []string
}

var mcFormatRE = regexp.MustCompile(`(?i)\x{00A7}[0-9A-FK-OR]`)

func stripMCFormatting(s string) string {
	return mcFormatRE.ReplaceAllString(s, "")
}

func StripFormatting(s string) string {
	return stripMCFormatting(s)
}

func RenderFormattingANSI(s string) string {
	var builder strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\u00A7' || i+1 >= len(runes) {
			builder.WriteRune(runes[i])
			continue
		}

		i++
		code := runes[i]
		if code >= 'A' && code <= 'Z' {
			code += 'a' - 'A'
		}
		if seq := minecraftANSISequence(code); seq != "" {
			builder.WriteString(seq)
		}
	}
	builder.WriteString("\033[0m")
	return builder.String()
}

func minecraftANSISequence(code rune) string {
	switch code {
	case '0':
		return "\033[30m"
	case '1':
		return "\033[34m"
	case '2':
		return "\033[32m"
	case '3':
		return "\033[36m"
	case '4':
		return "\033[31m"
	case '5':
		return "\033[35m"
	case '6':
		return "\033[33m"
	case '7':
		return "\033[37m"
	case '8':
		return "\033[90m"
	case '9':
		return "\033[94m"
	case 'a':
		return "\033[92m"
	case 'b':
		return "\033[96m"
	case 'c':
		return "\033[91m"
	case 'd':
		return "\033[95m"
	case 'e':
		return "\033[93m"
	case 'f':
		return "\033[97m"
	case 'l':
		return "\033[1m"
	case 'm':
		return "\033[9m"
	case 'n':
		return "\033[4m"
	case 'o':
		return "\033[3m"
	case 'r':
		return "\033[0m"
	default:
		return ""
	}
}

func (p BedrockPong) String() string {
	return fmt.Sprintf(
		"Edition: Bedrock\nGameID: %s\nMOTD: %s\nCleanMOTD: %s\nSubMOTD: %s\nProtocolVersion: %s\nGameVersion: %s\nPlayers: %s/%s\nGameMode: %s (%s)\nServerID: %s\nServerGUID: %d\nAdvertisedPorts: IPv4=%d IPv6=%d\nLatency(ms): %d",
		p.GameID,
		p.MOTD,
		p.CleanMOTD,
		p.SubMOTD,
		p.ProtocolVersion,
		p.GameVersion,
		p.CurrentPlayers,
		p.MaxPlayers,
		p.GameMode,
		p.GameModeNumeric,
		p.ServerID,
		p.ServerGUID,
		p.AdvertisedIPv4Port,
		p.AdvertisedIPv6Port,
		p.LatencyMillis,
	)
}

func (s JavaStatus) String() string {
	return fmt.Sprintf(
		"Edition: Java\nMOTD: %s\nCleanMOTD: %s\nVersion: %s\nProtocol: %d\nPlayers: %d/%d\nPlayerSample: %d\nLatency(ms): %d\nModLoader: %s\nMods: %d",
		s.MOTD,
		s.CleanMOTD,
		s.VersionName,
		s.ProtocolVersion,
		s.CurrentPlayers,
		s.MaxPlayers,
		len(s.PlayerSample),
		s.LatencyMillis,
		s.ModLoader,
		len(s.Mods),
	)
}
