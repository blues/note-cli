package lib

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blues/note-go/notehub"
)

// Data requests and credential checks share renewal, including persisted token
// rotation. This exercises the SDK helper with the CLI's real credential source.
func TestAuthenticatedConfigRequests(t *testing.T) {
	for _, path := range []string{"/req", "/v1/projects", "/v1/projects/app:1/files", "/userinfo"} {
		t.Run(path, func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(-time.Hour))
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			var calls []string
			http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
				calls = append(calls, req.URL.Path)
				body := `{"email":"user@example.com"}`
				if req.URL.Path == "/oauth2/token" {
					body = `{"access_token":"renewed","refresh_token":"rotated","expires_in":3600}`
				} else if req.Header.Get("Authorization") != "Bearer renewed" {
					t.Error("sent request before refreshing its credentials")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			if path == "/userinfo" {
				email, err := settings.IntrospectCredentials(context.Background(), settings.Hub)
				if err != nil || email != "user@example.com" {
					t.Fatalf("introspection failed: %q, %v", email, err)
				}
			} else {
				req, _ := http.NewRequest("POST", "https://"+settings.Hub+path, strings.NewReader(`{"req":"test"}`))
				response, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
			}
			if !slices.Equal(calls, []string{"/oauth2/token", path}) {
				t.Errorf("calls=%v", calls)
			}
			stored, err := readConfigFromFile()
			if err != nil || stored.SavedCredentials(settings.Hub).RefreshToken != "rotated" {
				t.Fatalf("rotation was not saved: %v", err)
			}
		})
	}
}

func TestAuthenticatedConfigEarlyRefreshFailure(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(strconv.FormatBool(rejected), func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(30*time.Second))
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			refreshes, requests := 0, 0
			http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/oauth2/token" {
					refreshes++
					return nil, errors.New("connection reset")
				}
				requests++
				if req.Header.Get("Authorization") != "Bearer old-access" {
					t.Error("lost the still-valid access token")
				}
				status := 200
				if rejected {
					status = 401
				}
				return &http.Response{StatusCode: status, Body: http.NoBody}, nil
			})
			req, _ := http.NewRequest("GET", "https://"+settings.Hub+"/v1/projects", nil)
			resp, err := settings.doAuthenticated(http.DefaultClient, req, settings.Hub)
			if resp != nil {
				resp.Body.Close()
			}
			wantRefreshes := 1
			if rejected {
				wantRefreshes = 2
			}
			if (err != nil) != rejected || requests != 1 || refreshes != wantRefreshes {
				t.Fatalf("requests=%d refreshes=%d err=%v", requests, refreshes, err)
			}
			if errors.Is(err, notehub.ErrRefreshTokenRejected) {
				t.Fatal("network failure reported as a rejected grant")
			}
		})
	}
}

func TestAuthenticatedConfig401(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(time.Hour))
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var calls []string
	http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.Path)
		code, body := 401, `{"err":"deleted sign-in"}`
		if req.URL.Path == "/oauth2/token" {
			code, body = 200, `{"access_token":"renewed","refresh_token":"rotated","expires_in":3600}`
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	_, err := settings.IntrospectCredentials(context.Background(), settings.Hub)
	if err == nil || !slices.Equal(calls, []string{"/userinfo", "/oauth2/token", "/userinfo"}) {
		t.Fatalf("revocation was not respected after one renewal: calls=%v, err=%v", calls, err)
	}
}
