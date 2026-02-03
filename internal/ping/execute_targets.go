package ping

import (
	"context"
	"fmt"
	"net"
)

func executeJava(ctx context.Context, config ExecuteConfig) (Result, ExecuteDetails, error) {
	details := ExecuteDetails{
		RequestedHost: config.Host,
		RequestedPort: config.Port,
		DialHost:      config.Host,
		DialPort:      config.Port,
	}

	dialHost := config.Host
	dialPort := config.Port
	if config.EnableSRV {
		srvHost, srvPort, err := resolveJavaSRV(ctx, config.Host)
		if err == nil {
			dialHost = srvHost
			dialPort = srvPort
			details.SRVUsed = true
			details.SRVHost = srvHost
			details.SRVPort = srvPort
		} else {
			details.SRVError = err.Error()
		}
	}

	selectedIP, resolved, err := resolveIP(ctx, dialHost, config.IPMode)
	if err != nil {
		return nil, details, err
	}
	details.DialHost = dialHost
	details.DialPort = dialPort
	details.SelectedIP = selectedIP
	details.ResolvedIPs = resolved

	status, err := PingJava(ctx, selectedIP, config.Host, dialPort)
	if err != nil {
		return nil, details, err
	}
	return status, details, nil
}

func executeBedrock(ctx context.Context, config ExecuteConfig) (Result, ExecuteDetails, error) {
	details := ExecuteDetails{
		RequestedHost: config.Host,
		RequestedPort: config.Port,
		DialHost:      config.Host,
		DialPort:      config.Port,
	}

	selectedIP, resolved, err := resolveIP(ctx, config.Host, config.IPMode)
	if err != nil {
		return nil, details, err
	}
	details.SelectedIP = selectedIP
	details.ResolvedIPs = resolved

	ip := net.ParseIP(selectedIP)
	if ip == nil {
		return nil, details, fmt.Errorf("invalid IP address: %s", selectedIP)
	}

	pong, err := PingBedrock(ctx, ip, config.Host, config.Port)
	if err != nil {
		return nil, details, err
	}
	return pong, details, nil
}
