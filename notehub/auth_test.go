// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
)

// Answers known locally must not touch the hub.  The hub here can't resolve, so any
// request would fail.
const unreachableHub = "whoami-test.invalid"

func TestSignInName(t *testing.T) {
	for _, test := range []struct {
		agentName string
		hostname  string
		want      string
	}{
		{"", "rays-macbook", "Notehub CLI - rays-macbook"},
		{"", "rays-macbook.local", "Notehub CLI - rays-macbook"},
		{"", "rays-macbook.example.com", "Notehub CLI - rays-macbook"},
		{"", "", ""},
		{"My Agent", "rays-macbook.local", "Notehub CLI - My Agent"},
		{"agent.example.com", "rays-macbook.local", "Notehub CLI - agent.example.com"},
		{"My Agent", "", "Notehub CLI - My Agent"},
	} {
		if got := authSignInName(test.agentName, test.hostname); got != test.want {
			t.Errorf("agent=%q hostname=%q: got %q, want %q", test.agentName, test.hostname, got, test.want)
		}
	}
}

// 'notehub login' logs in as its switches say: --with-token with the token after it, as
// --with-token=token, or from stdin when neither; --name for the login's name; --json for
// an agent; switches may come before or after the token.  Refused: a name with a token,
// more than one token, a blank token or name, and any other argument.
func TestLoginChoice(t *testing.T) {
	savedFlags, savedName, savedToken := flag.CommandLine, flagSignInAgent, flagWithToken
	t.Cleanup(func() {
		flag.CommandLine, flagSignInAgent, flagWithToken = savedFlags, savedName, savedToken
	})
	for _, test := range []struct {
		args      []string
		withToken bool
		token     string
		asJSON    bool
		name      string
		refused   bool
	}{
		{[]string{"login"}, false, "", false, "", false},
		{[]string{"login", "--hub", "example.com"}, false, "", false, "", false},
		{[]string{"login", "--name", "My Agent"}, false, "", false, "My Agent", false},
		{[]string{"login", "--json"}, false, "", true, "", false},
		{[]string{"login", "--name", "DotBot", "--json"}, false, "", true, "DotBot", false},
		{[]string{"login", "--json", "--hub", "example.com", "--name", "My Agent"}, false, "", true, "My Agent", false},
		{[]string{"login", "--name=My Agent", "--json"}, false, "", true, "My Agent", false},
		{[]string{"login", "-name", "agent.example.com"}, false, "", false, "agent.example.com", false},
		{[]string{"login", "--with-token", "api_key_x"}, true, "api_key_x", false, "", false},
		{[]string{"login", "--with-token=api_key_x"}, true, "api_key_x", false, "", false},
		{[]string{"login", "--with-token", "api_key_x", "--hub", "example.com"}, true, "api_key_x", false, "", false},
		{[]string{"login", "--hub", "example.com", "--with-token", "api_key_x"}, true, "api_key_x", false, "", false},
		{[]string{"login", "--with-token", "api_key_x", "--json"}, true, "api_key_x", true, "", false},

		// Without a token, --with-token means to read one from stdin, and never takes the
		// switch that follows as the token
		{[]string{"login", "--with-token"}, true, "", false, "", false},
		{[]string{"login", "--with-token", "--json"}, true, "", true, "", false},
		{[]string{"login", "--with-token", "--hub", "example.com"}, true, "", false, "", false},

		{[]string{"login", "--name", ""}, false, "", false, "", true},
		{[]string{"login", "--name", "  "}, false, "", false, "", true},
		{[]string{"login", "--name", "  ", "--json"}, false, "", false, "", true},
		{[]string{"login", "--with-token", "  "}, false, "", false, "", true},
		{[]string{"login", "--with-token", "api_key_x", "api_key_y"}, false, "", false, "", true},
		{[]string{"login", "--with-token=api_key_x", "api_key_y"}, false, "", false, "", true},
		{[]string{"login", "--with-token", "api_key_x", "--name", "My Agent"}, false, "", false, "", true},
		{[]string{"login", "--with-token", "--name", "My Agent"}, false, "", false, "", true},
		{[]string{"login", "My Agent"}, false, "", false, "", true},
		{[]string{"login", "--json", "extra"}, false, "", false, "", true},
		{[]string{"login", "--name", "My", "Agent"}, false, "", false, "", true},
		{[]string{"login", "--name"}, false, "", false, "", true},
		{[]string{"login", "--json", "--expires-in", "30"}, false, "", true, "", false},
		{[]string{"login", "--json", "--expires-in", "0"}, false, "", false, "", true},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			mode, args := cli.ExtractMode(test.args)
			if mode.Name != modeLogin {
				t.Fatalf("mode is %s", mode.Name)
			}
			flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			flagSignInAgent, flagWithToken, flagJson = "", cli.OptionalValue{}, false
			cli.RegisterSwitches(mode)
			flag.String("hub", "", "")

			// As runLogin does: the flag package first, then the switches after any argument
			var withToken, asJSON bool
			var token string
			err := flag.CommandLine.Parse(args)
			if err == nil {
				var positional []string
				if positional, err = cli.ParseCommandSwitches(mode, flag.Args()); err == nil {
					withToken, token, asJSON, err = authLoginChoice(positional)
				}
			}
			if test.refused {
				if err == nil {
					t.Fatalf("accepted, as withToken=%v token=%q asJSON=%v name=%q", withToken, token, asJSON, flagSignInAgent)
				}
				return
			}
			if err != nil || withToken != test.withToken || token != test.token || asJSON != test.asJSON || flagSignInAgent != test.name {
				t.Fatalf("got withToken=%v token=%q asJSON=%v name=%q err=%v, want withToken=%v token=%q asJSON=%v name=%q",
					withToken, token, asJSON, flagSignInAgent, err, test.withToken, test.token, test.asJSON, test.name)
			}
		})
	}
}

// A token on stdin is taken whole, without the whitespace around it, and nothing is no token
func TestReadToken(t *testing.T) {
	for input, want := range map[string]string{
		"api_key_x":         "api_key_x",
		"  api_key_x \n":    "api_key_x",
		"api_key_x\r\n":     "api_key_x",
		"\n\tapi_key_x\n\n": "api_key_x",
		"":                  "",
		"\n":                "",
		"   ":               "",
	} {
		token, err := authReadToken(strings.NewReader(input), io.Discard)
		if want == "" {
			if err == nil {
				t.Errorf("%q: read %q, expected a refusal", input, token)
			}
			continue
		}
		if err != nil || token != want {
			t.Errorf("%q: read %q (%v), expected %q", input, token, err, want)
		}
	}
}

func TestWhoAmINotSignedIn(t *testing.T) {
	status := authWhoAmI(unreachableHub, nil, time.Now())
	if status.SignedIn {
		t.Fatalf("reported signed in with no credentials")
	}
	if status.Hub != unreachableHub {
		t.Errorf("hub is %q, expected %q", status.Hub, unreachableHub)
	}
	if status.Reason != "not logged in" {
		t.Errorf("reason is %q, expected 'not logged in'", status.Reason)
	}
	if status.Action != authSignInAction {
		t.Errorf("action is %q, expected %q", status.Action, authSignInAction)
	}
}

// A recorded expiration that has passed is reported but not trusted: the hub is still
// asked, so here the answer is that the hub can't be reached, and not that we are signed
// out.  A token whose expiration lies ahead is asked about without remark.
func TestWhoAmIExpired(t *testing.T) {
	now := time.Now()
	for _, expiresAt := range []time.Time{now.Add(-time.Hour), now} {
		creds := &lib.ConfigCreds{User: "someone@example.com", Token: "ory_at_expired", ExpiresAt: &expiresAt}
		status := authWhoAmI(unreachableHub, creds, now)
		if status.SignedIn || status.Action != "" {
			t.Errorf("token that expired at %s gave %+v, expected neither signed in nor an action", expiresAt, status)
		}
		if !strings.HasPrefix(status.Reason, "unable to reach hub") || !strings.Contains(status.Reason, "expired at") {
			t.Errorf("token that expired at %s gave reason %q, expected the hub to have been asked, with a note about the expiration", expiresAt, status.Reason)
		}
		if !status.expired || status.ExpiresAt == nil || status.Method != authMethodOAuth {
			t.Errorf("token that expired at %s gave %+v, expected its method and expiration recorded", expiresAt, status)
		}
	}
	later := now.Add(time.Hour)
	status := authWhoAmI(unreachableHub, &lib.ConfigCreds{User: "someone@example.com", Token: "ory_at_fine", ExpiresAt: &later}, now)
	if status.expired || strings.Contains(status.Reason, "expiration") {
		t.Errorf("token expiring at %s gave %+v, expected no remark about its expiration", later, status)
	}
}

// When the hub accepts a token whose recorded expiration has passed, the answer says so
func TestWhoAmIExpiredDescription(t *testing.T) {
	expiresAt := time.Date(2026, 9, 27, 0, 57, 34, 0, time.Local)
	status := authStatus{SignedIn: true, Method: authMethodOAuth, ExpiresAt: &expiresAt}
	if got := authExpiresDescription(status); !strings.HasPrefix(got, "OAuth token (expires at 2026-09-27 00:57:34") {
		t.Errorf("description is %q, expected it to say when the token expires", got)
	}
	status.expired = true
	if got := authExpiresDescription(status); !strings.Contains(got, "still accepts") || !strings.Contains(got, "expired at") {
		t.Errorf("description is %q, expected it to say the hub still accepts a token that expired", got)
	}
	status.ExpiresAt = nil
	if got := authExpiresDescription(status); got != "OAuth token (expiration not recorded)" {
		t.Errorf("description is %q, expected 'OAuth token (expiration not recorded)'", got)
	}
}

// A hub we can't reach is not a reason to sign in again, so no action is suggested
func TestWhoAmIUnreachable(t *testing.T) {
	creds := &lib.ConfigCreds{User: "someone@example.com", Token: "api_key_whatever"}
	status := authWhoAmI(unreachableHub, creds, time.Now())
	if status.SignedIn {
		t.Fatalf("reported signed in to a hub that does not exist")
	}
	if status.Method != authMethodPAT {
		t.Errorf("method is %q, expected %q", status.Method, authMethodPAT)
	}
	if status.ExpiresAt != nil {
		t.Errorf("expires_at is %s, expected none for a personal access token", status.ExpiresAt)
	}
	if !strings.HasPrefix(status.Reason, "unable to reach hub") {
		t.Errorf("reason is %q, expected it to begin 'unable to reach hub'", status.Reason)
	}
	if status.Action != "" {
		t.Errorf("action is %q, expected none", status.Action)
	}
}

// The JSON form is what agents parse, so its shape is a contract
func TestWhoAmIJSON(t *testing.T) {
	statusJSON, err := note.JSONMarshal(authWhoAmI(unreachableHub, nil, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"hub":"whoami-test.invalid","active":false,"signed_in":false,"reason":"not logged in","action":"notehub login"}`
	if string(statusJSON) != expected {
		t.Errorf("got  %s\nwant %s", statusJSON, expected)
	}
}

func TestAgentSignInJSON(t *testing.T) {
	for _, result := range []struct {
		name    string
		err     error
		success bool
		status  string
	}{
		{"success", nil, true, "signed in successfully"},
		{"denial", errors.New("Authorization was not granted: The user decided not to sign in."), false, "authorization was not granted: the user decided not to sign in."},
		{"cancelled", context.Canceled, false, "sign-in was cancelled"},
		{"expired", context.DeadlineExceeded, false, "sign-in expired; please try again"},
		{"env token", errors.New("NOTEHUB_TOKEN is set: Clear it first"), false, "NOTEHUB_TOKEN is set: clear it first"},
	} {
		t.Run(result.name, func(t *testing.T) {
			var stdout strings.Builder
			reporter := newAgentSignInReporter(&stdout)
			const url = "https://notehub.example/oauth2/auth?client_id=notehub_cli&state=test"
			if err := reporter.url(url, 600); err != nil {
				t.Fatal(err)
			}
			if err := reporter.pending(590); err != nil {
				t.Fatal(err)
			}
			if err := reporter.finish(result.err); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) != 3 {
				t.Fatalf("expected three JSON lines, got %q", stdout.String())
			}
			if !strings.Contains(lines[0], url) {
				t.Fatalf("URL was escaped in JSON: %s", lines[0])
			}
			var messages []map[string]interface{}
			for _, line := range lines {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(line), &m); err != nil {
					t.Fatal(err)
				}
				messages = append(messages, m)
			}
			if len(messages[0]) != 3 || messages[0]["status"] != "please instruct the user to open their browser to this URL to complete sign in" || messages[0]["url"] != url || messages[0]["expires_in_seconds"] != float64(600) {
				t.Fatalf("unexpected first object: %v", messages[0])
			}
			if len(messages[1]) != 2 || messages[1]["status"] != "waiting for authorization" || messages[1]["expires_in_seconds"] != float64(590) {
				t.Fatalf("unexpected pending object: %v", messages[1])
			}
			if len(messages[2]) != 2 || messages[2]["success"] != result.success || messages[2]["status"] != result.status {
				t.Fatalf("unexpected final object: %v", messages[2])
			}
		})
	}
}

// A token from the environment is reported with its source, and a problem with it is the
// caller's to fix: signing in wouldn't displace it, so no sign-in action is suggested
func TestWhoAmIEnvToken(t *testing.T) {
	creds := &lib.ConfigCreds{Token: "api_key_env", Hub: unreachableHub, Source: lib.ConfigTokenEnvVar}
	status := authWhoAmI(unreachableHub, creds, time.Now())
	if status.SignedIn || status.Source != lib.ConfigTokenEnvVar || status.Method != authMethodPAT || status.Action != "" {
		t.Errorf("got %+v, expected the source recorded, as a personal access token, with no action", status)
	}
	statusJSON, err := note.JSONMarshal(authStatus{Hub: unreachableHub, SignedIn: true, User: "someone@example.com", Method: authMethodPAT, Source: lib.ConfigTokenEnvVar})
	if err != nil || !strings.Contains(string(statusJSON), `"source":"NOTEHUB_TOKEN"`) {
		t.Errorf("got %s (%v), expected the source in the JSON", statusJSON, err)
	}
	description := authExpiresDescription(authStatus{Method: authMethodPAT, Source: lib.ConfigTokenEnvVar})
	if description != "personal access token from NOTEHUB_TOKEN (expiration not recorded)" {
		t.Errorf("description is %q, expected it to name the variable", description)
	}
}

// While NOTEHUB_TOKEN is set, signing in or out is refused, since the token there is the
// one in use
// A login that is still good, or the token in the environment, means there is nothing to
// do: the decision is made locally, and told to the agent as the final line alone
func TestAlreadySignedIn(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	later, earlier := now.Add(90*time.Second), now.Add(-time.Second)
	config := &lib.ConfigSettings{Hub: "example.com", HubCreds: map[string]lib.ConfigCreds{}}
	t.Setenv(lib.ConfigTokenEnvVar, "")

	if creds := authCurrentLogin(config, now); creds != nil {
		t.Errorf("with no login saved, got %+v, expected nothing in place", creds)
	}
	config.HubCreds["example.com"] = lib.ConfigCreds{User: "ray@example.com", Token: "oauth_token", ExpiresAt: &earlier}
	if creds := authCurrentLogin(config, now); creds != nil {
		t.Errorf("with an expired login, got %+v, expected nothing in place", creds)
	}
	config.HubCreds["example.com"] = lib.ConfigCreds{User: "ray@example.com", Token: "oauth_token", ExpiresAt: &later}
	creds := authCurrentLogin(config, now)
	if creds == nil || creds.User != "ray@example.com" {
		t.Fatalf("with a good login, got %+v, expected it", creds)
	}
	if got := authAlreadyDescription(creds, "logged in"); got != "already logged in to example.com as ray@example.com" {
		t.Errorf("described as %q", got)
	}
	config.HubCreds["example.com"] = lib.ConfigCreds{User: "ray@example.com", Token: "api_key_saved"}
	if creds = authCurrentLogin(config, now); creds == nil {
		t.Errorf("a personal access token is in place for as long as the hub says: got %+v", creds)
	}

	t.Setenv(lib.ConfigTokenEnvVar, "api_key_env")
	creds = authCurrentLogin(config, now)
	if creds == nil || creds.Source != lib.ConfigTokenEnvVar {
		t.Fatalf("with %s set, got %+v, expected its token", lib.ConfigTokenEnvVar, creds)
	}
	if got := authAlreadyDescription(creds, "signed in"); got != "already signed in to example.com with the token in NOTEHUB_TOKEN" {
		t.Errorf("described as %q", got)
	}

	var stdout strings.Builder
	reporter := newAgentSignInReporter(&stdout)
	reporter.alreadySignedIn("already signed in to example.com as ray@example.com")
	if err := reporter.finish(nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != `{"status":"already signed in to example.com as ray@example.com","success":true}` {
		t.Errorf("the agent was told: %s", got)
	}
}

func TestEnvTokenRefusesSignIn(t *testing.T) {
	config := &lib.ConfigSettings{Hub: unreachableHub}
	t.Setenv(lib.ConfigTokenEnvVar, "")
	if err := authEnvTokenInUse(config, "logging in would have no effect"); err != nil {
		t.Errorf("without %s: %v, expected no refusal", lib.ConfigTokenEnvVar, err)
	}
	t.Setenv(lib.ConfigTokenEnvVar, "api_key_env")
	err := authEnvTokenInUse(config, "logging in would have no effect")
	if err == nil || !strings.Contains(err.Error(), lib.ConfigTokenEnvVar) || !strings.Contains(err.Error(), "logging in would have no effect") {
		t.Errorf("with %s: %v, expected a refusal naming it and the consequence", lib.ConfigTokenEnvVar, err)
	}
}

// 'auth status' reports the active hub first, even without a login, then every other hub
// with one, says which is active, and with --hub asks about that hub alone
func TestAuthStatusReport(t *testing.T) {
	t.Setenv(lib.ConfigTokenEnvVar, "")
	config := &lib.ConfigSettings{Hub: "b.invalid", HubCreds: map[string]lib.ConfigCreds{
		"c.invalid":     {User: "c@example.com", Token: "api_key_c"},
		"a.invalid":     {User: "a@example.com", Token: "api_key_a"},
		"b.invalid":     {User: "b@example.com", Token: "ory_at_b"},
		"stale.invalid": {User: "", Token: ""},
	}}
	hubs := authStatusHubs(config, "b.invalid", false)
	if !slices.Equal(hubs, []string{"b.invalid", "a.invalid", "c.invalid"}) {
		t.Errorf("hubs are %v, expected the active one first and the others sorted", hubs)
	}
	if only := authStatusHubs(config, "b.invalid", true); !slices.Equal(only, []string{"b.invalid"}) {
		t.Errorf("with --hub, hubs are %v, expected just the hub in use", only)
	}
	if none := authStatusHubs(config, "none.invalid", false); !slices.Equal(none, []string{"none.invalid", "a.invalid", "b.invalid", "c.invalid"}) {
		t.Errorf("with an active hub that has no login, hubs are %v, expected it listed first", none)
	}

	report := authStatusAll(config, "b.invalid", hubs, time.Now())
	if report.Active != "b.invalid" || len(report.Hubs) != 3 || !report.Hubs[0].Active || report.Hubs[1].Active || report.Hubs[2].Active {
		t.Errorf("report is %+v, expected b.invalid active and the others not", report)
	}
	for _, status := range report.Hubs {
		if status.SignedIn || !strings.HasPrefix(status.Reason, "unable to reach hub") {
			t.Errorf("%s: %+v, expected the hub to have been asked and not reached", status.Hub, status)
		}
	}
	reportJSON, err := note.JSONMarshal(report)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"active":"b.invalid","hubs":[{"hub":"b.invalid","active":true,"signed_in":false,"method":"oauth","reason":"unable to reach hub`
	if !strings.HasPrefix(string(reportJSON), expected) {
		t.Errorf("got  %s\nwant a prefix of %s", reportJSON, expected)
	}

	// The token in the environment is what the hub in use would get, and no other
	t.Setenv(lib.ConfigTokenEnvVar, "api_key_env")
	if creds := authCredentialsFor(config, "b.invalid"); creds == nil || creds.Source != lib.ConfigTokenEnvVar {
		t.Errorf("hub in use: %+v, expected the environment's token", creds)
	}
	if creds := authCredentialsFor(config, "a.invalid"); creds == nil || creds.Source != "" || creds.Token != "api_key_a" {
		t.Errorf("another hub: %+v, expected its saved login", creds)
	}
}
