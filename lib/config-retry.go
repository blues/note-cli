// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

// The hub answers 429 Too Many Requests when an account sends too much too fast: its
// API allows a number of requests a minute for each billing account, and the refusal
// says so in X-RateLimit-Limit and X-RateLimit-Period, with X-RateLimit-Reset saying
// how long until the whole allowance is back.  That isn't handed to the user straight
// away.  The request is held back and sent again: for as long as a Retry-After header
// asks when the hub gives one, or else for one request's share of the allowance to come
// back, or when the hub says neither for a delay that doubles each time, in each case
// with some randomness so that clients limited together don't all return together.  Only
// when the hub keeps refusing for a minute or so, or asks for a longer wait than that,
// does the 429 reach the caller.

package lib

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// rateLimitRetries is how many times a rate-limited request is sent again
	rateLimitRetries = 6
	// rateLimitFirstDelay is the first wait when the hub doesn't say how long; each wait
	// after it is twice the one before, up to rateLimitMaxDelay
	rateLimitFirstDelay = time.Second
	// rateLimitMaxDelay is the longest wait before one retry, whether the hub asked for
	// it or not: a hub asking for longer isn't retried, and its answer is returned
	rateLimitMaxDelay = 60 * time.Second
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
		wait, willing := rateLimitWait(rsp.Header, retry)
		// Preserve the refusal rather than shortening a delay the hub requires.
		if !willing || wait > rateLimitRetryBudget-time.Since(started) {
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

// rateLimitWait is how long to wait before the given retry, the first being 1: what a
// Retry-After header asks; or else one request's share of the allowance the hub's
// X-RateLimit headers describe, and up to a quarter again in case others are waiting for
// it too; or when the hub says neither an increasing delay with some randomness.  A wait
// longer than rateLimitMaxDelay isn't made.
func rateLimitWait(header http.Header, retry int) (wait time.Duration, willing bool) {
	if asked, given := retryAfter(header, time.Now()); given {
		if asked > rateLimitMaxDelay {
			return 0, false
		}
		if asked > 0 {
			return asked, true
		}
	}
	if share, given := rateLimitShare(header); given {
		if share > rateLimitMaxDelay {
			return 0, false
		}
		return min(share+rand.N(share/4+1), rateLimitMaxDelay), true
	}
	delay := rateLimitFirstDelay << (retry - 1)
	if delay > rateLimitMaxDelay || delay <= 0 {
		delay = rateLimitMaxDelay
	}
	// Somewhere between half the delay and the whole of it
	return delay/2 + rand.N(delay/2), true
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

// rateLimitShare is how long one request's share of the hub's allowance takes to come
// back, from the X-RateLimit-Limit and X-RateLimit-Period headers of a refusal: the hub
// admits that many requests in that period, so one more is admitted that fraction of the
// period after the last was.  Without those, X-RateLimit-Reset, how long until the whole
// allowance is back, is a wait that surely suffices.  The hub writes the durations as Go
// does, such as 1m0s.
func rateLimitShare(header http.Header) (share time.Duration, given bool) {
	limit, limitErr := strconv.Atoi(strings.TrimSpace(header.Get("X-RateLimit-Limit")))
	period, periodErr := time.ParseDuration(strings.TrimSpace(header.Get("X-RateLimit-Period")))
	if limitErr == nil && periodErr == nil && limit > 0 && period > 0 {
		if share = period / time.Duration(limit); share > 0 {
			return share, true
		}
	}
	if reset, err := time.ParseDuration(strings.TrimSpace(header.Get("X-RateLimit-Reset"))); err == nil && reset > 0 {
		return reset, true
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
