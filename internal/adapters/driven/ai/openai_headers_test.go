package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestParseOpenAILimits_HappyPath verifies that both headers present and
// parseable return the correct values with ok=true.
func TestParseOpenAILimits_HappyPath(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"950000"},
			"X-Ratelimit-Reset-Tokens":     {"6m0s"},
		},
	}

	before := time.Now()
	remaining, resetAt, ok := parseOpenAILimits(resp)
	after := time.Now()

	if !ok {
		t.Fatal("expected ok=true")
	}
	if remaining != 950000 {
		t.Errorf("expected remaining=950000, got %d", remaining)
	}
	// resetAt should be approximately now + 6 minutes.
	expectedMin := before.Add(6 * time.Minute)
	expectedMax := after.Add(6 * time.Minute)
	if resetAt.Before(expectedMin) || resetAt.After(expectedMax) {
		t.Errorf("resetAt out of range: got %v, want [%v, %v]", resetAt, expectedMin, expectedMax)
	}
}

// TestParseOpenAILimits_SubSecondDuration verifies parsing of sub-second
// duration strings like "1.386s".
func TestParseOpenAILimits_SubSecondDuration(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"100"},
			"X-Ratelimit-Reset-Tokens":     {"1.386s"},
		},
	}

	remaining, resetAt, ok := parseOpenAILimits(resp)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if remaining != 100 {
		t.Errorf("expected remaining=100, got %d", remaining)
	}
	// resetAt should be ~1.386s in the future.
	expectedDelta := time.Duration(1386 * time.Millisecond)
	now := time.Now()
	if resetAt.Sub(now) < expectedDelta-100*time.Millisecond ||
		resetAt.Sub(now) > expectedDelta+200*time.Millisecond {
		t.Errorf("resetAt delta out of range: got %v, want ~%v", resetAt.Sub(now), expectedDelta)
	}
}

// TestParseOpenAILimits_MissingRemainingHeader returns ok=false when the
// remaining-tokens header is absent.
func TestParseOpenAILimits_MissingRemainingHeader(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Reset-Tokens": {"6m0s"},
			// x-ratelimit-remaining-tokens intentionally absent.
		},
	}

	_, _, ok := parseOpenAILimits(resp)
	if ok {
		t.Error("expected ok=false when remaining header is absent")
	}
}

// TestParseOpenAILimits_MissingResetHeader returns ok=false when the
// reset-tokens header is absent.
func TestParseOpenAILimits_MissingResetHeader(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"100"},
			// x-ratelimit-reset-tokens intentionally absent.
		},
	}

	_, _, ok := parseOpenAILimits(resp)
	if ok {
		t.Error("expected ok=false when reset header is absent")
	}
}

// TestParseOpenAILimits_MalformedRemaining returns ok=false when the
// remaining header cannot be parsed as an integer.
func TestParseOpenAILimits_MalformedRemaining(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"not-a-number"},
			"X-Ratelimit-Reset-Tokens":     {"6m0s"},
		},
	}

	_, _, ok := parseOpenAILimits(resp)
	if ok {
		t.Error("expected ok=false for malformed remaining header")
	}
}

// TestParseOpenAILimits_MalformedDuration returns ok=false when the reset
// header cannot be parsed as a Go duration string.
func TestParseOpenAILimits_MalformedDuration(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"100"},
			"X-Ratelimit-Reset-Tokens":     {"not-a-duration"},
		},
	}

	_, _, ok := parseOpenAILimits(resp)
	if ok {
		t.Error("expected ok=false for malformed duration header")
	}
}

// TestParseOpenAILimits_ZeroRemaining verifies that zero remaining is valid.
func TestParseOpenAILimits_ZeroRemaining(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-Ratelimit-Remaining-Tokens": {"0"},
			"X-Ratelimit-Reset-Tokens":     {"30s"},
		},
	}

	remaining, _, ok := parseOpenAILimits(resp)
	if !ok {
		t.Fatal("expected ok=true for zero remaining")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}
}

// TestParseOpenAILimits_EmptyResponse verifies that a response with no
// rate-limit headers returns ok=false.
func TestParseOpenAILimits_EmptyResponse(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}

	_, _, ok := parseOpenAILimits(resp)
	if ok {
		t.Error("expected ok=false for response with no rate-limit headers")
	}
}
