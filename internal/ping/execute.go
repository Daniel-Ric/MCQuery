package ping

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Result interface {
	String() string
}

type ExecuteConfig struct {
	Edition    Edition
	Host       string
	Port       int
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
	EnableSRV  bool
	IPMode     IPMode
}

type ExecuteOptions struct {
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
	EnableSRV  bool
	IPMode     IPMode
}

type ExecuteDetails struct {
	RequestedHost string
	RequestedPort int
	DialHost      string
	DialPort      int
	SelectedIP    string
	ResolvedIPs   []string
	SRVUsed       bool
	SRVHost       string
	SRVPort       int
	SRVError      string
	Attempts      int
	LastError     string
}

func Execute(ctx context.Context, config ExecuteConfig) (Result, ExecuteDetails, error) {
	if config.Timeout < 0 {
		config.Timeout = 0
	}
	if config.RetryCount < 0 {
		config.RetryCount = 0
	}
	if config.RetryDelay < 0 {
		config.RetryDelay = 0
	}

	details := ExecuteDetails{
		RequestedHost: config.Host,
		RequestedPort: config.Port,
	}

	attempts := config.RetryCount + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx := ctx
		var cancel context.CancelFunc
		if config.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, config.Timeout)
		}
		result, attemptDetails, err := executeOnce(attemptCtx, config)
		if cancel != nil {
			cancel()
		}
		details = mergeDetails(details, attemptDetails)
		details.Attempts = attempt
		if err == nil {
			return result, details, nil
		}
		lastErr = err
		details.LastError = err.Error()
		if attempt < attempts && config.RetryDelay > 0 {
			select {
			case <-ctx.Done():
				return nil, details, ctx.Err()
			case <-time.After(config.RetryDelay):
			}
		}
	}
	if lastErr != nil {
		return nil, details, lastErr
	}
	return nil, details, fmt.Errorf("request failed")
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
		return 0, fmt.Errorf("invalid port")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range (1-65535)")
	}
	return port, nil
}

func executeOnce(ctx context.Context, config ExecuteConfig) (Result, ExecuteDetails, error) {
	switch config.Edition {
	case EditionJava:
		return executeJava(ctx, config)
	case EditionBedrock:
		return executeBedrock(ctx, config)
	default:
		return nil, ExecuteDetails{}, fmt.Errorf("unknown edition: %s", config.Edition)
	}
}

func mergeDetails(base ExecuteDetails, next ExecuteDetails) ExecuteDetails {
	if next.DialHost != "" {
		base.DialHost = next.DialHost
	}
	if next.DialPort != 0 {
		base.DialPort = next.DialPort
	}
	if next.SelectedIP != "" {
		base.SelectedIP = next.SelectedIP
	}
	if len(next.ResolvedIPs) > 0 {
		base.ResolvedIPs = next.ResolvedIPs
	}
	if next.SRVUsed {
		base.SRVUsed = true
	}
	if next.SRVHost != "" {
		base.SRVHost = next.SRVHost
	}
	if next.SRVPort != 0 {
		base.SRVPort = next.SRVPort
	}
	if next.SRVError != "" {
		base.SRVError = next.SRVError
	}
	return base
}
