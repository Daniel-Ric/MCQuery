package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
)

type IPMode string

const (
	IPModeAuto IPMode = "auto"
	IPModeIPv4 IPMode = "ipv4"
	IPModeIPv6 IPMode = "ipv6"
)

func resolveIP(ctx context.Context, host string, mode IPMode) (string, []string, error) {
	if host == "" {
		return "", nil, fmt.Errorf("host cannot be empty")
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if !matchesMode(ip, mode) {
			return "", nil, fmt.Errorf("host does not match IP mode %s", mode)
		}
		ipText := ip.String()
		return ipText, []string{ipText}, nil
	}

	resolver := net.DefaultResolver
	switch mode {
	case IPModeIPv4:
		ips, err := resolver.LookupIP(ctx, "ip4", host)
		if err != nil {
			return "", nil, err
		}
		return pickIP(host, ips)
	case IPModeIPv6:
		ips, err := resolver.LookupIP(ctx, "ip6", host)
		if err != nil {
			return "", nil, err
		}
		return pickIP(host, ips)
	default:
		ips, err := resolver.LookupIP(ctx, "ip4", host)
		if err == nil && len(ips) > 0 {
			return pickIP(host, ips)
		}
		if err != nil {
			var dnsErr *net.DNSError
			if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
				return "", nil, err
			}
		}
		ips, err = resolver.LookupIP(ctx, "ip6", host)
		if err != nil {
			return "", nil, err
		}
		return pickIP(host, ips)
	}
}

func pickIP(host string, ips []net.IP) (string, []string, error) {
	if len(ips) == 0 {
		return "", nil, fmt.Errorf("no IP address found for %s", host)
	}
	list := make([]string, 0, len(ips))
	for _, ip := range ips {
		list = append(list, ip.String())
	}
	return ips[0].String(), list, nil
}

func matchesMode(ip net.IP, mode IPMode) bool {
	if mode == IPModeAuto {
		return true
	}
	if mode == IPModeIPv4 {
		return ip.To4() != nil
	}
	return ip.To4() == nil
}

func resolveJavaSRV(ctx context.Context, host string) (string, int, error) {
	_, records, err := net.DefaultResolver.LookupSRV(ctx, "minecraft", "tcp", host)
	if err != nil {
		return "", 0, err
	}
	if len(records) == 0 {
		return "", 0, fmt.Errorf("no SRV records found")
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Priority == records[j].Priority {
			return records[i].Weight > records[j].Weight
		}
		return records[i].Priority < records[j].Priority
	})
	target := strings.TrimSuffix(records[0].Target, ".")
	return target, int(records[0].Port), nil
}
