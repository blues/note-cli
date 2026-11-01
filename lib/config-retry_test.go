package lib

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// rateLimitTestSleeps replaces the wait between retries with a record of each one
func rateLimitTestSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	original := rateLimitSleep
	t.Cleanup(func() { rateLimitSleep = original })
	sleeps := &[]time.Duration{}
	rateLimitSleep = func(ctx context.Context, wait time.Duration) error {
		*sleeps = append(*sleeps, wait)
		return ctx.Err()
	}
	return sleeps
}

// rateLimitTestTransport answers each request with the next status given, repeating the
// last, and records the body sent each time
func rateLimitTestTransport(t *testing.T, header http.Header, statuses ...int) (calls *int, bodies *[]string) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls, bodies = new(int), &[]string{}
	http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/oauth2/token" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"renewed","refresh_token":"rotated","expires_in":3600}`))}, nil
		}
		if req.Header.Get("Authorization") == "" {
			t.Error("a retried request lost its credentials")
		}
		body := ""
		if req.Body != nil {
			contents, _ := io.ReadAll(req.Body)
			body = string(contents)
		}
		*bodies = append(*bodies, body)
		status := statuses[min(*calls, len(statuses)-1)]
		*calls++
		rsp := &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"err":"status ` + strconv.Itoa(status) + `"}`))}
		if status == http.StatusTooManyRequests {
			rsp.Header = header.Clone()
		}
		return rsp, nil
	})
	return calls, bodies
}

// A rate-limited request is sent again with the same body, after waits that double
// each time, until the hub accepts it; the body of the refusal is never seen
func TestRateLimitedRequestIsRetried(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	sleeps := rateLimitTestSleeps(t)
	calls, bodies := rateLimitTestTransport(t, nil, 429, 429, 429, 200)
	req, _ := http.NewRequest("POST", "https://"+settings.Hub+"/req", strings.NewReader(`{"req":"test"}`))
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != 200 || *calls != 4 {
		t.Fatalf("status=%d calls=%d", rsp.StatusCode, *calls)
	}
	for i, body := range *bodies {
		if body != `{"req":"test"}` {
			t.Errorf("request %d sent body %q", i+1, body)
		}
	}
	if len(*sleeps) != 3 {
		t.Fatalf("sleeps=%v", *sleeps)
	}
	for i, wait := range *sleeps {
		full := rateLimitFirstDelay << i
		if wait < full/2 || wait > full {
			t.Errorf("wait %d was %s, not between %s and %s", i+1, wait, full/2, full)
		}
	}
}

// A request with no body is retried too
func TestRateLimitedRequestWithoutBodyIsRetried(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	rateLimitTestSleeps(t)
	calls, _ := rateLimitTestTransport(t, nil, 429, 200)
	req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	rsp.Body.Close()
	if rsp.StatusCode != 200 || *calls != 2 {
		t.Fatalf("status=%d calls=%d", rsp.StatusCode, *calls)
	}
}

// The hub's headers say how long to wait: a Retry-After exactly, or one request's share
// of the allowance that X-RateLimit-Limit and X-RateLimit-Period describe, with up to a
// quarter again, or the X-RateLimit-Reset until the whole allowance is back; and the delay
// when the hub says nothing readable
func TestRateLimitedRequestHonorsHeaders(t *testing.T) {
	hubHeaders := headerOf("X-RateLimit-Limit", "7", "X-RateLimit-Period", "1m0s", "X-RateLimit-Reset", "48.571428571s", "X-RateLimit-Remaining", "0")
	share := time.Minute / 7
	for _, test := range []struct {
		name   string
		header func() http.Header
		least  time.Duration
		most   time.Duration
	}{
		{"Retry-After seconds", func() http.Header { return http.Header{"Retry-After": {"3"}} }, 3 * time.Second, 3 * time.Second},
		{"Retry-After date", func() http.Header {
			return http.Header{"Retry-After": {time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)}}
		}, 3 * time.Second, 5 * time.Second},
		{"Retry-After zero falls back to the delay", func() http.Header { return http.Header{"Retry-After": {"0"}} }, rateLimitFirstDelay / 2, rateLimitFirstDelay},
		{"Retry-After unreadable falls back to the delay", func() http.Header { return http.Header{"Retry-After": {"soon"}} }, rateLimitFirstDelay / 2, rateLimitFirstDelay},
		{"the hub's X-RateLimit headers", func() http.Header { return hubHeaders.Clone() }, share, share + share/4},
		{"X-RateLimit-Reset alone", func() http.Header { return headerOf("X-RateLimit-Reset", "12.5s") }, 12500 * time.Millisecond, 12500*time.Millisecond + 12500*time.Millisecond/4},
		{"Retry-After over X-RateLimit", func() http.Header { h := hubHeaders.Clone(); h.Set("Retry-After", "3"); return h }, 3 * time.Second, 3 * time.Second},
		{"unreadable X-RateLimit falls back to the delay", func() http.Header {
			return headerOf("X-RateLimit-Limit", "many", "X-RateLimit-Period", "a while", "X-RateLimit-Reset", "soon")
		}, rateLimitFirstDelay / 2, rateLimitFirstDelay},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(time.Hour))
			sleeps := rateLimitTestSleeps(t)
			calls, _ := rateLimitTestTransport(t, test.header(), 429, 200)
			req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
			rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
			if err != nil {
				t.Fatal(err)
			}
			rsp.Body.Close()
			if *calls != 2 || len(*sleeps) != 1 {
				t.Fatalf("calls=%d sleeps=%v", *calls, *sleeps)
			}
			if wait := (*sleeps)[0]; wait < test.least || wait > test.most {
				t.Fatalf("waited %s, not between %s and %s", wait, test.least, test.most)
			}
		})
	}
}

// A hub that asks for a longer wait than the CLI is willing to make isn't retried, and
// its answer is returned for the caller to explain
func TestRateLimitedRequestWaitTooLong(t *testing.T) {
	for _, test := range []struct {
		name   string
		header http.Header
	}{
		{"Retry-After", http.Header{"Retry-After": {"600"}}},
		{"X-RateLimit-Reset", headerOf("X-RateLimit-Reset", "5m0s")},
		{"X-RateLimit share", headerOf("X-RateLimit-Limit", "1", "X-RateLimit-Period", "2m0s")},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(time.Hour))
			sleeps := rateLimitTestSleeps(t)
			calls, _ := rateLimitTestTransport(t, test.header, 429, 200)
			req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
			rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
			if err != nil {
				t.Fatal(err)
			}
			defer rsp.Body.Close()
			if rsp.StatusCode != 429 || *calls != 1 || len(*sleeps) != 0 {
				t.Fatalf("status=%d calls=%d sleeps=%v", rsp.StatusCode, *calls, *sleeps)
			}
		})
	}
}

// The retry budget includes time spent sending requests and waiting. A delay that
// won't fit is never shortened, and the final refusal is left intact for the caller.
func TestRateLimitedRequestRetryBudget(t *testing.T) {
	for _, test := range []struct {
		name        string
		header      http.Header
		requestTime time.Duration
		finalStatus int
		wantCalls   int
		wantElapsed time.Duration
		wantWait    time.Duration
	}{
		{"repeated minute waits", headerOf("Retry-After", "60"), 0, 429, 2, time.Minute, time.Minute},
		{"do not shorten Retry-After", headerOf("Retry-After", "40"), 0, 429, 2, 40 * time.Second, 40 * time.Second},
		{"request time counts", headerOf("Retry-After", "25"), 10 * time.Second, 429, 2, 45 * time.Second, 25 * time.Second},
		{"request exhausts budget", headerOf("Retry-After", "1"), time.Minute, 429, 1, time.Minute, 0},
		{"X-RateLimit waits", headerOf("X-RateLimit-Limit", "1", "X-RateLimit-Period", "1m"), 0, 429, 2, time.Minute, time.Minute},
		{"successful retry", headerOf("Retry-After", "10"), time.Second, 200, 2, 12 * time.Second, 10 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := refreshTestConfig(t, time.Now().Add(time.Hour))
				originalNotify := ConfigRateLimitNotify
				t.Cleanup(func() { ConfigRateLimitNotify = originalNotify })
				var waits []time.Duration
				ConfigRateLimitNotify = func(wait time.Duration, _, _ int) { waits = append(waits, wait) }
				calls := 0
				client := &http.Client{Transport: refreshTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					time.Sleep(test.requestTime)
					status := http.StatusTooManyRequests
					if calls > 1 {
						status = test.finalStatus
					}
					return &http.Response{StatusCode: status, Header: test.header.Clone(), Body: io.NopCloser(strings.NewReader("response body"))}, nil
				})}
				req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
				started := time.Now()
				rsp, err := settings.doAuthenticated(client, req, settings.Hub)
				if err != nil {
					t.Fatal(err)
				}
				defer rsp.Body.Close()
				if elapsed := time.Since(started); elapsed != test.wantElapsed || calls != test.wantCalls || rsp.StatusCode != test.finalStatus {
					t.Fatalf("elapsed=%s calls=%d status=%d, want %s/%d/%d", elapsed, calls, rsp.StatusCode, test.wantElapsed, test.wantCalls, test.finalStatus)
				}
				wantWaits := 0
				if test.wantWait > 0 {
					wantWaits = 1
				}
				if len(waits) != wantWaits || (wantWaits == 1 && waits[0] != test.wantWait) {
					t.Fatalf("waits=%v, want %d waits of %s", waits, wantWaits, test.wantWait)
				}
				if body, err := io.ReadAll(rsp.Body); err != nil || string(body) != "response body" {
					t.Fatalf("final body=%q err=%v", body, err)
				}
			})
		})
	}
}

// When the hub keeps refusing, the retries run out and the last refusal is returned
// whole, so the caller can read what the hub said
func TestRateLimitedRequestGivesUp(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	sleeps := rateLimitTestSleeps(t)
	calls, _ := rateLimitTestTransport(t, nil, 429)
	req, _ := http.NewRequest("POST", "https://"+settings.Hub+"/req", strings.NewReader(`{"req":"test"}`))
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	body, _ := io.ReadAll(rsp.Body)
	if rsp.StatusCode != 429 || *calls != 1+rateLimitRetries || len(*sleeps) != rateLimitRetries || string(body) != `{"err":"status 429"}` {
		t.Fatalf("status=%d calls=%d sleeps=%d body=%q", rsp.StatusCode, *calls, len(*sleeps), body)
	}
	if last := (*sleeps)[len(*sleeps)-1]; last > rateLimitMaxDelay {
		t.Fatalf("waited %s, longer than %s", last, rateLimitMaxDelay)
	}
}

// A body that can't be read again isn't sent again
func TestRateLimitedRequestUnreplayableBody(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	sleeps := rateLimitTestSleeps(t)
	calls, _ := rateLimitTestTransport(t, nil, 429, 200)
	req, _ := http.NewRequest("POST", "https://"+settings.Hub+"/req", struct{ io.Reader }{strings.NewReader(`{"req":"test"}`)})
	if req.GetBody != nil {
		t.Fatal("the test needs a body that can't be read again")
	}
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != 429 || *calls != 1 || len(*sleeps) != 0 {
		t.Fatalf("status=%d calls=%d sleeps=%v", rsp.StatusCode, *calls, *sleeps)
	}
}

// Rate limiting and credential renewal work together: a retried request that is then
// refused as unauthorized is renewed and sent once more, with its body
func TestRateLimitedThenUnauthorizedRequest(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	rateLimitTestSleeps(t)
	calls, bodies := rateLimitTestTransport(t, nil, 429, 401, 200)
	req, _ := http.NewRequest("POST", "https://"+settings.Hub+"/req", strings.NewReader(`{"req":"test"}`))
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	rsp.Body.Close()
	if rsp.StatusCode != 200 || *calls != 3 || len(*bodies) != 3 {
		t.Fatalf("status=%d calls=%d bodies=%v", rsp.StatusCode, *calls, *bodies)
	}
	for i, body := range *bodies {
		if body != `{"req":"test"}` {
			t.Errorf("request %d sent body %q", i+1, body)
		}
	}
}

// Each wait is reported when something is listening
func TestRateLimitedRequestNotifies(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	sleeps := rateLimitTestSleeps(t)
	rateLimitTestTransport(t, http.Header{"Retry-After": {"2"}}, 429, 429, 200)
	original := ConfigRateLimitNotify
	t.Cleanup(func() { ConfigRateLimitNotify = original })
	var told []string
	ConfigRateLimitNotify = func(wait time.Duration, retry int, retries int) {
		told = append(told, wait.String()+" "+strconv.Itoa(retry)+"/"+strconv.Itoa(retries))
	}
	req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if err != nil {
		t.Fatal(err)
	}
	rsp.Body.Close()
	want := []string{"2s 1/" + strconv.Itoa(rateLimitRetries), "2s 2/" + strconv.Itoa(rateLimitRetries)}
	if len(*sleeps) != 2 || strings.Join(told, ",") != strings.Join(want, ",") {
		t.Fatalf("told=%v sleeps=%v", told, *sleeps)
	}
}

// A cancelled request stops waiting
func TestRateLimitedRequestCancelled(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	calls, _ := rateLimitTestTransport(t, nil, 429, 200)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+settings.Hub+"/v1/projects", nil)
	rsp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
	if rsp != nil {
		rsp.Body.Close()
	}
	if !errors.Is(err, context.Canceled) || *calls != 1 {
		t.Fatalf("calls=%d err=%v", *calls, err)
	}
}

// The waits are read from Retry-After as the HTTP spec writes it, and otherwise grow
func TestRateLimitWait(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		value string
		wait  time.Duration
		given bool
	}{
		{"", 0, false},
		{"soon", 0, false},
		{"0", 0, true},
		{"-1", -time.Second, true},
		{"7", 7 * time.Second, true},
		{" 7 ", 7 * time.Second, true},
		{now.Add(90 * time.Second).UTC().Format(http.TimeFormat), 90 * time.Second, true},
	} {
		wait, given := retryAfter(http.Header{"Retry-After": {test.value}}, now)
		if given != test.given || (given && (wait < test.wait-time.Second || wait > test.wait)) {
			t.Errorf("Retry-After %q: wait=%s given=%v", test.value, wait, given)
		}
	}
	for retry := 1; retry <= rateLimitRetries+5; retry++ {
		wait, willing := rateLimitWait(http.Header{}, retry)
		full := min(rateLimitFirstDelay<<(retry-1), rateLimitMaxDelay)
		if !willing || wait < full/2 || wait > full {
			t.Errorf("retry %d: wait=%s willing=%v, expected between %s and %s", retry, wait, willing, full/2, full)
		}
	}
	if _, willing := rateLimitWait(http.Header{"Retry-After": {"61"}}, 1); willing {
		t.Error("willing to wait longer than the most allowed")
	}
	if wait, willing := rateLimitWait(http.Header{"Retry-After": {"60"}}, 1); !willing || wait != 60*time.Second {
		t.Errorf("the most allowed: wait=%s willing=%v", wait, willing)
	}
	for _, test := range []struct {
		limit, period, reset string
		share                time.Duration
		given                bool
	}{
		{"7", "1m0s", "48.5s", time.Minute / 7, true},
		{"1000", "1m0s", "", 60 * time.Millisecond, true},
		{"", "", "48.5s", 48500 * time.Millisecond, true},
		{"7", "", "48.5s", 48500 * time.Millisecond, true},
		{"many", "1m0s", "soon", 0, false},
		{"0", "1m0s", "", 0, false},
		{"7", "0s", "", 0, false},
		{"7", "-1m0s", "-5s", 0, false},
		{"", "", "", 0, false},
	} {
		header := headerOf("X-RateLimit-Limit", test.limit, "X-RateLimit-Period", test.period, "X-RateLimit-Reset", test.reset)
		if share, given := rateLimitShare(header); given != test.given || share != test.share {
			t.Errorf("X-RateLimit %q/%q/%q: share=%s given=%v", test.limit, test.period, test.reset, share, given)
		}
	}
}

// headerOf builds a header from key and value pairs as a response's arrives, with the keys
// in canonical form: the hub's spellings, such as X-RateLimit-Limit, aren't, and a map
// literal written with them isn't found by Get
func headerOf(pairs ...string) http.Header {
	header := http.Header{}
	for i := 0; i+1 < len(pairs); i += 2 {
		header.Set(pairs[i], pairs[i+1])
	}
	return header
}
