package ping

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type Result interface {
	String() string
}

func Execute(ctx context.Context, edition Edition, host string, port int) (Result, error) {
	switch edition {
	case EditionJava:
		status, err := PingJava(ctx, host, port)
		if err != nil {
			return nil, err
		}
		return status, nil
	case EditionBedrock:
		pong, err := PingBedrock(ctx, host, port)
		if err != nil {
			return nil, err
		}
		return pong, nil
	default:
		return nil, fmt.Errorf("unbekannte edition: %s", edition)
	}
}

func DefaultPort(edition Edition) int {
	if edition == EditionJava {
		return 25565
	}
	return 19132
}

func ParsePort(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("ungültiger port")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port außerhalb des bereichs 1-65535")
	}
	return port, nil
}
