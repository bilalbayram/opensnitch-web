package blocklist

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// FetchDomains downloads a blocklist URL and parses domains from it.
// Supports two formats:
//   - Hosts format: "0.0.0.0 domain.com" or "127.0.0.1 domain.com"
//   - Domain-only: one domain per line
func (f *Fetcher) FetchDomains(url string) ([]string, error) {
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	var domains []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		domain := parseDomain(line)
		if domain == "" {
			continue
		}

		// Skip localhost entries
		if domain == "localhost" || domain == "localhost.localdomain" ||
			domain == "local" || domain == "broadcasthost" ||
			domain == "ip6-localhost" || domain == "ip6-loopback" {
			continue
		}

		if _, exists := seen[domain]; !exists {
			seen[domain] = struct{}{}
			domains = append(domains, domain)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return domains, nil
}

// parseDomain extracts a domain from either hosts format or domain-only format.
func parseDomain(line string) string {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if line == "" {
		return ""
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	// Hosts format: "0.0.0.0 domain.com" or "127.0.0.1 domain.com"
	if len(fields) >= 2 && isIPPrefix(fields[0]) {
		return strings.ToLower(fields[1])
	}

	// Domain-only format (may have wildcard prefix like "*.domain.com")
	domain := strings.ToLower(fields[0])
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, "||")
	domain = strings.TrimSuffix(domain, "^")

	// Basic validation: must contain a dot
	if !strings.Contains(domain, ".") {
		return ""
	}

	return domain
}

func isIPPrefix(s string) bool {
	return s == "0.0.0.0" || s == "127.0.0.1" || s == "::1" || s == "::0" || s == "::"
}
