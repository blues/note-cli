// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
)

type authStatusTransport func(*http.Request) (*http.Response, error)

func (fn authStatusTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// Selection must happen before introspection: default status must not contact or
// expose inactive connections. --all retains the full report and its JSON shape.
func TestAuthStatusCommand(t *testing.T) {
	t.Setenv(lib.ConfigTokenEnvVar, "")
	savedFlags, savedClient := flag.CommandLine, http.DefaultClient
	savedAll, savedJSON, savedPretty := flagAuthAll, flagJson, flagPretty
	savedConfig := &lib.ConfigSettings{Hub: lib.ConfigSavedHub()}
	t.Cleanup(func() {
		flag.CommandLine, http.DefaultClient = savedFlags, savedClient
		flagAuthAll, flagJson, flagPretty = savedAll, savedJSON, savedPretty
		savedConfig.AdoptHub()
	})

	for _, test := range []struct {
		name     string
		args     []string
		wantHubs []string
		empty    bool
		rejected bool
		wantErr  bool
	}{
		{name: "text", wantHubs: []string{"b.invalid"}},
		{name: "json", args: []string{"--json"}, wantHubs: []string{"b.invalid"}},
		{name: "pretty json", args: []string{"--json", "--pretty"}, wantHubs: []string{"b.invalid"}},
		{name: "all text", args: []string{"--all"}, wantHubs: []string{"b.invalid", "a.invalid", "c.invalid"}},
		{name: "all json", args: []string{"--all", "--json"}, wantHubs: []string{"b.invalid", "a.invalid", "c.invalid"}},
		{name: "all pretty json", args: []string{"--all", "--json", "--pretty"}, wantHubs: []string{"b.invalid", "a.invalid", "c.invalid"}},
		{name: "all false", args: []string{"--all=false", "--json"}, wantHubs: []string{"b.invalid"}},
		{name: "explicit hub", args: []string{"--hub", "c.invalid", "--json"}, wantHubs: []string{"c.invalid"}},
		{name: "rejected json", args: []string{"--json"}, wantHubs: []string{"b.invalid"}, rejected: true},
		{name: "no login json", args: []string{"--json"}, wantHubs: []string{"b.invalid"}, empty: true},
		{name: "all without active login", args: []string{"--all", "--json"}, wantHubs: []string{"b.invalid", "a.invalid", "c.invalid"}, empty: true},
		{name: "conflicting selectors", args: []string{"--all", "--hub", "c.invalid"}, wantErr: true},
		{name: "unexpected argument", args: []string{"extra"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := &lib.ConfigSettings{Hub: "b.invalid", HubCreds: map[string]lib.ConfigCreds{
				"a.invalid": {Token: "api_key_a", User: "person@example.com"},
				"b.invalid": {Token: "api_key_b", User: "person@example.com"},
				"c.invalid": {Token: "api_key_c", User: "person@example.com"},
			}}
			config.AdoptHub()
			if test.empty {
				delete(config.HubCreds, "b.invalid")
			}
			flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
			cli.RegisterSwitches(cli.ModeNamed(modeAuth))
			flag.StringVar(&config.Hub, "hub", config.Hub, "")
			if err := flag.CommandLine.Parse(test.args); err != nil {
				t.Fatal(err)
			}

			var contacted []string
			http.DefaultClient = &http.Client{Transport: authStatusTransport(func(req *http.Request) (*http.Response, error) {
				hub := strings.TrimPrefix(req.URL.Host, "api.")
				contacted = append(contacted, hub)
				if req.Method != http.MethodGet || req.URL.Path != "/userinfo" {
					t.Errorf("unexpected service ping: %s %s", req.Method, req.URL)
				}
				if creds := config.SavedCredentials(hub); creds == nil || req.Header.Get("Authorization") != "Bearer "+creds.Token {
					t.Error("service ping did not use the connection's credentials")
				}
				code, body := http.StatusOK, `{"email":"person@example.com"}`
				if test.rejected {
					code, body = http.StatusUnauthorized, `{"err":"invalid token"}`
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}

			var err error
			out := captureStdout(t, func() { err = authStatusCommand(config, flag.Args()) })
			wantContacted := slices.Clone(test.wantHubs)
			if test.empty {
				wantContacted = slices.DeleteFunc(wantContacted, func(hub string) bool { return hub == "b.invalid" })
			}
			if !slices.Equal(contacted, wantContacted) {
				t.Errorf("contacted %v, want %v", contacted, wantContacted)
			}
			if test.wantErr {
				if err == nil || out != "" {
					t.Errorf("invalid arguments: err=%v, stdout=%q", err, out)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "api_key_") {
				t.Error("status exposed a credential")
			}
			if !flagJson {
				var want strings.Builder
				for _, hub := range test.wantHubs {
					want.WriteString(hub)
					if hub == "b.invalid" {
						want.WriteString(" (active)")
					}
					want.WriteString("\n  logged in as person@example.com via personal access token (expiration not recorded)\n")
				}
				if out != want.String() {
					t.Errorf("got %q, want %q", out, want.String())
				}
				return
			}

			var statuses []authStatus
			if flagAuthAll {
				var report authStatusReport
				if err := json.Unmarshal([]byte(out), &report); err != nil {
					t.Fatal(err)
				}
				if report.Active != "b.invalid" {
					t.Errorf("active is %q", report.Active)
				}
				statuses = report.Hubs
			} else {
				var status authStatus
				if err := json.Unmarshal([]byte(out), &status); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out, `"hubs"`) {
					t.Error("single-connection status included an enumeration")
				}
				statuses = []authStatus{status}
			}
			if len(statuses) != len(test.wantHubs) {
				t.Fatalf("got %d statuses, want %d", len(statuses), len(test.wantHubs))
			}
			for i, status := range statuses {
				wantSignedIn := !test.rejected && !(test.empty && status.Hub == "b.invalid")
				if status.Hub != test.wantHubs[i] || status.Active != (status.Hub == "b.invalid") || status.SignedIn != wantSignedIn {
					t.Errorf("unexpected status: %+v", status)
				}
			}
			if flagPretty && !strings.Contains(out, "\n    ") {
				t.Error("--pretty did not indent the JSON")
			}
		})
	}
}
