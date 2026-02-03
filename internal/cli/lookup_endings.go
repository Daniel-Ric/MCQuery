package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const ianaTLDSource = "https://data.iana.org/TLD/tlds-alpha-by-domain.txt"

var (
	domainEndingsOnce  sync.Once
	domainEndingsCache []string
	domainEndingsErr   error
)

func loadDomainEndings() ([]string, error) {
	domainEndingsOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		iana, err := fetchIANATLDs(ctx)
		if err != nil {
			domainEndingsErr = fmt.Errorf("konnte endungs-pool nicht laden: %w", err)
			domainEndingsCache = domainEndingPool
			return
		}
		domainEndingsCache = mergeUniqueEndings(iana, domainEndingPool)
	})

	if domainEndingsErr != nil {
		return domainEndingsCache, domainEndingsErr
	}
	return domainEndingsCache, nil
}

func fetchIANATLDs(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ianaTLDSource, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iana antwort: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	list := make([]string, 0, 2000)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		list = append(list, strings.ToLower(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("keine endungen gefunden")
	}
	return list, nil
}

func mergeUniqueEndings(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	list := make([]string, 0, len(primary)+len(secondary))
	for _, value := range primary {
		value = normalizeEnding(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	for _, value := range secondary {
		value = normalizeEnding(value)
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
