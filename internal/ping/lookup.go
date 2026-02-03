package ping

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type LookupConfig struct {
	Edition       Edition
	Port          int
	BaseHost      string
	Subdomains    []string
	DomainEndings []string
	Concurrency   int
}

type LookupMatch struct {
	Host   string
	Result Result
}

type LookupResult struct {
	Matches   []LookupMatch
	Attempts  int
	Completed int
}

func LookupDomains(ctx context.Context, config LookupConfig) (LookupResult, error) {
	baseHost := strings.TrimSpace(config.BaseHost)
	if baseHost == "" {
		return LookupResult{}, fmt.Errorf("basis host darf nicht leer sein")
	}

	subdomains := normalizeSubdomains(config.Subdomains)
	endings := normalizeEndings(config.DomainEndings)
	if len(endings) == 0 {
		return LookupResult{}, fmt.Errorf("keine domain-endungen angegeben")
	}
	if len(subdomains) == 0 {
		subdomains = []string{""}
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	total := len(subdomains) * len(endings)
	if total == 0 {
		return LookupResult{}, fmt.Errorf("keine kombinationen verfügbar")
	}

	candidates := make(chan string, concurrency)
	results := make(chan LookupMatch, concurrency)
	var completed int64

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for host := range candidates {
			select {
			case <-ctx.Done():
				return
			default:
			}

			res, err := Execute(ctx, config.Edition, host, config.Port)
			atomic.AddInt64(&completed, 1)
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case results <- LookupMatch{Host: host, Result: res}:
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		defer close(candidates)
		for _, sub := range subdomains {
			for _, ending := range endings {
				select {
				case <-ctx.Done():
					return
				case candidates <- buildHost(sub, baseHost, ending):
				}
			}
		}
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
