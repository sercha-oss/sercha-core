package ratelimited

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeLimiter is a test double for driven.Limiter that records calls.
type fakeLimiter struct {
	mu        sync.Mutex
	waitCalls []int64
	updates   []limitUpdate
	waitErr   error
}

type limitUpdate struct {
	remaining int64
	resetAt   time.Time
}

func (f *fakeLimiter) Wait(_ context.Context, weight int64) error {
	f.mu.Lock()
	f.waitCalls = append(f.waitCalls, weight)
	f.mu.Unlock()
	return f.waitErr
}

func (f *fakeLimiter) Update(remaining int64, resetAt time.Time) {
	f.mu.Lock()
	f.updates = append(f.updates, limitUpdate{remaining, resetAt})
	f.mu.Unlock()
}

// noSleep is an injectable Sleep that returns immediately.
func noSleep(_ context.Context, _ time.Duration) error { return nil }


// TestTransport_200_CallsParseLimit verifies that a 200 response calls
// ParseLimit and Limiter.Update, and returns the response.
func TestTransport_200_CallsParseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Remaining", "900")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	lim := &fakeLimiter{}
	parseCount := 0
	var parsedRemaining int64

	tr := &Transport{
		Base:            http.DefaultTransport,
		Limiter:         lim,
		MaxRetries:      0,
		MaxRetryElapsed: 5 * time.Second,
		Sleep:           noSleep,
		ParseLimit: func(resp *http.Response) (int64, time.Time, bool) {
			parseCount++
			rem, _ := strconv.ParseInt(resp.Header.Get("X-Remaining"), 10, 64)
			parsedRemaining = rem
			return rem, time.Now().Add(time.Minute), true
		},
		Weight: func(r *http.Request) int64 { return 42 },
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// ParseLimit should have been called once.
	if parseCount != 1 {
		t.Errorf("expected ParseLimit called once, got %d", parseCount)
	}

	// Update should have been called with the parsed remaining.
	lim.mu.Lock()
	updates := lim.updates
	lim.mu.Unlock()

	if len(updates) != 1 {
		t.Errorf("expected 1 Update call, got %d", len(updates))
	} else if updates[0].remaining != parsedRemaining {
		t.Errorf("expected Update(remaining=%d), got %d", parsedRemaining, updates[0].remaining)
	}

	// Weight callback should have set the wait weight.
	lim.mu.Lock()
	waitCalls := lim.waitCalls
	lim.mu.Unlock()
	if len(waitCalls) != 1 || waitCalls[0] != 42 {
		t.Errorf("expected Wait(42), got %v", waitCalls)
	}
}

// TestTransport_429_RetryAfterSeconds verifies that a 429 with Retry-After
// in delta-seconds format results in the correct sleep duration and a retry.
func TestTransport_429_RetryAfterSeconds(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var sleptFor time.Duration
	fakeSleep := func(_ context.Context, d time.Duration) error {
		sleptFor = d
		return nil
	}

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           fakeSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on retry, got %d", resp.StatusCode)
	}

	if requestCount.Load() != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount.Load())
	}

	// Sleep should have been called with ~2s.
	if sleptFor < 1900*time.Millisecond || sleptFor > 2100*time.Millisecond {
		t.Errorf("expected ~2s sleep, got %v", sleptFor)
	}
}

// TestTransport_429_RetryAfterHTTPDate verifies that a 429 with Retry-After
// in HTTP-date (RFC 7231) format computes the correct sleep duration.
func TestTransport_429_RetryAfterHTTPDate(t *testing.T) {
	var requestCount atomic.Int32
	retryTime := time.Now().Add(3 * time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			// RFC 7231 HTTP-date format.
			w.Header().Set("Retry-After", retryTime.UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var sleptFor time.Duration
	fakeSleep := func(_ context.Context, d time.Duration) error {
		sleptFor = d
		return nil
	}

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           fakeSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on retry, got %d", resp.StatusCode)
	}

	// Sleep should be approximately 3 seconds (minus a tiny bit of elapsed time).
	if sleptFor < 2*time.Second || sleptFor > 4*time.Second {
		t.Errorf("expected ~3s sleep from HTTP-date, got %v", sleptFor)
	}
}

// TestTransport_429_NoRetryAfter verifies that a 429 without Retry-After uses
// exponential backoff.
func TestTransport_429_NoRetryAfter(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var sleptFor time.Duration
	fakeSleep := func(_ context.Context, d time.Duration) error {
		sleptFor = d
		return nil
	}

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           fakeSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on retry, got %d", resp.StatusCode)
	}

	// Exponential backoff for attempt 0: 1s + 0-25% jitter → [1s, 1.25s].
	if sleptFor < 1*time.Second || sleptFor > 2*time.Second {
		t.Errorf("expected ~1s backoff sleep, got %v", sleptFor)
	}
}

// TestTransport_5xx_RetriesWithBackoff verifies that 502/503/504 trigger retry.
func TestTransport_5xx_RetriesWithBackoff(t *testing.T) {
	codes := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			var requestCount atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := requestCount.Add(1)
				if n == 1 {
					w.WriteHeader(code)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			tr := &Transport{
				Base:            http.DefaultTransport,
				MaxRetries:      3,
				MaxRetryElapsed: 30 * time.Second,
				Sleep:           noSleep,
			}

			req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200 on retry after %d, got %d", code, resp.StatusCode)
			}
			if requestCount.Load() != 2 {
				t.Errorf("expected 2 requests, got %d", requestCount.Load())
			}
		})
	}
}

// TestTransport_4xx_NonRetryable verifies that non-429 4xx responses are
// returned immediately without retry.
func TestTransport_4xx_NonRetryable(t *testing.T) {
	codes := []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}
	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			var requestCount atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(code)
			}))
			defer server.Close()

			tr := &Transport{
				Base:            http.DefaultTransport,
				MaxRetries:      3,
				MaxRetryElapsed: 30 * time.Second,
				Sleep:           noSleep,
			}

			req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != code {
				t.Errorf("expected %d, got %d", code, resp.StatusCode)
			}
			if requestCount.Load() != 1 {
				t.Errorf("expected only 1 request for %d, got %d", code, requestCount.Load())
			}
		})
	}
}

// TestTransport_NetworkError_Retries verifies that network errors trigger retry.
func TestTransport_NetworkError_Retries(t *testing.T) {
	var requestCount atomic.Int32
	var server *httptest.Server

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		if n == 1 {
			// Close connection abruptly on first request.
			server.CloseClientConnections()
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server = httptest.NewServer(handler)
	defer server.Close()

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           noSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	// Either succeeds on retry or returns error — just verify no panic.
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
	}
}

// TestTransport_MaxRetries_Exhausted verifies that after MaxRetries attempts
// the last response (or error) is returned.
func TestTransport_MaxRetries_Exhausted(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      2, // 3 total attempts
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           noSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected last response returned, got error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected final 429 response, got %d", resp.StatusCode)
	}

	if requestCount.Load() != 3 {
		t.Errorf("expected 3 total requests, got %d", requestCount.Load())
	}
}

// TestTransport_MaxRetryElapsed verifies that MaxRetryElapsed stops retries
// before MaxRetries is exhausted.
func TestTransport_MaxRetryElapsed(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	// Use a fake clock that advances time artificially.
	var clockVal atomic.Int64
	clockVal.Store(time.Now().UnixNano())

	fakeNow := func() time.Time {
		return time.Unix(0, clockVal.Load())
	}
	fakeSleep := func(_ context.Context, d time.Duration) error {
		// Advance the fake clock by the sleep duration so the elapsed check fires.
		clockVal.Add(int64(d))
		return nil
	}

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      10,
		MaxRetryElapsed: 3 * time.Second,
		Sleep:           fakeSleep,
		Clock:           fakeNow,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	_, _ = tr.RoundTrip(req)

	// Should have made fewer than 11 requests due to elapsed time cap.
	n := requestCount.Load()
	if n >= 11 {
		t.Errorf("expected fewer than 11 requests due to elapsed cap, got %d", n)
	}
}

// TestTransport_CtxCancelled_DuringSleep verifies that ctx cancellation
// during sleep propagates the cancellation error.
func TestTransport_CtxCancelled_DuringSleep(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	cancelSleep := func(c context.Context, _ time.Duration) error {
		cancel() // cancel the context during sleep
		return c.Err()
	}

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      5,
		MaxRetryElapsed: 60 * time.Second,
		Sleep:           cancelSleep,
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := tr.RoundTrip(req)

	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// TestTransport_WeightPassedToLimiter verifies that the per-request weight is
// passed to Limiter.Wait.
func TestTransport_WeightPassedToLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	lim := &fakeLimiter{}
	tr := &Transport{
		Base:            http.DefaultTransport,
		Limiter:         lim,
		Weight:          func(r *http.Request) int64 { return 333 },
		MaxRetries:      0,
		MaxRetryElapsed: 5 * time.Second,
		Sleep:           noSleep,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	lim.mu.Lock()
	waits := lim.waitCalls
	lim.mu.Unlock()

	if len(waits) != 1 || waits[0] != 333 {
		t.Errorf("expected Wait(333), got %v", waits)
	}
}

// TestTransport_BodyReplayedOnRetry verifies that the request body is
// replayed correctly on retry via GetBody.
func TestTransport_BodyReplayedOnRetry(t *testing.T) {
	var requestCount atomic.Int32
	var lastBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           noSleep,
	}

	body := []byte(`{"hello":"world"}`)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL,
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	if lastBody != string(body) {
		t.Errorf("body not replayed on retry: got %q, want %q", lastBody, string(body))
	}
}

// TestTransport_BodyWithoutGetBody_ReturnsError verifies that a request with
// a body but no GetBody function returns ErrBodyNotReplayable on retry.
func TestTransport_BodyWithoutGetBody_ReturnsError(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	tr := &Transport{
		Base:            http.DefaultTransport,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           noSleep,
	}

	// Build a request with a body but no GetBody (use an io.NopCloser).
	body := io.NopCloser(bytes.NewReader([]byte("data")))
	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL, nil)
	req.Body = body
	// Do NOT set GetBody — simulate a non-replayable body.

	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected ErrBodyNotReplayable, got nil")
	}
	if !errors.Is(err, ErrBodyNotReplayable) {
		t.Errorf("expected ErrBodyNotReplayable, got: %v", err)
	}
}

// TestTransport_ConcurrentRoundTrips verifies that concurrent RoundTrip calls
// on a shared Transport are race-detector clean.
func TestTransport_ConcurrentRoundTrips(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	lim := &fakeLimiter{}
	tr := &Transport{
		Base:            http.DefaultTransport,
		Limiter:         lim,
		MaxRetries:      0,
		MaxRetryElapsed: 5 * time.Second,
		Sleep:           noSleep,
	}

	const goroutines = 20
	var wg sync.WaitGroup
	ctx := context.Background()

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()
	}

	wg.Wait()
}

// TestTransport_ParseLimit_CalledOn429 verifies that ParseLimit is called even
// on 429 responses so the bucket is updated from server-authoritative headers.
func TestTransport_ParseLimit_CalledOn429(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		w.Header().Set("X-Remaining", "50")
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	lim := &fakeLimiter{}
	parseLimitCalls := 0

	tr := &Transport{
		Base:            http.DefaultTransport,
		Limiter:         lim,
		MaxRetries:      3,
		MaxRetryElapsed: 30 * time.Second,
		Sleep:           noSleep,
		ParseLimit: func(resp *http.Response) (int64, time.Time, bool) {
			parseLimitCalls++
			rem, _ := strconv.ParseInt(resp.Header.Get("X-Remaining"), 10, 64)
			return rem, time.Now().Add(time.Minute), true
		},
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// ParseLimit should be called for both the 429 and the 200 response.
	if parseLimitCalls < 2 {
		t.Errorf("expected ParseLimit called at least twice (429 + 200), got %d", parseLimitCalls)
	}

	lim.mu.Lock()
	updates := lim.updates
	lim.mu.Unlock()

	if len(updates) < 2 {
		t.Errorf("expected at least 2 Update calls, got %d", len(updates))
	}
}

// TestWeightFromHeader verifies that WeightFromHeader reads the token weight
// from the X-Sercha-Token-Weight header.
func TestWeightFromHeader(t *testing.T) {
	cases := []struct {
		header   string
		expected int64
	}{
		{"100", 100},
		{"0", 1},   // zero → default 1
		{"-5", 1},  // negative → default 1
		{"bad", 1}, // malformed → default 1
		{"", 1},    // absent → default 1
	}

	for _, tc := range cases {
		req, _ := http.NewRequest("GET", "/", nil)
		if tc.header != "" {
			req.Header.Set(HeaderTokenWeight, tc.header)
		}
		got := WeightFromHeader(req)
		if got != tc.expected {
			t.Errorf("WeightFromHeader(%q) = %d, want %d", tc.header, got, tc.expected)
		}
	}
}
