package cli

import (
	"fmt"
	"strings"

	"UWP-TCP-Con/internal/ping"
)

type resultFormatOptions struct {
	Verbose   bool
	ColorMOTD bool
}

const resultEntryDivider = "━━━━━━━━━ NEXT SERVER ━━━━━━━━━"

func writeResultEntryDivider(builder *strings.Builder, index int) {
	if index <= 0 {
		return
	}
	builder.WriteString("\n")
	builder.WriteString(resultEntryDivider)
	builder.WriteString("\n\n")
}

func formatDirectResult(result ping.Result, details ping.ExecuteDetails, options resultFormatOptions) string {
	var builder strings.Builder
	builder.WriteString(formatResultSummary(result, options))
	if options.Verbose {
		builder.WriteString("\n")
		builder.WriteString("Debug\n")
		builder.WriteString(fmt.Sprintf("Requested: %s:%d\n", details.RequestedHost, details.RequestedPort))
		if details.DialHost != "" {
			builder.WriteString(fmt.Sprintf("Dial target: %s:%d\n", details.DialHost, details.DialPort))
		}
		if details.SelectedIP != "" {
			builder.WriteString(fmt.Sprintf("Selected IP: %s\n", details.SelectedIP))
		}
		if len(details.ResolvedIPs) > 0 {
			builder.WriteString(fmt.Sprintf("Resolved IPs: %s\n", strings.Join(details.ResolvedIPs, ", ")))
		}
		if details.SRVUsed {
			builder.WriteString(fmt.Sprintf("SRV: %s:%d\n", details.SRVHost, details.SRVPort))
		} else if details.SRVError != "" {
			builder.WriteString(fmt.Sprintf("SRV error: %s\n", details.SRVError))
		}
		if details.Attempts > 0 {
			builder.WriteString(fmt.Sprintf("Attempts: %d\n", details.Attempts))
		}
		if details.LastError != "" {
			builder.WriteString(fmt.Sprintf("Last error: %s\n", details.LastError))
		}
	}
	return builder.String()
}

func formatResultSummary(result ping.Result, options resultFormatOptions) string {
	switch value := result.(type) {
	case ping.BedrockPong:
		return formatBedrockSummary(value, options)
	case ping.JavaStatus:
		return formatJavaSummary(value, options)
	case nil:
		return "Server\nStatus: unavailable"
	default:
		return result.String()
	}
}

func formatBedrockSummary(value ping.BedrockPong, options resultFormatOptions) string {
	var builder strings.Builder
	builder.WriteString("Server\n")
	builder.WriteString("Status: online\n")
	builder.WriteString("Edition: Bedrock\n")
	writeResultValue(&builder, "Edition code", value.GameID)
	writeResultValue(&builder, "Version", value.GameVersion)
	writeResultValue(&builder, "Protocol", value.ProtocolVersion)

	builder.WriteString("\nWorld\n")
	writeResultValue(&builder, "MOTD", formatMOTD(value.MOTD, options))
	writeResultValue(&builder, "Clean MOTD", value.CleanMOTD)
	writeResultValue(&builder, "World / second MOTD", formatMOTD(value.SubMOTD, options))
	if value.CleanSubMOTD != "" && value.CleanSubMOTD != value.SubMOTD {
		writeResultValue(&builder, "Clean world name", value.CleanSubMOTD)
	}
	writeResultValue(&builder, "Game mode", value.GameMode)
	writeResultValue(&builder, "Game mode ID", value.GameModeNumeric)

	builder.WriteString("\nPlayers\n")
	writeResultValue(&builder, "Online", value.CurrentPlayers)
	writeResultValue(&builder, "Max", value.MaxPlayers)

	builder.WriteString("\nNetwork\n")
	if value.LatencyMillis >= 0 {
		builder.WriteString(fmt.Sprintf("Latency: %d ms\n", value.LatencyMillis))
	}
	writeResultInt(&builder, "Advertised IPv4 port", value.AdvertisedIPv4Port)
	writeResultInt(&builder, "Advertised IPv6 port", value.AdvertisedIPv6Port)

	builder.WriteString("\nIdentity\n")
	writeResultValue(&builder, "Server ID", value.ServerID)
	if value.ServerGUID != 0 {
		builder.WriteString(fmt.Sprintf("RakNet GUID: %d\n", value.ServerGUID))
	}
	if options.Verbose {
		writeResultInt(&builder, "Response size", value.ResponseBytes)
		if len(value.ExtraFields) > 0 {
			writeResultValue(&builder, "Extension fields", strings.Join(value.ExtraFields, ", "))
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func formatJavaSummary(value ping.JavaStatus, options resultFormatOptions) string {
	iconStatus := "not provided"
	if len(value.IconPNG) > 0 {
		iconStatus = fmt.Sprintf("available (%s, %d bytes", value.IconType, len(value.IconPNG))
		if value.IconWidth > 0 && value.IconHeight > 0 {
			iconStatus += fmt.Sprintf(", %dx%d", value.IconWidth, value.IconHeight)
		}
		iconStatus += ")"
	}
	var builder strings.Builder
	builder.WriteString("Server\n")
	builder.WriteString("Status: online\n")
	builder.WriteString("Edition: Java\n")
	writeResultValue(&builder, "Version", value.VersionName)
	builder.WriteString(fmt.Sprintf("Protocol: %d\n", value.ProtocolVersion))
	writeResultValue(&builder, "MOTD", formatMOTD(value.MOTD, options))
	writeResultValue(&builder, "Clean MOTD", value.CleanMOTD)
	writeResultValue(&builder, "Server icon", iconStatus)

	builder.WriteString("\nPlayers\n")
	builder.WriteString(fmt.Sprintf("Online: %d\nMax: %d\n", value.CurrentPlayers, value.MaxPlayers))
	if len(value.PlayerSample) > 0 {
		builder.WriteString(fmt.Sprintf("Advertised sample: %d player(s)\n", len(value.PlayerSample)))
		limit := minInt(len(value.PlayerSample), 10)
		for _, player := range value.PlayerSample[:limit] {
			line := player.Name
			if player.ID != "" {
				line += " (" + player.ID + ")"
			}
			builder.WriteString("- " + line + "\n")
		}
		if remaining := len(value.PlayerSample) - limit; remaining > 0 {
			builder.WriteString(fmt.Sprintf("- … %d more advertised player(s)\n", remaining))
		}
	}

	if value.EnforcesSecureChat != nil || value.PreviewsChat != nil || value.PreventsChatReports != nil {
		builder.WriteString("\nSecurity\n")
		writeResultBool(&builder, "Enforces secure chat", value.EnforcesSecureChat)
		writeResultBool(&builder, "Chat previews", value.PreviewsChat)
		writeResultBool(&builder, "Prevents chat reports", value.PreventsChatReports)
	}

	if value.ModLoader != "" || len(value.Mods) > 0 || value.ModChannels > 0 {
		builder.WriteString("\nModding\n")
		writeResultValue(&builder, "Loader", value.ModLoader)
		writeResultInt(&builder, "Advertised channels", value.ModChannels)
		if len(value.Mods) > 0 {
			builder.WriteString(fmt.Sprintf("Advertised mods: %d\n", len(value.Mods)))
			limit := minInt(len(value.Mods), 12)
			for _, mod := range value.Mods[:limit] {
				line := mod.ID
				if mod.Version != "" {
					line += " " + mod.Version
				}
				builder.WriteString("- " + line + "\n")
			}
			if remaining := len(value.Mods) - limit; remaining > 0 {
				builder.WriteString(fmt.Sprintf("- … %d more advertised mod(s)\n", remaining))
			}
		}
		if value.ModDataTruncated {
			builder.WriteString("[WARN] Server marked its mod metadata as truncated\n")
		}
	}

	builder.WriteString("\nPerformance\n")
	builder.WriteString(fmt.Sprintf("Latency: %d ms\n", value.LatencyMillis))
	if options.Verbose {
		writeResultInt(&builder, "Status JSON size", value.StatusJSONBytes)
		if len(value.ExtraFields) > 0 {
			writeResultValue(&builder, "Custom status fields", strings.Join(value.ExtraFields, ", "))
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func writeResultValue(builder *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	builder.WriteString(label + ": " + value + "\n")
}

func writeResultInt(builder *strings.Builder, label string, value int) {
	if value <= 0 {
		return
	}
	builder.WriteString(fmt.Sprintf("%s: %d\n", label, value))
}

func writeResultBool(builder *strings.Builder, label string, value *bool) {
	if value == nil {
		return
	}
	text := "no"
	if *value {
		text = "yes"
	}
	builder.WriteString(label + ": " + text + "\n")
}

func formatMOTD(value string, options resultFormatOptions) string {
	if !options.ColorMOTD || !supportsColor() {
		return value
	}
	return ping.RenderFormattingANSI(value)
}
