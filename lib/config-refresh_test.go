package lib

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blues/note-go/notehub"
)

type refreshTransport func(*http.Request) (*http.Response, error)

func (fn refreshTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func refreshTestConfig(t *testing.T, expires time.Time) *ConfigSettings {
	t.Helper()
	oldPath, oldHub := configSettingsPath, configSavedHub
	t.Cleanup(func() { configSettingsPath, configSavedHub = oldPath, oldHub })
	path := filepath.Join(t.TempDir(), "config.json")
	configSettingsPath = func() string { return path }
	configSavedHub = "api.other.example"
	t.Setenv(ConfigTokenEnvVar, "")
	settings := &ConfigSettings{Hub: "api.example.com", HubCreds: map[string]ConfigCreds{
		"api.other.example": {User: "other@example.com", Token: "api_key_other"},
	}}
	settings.SetOAuthCredentials(&notehub.AccessToken{Email: "user@example.com", AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: expires})
	if err := settings.Write(); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestCredentialRefresh(t *testing.T) {
	for _, test := range []struct {
		name      string
		expires   time.Duration
		rejected  string
		env       string
		legacy    bool
		status    int
		body      string
		wantCalls int
		wantErr   bool
	}{
		{"expired", -time.Hour, "", "", false, 200, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`, 1, false},
		{"nearly expired", 30 * time.Second, "", "", false, 200, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`, 1, false},
		{"fresh", time.Hour, "", "", false, 0, "", 0, false},
		{"401", time.Hour, "old-access", "", false, 200, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`, 1, false},
		{"environment", -time.Hour, "", "api_key_env", false, 0, "", 0, false},
		{"legacy", -time.Hour, "old-access", "", true, 0, "", 0, false},
		{"revoked", -time.Hour, "", "", false, 400, `{"error":"invalid_grant"}`, 1, true},
		{"outage", -time.Hour, "", "", false, 503, `unavailable`, 1, true},
		{"early outage", 30 * time.Second, "", "", false, 503, `unavailable`, 1, false},
		{"early network failure", 30 * time.Second, "", "", false, 0, "", 1, false},
		{"expired network failure", -time.Hour, "", "", false, 0, "", 1, true},
		{"401 during outage", 30 * time.Second, "old-access", "", false, 503, `unavailable`, 1, true},
		{"early revoked", 30 * time.Second, "", "", false, 400, `{"error":"invalid_grant"}`, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(test.expires))
			if test.legacy {
				settings.SetDefaultCredentials("old-access", "user@example.com", settings.DefaultCredentials().ExpiresAt)
				if err := settings.Write(); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv(ConfigTokenEnvVar, test.env)
			before, _ := os.ReadFile(configSettingsPath())
			calls := 0
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.String() != "https://example.com/oauth2/token" {
					t.Errorf("wrong issuer: %s", req.URL)
				}
				if err := req.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if req.Form.Get("refresh_token") != "old-refresh" {
					t.Error("wrong refresh token")
				}
				if test.status == 0 {
					return nil, errors.New("connection reset")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: http.Header{}}, nil
			})
			changed, err := settings.RefreshCredentials(context.Background(), test.rejected)
			if (err != nil) != test.wantErr || calls != test.wantCalls || changed != (test.wantCalls == 1 && test.status == 200 && !test.wantErr) {
				t.Fatalf("changed=%v calls=%d err=%v", changed, calls, err)
			}
			if test.status == 400 && !errors.Is(err, notehub.ErrRefreshTokenRejected) {
				t.Fatalf("lost rejection: %v", err)
			}
			after, _ := os.ReadFile(configSettingsPath())
			if !changed {
				if !bytes.Equal(before, after) {
					t.Error("changed credentials without a successful refresh")
				}
				return
			}
			stored, err := readConfigFromFile()
			if err != nil {
				t.Fatal(err)
			}
			creds := stored.SavedCredentials(settings.Hub)
			if creds.Token != "new-access" || creds.RefreshToken != "new-refresh" || creds.User != "user@example.com" || time.Until(*creds.ExpiresAt) < 59*time.Minute {
				t.Error("did not preserve the rotated token pair and identity")
			}
			if stored.Hub != "api.other.example" || stored.SavedCredentials("api.other.example").Token != "api_key_other" {
				t.Error("changed the saved hub or another login")
			}
		})
	}
}

func TestRefreshReusesOtherProcessCredentials(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(-time.Hour))
	stale, err := readConfigFromFile()
	if err != nil {
		t.Fatal(err)
	}
	stale.Hub = settings.Hub
	later := time.Now().Add(time.Hour)
	settings.SetOAuthCredentials(&notehub.AccessToken{Email: "user@example.com", AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: later})
	if err := settings.Write(); err != nil {
		t.Fatal(err)
	}
	changed, err := stale.RefreshCredentials(context.Background(), "old-access")
	if err != nil || !changed || stale.DefaultCredentials().Token != "new-access" {
		t.Fatalf("did not reuse newer credentials: %v", err)
	}
	// A stale settings write cannot restore the old pair, nor an already removed login.
	settings.HubCreds[settings.Hub] = ConfigCreds{Token: "old-access", User: "user@example.com"}
	if err := settings.Write(); err != nil {
		t.Fatal(err)
	}
	stored, _ := readConfigFromFile()
	if stored.SavedCredentials(settings.Hub).RefreshToken != "new-refresh" {
		t.Fatal("stale settings write restored an old token")
	}
	delete(stored.HubCreds, settings.Hub)
	if err := stored.withStoredCredentials(context.Background(), func(current *ConfigSettings) (bool, error) { delete(current.HubCreds, settings.Hub); return true, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := stale.RefreshCredentials(context.Background(), "new-access"); err == nil {
		t.Fatal("resurrected a removed login")
	}
}

func TestRefreshLockCancellation(t *testing.T) {
	settings := refreshTestConfig(t, time.Now().Add(-time.Hour))
	err := withConfigLock(context.Background(), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := settings.RefreshCredentials(ctx, "")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected lock cancellation: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	for _, tc := range []struct{ hub, issuer string }{
		{"api.notefile.net", "notehub.io"},
		{"api.example.com", "example.com"},
	} {
		t.Run(tc.hub, func(t *testing.T) {
			settings := refreshTestConfig(t, time.Now().Add(time.Hour))
			settings.Hub = tc.hub
			settings.SetOAuthCredentials(&notehub.AccessToken{Email: "user@example.com", AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: time.Now().Add(time.Hour)})
			if err := settings.Write(); err != nil {
				t.Fatal(err)
			}
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			revoked := map[string]string{}
			deleted := false
			http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				if req.URL.Path == "/auth/logout" {
					if req.URL.Host != tc.hub {
						t.Errorf("sign-in deletion went to %s, want %s", req.URL.Host, tc.hub)
					}
					deleted = req.Header.Get("Authorization") == "Bearer old-access"
					status = http.StatusNoContent
				} else if req.URL.Path == "/oauth2/revoke" {
					if req.URL.Host != tc.issuer {
						t.Errorf("revocation went to %s, want %s", req.URL.Host, tc.issuer)
					}
					if err := req.ParseForm(); err != nil {
						t.Fatal(err)
					}
					revoked[req.Form.Get("token_type_hint")] = req.Form.Get("token")
				} else {
					t.Error("unexpected logout request")
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
			})
			if err := settings.RemoveDefaultCredentials(); err != nil {
				t.Fatal(err)
			}
			stored, err := readConfigFromFile()
			if err != nil {
				t.Fatal(err)
			}
			if !deleted || revoked["access_token"] != "old-access" || revoked["refresh_token"] != "old-refresh" {
				t.Error("logout did not end the entire OAuth grant")
			}
			if settings.DefaultCredentials() != nil || stored.SavedCredentials(settings.Hub) != nil || stored.SavedCredentials("api.other.example") == nil {
				t.Error("logout did not remove only the selected login")
			}
		})
	}
}

// This subprocess helper loads the old pair before the parent releases either CLI.
func TestConfigRefreshProcess(t *testing.T) {
	path := os.Getenv("NOTECLI_REFRESH_TEST_PATH")
	if path == "" {
		return
	}
	configSettingsPath = func() string { return path }
	settings, err := readConfigFromFile()
	if err != nil {
		t.Fatal(err)
	}
	settings.Hub = "api.example.com"
	endpoint, err := url.Parse(os.Getenv("NOTECLI_REFRESH_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = refreshTransport(func(req *http.Request) (*http.Response, error) {
		copy := req.Clone(req.Context())
		copy.URL.Scheme, copy.URL.Host = endpoint.Scheme, endpoint.Host
		return original.RoundTrip(copy)
	})
	fmt.Println("ready")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.RefreshCredentials(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if settings.DefaultCredentials().Token != "new-access" {
		t.Fatal("did not load renewed credentials")
	}
}

func TestConcurrentProcessRefresh(t *testing.T) {
	refreshTestConfig(t, time.Now().Add(-time.Hour))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if calls.Add(1) != 1 {
			t.Error("redeemed a rotating token more than once")
		}
		time.Sleep(100 * time.Millisecond)
		io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)
	}))
	defer server.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	type child struct {
		cmd    *exec.Cmd
		stdin  io.WriteCloser
		output *bufio.Reader
		stderr bytes.Buffer
	}
	children := make([]*child, 2)
	for i := range children {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)
		c := &child{cmd: exec.CommandContext(ctx, executable, "-test.run=^TestConfigRefreshProcess$")}
		c.cmd.Env = append(os.Environ(), "NOTECLI_REFRESH_TEST_PATH="+configSettingsPath(), "NOTECLI_REFRESH_TEST_URL="+server.URL)
		c.cmd.Stderr = &c.stderr
		c.stdin, err = c.cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		c.output = bufio.NewReader(stdout)
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if ready, err := c.output.ReadString('\n'); err != nil || ready != "ready\n" {
			t.Fatalf("child not ready: %q %v", ready, err)
		}
		children[i] = c
	}
	for _, c := range children {
		io.WriteString(c.stdin, "go\n")
		c.stdin.Close()
	}
	for _, c := range children {
		output, _ := io.ReadAll(c.output)
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("child failed: %v\n%s\n%s", err, output, c.stderr.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one renewal, got %d", calls.Load())
	}
}
