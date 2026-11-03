// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

// When the hub answers 429 Too Many Requests, wait as long as Retry-After asks,
// or one second if it gives no usable positive delay, and send the request again.
// Retries are bounded by a count and a time budget; the final refusal reaches the caller.

package lib

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// rateLimitRetries is how many times a rate-limited request is sent again
	rateLimitRetries = 6
	// rateLimitRetryBudget includes requests and waits; no retry is scheduled beyond it.
	// Requests already in progress and successful streaming responses are not cut short.
	rateLimitRetryBudget = time.Minute
)

// ConfigRateLimitNotify, when set, is told how long each wait is and which retry follows
// it, so that a verbose CLI can say what is happening; by default nothing is said
var ConfigRateLimitNotify func(wait time.Duration, retry int, retries int)

// rateLimitSleep waits, or stops early when the request is cancelled; a test replaces it
var rateLimitSleep = func(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// doRetryingRateLimited sends a request with send, and when the hub answers 429 Too Many
// Requests waits and sends it again, as long as its body can be sent again, until the hub
// answers otherwise or the retries or time budget run out. The response returned is the
// caller's to close.
func doRetryingRateLimited(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	started := time.Now()
	for retry := 1; ; retry++ {
		rsp, err := send(req)
		if err != nil || rsp.StatusCode != http.StatusTooManyRequests || retry > rateLimitRetries {
			return rsp, err
		}
		wait, _ := retryAfter(rsp.Header, time.Now())
		if wait <= 0 {
			wait = time.Second
		}
		// Preserve the refusal rather than shortening a delay the hub requires.
		if wait > rateLimitRetryBudget-time.Since(started) {
			return rsp, nil
		}
		again, replayable := requestAgain(req)
		if !replayable {
			return rsp, nil
		}
		// The refusal's body isn't wanted; reading what little there is lets the
		// connection be used again
		io.Copy(io.Discard, io.LimitReader(rsp.Body, 64*1024))
		rsp.Body.Close()
		if ConfigRateLimitNotify != nil {
			ConfigRateLimitNotify(wait, retry, rateLimitRetries)
		}
		if err := rateLimitSleep(req.Context(), wait); err != nil {
			if again.Body != nil {
				again.Body.Close()
			}
			return nil, err
		}
		req = again
	}
}

// retryAfter reads a Retry-After header, which gives a number of seconds or an HTTP date,
// as a wait from now; a header that is missing or can't be read gives none
func retryAfter(header http.Header, now time.Time) (wait time.Duration, given bool) {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return at.Sub(now), true
	}
	return 0, false
}

// requestAgain is the request to send again: the same request with its body, which the
// first sending consumed, read afresh.  That can be done when the body is empty or the
// request knows how to read it again, as http.NewRequest arranges for a request built
// from bytes or a string, which every request the CLI builds is.
func requestAgain(req *http.Request) (again *http.Request, replayable bool) {
	again = req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return again, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	again.Body = body
	return again, true
}
