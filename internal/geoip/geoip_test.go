package geoip

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
}

func (c *fakeClock) Sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]time.Duration, len(c.sleeps))
	copy(out, c.sleeps)
	return out
}

func newTestResolver(t *testing.T, transport roundTripFunc) (*Resolver, *db.Database, *fakeClock) {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	clock := newFakeClock()
	resolver := NewResolver(database, true)
	resolver.client = &http.Client{
		Timeout:   defaultRequestTimeout,
		Transport: transport,
	}
	resolver.minRequestGap = 0
	resolver.now = clock.Now
	resolver.sleep = clock.Sleep

	return resolver, database, clock
}

func newHTTPResponse(statusCode int, body string, headers map[string]string) *http.Response {
	header := make(http.Header)
	for k, v := range headers {
		header.Set(k, v)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition not met before timeout")
}

func TestFetchAndCacheRetriesEOFThenSucceeds(t *testing.T) {
	var calls int32

	resolver, database, clock := newTestResolver(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}

		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			return nil, io.EOF
		}

		return newHTTPResponse(http.StatusOK, `[{"status":"success","query":"8.8.8.8","country":"United States","countryCode":"US","city":"Mountain View","lat":37.3861,"lon":-122.0839}]`, nil), nil
	})

	resolver.fetchAndCache([]string{"8.8.8.8"})

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", got)
	}

	sleeps := clock.Sleeps()
	if len(sleeps) != 1 || sleeps[0] != 1*time.Second {
		t.Fatalf("expected single 1s retry sleep, got %v", sleeps)
	}

	entry, err := database.GetGeoIP("8.8.8.8")
	if err != nil {
		t.Fatalf("expected cached db entry after retry, got err=%v", err)
	}
	if entry.CountryCode != "US" {
		t.Fatalf("expected US country code, got %+v", entry)
	}

	result := resolver.LookupBatch([]string{"8.8.8.8"})
	if result["8.8.8.8"] == nil {
		t.Fatalf("expected in-memory cache hit after fetch, got %+v", result)
	}
}

func TestFetchBatchRetriesRateLimitedResponseAndHonorsCooldown(t *testing.T) {
	var calls int32

	resolver, _, clock := newTestResolver(t, func(req *http.Request) (*http.Response, error) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			return newHTTPResponse(http.StatusTooManyRequests, `rate limited`, map[string]string{
				"X-Ttl": "5",
				"X-Rl":  "0",
			}), nil
		}

		return newHTTPResponse(http.StatusOK, `[{"status":"success","query":"1.1.1.1","country":"Australia","countryCode":"AU","city":"Sydney","lat":-33.8688,"lon":151.2093}]`, nil), nil
	})

	entries, err := resolver.fetchBatch([]string{"1.1.1.1"})
	if err != nil {
		t.Fatalf("expected successful retry after 429, got %v", err)
	}
	if len(entries) != 1 || entries[0].IP != "1.1.1.1" {
		t.Fatalf("unexpected entries after retry: %+v", entries)
	}

	sleeps := clock.Sleeps()
	if len(sleeps) != 1 || sleeps[0] != 6*time.Second {
		t.Fatalf("expected single 6s cooldown sleep, got %v", sleeps)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", got)
	}
}

func TestFetchBatchHonorsCooldownWhenRateLimitRemainingIsZero(t *testing.T) {
	var calls int32

	resolver, _, clock := newTestResolver(t, func(req *http.Request) (*http.Response, error) {
		call := atomic.AddInt32(&calls, 1)
		switch call {
		case 1:
			return newHTTPResponse(http.StatusOK, `[{"status":"success","query":"8.8.8.8","country":"United States","countryCode":"US","city":"Mountain View","lat":37.3861,"lon":-122.0839}]`, map[string]string{
				"X-Rl":  "0",
				"X-Ttl": "7",
			}), nil
		case 2:
			return newHTTPResponse(http.StatusOK, `[{"status":"success","query":"1.1.1.1","country":"Australia","countryCode":"AU","city":"Sydney","lat":-33.8688,"lon":151.2093}]`, nil), nil
		default:
			t.Fatalf("unexpected extra upstream call %d", call)
			return nil, nil
		}
	})

	if _, err := resolver.fetchBatch([]string{"8.8.8.8"}); err != nil {
		t.Fatalf("expected first batch request to succeed, got %v", err)
	}
	if _, err := resolver.fetchBatch([]string{"1.1.1.1"}); err != nil {
		t.Fatalf("expected second batch request to succeed after cooldown, got %v", err)
	}

	sleeps := clock.Sleeps()
	if len(sleeps) != 1 || sleeps[0] != 8*time.Second {
		t.Fatalf("expected single 8s cooldown sleep before follow-up request, got %v", sleeps)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", got)
	}
}

func TestLookupBatchDeduplicatesInflightFetches(t *testing.T) {
	var calls int32

	resolver, database, _ := newTestResolver(t, func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return newHTTPResponse(http.StatusOK, `[{"status":"success","query":"8.8.8.8","country":"United States","countryCode":"US","city":"Mountain View","lat":37.3861,"lon":-122.0839}]`, nil), nil
	})

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = resolver.LookupBatch([]string{"8.8.8.8", "8.8.8.8"})
		}()
	}

	close(start)
	wg.Wait()

	waitFor(t, time.Second, func() bool {
		return atomic.LoadInt32(&calls) == 1
	})
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 upstream call for deduplicated inflight fetch, got %d", got)
	}

	waitFor(t, time.Second, func() bool {
		_, err := database.GetGeoIP("8.8.8.8")
		return err == nil
	})

	result := resolver.LookupBatch([]string{"8.8.8.8"})
	if result["8.8.8.8"] == nil {
		t.Fatalf("expected cached result after inflight fetch completed, got %+v", result)
	}
}

func TestFetchBatchReturnsClassifiedDecodeErrorWithoutCaching(t *testing.T) {
	resolver, database, _ := newTestResolver(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusOK, `{"broken":`, nil), nil
	})

	_, err := resolver.fetchBatch([]string{"8.8.8.8"})
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}

	var fetchErr *batchFetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("expected batchFetchError, got %T", err)
	}
	if fetchErr.class != "decode_error" {
		t.Fatalf("expected decode_error classification, got %+v", fetchErr)
	}
	if fetchErr.statusCode != http.StatusOK {
		t.Fatalf("expected status=200, got %+v", fetchErr)
	}
	if fetchErr.bodySnippet == "" {
		t.Fatalf("expected body snippet to be captured, got %+v", fetchErr)
	}

	if _, ok := resolver.cache["8.8.8.8"]; ok {
		t.Fatalf("did not expect malformed response to populate memory cache")
	}
	if _, err := database.GetGeoIP("8.8.8.8"); err == nil {
		t.Fatal("did not expect malformed response to populate database cache")
	}
}
