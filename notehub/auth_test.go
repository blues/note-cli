// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

// The whole point of -whoami is that an agent can run it cheaply before every session,
// so the cases whose answer is known locally must be answered without touching the hub.
// These tests use a hub that cannot resolve, so any request would fail loudly.
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

// An expired token is indistinguishable from having none, because that is what it
// amounts to: no method or expiration is described, just the fact of not being signed in
func TestWhoAmIExpired(t *testing.T) {
	now := time.Now()
	notSignedIn := authWhoAmI(unreachableHub, nil, now)
	for _, expiresAt := range []time.Time{now.Add(-time.Hour), now} {
		creds := &lib.ConfigCreds{User: "someone@example.com", Token: "ory_at_expired", ExpiresAt: &expiresAt}
		status := authWhoAmI(unreachableHub, creds, now)
		if status != notSignedIn {
			t.Errorf("token that expired at %s gave %+v, expected the same as no credentials: %+v", expiresAt, status, notSignedIn)
		}
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

// The JSON form is what an agent is most likely to parse, so its shape is a contract
func TestWhoAmIJSON(t *testing.T) {
	statusJSON, err := note.JSONMarshal(authWhoAmI(unreachableHub, nil, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"hub":"whoami-test.invalid","signed_in":false,"reason":"not signed in","action":"notehub -signin"}`
	if string(statusJSON) != expected {
		t.Errorf("got  %s\nwant %s", statusJSON, expected)
	}
}
