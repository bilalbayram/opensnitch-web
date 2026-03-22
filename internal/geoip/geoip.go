package geoip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

const (
	ipAPIBatchEndpoint         = "http://ip-api.com/batch?fields=status,query,country,countryCode,city,lat,lon"
	defaultRequestTimeout      = 10 * time.Second
	defaultMinRequestGap       = 4 * time.Second
	defaultRateLimitCooldown   = 60 * time.Second
	maxBatchAttempts           = 3
	maxLoggedResponseBodyBytes = 256
)

type Resolver struct {
	database *db.Database
	cache    map[string]*db.GeoIPEntry
	inFlight map[string]struct{}
	mu       sync.RWMutex
	enabled  bool

	// ip-api.com free tier only supports HTTP (not HTTPS).
	client   *http.Client
	endpoint string

	minRequestGap time.Duration
	now           func() time.Time
	sleep         func(time.Duration)

	gateMu        sync.Mutex
	nextRequestAt time.Time
}

func NewResolver(database *db.Database, enabled bool) *Resolver {
	return &Resolver{
		database: database,
		cache:    make(map[string]*db.GeoIPEntry),
		inFlight: make(map[string]struct{}),
		enabled:  enabled,
		client:   &http.Client{Timeout: defaultRequestTimeout},
		endpoint: ipAPIBatchEndpoint,

		minRequestGap: defaultMinRequestGap,
		now:           time.Now,
		sleep:         time.Sleep,
	}
}

func (r *Resolver) Stop() {
	// No-op for now. Kept for compatibility with main shutdown flow.
}

func (r *Resolver) Enabled() bool {
	return r.enabled
}

// LookupBatch resolves a list of IPs, returning cached results and fetching missing ones.
// Best-effort: errors from external API or DB are logged but do not fail the call.
func (r *Resolver) LookupBatch(ips []string) map[string]*db.GeoIPEntry {
	if !r.enabled {
		return nil
	}

	result := make(map[string]*db.GeoIPEntry)
	var missing []string

	// Check in-memory cache first
	r.mu.RLock()
	for _, ip := range uniqueLookupIPs(ips) {
		if e, ok := r.cache[ip]; ok {
			result[ip] = e
			continue
		}
		missing = append(missing, ip)
	}
	r.mu.RUnlock()

	if len(missing) == 0 {
		return result
	}

	// Check SQLite cache
	dbEntries, err := r.database.GetGeoIPBatch(missing)
	if err == nil {
		r.mu.Lock()
		for ip, e := range dbEntries {
			result[ip] = e
			r.cache[ip] = e
		}
		r.mu.Unlock()
	}

	// Collect still-missing IPs
	var toFetch []string
	r.mu.Lock()
	for _, ip := range missing {
		if e, ok := r.cache[ip]; ok {
			result[ip] = e
			continue
		}
		if _, ok := result[ip]; ok {
			continue
		}
		if _, ok := r.inFlight[ip]; ok {
			continue
		}
		r.inFlight[ip] = struct{}{}
		toFetch = append(toFetch, ip)
	}
	r.mu.Unlock()

	if len(toFetch) == 0 {
		return result
	}

	// Resolve missing IPs asynchronously so the HTTP handler is not blocked
	go r.fetchAndCache(toFetch)

	return result
}

type ipAPIResponse struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	Query       string  `json:"query"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type batchFetchError struct {
	attempts    int
	batchSize   int
	class       string
	statusCode  int
	rateRemain  string
	rateReset   string
	bodySnippet string
	cause       error
}

func (e *batchFetchError) Error() string {
	parts := []string{
		fmt.Sprintf("class=%s", e.class),
		fmt.Sprintf("attempt=%d", e.attempts),
		fmt.Sprintf("batch_size=%d", e.batchSize),
	}
	if e.statusCode > 0 {
		parts = append(parts, fmt.Sprintf("status=%d", e.statusCode))
	}
	if e.cause != nil {
		parts = append(parts, e.cause.Error())
	}
	return strings.Join(parts, " ")
}

func (e *batchFetchError) Unwrap() error {
	return e.cause
}

// fetchAndCache resolves IPs via ip-api.com in the background, populating both caches.
func (r *Resolver) fetchAndCache(ips []string) {
	for i := 0; i < len(ips); i += 100 {
		end := min(i+100, len(ips))
		batch := ips[i:end]

		entries, err := r.fetchBatch(batch)
		if err != nil {
			r.logFetchError(err)
			r.clearInFlight(batch)
			continue
		}

		r.mu.Lock()
		// Evict half the cache when it exceeds the limit to amortize eviction cost.
		if len(r.cache) > 10000 {
			i := 0
			half := len(r.cache) / 2
			for k := range r.cache {
				delete(r.cache, k)
				i++
				if i >= half {
					break
				}
			}
		}
		for _, e := range entries {
			r.cache[e.IP] = e
		}
		for _, ip := range batch {
			delete(r.inFlight, ip)
		}
		r.mu.Unlock()

		for _, e := range entries {
			if err := r.database.UpsertGeoIP(e); err != nil {
				log.Printf("[geoip] cache write failed for %s: %v", e.IP, err)
			}
		}
	}
}

func (r *Resolver) fetchBatch(ips []string) ([]*db.GeoIPEntry, error) {
	body, err := json.Marshal(ips)
	if err != nil {
		return nil, err
	}

	for attempt := 1; attempt <= maxBatchAttempts; attempt++ {
		resp, err := r.doBatchRequest(body)
		if err != nil {
			fetchErr := &batchFetchError{
				attempts:  attempt,
				batchSize: len(ips),
				class:     classifyTransportError(err),
				cause:     err,
			}
			if attempt < maxBatchAttempts {
				r.sleepFor(retryBackoff(attempt))
				continue
			}
			return nil, fetchErr
		}

		entries, fetchErr, retryable := r.decodeBatchResponse(resp, len(ips), attempt)
		if fetchErr == nil {
			return entries, nil
		}
		if retryable && attempt < maxBatchAttempts {
			if fetchErr.class != "http_429" {
				r.sleepFor(retryBackoff(attempt))
			}
			continue
		}
		return nil, fetchErr
	}

	return nil, &batchFetchError{
		attempts:  maxBatchAttempts,
		batchSize: len(ips),
		class:     "fetch_exhausted",
	}
}

// shouldSkipIP returns true for IPs that should not be sent to the external
// geo-lookup service: private, loopback, link-local, or unparseable addresses.
func shouldSkipIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func (r *Resolver) doBatchRequest(body []byte) (*http.Response, error) {
	r.gateMu.Lock()
	defer r.gateMu.Unlock()

	now := r.timeNow()
	if now.Before(r.nextRequestAt) {
		r.sleepFor(r.nextRequestAt.Sub(now))
	}

	startedAt := r.timeNow()
	r.nextRequestAt = startedAt.Add(r.minGap())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests || strings.TrimSpace(resp.Header.Get("X-Rl")) == "0" {
		r.applyCooldown(resp.Header.Get("X-Ttl"))
	}

	return resp, nil
}

func (r *Resolver) decodeBatchResponse(resp *http.Response, batchSize, attempt int) ([]*db.GeoIPEntry, *batchFetchError, bool) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fetchErr := &batchFetchError{
			attempts:   attempt,
			batchSize:  batchSize,
			class:      classifyTransportError(err),
			statusCode: resp.StatusCode,
			rateRemain: resp.Header.Get("X-Rl"),
			rateReset:  resp.Header.Get("X-Ttl"),
			cause:      err,
		}
		return nil, fetchErr, true
	}

	if resp.StatusCode != http.StatusOK {
		class, retryable := classifyHTTPStatus(resp.StatusCode)
		fetchErr := &batchFetchError{
			attempts:    attempt,
			batchSize:   batchSize,
			class:       class,
			statusCode:  resp.StatusCode,
			rateRemain:  resp.Header.Get("X-Rl"),
			rateReset:   resp.Header.Get("X-Ttl"),
			bodySnippet: summarizeBody(body),
			cause:       fmt.Errorf("unexpected status %d", resp.StatusCode),
		}
		return nil, fetchErr, retryable
	}

	var results []ipAPIResponse
	if err := json.Unmarshal(body, &results); err != nil {
		fetchErr := &batchFetchError{
			attempts:    attempt,
			batchSize:   batchSize,
			class:       "decode_error",
			statusCode:  resp.StatusCode,
			rateRemain:  resp.Header.Get("X-Rl"),
			rateReset:   resp.Header.Get("X-Ttl"),
			bodySnippet: summarizeBody(body),
			cause:       err,
		}
		return nil, fetchErr, false
	}

	var entries []*db.GeoIPEntry
	for _, res := range results {
		if res.Status != "success" {
			continue
		}
		entries = append(entries, &db.GeoIPEntry{
			IP:          res.Query,
			Country:     res.Country,
			CountryCode: res.CountryCode,
			City:        res.City,
			Lat:         res.Lat,
			Lon:         res.Lon,
		})
	}
	return entries, nil, false
}

func (r *Resolver) applyCooldown(ttlHeader string) {
	until := r.timeNow().Add(parseCooldown(ttlHeader))
	if until.After(r.nextRequestAt) {
		r.nextRequestAt = until
	}
}

func (r *Resolver) clearInFlight(ips []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ip := range ips {
		delete(r.inFlight, ip)
	}
}

func (r *Resolver) logFetchError(err error) {
	var fetchErr *batchFetchError
	if errors.As(err, &fetchErr) {
		log.Printf(
			"[geoip] batch fetch failed: attempts=%d batch_size=%d class=%s status=%d x_rl=%q x_ttl=%q body=%q err=%v",
			fetchErr.attempts,
			fetchErr.batchSize,
			fetchErr.class,
			fetchErr.statusCode,
			fetchErr.rateRemain,
			fetchErr.rateReset,
			fetchErr.bodySnippet,
			fetchErr.cause,
		)
		return
	}

	log.Printf("[geoip] batch fetch failed: %v", err)
}

func (r *Resolver) timeNow() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (r *Resolver) sleepFor(d time.Duration) {
	if d <= 0 {
		return
	}
	if r.sleep != nil {
		r.sleep(d)
		return
	}
	time.Sleep(d)
}

func (r *Resolver) minGap() time.Duration {
	if r.minRequestGap < 0 {
		return 0
	}
	return r.minRequestGap
}

func classifyTransportError(err error) string {
	switch {
	case errors.Is(err, io.EOF):
		return "transport_eof"
	case errors.Is(err, context.DeadlineExceeded):
		return "transport_timeout"
	case errors.Is(err, syscall.ECONNRESET):
		return "transport_conn_reset"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "transport_timeout"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection reset by peer"):
		return "transport_conn_reset"
	case strings.Contains(msg, "timeout"):
		return "transport_timeout"
	case strings.Contains(msg, "eof"):
		return "transport_eof"
	default:
		return "transport_error"
	}
}

func classifyHTTPStatus(statusCode int) (string, bool) {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return "http_429", true
	case statusCode >= 500:
		return "http_5xx", true
	default:
		return "http_status", false
	}
}

func retryBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Second
	default:
		return 2 * time.Second
	}
}

func parseCooldown(ttlHeader string) time.Duration {
	ttlHeader = strings.TrimSpace(ttlHeader)
	if ttlHeader != "" {
		if seconds, err := strconv.Atoi(ttlHeader); err == nil && seconds > 0 {
			return time.Duration(seconds+1) * time.Second
		}
	}
	return defaultRateLimitCooldown
}

func summarizeBody(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	if len(body) > maxLoggedResponseBodyBytes {
		return string(body[:maxLoggedResponseBodyBytes]) + "..."
	}
	return string(body)
}

func uniqueLookupIPs(ips []string) []string {
	seen := make(map[string]struct{}, len(ips))
	result := make([]string, 0, len(ips))

	for _, ip := range ips {
		if shouldSkipIP(ip) {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		result = append(result, ip)
	}

	return result
}
