// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

// Answers known locally must not touch the hub.  The hub here can't resolve, so any
// request would fail.
const unreachableHub = "whoami-test.invalid"

func TestWhoAmINotSignedIn(t *testing.T) {
	status := authWhoAmI(unreachableHub, nil, time.Now())
	if status.SignedIn {
		t.Fatalf("reported signed in with no credentials")
	}
	if status.Hub != unreachableHub {
		t.Errorf("hub is %q, expected %q", status.Hub, unreachableHub)
	}
	if status.Reason != "not signed in" {
		t.Errorf("reason is %q, expected 'not signed in'", status.Reason)
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
		if !strings.HasPrefix(status.Reason, "unable to reach hub") || !strings.Contains(status.Reason, "recorded expiration has passed") {
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
	if got := authExpiresDescription(status); !strings.HasPrefix(got, "OAuth token expiring 2026-09-27 00:57:34") {
		t.Errorf("description is %q, expected it to say when the token expires", got)
	}
	status.expired = true
	if got := authExpiresDescription(status); !strings.Contains(got, "still accepts") || !strings.Contains(got, "has passed") {
		t.Errorf("description is %q, expected it to say the hub still accepts a token whose expiration has passed", got)
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
	expected := `{"hub":"whoami-test.invalid","signed_in":false,"reason":"not signed in","action":"notehub --signin"}`
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
	} {
		t.Run(result.name, func(t *testing.T) {
			var stdout strings.Builder
			reporter := newAgentSignInReporter(&stdout)
			const url = "https://notehub.example/oauth2/auth?client_id=notehub_cli&state=test"
			if err := reporter.url(url); err != nil {
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
			if len(messages[0]) != 2 || messages[0]["status"] != "URL to be sent to the user so they can open a browser to sign in" || messages[0]["url"] != url {
				t.Fatalf("unexpected first object: %v", messages[0])
			}
			if len(messages[1]) != 2 || messages[1]["status"] != "waiting for authorization" || messages[1]["remaining_seconds"] != float64(590) {
				t.Fatalf("unexpected pending object: %v", messages[1])
			}
			if len(messages[2]) != 2 || messages[2]["success"] != result.success || messages[2]["status"] != result.status {
				t.Fatalf("unexpected final object: %v", messages[2])
			}
		})
	}
}
