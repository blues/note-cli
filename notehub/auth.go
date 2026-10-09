// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/notehub"
)

// Set true to test the Notehub polling callback; false uses localhost login.  Note
// that polling is superior in that agents can use it without opening a local HTTP server.
const authUsePolling = true

// Sign into the notehub account with a personal access token
func authSignInToken(personalAccessToken string) error {
	// TODO: maybe call configInit() to set defaults?
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	if err := authEnvTokenInUse(config, "logging in would have no effect"); err != nil {
		return err
	}

	// Print hub if not the default
	fmt.Fprintf(os.Stderr, "notehub: %s\n", config.Hub)

	email, err := lib.IntrospectToken(config.Hub, personalAccessToken)
	if err != nil {
		return err
	}

	config.SetDefaultCredentials(personalAccessToken, email, nil)

	if err := config.Write(); err != nil {
		return err
	}

	// Done
	fmt.Fprintf(os.Stderr, "logged in to %s as %s with a personal access token\n", config.Hub, email)
	return nil
}

// authStatus is the answer to "am I signed in to the configured hub?"
type authStatus struct {
	Hub       string     `json:"hub"`
	Active    bool       `json:"active"`
	SignedIn  bool       `json:"signed_in"`
	User      string     `json:"user,omitempty"`
	Method    string     `json:"method,omitempty"`
	Source    string     `json:"source,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Action    string     `json:"action,omitempty"`

	// expired notes that the recorded expiration has passed, for the wording of the
	// answer; the hub is the judge of whether the token still works
	expired bool
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

// authExpiresFormat is how an expiration is displayed, as in the config display
const authExpiresFormat = lib.ConfigTimeFormat

// authSignInAction is the remedy suggested when sign-in is needed.  It opens a browser,
// so an agent should hand it to the person rather than run it.
const authSignInAction = "notehub login"

// authWhoAmI reports whether we are signed in to the hub, and as whom.  Whenever there
// are saved credentials it asks the hub, with one small request, because the hub is the
// judge of them: a recorded expiration that has passed is reported, but not trusted, since
// the hub may well still accept the token.
func authWhoAmI(hub string, credentials *lib.ConfigCreds, now time.Time) (status authStatus) {
	status.Hub = hub

	// With no credentials, we are simply not signed in
	if credentials == nil {
		status.Reason = "not logged in"
		status.Action = authSignInAction
		return
	}

	// How we are signed in.  Only an OAuth token's expiration is known to us.
	status.Method = authMethodOAuth
	if !credentials.IsOAuthAccessToken() {
		status.Method = authMethodPAT
	}
	status.Source = credentials.Source
	if credentials.ExpiresAt != nil {
		expiresAt := credentials.ExpiresAt.Local()
		status.ExpiresAt = &expiresAt
	}
	status.expired = credentials.ExpiredAt(now)

	// Let the hub be the judge
	email, err := lib.IntrospectToken(hub, credentials.Token)
	if err != nil {
		// Failing to reach the hub says nothing about the token, so don't suggest signing in
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			status.Reason = fmt.Sprintf("unable to reach hub: %s", urlErr.Err)
			if status.expired {
				status.Reason += fmt.Sprintf(" (the saved token expired at %s)", status.ExpiresAt.Format(authExpiresFormat))
			}
			return
		}
		status.Reason = fmt.Sprintf("%s rejected: %s", authMethodDescription(status), err)
		if status.expired {
			status.Reason = fmt.Sprintf("%s expired at %s, and the hub rejected it: %s", authMethodDescription(status),
				status.ExpiresAt.Format(authExpiresFormat), err)
		}
		// Signing in wouldn't help while the environment's token is the one in use
		if status.Source == "" {
			status.Action = authSignInAction
		}
		return
	}

	status.SignedIn = true
	status.User = email
	return
}

// authExpiresDescription describes how we are signed in and when that expires
func authExpiresDescription(status authStatus) string {
	switch {
	case status.ExpiresAt == nil:
		return fmt.Sprintf("%s (expiration not recorded)", authMethodDescription(status))
	case status.expired:
		return fmt.Sprintf("%s the hub still accepts, though it expired at %s",
			authMethodDescription(status), status.ExpiresAt.Format(authExpiresFormat))
	}
	return fmt.Sprintf("%s (expires at %s)", authMethodDescription(status), status.ExpiresAt.Format(authExpiresFormat))
}

// authMethodDescription describes how we are signed in: the kind of token, and where it
// came from when that is the environment rather than a saved sign-in
func authMethodDescription(status authStatus) string {
	description := authMethodName(status.Method)
	if status.Source != "" {
		description += " from " + status.Source
	}
	return description
}

// authEnvTokenInUse refuses to change the saved sign-in while NOTEHUB_TOKEN is set.  Its
// token is used for every request, so a sign-in would take no effect, and a sign-out would
// leave it in use.  The consequence is worded by the caller.
func authEnvTokenInUse(config *lib.ConfigSettings, consequence string) error {
	if config.EnvCredentials() == nil {
		return nil
	}
	return fmt.Errorf("%s is set, and its token is used for every request, so %s - clear it from the environment first",
		lib.ConfigTokenEnvVar, consequence)
}

// authCurrentLogin returns the credentials in use for the configured hub when they are
// good as far as can be told locally: the token in NOTEHUB_TOKEN, or a saved login that
// hasn't expired.  It returns nil when signing in would change something.
func authCurrentLogin(config *lib.ConfigSettings, now time.Time) *lib.ConfigCreds {
	credentials := config.DefaultCredentials()
	if credentials == nil || credentials.ExpiredAt(now) {
		return nil
	}
	return credentials
}

// authAlreadyDescription says that the credentials are already in place, with the verb
// the reader expects: "logged in" on the command line, "signed in" for an agent
func authAlreadyDescription(credentials *lib.ConfigCreds, verb string) string {
	if credentials.Source != "" {
		return fmt.Sprintf("already %s to %s with the token in %s", verb, credentials.Hub, credentials.Source)
	}
	return fmt.Sprintf("already %s to %s as %s", verb, credentials.Hub, credentials.User)
}

// authUnauthorizedError is the error for a request the hub refused as unauthorized.  The
// remedy depends on where the token came from: one from the environment is the caller's
// to replace, since signing in wouldn't displace it.
func authUnauthorizedError(hub string) error {
	if credentials := lib.ConfigSignedIn(); credentials != nil && credentials.Source != "" {
		return fmt.Errorf("%s rejected the token in %s", hub, credentials.Source)
	}
	return fmt.Errorf("%s rejected the login - run '%s login' to log in again", hub, cliName)
}

// Sign into the Notehub account with browser-based OAuth2 flow
func authSignIn() error { return authSignInWithAgent("", nil) }

// authSignInWithAgent signs in under the agent's name, or the computer's, reporting to the
// agent as it goes when there is a reporter
func authSignInWithAgent(agentName string, reporter *agentSignInReporter) (err error) {

	// load config
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	// Someone who is signed in already has nothing to do.  Agents sign in to make sure
	// that they are, so this is decided from the saved login, without asking the hub: a
	// login the hub has revoked is found out by the first request, as it would be anyway.
	now := time.Now()
	if credentials := authCurrentLogin(config, now); credentials != nil {
		if reporter != nil {
			reporter.alreadySignedIn(authAlreadyDescription(credentials, "signed in"))
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", authAlreadyDescription(credentials, "logged in"))
			if credentials.ExpiresAt != nil {
				fmt.Fprintf(os.Stderr, "token expires at %s\n", credentials.ExpiresAt.Format(authExpiresFormat))
			}
		}
		return nil
	}

	// The login this one replaces, if any, is ended only once this one has succeeded
	credentials := config.DefaultCredentials()

	hostname, _ := os.Hostname()
	signInName := authSignInName(agentName, hostname)

	// initiate the browser-based OAuth2 login flow
	var accessToken *notehub.AccessToken
	if authUsePolling {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		options := notehub.PollingLoginOptions{SignInName: signInName, ExpiresInSeconds: flagLoginExpiresIn}
		if reporter != nil {
			options.OnURL, options.OnPending = reporter.url, reporter.pending
		}
		accessToken, err = notehub.InitiatePollingLoginWithOptions(ctx, config.Hub, options)
	} else {
		accessToken, err = notehub.InitiateBrowserBasedLoginWithOptions(config.Hub, notehub.BrowserLoginOptions{SignInName: signInName})
	}
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// A browser login the hub still accepts is ended now, so that a login that fails or is
	// cancelled leaves the one before it in place.  One that has expired has lapsed at the
	// hub already.  A personal access token is left alone: its expiration is the user's to
	// manage at Notehub.
	if credentials != nil && credentials.IsOAuthAccessToken() && !credentials.ExpiredAt(time.Now()) {
		if err := config.RemoveDefaultCredentials(); err != nil {
			return err
		}
	}

	config.SetDefaultCredentials(accessToken.AccessToken, accessToken.Email, &accessToken.ExpiresAt)

	// save the config with the new credentials
	if err := config.Write(); err != nil {
		return err
	}

	// print out information about the session
	if accessToken != nil && reporter == nil {
		fmt.Fprintf(os.Stderr, "%s\n", banner())
		fmt.Fprintf(os.Stderr, "logged in to %s as %s\n", config.Hub, accessToken.Email)
		fmt.Fprintf(os.Stderr, "token expires at %s\n", accessToken.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
	}

	// Done
	return nil
}

// Agent sign-in is a stream of one JSON object per line: the URL for the user, with how
// long it is good for, progress with how long remains, and the result.  Only terminal
// objects carry success; a false value must still appear in the JSON.
type agentSignInMessage struct {
	Status           string `json:"status"`
	URL              string `json:"url,omitempty"`
	Success          *bool  `json:"success,omitempty"`
	ExpiresInSeconds *int   `json:"expires_in_seconds,omitempty"`
}

type agentSignInReporter struct {
	encoder *json.Encoder
	// What the final message says when the sign in ended early with nothing to do
	already string
}

// alreadySignedIn records that the sign in found a login in place, for the final message
func (r *agentSignInReporter) alreadySignedIn(status string) {
	r.already = status
}

func newAgentSignInReporter(w io.Writer) *agentSignInReporter {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return &agentSignInReporter{encoder: encoder}
}

// url reports the URL for the user, and the seconds the hub granted it
func (r *agentSignInReporter) url(url string, expiresInSeconds int) error {
	message := agentSignInMessage{Status: "please instruct the user to open their browser to this URL to complete sign in", URL: url}
	if expiresInSeconds > 0 {
		message.ExpiresInSeconds = &expiresInSeconds
	}
	return r.encoder.Encode(message)
}
func (r *agentSignInReporter) pending(remainingSeconds int) error {
	return r.encoder.Encode(agentSignInMessage{Status: "waiting for authorization", ExpiresInSeconds: &remainingSeconds})
}
func (r *agentSignInReporter) finish(err error) error {
	success := err == nil
	message := agentSignInMessage{Status: "signed in successfully", Success: &success}
	switch {
	case err == nil && r.already != "":
		message.Status = r.already
	case errors.Is(err, context.Canceled):
		message.Status = "sign-in was cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		message.Status = "sign-in expired; please try again"
	case err != nil:
		message.Status = authLowerStatus(err.Error())
	}
	return r.encoder.Encode(message)
}

// Use the same reporting for failures before the login routine can start.
func authAgentFailure(w io.Writer, err error) {
	reporter := newAgentSignInReporter(w)
	_ = reporter.finish(err)
}

// authLowerStatus makes an error read as a status, lowercasing the word that begins it and
// each sentence within it, as "Authorization was not granted: The user decided not to sign
// in." is reported.  A name such as NOTEHUB_TOKEN keeps its case.
func authLowerStatus(text string) string {
	runes := []rune(text)
	sentenceStart := true
	for i, r := range runes {
		if sentenceStart && unicode.IsUpper(r) && (i+1 == len(runes) || !unicode.IsUpper(runes[i+1])) {
			runes[i] = unicode.ToLower(r)
		}
		switch {
		case unicode.IsSpace(r):
			// A sentence starts after the spaces that follow its predecessor
		case r == ':' || r == '.' || r == ';':
			sentenceStart = true
		default:
			sentenceStart = false
		}
	}
	return string(runes)
}

// authSignInName seeds the editable name with the agent name or short hostname.
func authSignInName(agentName, hostname string) string {
	name := agentName
	if name == "" {
		name, _, _ = strings.Cut(hostname, ".")
	}
	if name == "" {
		return ""
	}
	return "Notehub CLI - " + name
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
