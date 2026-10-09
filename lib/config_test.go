// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFlagParseReturnsArgumentErrors(t *testing.T) {
	savedFlags, savedArgs := flag.CommandLine, os.Args
	t.Cleanup(func() {
		flag.CommandLine, os.Args = savedFlags, savedArgs
	})
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	flag.String("signin-agent", "", "agent name")
	os.Args = []string{"test", "--signin-agent"}
	if err := FlagParse(false, false); err == nil || !strings.Contains(err.Error(), "flag needs an argument") {
		t.Fatalf("expected a missing argument error, got %v", err)
	}
}

// A command line that only sets configuration saves it, in either spelling; one that
// also does anything else doesn't
func TestConfigFlagsOnly(t *testing.T) {
	tests := []struct {
		name string
		args []string
		save bool
	}{
		{"legacy", []string{"-hub", "example.com"}, true},
		{"modern", []string{"--hub", "example.com"}, true},
		{"legacy equals", []string{"-hub=example.com"}, true},
		{"modern equals", []string{"--hub=example.com"}, true},
		{"mixed", []string{"-interface", "serial", "--port=/dev/example", "-portconfig=9600"}, true},
		{"legacy notecard", []string{"-interface", "serial", "-port", "/dev/example", "-portconfig", "9600"}, true},
		{"modern notecard", []string{"--interface", "serial", "--port", "/dev/example", "--portconfig", "9600"}, true},
		{"repeated", []string{"--hub=first.example", "-hub", "second.example"}, true},
		{"dash value", []string{"--port", "-"}, true},
		{"negative value", []string{"--portconfig", "-1"}, true},
		{"flag-like value", []string{"--port", "--version"}, true},
		{"terminator as value", []string{"--port", "--"}, true},
		{"empty", nil, false},
		{"terminator only", []string{"--"}, false},
		{"trailing terminator", []string{"--hub=example.com", "--"}, true},
		{"legacy operation", []string{"-hub", "example.com", "-version"}, false},
		{"modern operation", []string{"--hub=example.com", "--version"}, false},
		{"false operation", []string{"--version=false", "--hub", "example.com"}, false},
		{"operation only", []string{"--version"}, false},
		{"request", []string{"--hub", "example.com", `{"req":"hub.app.get"}`}, false},
		{"command", []string{"--hub=example.com", "list"}, false},
		{"flags after positional", []string{"list", "--hub", "example.com"}, false},
		{"terminated positional", []string{"--hub=example.com", "--", "--version"}, false},
		{"terminated config flag", []string{"--", "--hub=example.com"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet("config", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.String("hub", "", "")
			flags.String("interface", "", "")
			flags.String("port", "", "")
			flags.Int("portconfig", 0, "")
			flags.Bool("version", false, "")
			if err := flags.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			if got := configFlagsOnly(flags); got != test.save {
				t.Errorf("%v: save is %v, expected %v", test.args, got, test.save)
			}
		})
	}
}

// Signing out of an OAuth sign-in deletes it at Notehub with the token that
// made it, and reports a refusal rather than hiding it.
func TestDeleteSignIn(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	status := http.StatusNoContent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(status)
	}))
	defer server.Close()

	if err := deleteSignIn(server.URL, "ory_at_example"); err != nil {
		t.Fatalf("deleteSignIn: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/auth/logout" || gotAuth != "Bearer ory_at_example" {
		t.Errorf("got %s %s with %q", gotMethod, gotPath, gotAuth)
	}

	status = http.StatusUnauthorized
	if err := deleteSignIn(server.URL, "ory_at_example"); err == nil {
		t.Error("a refused sign-out should be reported")
	}
}

// The API lives at the "api." host whichever way the hub was configured
func TestAPIBaseURL(t *testing.T) {
	for hub, want := range map[string]string{
		"notehub.io":     "https://api.notehub.io",
		"api.notehub.io": "https://api.notehub.io",
	} {
		if got := apiBaseURL(hub); got != want {
			t.Errorf("apiBaseURL(%q) = %q, want %q", hub, got, want)
		}
	}
}

// A token in the environment is used in place of the saved sign-in, for whatever hub is
// configured, and says where it came from; a blank one is no token at all
func TestEnvCredentials(t *testing.T) {
	settings := &ConfigSettings{Hub: "example.com", HubCreds: map[string]ConfigCreds{
		"example.com": {User: "someone@example.com", Token: "api_key_saved"},
	}}
	t.Setenv(ConfigTokenEnvVar, "")
	if creds := settings.DefaultCredentials(); creds == nil || creds.Token != "api_key_saved" || creds.Source != "" || creds.Hub != "example.com" {
		t.Errorf("without %s, credentials are %+v, expected the saved sign-in", ConfigTokenEnvVar, creds)
	}
	t.Setenv(ConfigTokenEnvVar, " api_key_env ")
	if creds := settings.DefaultCredentials(); creds == nil || creds.Token != "api_key_env" || creds.Source != ConfigTokenEnvVar || creds.Hub != "example.com" || creds.User != "" {
		t.Errorf("with %s, credentials are %+v, expected its token", ConfigTokenEnvVar, creds)
	}
	settings.Hub = "other.example.com"
	if creds := settings.DefaultCredentials(); creds == nil || creds.Token != "api_key_env" || creds.Hub != "other.example.com" {
		t.Errorf("with %s, credentials for a hub with no sign-in are %+v, expected its token", ConfigTokenEnvVar, creds)
	}
	t.Setenv(ConfigTokenEnvVar, "  ")
	if creds := settings.DefaultCredentials(); creds != nil {
		t.Errorf("with %s blank, credentials for a hub with no sign-in are %+v, expected none", ConfigTokenEnvVar, creds)
	}
}

// Every request carries the environment's token when there is one
func TestAuthenticationHeaderFromEnv(t *testing.T) {
	saved := config
	t.Cleanup(func() { config = saved })
	config = &ConfigSettings{Hub: "example.com", HubCreds: map[string]ConfigCreds{}}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/projects", nil)
	t.Setenv(ConfigTokenEnvVar, "")
	if err := ConfigAuthenticationHeader(req); err == nil || !strings.Contains(err.Error(), ConfigTokenEnvVar) {
		t.Errorf("with no sign-in and no %s, got %v, expected an error naming the variable", ConfigTokenEnvVar, err)
	}
	t.Setenv(ConfigTokenEnvVar, "api_key_env")
	if err := ConfigAuthenticationHeader(req); err != nil || req.Header.Get("Authorization") != "Bearer api_key_env" {
		t.Errorf("with %s, got %v and authorization %q, expected its token", ConfigTokenEnvVar, err, req.Header.Get("Authorization"))
	}
}

// What is written names the saved hub, not one given for this command alone, until that
// one is adopted
func TestWriteKeepsSavedHub(t *testing.T) {
	saved := configSavedHub
	t.Cleanup(func() { configSavedHub = saved })
	settings := &ConfigSettings{Hub: "session.example.com"}
	configSavedHub = ""
	if out, _ := settings.savedJSON(); !strings.Contains(string(out), `"hub": "session.example.com"`) {
		t.Errorf("with no saved hub known, wrote %s", out)
	}
	configSavedHub = "saved.example.com"
	if out, _ := settings.savedJSON(); !strings.Contains(string(out), `"hub": "saved.example.com"`) || settings.Hub != "session.example.com" {
		t.Errorf("with a saved hub, wrote %s and left the hub in use as %q", out, settings.Hub)
	}
	settings.AdoptHub()
	if out, _ := settings.savedJSON(); !strings.Contains(string(out), `"hub": "session.example.com"`) {
		t.Errorf("after adopting the hub in use, wrote %s", out)
	}
}

// A login is saved per hub, and one with no token or user is no login at all
func TestSavedCredentials(t *testing.T) {
	settings := &ConfigSettings{Hub: "a.example.com", HubCreds: map[string]ConfigCreds{
		"a.example.com": {User: "a@example.com", Token: "api_key_a"},
		"b.example.com": {User: "", Token: "api_key_b"},
	}}
	if creds := settings.SavedCredentials("a.example.com"); creds == nil || creds.Hub != "a.example.com" || creds.User != "a@example.com" {
		t.Errorf("a.example.com: %+v", creds)
	}
	if creds := settings.SavedCredentials("b.example.com"); creds != nil {
		t.Errorf("b.example.com has no user, and so no login: %+v", creds)
	}
	if creds := settings.SavedCredentials("c.example.com"); creds != nil {
		t.Errorf("c.example.com was never logged in to: %+v", creds)
	}
}

// A browser login says when it expires, or expired; a personal access token records no
// expiration, since Notehub keeps that
func TestExpirationDescription(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	for _, test := range []struct {
		expiresAt *time.Time
		want      string
	}{
		{nil, ""},
		{&future, "expires at 2026-10-08 13:00:00 UTC"},
		{&past, "expired at 2026-10-08 11:00:00 UTC"},
		{&now, "expired at 2026-10-08 12:00:00 UTC"},
	} {
		creds := ConfigCreds{Token: "ory_at_x", ExpiresAt: test.expiresAt}
		if got := creds.ExpirationDescription(now); got != test.want {
			t.Errorf("expires at %v: got %q, want %q", test.expiresAt, got, test.want)
		}
	}
}
