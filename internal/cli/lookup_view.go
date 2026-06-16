package cli

import (
	"sort"
	"strings"

	"UWP-TCP-Con/internal/ping"
)

type lookupSort string
type lookupFilter string

const (
	lookupSortFound       lookupSort = "found"
	lookupSortHost        lookupSort = "host"
	lookupSortPlayersDesc lookupSort = "players_desc"
	lookupSortLatencyAsc  lookupSort = "latency_asc"
	lookupSortVersion     lookupSort = "version"

	lookupFilterAll         lookupFilter = "all"
	lookupFilterWithPlayers lookupFilter = "with_players"
	lookupFilterEmpty       lookupFilter = "empty"
	lookupFilterWithMOTD    lookupFilter = "with_motd"
	lookupFilterWithIcon    lookupFilter = "with_icon"
)

func (a *App) askLookupSort() (lookupSort, error) {
	options := []string{
		"Found order: Keep discovery order",
		"Host A-Z: Group by hostname",
		"Players: High to low",
		"Latency: Low to high",
		"Version: A-Z",
	}
	index, err := selectOption("Lookup sort", options)
	if err != nil {
		return lookupSortFound, err
	}
	switch index {
	case 1:
		return lookupSortHost, nil
	case 2:
		return lookupSortPlayersDesc, nil
	case 3:
		return lookupSortLatencyAsc, nil
	case 4:
		return lookupSortVersion, nil
	default:
		return lookupSortFound, nil
	}
}

func (a *App) askLookupFilter() (lookupFilter, error) {
	options := []string{
		"All matches: No filter",
		"With players: Online count above zero",
		"Empty servers: Online count is zero",
		"With MOTD: Description present",
		"Java icon: Java servers with icon",
	}
	index, err := selectOption("Lookup filter", options)
	if err != nil {
		return lookupFilterAll, err
	}
	switch index {
	case 1:
		return lookupFilterWithPlayers, nil
	case 2:
		return lookupFilterEmpty, nil
	case 3:
		return lookupFilterWithMOTD, nil
	case 4:
		return lookupFilterWithIcon, nil
	default:
		return lookupFilterAll, nil
	}
}

func lookupSortLabel(value lookupSort) string {
	switch value {
	case lookupSortHost:
		return "Host A-Z"
	case lookupSortPlayersDesc:
		return "Players high to low"
	case lookupSortLatencyAsc:
		return "Latency low to high"
	case lookupSortVersion:
		return "Version A-Z"
	default:
		return "Found order"
	}
}

func lookupFilterLabel(value lookupFilter) string {
	switch value {
	case lookupFilterWithPlayers:
		return "With players"
	case lookupFilterEmpty:
		return "Empty servers"
	case lookupFilterWithMOTD:
		return "With MOTD"
	case lookupFilterWithIcon:
		return "Java icon"
	default:
		return "All matches"
	}
}

func applyLookupView(matches []ping.LookupMatch, sortMode lookupSort, filterMode lookupFilter) []ping.LookupMatch {
	filtered := make([]ping.LookupMatch, 0, len(matches))
	for _, match := range matches {
		if lookupMatchPasses(match, filterMode) {
			filtered = append(filtered, match)
		}
	}

	switch sortMode {
	case lookupSortHost:
		sort.SliceStable(filtered, func(i, j int) bool {
			iHost := strings.ToLower(filtered[i].Host)
			jHost := strings.ToLower(filtered[j].Host)
			if iHost == jHost {
				return filtered[i].Port < filtered[j].Port
			}
			return iHost < jHost
		})
	case lookupSortPlayersDesc:
		sort.SliceStable(filtered, func(i, j int) bool {
			iOnline, _ := lookupPlayerCounts(filtered[i].Result)
			jOnline, _ := lookupPlayerCounts(filtered[j].Result)
			if iOnline == jOnline {
				return compareLookupHostPort(filtered[i], filtered[j])
			}
			return iOnline > jOnline
		})
	case lookupSortLatencyAsc:
		sort.SliceStable(filtered, func(i, j int) bool {
			iLatency := lookupLatency(filtered[i].Result)
			jLatency := lookupLatency(filtered[j].Result)
			if iLatency == jLatency {
				return compareLookupHostPort(filtered[i], filtered[j])
			}
			return iLatency < jLatency
		})
	case lookupSortVersion:
		sort.SliceStable(filtered, func(i, j int) bool {
			iVersion := strings.ToLower(lookupVersion(filtered[i].Result))
			jVersion := strings.ToLower(lookupVersion(filtered[j].Result))
			if iVersion == jVersion {
				return compareLookupHostPort(filtered[i], filtered[j])
			}
			return iVersion < jVersion
		})
	}
	return filtered
}

func compareLookupHostPort(left, right ping.LookupMatch) bool {
	leftHost := strings.ToLower(left.Host)
	rightHost := strings.ToLower(right.Host)
	if leftHost == rightHost {
		return left.Port < right.Port
	}
	return leftHost < rightHost
}

func lookupMatchPasses(match ping.LookupMatch, filterMode lookupFilter) bool {
	online, _ := lookupPlayerCounts(match.Result)
	switch filterMode {
	case lookupFilterWithPlayers:
		return online > 0
	case lookupFilterEmpty:
		return online == 0
	case lookupFilterWithMOTD:
		return strings.TrimSpace(lookupMOTD(match.Result)) != ""
	case lookupFilterWithIcon:
		status, ok := match.Result.(ping.JavaStatus)
		return ok && len(status.IconPNG) > 0
	default:
		return true
	}
}

func lookupPlayerCounts(result ping.Result) (int, int) {
	switch value := result.(type) {
	case ping.BedrockPong:
		return parseCount(value.CurrentPlayers), parseCount(value.MaxPlayers)
	case ping.JavaStatus:
		return value.CurrentPlayers, value.MaxPlayers
	default:
		return 0, 0
	}
}

func lookupLatency(result ping.Result) int64 {
	switch value := result.(type) {
	case ping.JavaStatus:
		return value.LatencyMillis
	default:
		return 1<<62 - 1
	}
}

func lookupVersion(result ping.Result) string {
	switch value := result.(type) {
	case ping.BedrockPong:
		return value.GameVersion
	case ping.JavaStatus:
		return value.VersionName
	default:
		return ""
	}
}

func lookupMOTD(result ping.Result) string {
	switch value := result.(type) {
	case ping.BedrockPong:
		return value.CleanMOTD
	case ping.JavaStatus:
		return value.CleanMOTD
	default:
		return ""
	}
}
