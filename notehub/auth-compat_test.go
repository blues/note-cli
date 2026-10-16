package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/notehub"
)

func TestOAuthLoginFallback(t *testing.T) {
	originalPolling, originalBrowser := authPollingLogin, authBrowserLogin
	t.Cleanup(func() { authPollingLogin, authBrowserLogin = originalPolling, originalBrowser })
	denied := errors.New("authorization denied")
	for _, tc := range []struct {
		name        string
		pollErr     error
		agent       bool
		cancel      bool
		wantBrowser bool
	}{
		{"polling success", nil, false, false, false},
		{"interactive fallback", fmt.Errorf("start: %w", notehub.ErrPollingLoginUnavailable), false, false, true},
		{"agent success", nil, true, false, false},
		{"agent unavailable", notehub.ErrPollingLoginUnavailable, true, false, false},
		{"denied", denied, false, false, false},
		{"cancelled before fallback", notehub.ErrPollingLoginUnavailable, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantToken := &notehub.AccessToken{AccessToken: "access"}
			authPollingLogin = func(_ context.Context, hub string, options notehub.PollingLoginOptions) (*notehub.AccessToken, error) {
				if hub != "api.notefile.net" || options.SignInName != "My CLI" || options.ExpiresInSeconds != flagLoginExpiresIn || (options.OnURL != nil) != tc.agent || (options.OnPending != nil) != tc.agent {
					t.Error("lost login options")
				}
				if tc.cancel {
					cancel()
				}
				if tc.pollErr != nil {
					return nil, tc.pollErr
				}
				return wantToken, nil
			}
			browserCalls := 0
			authBrowserLogin = func(hub string, options notehub.BrowserLoginOptions) (*notehub.AccessToken, error) {
				browserCalls++
				if hub != "api.notefile.net" || options.SignInName != "My CLI" {
					t.Error("lost browser login options")
				}
				return wantToken, nil
			}
			var reporter *agentSignInReporter
			if tc.agent {
				reporter = newAgentSignInReporter(io.Discard)
			}
			token, err := authOAuthLogin(ctx, "api.notefile.net", "My CLI", reporter)
			if (browserCalls == 1) != tc.wantBrowser {
				t.Errorf("browser called %d times", browserCalls)
			}
			switch {
			case tc.cancel:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("lost cancellation: %v", err)
				}
			case tc.pollErr != nil && !tc.wantBrowser:
				if !errors.Is(err, tc.pollErr) || token != nil {
					t.Errorf("lost failure: %v", err)
				}
				if tc.agent && (err == nil || !strings.Contains(err.Error(), "notehub login") || !strings.Contains(err.Error(), "NOTEHUB_TOKEN")) {
					t.Errorf("agent has no recovery guidance: %v", err)
				}
			default:
				if err != nil || token != wantToken {
					t.Errorf("login failed: %v", err)
				}
			}
		})
	}
}

// A valid local expiration cannot establish that Notehub still permits the
// sign-in. Only a 401 should start new approval; outages must preserve the login.
func TestReusableLoginChecksRevocation(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		env      string
		wantKeep bool
		wantErr  bool
	}{
		{"accepted", 200, `{"email":"person@example.com"}`, "", true, false},
		{"deleted session", 401, `{"err":"deleted sign-in"}`, "", false, false},
		{"plain 401", 401, "unauthorized", "", false, false},
		{"server outage", 503, `{"err":"unavailable"}`, "", false, true},
		{"malformed response", 200, "invalid json", "", false, true},
		{"network outage", 0, "", "", false, true},
		{"environment accepted", 200, `{"email":"person@example.com"}`, "api_key_env", true, false},
		{"environment rejected", 401, `{"err":"invalid token"}`, "api_key_env", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(lib.ConfigTokenEnvVar, tc.env)
			expires := time.Now().Add(time.Hour)
			config := &lib.ConfigSettings{Hub: "api.notefile.net", HubCreds: map[string]lib.ConfigCreds{
				"api.notefile.net": {Token: "oauth_access", User: "person@example.com", ExpiresAt: &expires},
			}}
			requests := 0
			http.DefaultClient = &http.Client{Transport: authStatusTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.String() != "https://api.notefile.net/userinfo" {
					t.Errorf("unexpected validation request: %s", r.URL)
				}
				if tc.status == 0 {
					return nil, errors.New("network unavailable")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			credentials, err := authReusableLogin(config, time.Now())
			if requests != 1 || (credentials != nil) != tc.wantKeep || (err != nil) != tc.wantErr {
				t.Errorf("requests=%d kept=%v err=%v", requests, credentials != nil, err)
			}
			if config.SavedCredentials(config.Hub) == nil {
				t.Error("validation removed the saved credentials")
			}
		})
	}
}
