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
