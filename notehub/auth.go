// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// Sign into the notehub account with a personal access token
func authSignInToken(personalAccessToken string) error {
	// TODO: maybe call configInit() to set defaults?
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	// Print hub if not the default
	fmt.Printf("notehub: %s\n", config.Hub)

	email, err := lib.IntrospectToken(config.Hub, personalAccessToken)
	if err != nil {
		return err
	}

	config.SetDefaultCredentials(personalAccessToken, email, nil)

	if err := config.Write(); err != nil {
		return err
	}

	// Done
	fmt.Printf("signed in successfully with token\n")
	return nil
}

// authStatus is the answer to "am I signed in to the configured hub?"
type authStatus struct {
	Hub       string     `json:"hub"`
	SignedIn  bool       `json:"signed_in"`
	User      string     `json:"user,omitempty"`
	Method    string     `json:"method,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Action    string     `json:"action,omitempty"`
}

// The ways in which we can be signed in, as they appear in the JSON form of authStatus
const (
	authMethodOAuth = "oauth"
	authMethodPAT   = "pat"
)

// authMethodName describes an auth method in words
func authMethodName(method string) string {
	if method == authMethodPAT {
		return "personal access token"
	}
	return "OAuth token"
}

// authExpiresFormat is how we display an expiration, and is the same format that the
// config display uses so that the two agree
const authExpiresFormat = "2006-01-02 15:04:05 MST"

// The remedy that we suggest when a sign-in is required.  Sign-in opens a browser, so
// an agent that reads this must hand it to the person rather than run it itself.
const authSignInAction = "notehub --signin"

// authWhoAmI determines whether we are signed in to the configured hub, and as whom.
// This is intended to be cheap enough that an agent can run it at the start of every
// session, so it makes no request at all when the answer is already known locally
// (no credentials, or credentials that have expired), and otherwise makes exactly one
// small request to the hub.
func authWhoAmI(hub string, credentials *lib.ConfigCreds, now time.Time) (status authStatus) {
	status.Hub = hub

	// Without stored credentials there is nothing to ask the hub about, and expired
	// credentials are no different because the hub would reject them.  In neither case
	// is there a method or an expiration worth describing: we are simply not signed in.
	if credentials == nil || credentials.ExpiredAt(now) {
		status.Reason = "not signed in"
		status.Action = authSignInAction
		return
	}

	// Describe how we are signed in.  A personal access token has an expiration that
	// the person chose when they created it in Notehub, but it is not told to us, so
	// only an OAuth token has an expiration that we can report.
	status.Method = authMethodOAuth
	if !credentials.IsOAuthAccessToken() {
		status.Method = authMethodPAT
	}
	if credentials.ExpiresAt != nil {
		expiresAt := credentials.ExpiresAt.Local()
		status.ExpiresAt = &expiresAt
	}

	// The token looks usable, so let the hub be the judge
	email, err := lib.IntrospectToken(hub, credentials.Token)
	if err != nil {
		// A transport failure says nothing about whether we are signed in, and the
		// remedy is not to sign in again, so say what actually happened
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			status.Reason = fmt.Sprintf("unable to reach hub: %s", urlErr.Err)
			return
		}
		status.Reason = fmt.Sprintf("%s rejected: %s", authMethodName(status.Method), err)
		status.Action = authSignInAction
		return
	}

	status.SignedIn = true
	status.User = email
	return
}

// authWhoAmIPrint prints an authStatus as a single line, in JSON if requested
func authWhoAmIPrint(status authStatus, asJSON bool) {
	if asJSON {
		statusJSON, err := note.JSONMarshal(status)
		if err != nil {
			fmt.Printf("%s\n", err)
			return
		}
		fmt.Printf("%s\n", statusJSON)
		return
	}
	if status.SignedIn {
		fmt.Printf("%s: signed in as %s via %s\n", status.Hub, status.User, authExpiresDescription(status))
		return
	}
	if status.Action != "" {
		fmt.Printf("%s: %s - run '%s'\n", status.Hub, status.Reason, status.Action)
		return
	}
	fmt.Printf("%s: %s\n", status.Hub, status.Reason)
}

// authExpiresDescription describes the method by which we are signed in and when it
// will expire, in words
func authExpiresDescription(status authStatus) string {
	if status.ExpiresAt == nil {
		return fmt.Sprintf("%s (expiration not recorded)", authMethodName(status.Method))
	}
	return fmt.Sprintf("%s expiring %s", authMethodName(status.Method), status.ExpiresAt.Format(authExpiresFormat))
}

// Sign into the Notehub account with browser-based OAuth2 flow
func authSignIn() error {

	// load config
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	credentials := config.DefaultCredentials()

	// if signed in with an access token via OAuth, then revoke the access token
	// we don't want to revoke a PAT because the user explicitly set an
	// expiration date on that token
	if credentials != nil && credentials.IsOAuthAccessToken() {
		if err := config.RemoveDefaultCredentials(); err != nil {
			return err
		}
	}

	// initiate the browser-based OAuth2 login flow
	accessToken, err := notehub.InitiateBrowserBasedLogin(config.Hub)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	config.SetDefaultCredentials(accessToken.AccessToken, accessToken.Email, &accessToken.ExpiresAt)

	// save the config with the new credentials
	if err := config.Write(); err != nil {
		return err
	}

	// print out information about the session
	if accessToken != nil {
		fmt.Printf("%s\n", banner())
		fmt.Printf("signed in as %s\n", accessToken.Email)
		fmt.Printf("token expires at %s\n", accessToken.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
	}

	// Done
	return nil
}

// Banner for authentication
// http://patorjk.com/software/taag
// "Big" font

func banner() (s string) {
	s += "             _       _           _       \r\n"
	s += "            | |     | |         | |      \r\n"
	s += " _ __   ___ | |_ ___| |__  _   _| |__    \r\n"
	s += "| '_ \\ / _ \\| __/ _ \\ '_ \\| | | | '_ \\   \r\n"
	s += "| | | | (_) | ||  __/ | | | |_| | |_) |  \r\n"
	s += "|_| |_|\\___/ \\__\\___|_| |_|\\__,_|_.__/   \r\n"
	s += "\r\n"
	return
}
