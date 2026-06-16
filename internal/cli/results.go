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
	return fmt.Sprintf(
		"Server\nStatus: online\nEdition: Bedrock\nGame ID: %s\nVersion: %s\nProtocol: %s\nMOTD: %s\nClean MOTD: %s\n\nPlayers\nOnline: %s\nMax: %s",
		value.GameID,
		value.GameVersion,
		value.ProtocolVersion,
		formatMOTD(value.MOTD, options),
		value.CleanMOTD,
		value.CurrentPlayers,
		value.MaxPlayers,
	)
}

func formatJavaSummary(value ping.JavaStatus, options resultFormatOptions) string {
	iconStatus := "not provided"
	if len(value.IconPNG) > 0 {
		iconStatus = "available"
	}
	return fmt.Sprintf(
		"Server\nStatus: online\nEdition: Java\nVersion: %s\nProtocol: %d\nMOTD: %s\nClean MOTD: %s\nServer icon: %s\n\nPlayers\nOnline: %d\nMax: %d\n\nPerformance\nLatency: %d ms",
		value.VersionName,
		value.ProtocolVersion,
		formatMOTD(value.MOTD, options),
		value.CleanMOTD,
		iconStatus,
		value.CurrentPlayers,
		value.MaxPlayers,
		value.LatencyMillis,
	)
}

func formatMOTD(value string, options resultFormatOptions) string {
	if !options.ColorMOTD || !supportsColor() {
		return value
	}
	return ping.RenderFormattingANSI(value)
}
