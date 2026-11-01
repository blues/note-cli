package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Every data-command request family must pass through shared authentication,
// including the legacy paths that used to validate an expired token first.
func TestRequestPathsUseSharedAuthentication(t *testing.T) {
	apiTestReset(t)
	original := authenticatedDo
	t.Cleanup(func() { authenticatedDo = original })
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"api and V1 commands", func() error {
			rsp, err := apiOpen(http.DefaultClient, &apiRequest{Method: "GET", URL: &url.URL{Path: "/v1/projects"}, Header: http.Header{}})
			if rsp != nil {
				rsp.Body.Close()
			}
			return err
		}},
		{"legacy V0", func() error {
			_, err := reqHubV0JSON(false, "api.notefile.net", []byte(`{"req":"hub.app.get"}`), "", "", "", "", false, false, nil)
			return err
		}},
		{"legacy V1", func() error {
			_, err := reqHubV1JSON(false, "api.notefile.net", "GET", "/v1/projects", nil)
			return err
		}},
		{"skills", func() error {
			_, err := skillsV1Request("app:1", http.MethodGet, "", nil, nil, nil, nil)
			return err
		}},
		{"uploads", func() error {
			_, err := uploadsRequest(uploadsScope{Type: "file"}, "query", map[string]any{})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			unavailable := errors.New("refresh unavailable")
			authenticatedDo = func(client *http.Client, req *http.Request) (*http.Response, error) { calls++; return nil, unavailable }
			if err := tc.run(); !errors.Is(err, unavailable) || calls != 1 {
				t.Fatalf("request bypassed renewal failure: calls=%d, err=%v", calls, err)
			}
			authenticatedDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			}
			if err := tc.run(); err != nil {
				t.Fatalf("request failed after authentication: %v", err)
			}
		})
	}
}

// The shared request path waits out rate limiting and sends again, so a 429 reaches the
// legacy paths only once the retries have run out, and is then explained from the headers
// the hub puts on its bodiless refusal rather than read as an answer
func TestLegacyRequestsExplainRateLimiting(t *testing.T) {
	apiTestReset(t)
	original := authenticatedDo
	t.Cleanup(func() { authenticatedDo = original })
	header := headerOf("X-RateLimit-Limit", "7", "X-RateLimit-Period", "1m0s", "X-RateLimit-Reset", "48.2s", "X-RateLimit-Remaining", "0")
	authenticatedDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests", Header: header.Clone(), Body: http.NoBody}, nil
	}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"legacy V0", func() error {
			_, err := reqHubV0JSON(false, "api.notefile.net", []byte(`{"req":"hub.app.get"}`), "", "", "", "", false, false, nil)
			return err
		}},
		{"legacy V1", func() error {
			var rsp map[string]any
			return reqHubV1(false, "api.notefile.net", "GET", "/v1/projects", nil, &rsp)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			want := "api.notefile.net is limiting requests: the hub allows 7 requests a minute for your account, whole again in 48s"
			if err == nil || err.Error() != want {
				t.Fatalf("err=%v", err)
			}
		})
	}
	authenticatedDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests", Header: http.Header{}, Body: http.NoBody}, nil
	}
	if _, err := reqHubV1JSON(false, "api.notefile.net", "GET", "/v1/projects", nil); err == nil || !strings.Contains(err.Error(), "api.notefile.net is limiting requests right now") {
		t.Fatalf("without headers: err=%v", err)
	}
}

// The hub's refusal headers are read into words, in every form the hub might write them
func TestRateLimitDetail(t *testing.T) {
	for _, test := range []struct {
		limit, period, reset string
		want                 string
	}{
		{"7", "1m0s", "48.2s", "the hub allows 7 requests a minute for your account, whole again in 48s"},
		{"7", "1m0s", "48.5s", "the hub allows 7 requests a minute for your account, whole again in 49s"},
		{"7", "1m0s", "", "the hub allows 7 requests a minute for your account"},
		{"", "", "48.2s", "the hub's allowance for your account is whole again in 48s"},
		{"7", "1m0s", "300ms", "the hub allows 7 requests a minute for your account, whole again in under a second"},
		{"100", "5m0s", "", "the hub allows 100 requests every 5 minutes for your account"},
		{"1000", "1h0m0s", "", "the hub allows 1000 requests an hour for your account"},
		{"10", "1s", "", "the hub allows 10 requests a second for your account"},
		{"10", "30s", "", "the hub allows 10 requests every 30 seconds for your account"},
		{"10", "1m30s", "", "the hub allows 10 requests every 90 seconds for your account"},
		{"10", "1.5s", "", "the hub allows 10 requests every 1.5s for your account"},
		{"many", "1m0s", "soon", ""},
		{"", "", "", ""},
	} {
		header := headerOf("X-RateLimit-Limit", test.limit, "X-RateLimit-Period", test.period, "X-RateLimit-Reset", test.reset)
		if got := rateLimitDetail(header); got != test.want {
			t.Errorf("X-RateLimit %q/%q/%q: %q, want %q", test.limit, test.period, test.reset, got, test.want)
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
