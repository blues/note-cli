// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
