package ping

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LookupConfig struct {
	Edition       Edition
	Port          int
	Ports         []int
	BaseHost      string
	Subdomains    []string
	DomainEndings []string
	Concurrency   int
	RateLimit     int
	Options       ExecuteOptions
	Progress      func(progress LookupProgress)
	Paused        func() bool
}

type LookupMatch struct {
	Host   string
	Port   int
	Result Result
	Detail ExecuteDetails
}

type LookupResult struct {
	Matches   []LookupMatch
	Attempts  int
	Completed int
}

type LookupProgress struct {
	Subdomain string
	Ending    string
	Host      string
	Port      int
	Attempt   int
	Total     int
	Completed int
}

type lookupCandidate struct {
	subdomain string
	ending    string
	host      string
	port      int
	attempt   int
}

const (
	lookupAutoWorkersPerCPU = 16
	lookupAutoWorkersMin    = 64
)

func AutoLookupConcurrencyTarget() int {
	concurrency := runtime.NumCPU() * lookupAutoWorkersPerCPU
	if concurrency < lookupAutoWorkersMin {
		concurrency = lookupAutoWorkersMin
	}
	return concurrency
}

func DefaultLookupConcurrency(total int) int {
	if total <= 0 {
		return 0
	}
	concurrency := AutoLookupConcurrencyTarget()
	if concurrency > total {
		return total
	}
	return concurrency
}

func LookupDomains(ctx context.Context, config LookupConfig) (LookupResult, error) {
	baseHost := strings.TrimSpace(config.BaseHost)
	if baseHost == "" {
		return LookupResult{}, fmt.Errorf("base host cannot be empty")
	}

	subdomains := normalizeSubdomains(config.Subdomains)
	endings := normalizeEndings(config.DomainEndings)
	if len(endings) == 0 {
		return LookupResult{}, fmt.Errorf("no domain endings provided")
	}
	if len(subdomains) == 0 {
		subdomains = []string{""}
	}
	ports := normalizeLookupPorts(config.Port, config.Ports)

	total := len(subdomains) * len(endings) * len(ports)
	if total == 0 {
		return LookupResult{}, fmt.Errorf("no combinations available")
	}
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultLookupConcurrency(total)
	}
	if concurrency > total {
		concurrency = total
	}

	candidates := make(chan lookupCandidate, concurrency)
	results := make(chan LookupMatch, concurrency)
	var completed int64

	var limiter <-chan time.Time
	if config.RateLimit > 0 {
		interval := time.Second / time.Duration(config.RateLimit)
		if interval <= 0 {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		limiter = ticker.C
	}

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for candidate := range candidates {
			select {
			case <-ctx.Done():
				return
			default:
			}

			res, detail, err := Execute(ctx, ExecuteConfig{
				Edition:    config.Edition,
				Host:       candidate.host,
				Port:       candidate.port,
				Timeout:    config.Options.Timeout,
				RetryCount: config.Options.RetryCount,
				RetryDelay: config.Options.RetryDelay,
				EnableSRV:  config.Options.EnableSRV,
				IPMode:     config.Options.IPMode,
			})
			currentCompleted := int(atomic.AddInt64(&completed, 1))
			if config.Progress != nil {
				config.Progress(LookupProgress{
					Subdomain: candidate.subdomain,
					Ending:    candidate.ending,
					Host:      candidate.host,
					Port:      candidate.port,
					Attempt:   candidate.attempt,
					Total:     total,
					Completed: currentCompleted,
				})
			}
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case results <- LookupMatch{Host: candidate.host, Port: candidate.port, Result: res, Detail: detail}:
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		enqueueLookupCandidates(ctx, candidates, subdomains, endings, ports, baseHost, limiter, config.Paused)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	matches := make([]LookupMatch, 0)
	for match := range results {
		matches = append(matches, match)
	}

	return LookupResult{
		Matches:   matches,
		Attempts:  total,
		Completed: int(atomic.LoadInt64(&completed)),
	}, ctx.Err()
}

func normalizeLookupPorts(defaultPort int, values []int) []int {
	seen := make(map[int]struct{}, len(values)+1)
	list := make([]int, 0, len(values)+1)
	for _, value := range values {
		if value < 1 || value > 65535 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	if len(list) == 0 && defaultPort >= 1 && defaultPort <= 65535 {
		list = append(list, defaultPort)
	}
	return list
}

func normalizeSubdomains(values []string) []string {
	list := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			if _, ok := seen[""]; ok {
				continue
			}
			seen[""] = struct{}{}
			list = append(list, "")
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func normalizeEndings(values []string) []string {
	list := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		value = strings.TrimPrefix(value, ".")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func buildHost(subdomain, baseHost, ending string) string {
	parts := make([]string, 0, 3)
	if subdomain != "" {
		parts = append(parts, subdomain)
	}
	parts = append(parts, baseHost)
	if ending != "" {
		parts = append(parts, ending)
	}
	return strings.Join(parts, ".")
}

func enqueueLookupCandidates(ctx context.Context, candidates chan<- lookupCandidate, subdomains, endings []string, ports []int, baseHost string, limiter <-chan time.Time, paused func() bool) {
	defer close(candidates)

	attempt := 0
	for _, sub := range subdomains {
		for _, ending := range endings {
			host := buildHost(sub, baseHost, ending)
			for _, port := range ports {
				attempt++
				if paused != nil {
					for paused() {
						select {
						case <-ctx.Done():
							return
						case <-time.After(120 * time.Millisecond):
						}
					}
				}
				if limiter != nil {
					select {
					case <-ctx.Done():
						return
					case <-limiter:
					}
				}

				candidate := lookupCandidate{
					subdomain: sub,
					ending:    ending,
					host:      host,
					port:      port,
					attempt:   attempt,
				}

				select {
				case <-ctx.Done():
					return
				case candidates <- candidate:
				}
			}
		}
	}
}
