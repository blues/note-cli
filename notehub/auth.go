// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// Set true to test the Notehub polling callback; false uses localhost login.  Note
// that polling is superior in that agents can use it without opening a local HTTP server.
const authUsePolling = false

// Sign into the notehub account with a personal access token
func authSignInToken(personalAccessToken string) error {
	// TODO: maybe call configInit() to set defaults?
	config, err := lib.GetConfig()
	if err != nil {
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
	fmt.Fprintf(os.Stderr, "signed in successfully with token\n")
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
const authExpiresFormat = "2006-01-02 15:04:05 MST"

// authSignInAction is the remedy suggested when sign-in is needed.  It opens a browser,
// so an agent should hand it to the person rather than run it.
const authSignInAction = "notehub --signin"

// authWhoAmI reports whether we are signed in to the hub, and as whom.  Whenever there
// are saved credentials it asks the hub, with one small request, because the hub is the
// judge of them: a recorded expiration that has passed is reported, but not trusted, since
// the hub may well still accept the token.
func authWhoAmI(hub string, credentials *lib.ConfigCreds, now time.Time) (status authStatus) {
	status.Hub = hub

	// With no credentials, we are simply not signed in
	if credentials == nil {
		status.Reason = "not signed in"
		status.Action = authSignInAction
		return
	}

	// How we are signed in.  Only an OAuth token's expiration is known to us.
	status.Method = authMethodOAuth
	if !credentials.IsOAuthAccessToken() {
		status.Method = authMethodPAT
	}
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
				status.Reason += " (the saved token's recorded expiration has passed)"
			}
			return
		}
		status.Reason = fmt.Sprintf("%s rejected: %s", authMethodName(status.Method), err)
		if status.expired {
			status.Reason = fmt.Sprintf("%s expired %s, and the hub rejected it: %s", authMethodName(status.Method),
				status.ExpiresAt.Format(authExpiresFormat), err)
		}
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

// authExpiresDescription describes how we are signed in and when that expires
func authExpiresDescription(status authStatus) string {
	switch {
	case status.ExpiresAt == nil:
		return fmt.Sprintf("%s (expiration not recorded)", authMethodName(status.Method))
	case status.expired:
		return fmt.Sprintf("%s the hub still accepts, though its recorded expiration of %s has passed",
			authMethodName(status.Method), status.ExpiresAt.Format(authExpiresFormat))
	}
	return fmt.Sprintf("%s expiring %s", authMethodName(status.Method), status.ExpiresAt.Format(authExpiresFormat))
}

// Sign into the Notehub account with browser-based OAuth2 flow
func authSignIn() error { return authSignInWithAgent("") }

func authSignInWithAgent(agentName string) (err error) {
	var reporter *agentSignInReporter
	if agentName != "" && authUsePolling {
		reporter = newAgentSignInReporter(os.Stdout)
		defer func() {
			if outputErr := reporter.finish(err); err == nil {
				err = outputErr
			}
		}()
	}

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

	hostname, _ := os.Hostname()
	signInName := authSignInName(agentName, hostname)

	// initiate the browser-based OAuth2 login flow
	var accessToken *notehub.AccessToken
	if authUsePolling {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		options := notehub.PollingLoginOptions{SignInName: signInName}
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

	config.SetDefaultCredentials(accessToken.AccessToken, accessToken.Email, &accessToken.ExpiresAt)

	// save the config with the new credentials
	if err := config.Write(); err != nil {
		return err
	}

	// print out information about the session
	if accessToken != nil && reporter == nil {
		fmt.Fprintf(os.Stderr, "%s\n", banner())
		fmt.Fprintf(os.Stderr, "signed in as %s\n", accessToken.Email)
		fmt.Fprintf(os.Stderr, "token expires at %s\n", accessToken.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
	}

	// Done
	return nil
}

// Agent sign-in is a stream of one JSON object per line. Only terminal objects
// carry success; a false value must still appear in the JSON.
type agentSignInMessage struct {
	Status           string `json:"status"`
	URL              string `json:"url,omitempty"`
	Success          *bool  `json:"success,omitempty"`
	RemainingSeconds *int   `json:"remaining_seconds,omitempty"`
}

type agentSignInReporter struct{ encoder *json.Encoder }

func newAgentSignInReporter(w io.Writer) *agentSignInReporter {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return &agentSignInReporter{encoder: encoder}
}

func (r *agentSignInReporter) url(url string) error {
	return r.encoder.Encode(agentSignInMessage{Status: "URL to be sent to the user so they can open a browser to sign in", URL: url})
}
func (r *agentSignInReporter) pending(remainingSeconds int) error {
	return r.encoder.Encode(agentSignInMessage{Status: "waiting for authorization", RemainingSeconds: &remainingSeconds})
}
func (r *agentSignInReporter) finish(err error) error {
	success := err == nil
	status := "signed in successfully"
	if err != nil {
		status = strings.ToLower(err.Error())
		if errors.Is(err, context.Canceled) {
			status = "sign-in was cancelled"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			status = "sign-in expired; please try again"
		}
	}
	return r.encoder.Encode(agentSignInMessage{Status: status, Success: &success})
}

// Use the same reporting for failures before the login routine can start.
func authAgentFailure(w io.Writer, err error) {
	reporter := newAgentSignInReporter(w)
	_ = reporter.finish(err)
}

// runSignIn is the handler for 'notehub signin', the same as --signin
func runSignIn(config *lib.ConfigSettings) error {
	if args := flag.Args(); len(args) != 0 {
		return fmt.Errorf("'%s %s' takes no arguments, but was given: %s", cliName, modeSignIn, strings.Join(args, " "))
	}
	if err := authSignIn(); err != nil {
		return fmt.Errorf("sign-in: %w", err)
	}
	return nil
}

// authAgentName validates the required name before sign-in can replace credentials.
func authAgentName(mode *cliMode) (string, error) {
	name := flagSignInAgent
	if mode.Name == modeSignInAgent {
		if flag.NArg() != 1 {
			return "", fmt.Errorf("'%s %s' requires one agent name; quote names containing spaces", cliName, modeSignInAgent)
		}
		name = flag.Arg(0)
	}
	if strings.TrimSpace(name) == "" {
		return "", errors.New("sign-in agent name must not be empty")
	}
	return name, nil
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

// runSignInAgent is the handler for 'notehub signin-agent', the same as --signin-agent.
func runSignInAgent(config *lib.ConfigSettings) error {
	return authSignInWithAgent(flagSignInAgent)
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
