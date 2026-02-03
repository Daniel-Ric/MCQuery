package ping

import (
	"context"
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

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		ips = append(ips, addr.IP)
	}
	selected, list := filterIPs(ips, mode)
	if selected == nil {
		return "", nil, fmt.Errorf("no IP address found for %s", host)
	}
	return selected.String(), list, nil
}

func filterIPs(ips []net.IP, mode IPMode) (net.IP, []string) {
	if len(ips) == 0 {
		return nil, nil
	}
	filtered := make([]net.IP, 0, len(ips))
	switch mode {
	case IPModeIPv6:
		for _, ip := range ips {
			if ip.To4() == nil {
				filtered = append(filtered, ip)
			}
		}
	case IPModeIPv4:
		for _, ip := range ips {
			if ip.To4() != nil {
				filtered = append(filtered, ip)
			}
		}
	default:
		for _, ip := range ips {
			if ip.To4() != nil {
				filtered = append(filtered, ip)
			}
		}
		if len(filtered) == 0 {
			for _, ip := range ips {
				if ip.To4() == nil {
					filtered = append(filtered, ip)
				}
			}
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	list := make([]string, 0, len(filtered))
	for _, ip := range filtered {
		list = append(list, ip.String())
	}
	return filtered[0], list
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
