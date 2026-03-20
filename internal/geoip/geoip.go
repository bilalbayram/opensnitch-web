package geoip

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
)

type Resolver struct {
	database *db.Database
	cache    map[string]*db.GeoIPEntry
	mu       sync.RWMutex
	enabled  bool
	limiter  *time.Ticker
	// ip-api.com free tier only supports HTTP (not HTTPS).
	client *http.Client
}

func NewResolver(database *db.Database, enabled bool) *Resolver {
	r := &Resolver{
		database: database,
		cache:    make(map[string]*db.GeoIPEntry),
		enabled:  enabled,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
	if enabled {
		// ip-api.com free tier: 45 requests/minute
		r.limiter = time.NewTicker(time.Second * 2)
	}
	return r
}

func (r *Resolver) Stop() {
	if r.limiter != nil {
		r.limiter.Stop()
	}
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
	for _, ip := range ips {
		if e, ok := r.cache[ip]; ok {
			result[ip] = e
		}
	}
	r.mu.RUnlock()

	// Collect IPs not in memory cache
	for _, ip := range ips {
		if _, ok := result[ip]; ok {
			continue
		}
		if shouldSkipIP(ip) {
			continue
		}
		missing = append(missing, ip)
	}

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
	for _, ip := range missing {
		if _, ok := result[ip]; !ok {
			toFetch = append(toFetch, ip)
		}
	}

	if len(toFetch) == 0 {
		return result
	}

	// Resolve missing IPs asynchronously so the HTTP handler is not blocked
	go r.fetchAndCache(toFetch)

	return result
}

type ipAPIResponse struct {
	Status      string  `json:"status"`
	Query       string  `json:"query"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// fetchAndCache resolves IPs via ip-api.com in the background, populating both caches.
func (r *Resolver) fetchAndCache(ips []string) {
	for i := 0; i < len(ips); i += 100 {
		end := min(i+100, len(ips))
		batch := ips[i:end]

		// Rate limit
		<-r.limiter.C

		entries, err := r.fetchBatch(batch)
		if err != nil {
			log.Printf("[geoip] batch fetch failed: %v", err)
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

	req, err := http.NewRequest("POST", "http://ip-api.com/batch?fields=status,query,country,countryCode,city,lat,lon", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
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
	return entries, nil
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
