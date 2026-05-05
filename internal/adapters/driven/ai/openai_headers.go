package ai

import (
	"net/http"
	"strconv"
	"time"
)

// parseOpenAILimits extracts the authoritative rate-limit state from an OpenAI
// response. It reads:
//
//   - x-ratelimit-remaining-tokens: the server's current remaining token budget
//   - x-ratelimit-reset-tokens: a Go duration string (e.g. "1.386s", "6m0s")
//     indicating when the token budget resets
//
// The resetAt time is computed as time.Now() + parsed duration.
//
// Returns ok=false if either header is absent or cannot be parsed, in which
// case the caller should leave the Limiter unchanged.
func parseOpenAILimits(resp *http.Response) (remaining int64, resetAt time.Time, ok bool) {
	remStr := resp.Header.Get("x-ratelimit-remaining-tokens")
	resetStr := resp.Header.Get("x-ratelimit-reset-tokens")

	if remStr == "" || resetStr == "" {
		return 0, time.Time{}, false
	}

	rem, err := strconv.ParseInt(remStr, 10, 64)
	if err != nil {
		return 0, time.Time{}, false
	}

	d, err := time.ParseDuration(resetStr)
	if err != nil {
		return 0, time.Time{}, false
	}

	return rem, time.Now().Add(d), true
}
