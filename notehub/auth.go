// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

func authIntrospectToken(config *lib.ConfigSettings, personalAccessToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://"+config.Hub+"/userinfo", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+personalAccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	userinfo := map[string]interface{}{}
	if err := note.JSONUnmarshal(body, &userinfo); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		err := userinfo["err"]
		return "", fmt.Errorf("error introspecting token: %s (http %d)", err, resp.StatusCode)
	}

	if email, ok := userinfo["email"].(string); !ok || email == "" {
		return "", fmt.Errorf("error introspecting token: no email in response")
	} else {
		return email, nil
	}
}

// Sign into the notehub account with a personal access token
func authSignInToken(personalAccessToken string) error {
	// TODO: maybe call configInit() to set defaults?
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	// Print hub if not the default
	fmt.Printf("notehub: %s\n", config.Hub)

	email, err := authIntrospectToken(config, personalAccessToken)
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

func authRevokeAccessToken(ctx context.Context, credentials *lib.ConfigCreds) error {
	// TODO: assert that it's an access token and not a PAT

	form := url.Values{
		"token":           {credentials.Token},
		"token_type_hint": {"access_token"},
		"client_id":       {"notehub_cli"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+credentials.Hub+"/oauth2/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Per RFC 7009: 200 OK even if token is already invalid; treat as success
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

type AccessToken struct {
	Host        string
	Email       string
	AccessToken string
	ExpiresAt   time.Time
}

func initiateBrowserBasedLogin(hub string) (*AccessToken, error) {
	// these are configured on the OAuth Client within Hydra
	clientId := "notehub_cli"
	port := 58766

	// return value
	var accessToken *AccessToken
	var accessTokenErr error

	randString := func(n int) string {
		letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		b := make([]rune, n)
		for i := range b {
			b[i] = letterRunes[rand.Intn(len(letterRunes))]
		}
		return string(b)
	}

	state := randString(16)
	codeVerifier := randString(50) // must be at least 43 characters
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt)
	defer signal.Reset(os.Interrupt)

	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authorizationCode := r.URL.Query().Get("code")
		callbackState := r.URL.Query().Get("state")

		errHandler := func(msg string) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %s", msg)
			fmt.Printf("error: %s\n", msg)
			accessTokenErr = errors.New(msg)
		}

		if callbackState != state {
			errHandler("state mismatch")
			return
		}

		///////////////////////////////////////////
		// Get the access token from the authorization code
		///////////////////////////////////////////

		tokenResp, err := http.Post(
			(&url.URL{
				Scheme: "https",
				Host:   hub,
				Path:   "/oauth2/token",
			}).String(),
			"application/x-www-form-urlencoded",
			strings.NewReader(url.Values{
				"client_id":     {clientId},
				"code":          {authorizationCode},
				"code_verifier": {codeVerifier},
				"grant_type":    {"authorization_code"},
				"redirect_uri":  {fmt.Sprintf("http://localhost:%d", port)},
			}.Encode()),
		)

		if err != nil {
			errHandler("error on /oauth2/token: " + err.Error())
			return
		}

		body, err := io.ReadAll(tokenResp.Body)
		if err != nil {
			errHandler("could not read body from /oauth2/token: " + err.Error())
			return
		}
		defer tokenResp.Body.Close()

		var tokenData map[string]interface{}
		if err := json.Unmarshal(body, &tokenData); err != nil {
			errHandler("could not unmarshal body from /oauth2/token: " + err.Error())
			return
		}

		// TODO: check for error in response
		// or this could panic when an error is returned
		accessTokenString := tokenData["access_token"].(string)
		expiresIn := time.Duration(tokenData["expires_in"].(float64)) * time.Second

		///////////////////////////////////////////
		// Get user's information (specifically email)
		///////////////////////////////////////////

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/userinfo", hub), nil)
		if err != nil {
			errHandler("could not create request for /userinfo: " + err.Error())
			return
		}
		req.Header.Set("Authorization", "Bearer "+accessTokenString)
		userinfoResp, err := http.DefaultClient.Do(req)
		if err != nil {
			errHandler("could not get userinfo: " + err.Error())
			return
		}

		userinfoBody, err := io.ReadAll(userinfoResp.Body)
		if err != nil {
			errHandler("could not read body from /userinfo: " + err.Error())
			return
		}
		defer userinfoResp.Body.Close()

		var userinfoData map[string]interface{}
		if err := json.Unmarshal(userinfoBody, &userinfoData); err != nil {
			errHandler("could not unmarshal body from /userinfo: " + err.Error())
			return
		}

		email := userinfoData["email"].(string)

		///////////////////////////////////////////
		// Build the access token response
		///////////////////////////////////////////

		accessToken = &AccessToken{
			Host:        hub,
			Email:       email,
			AccessToken: accessTokenString,
			ExpiresAt:   time.Now().Add(expiresIn),
		}

		///////////////////////////////////////////
		// respond to the browser and quit
		///////////////////////////////////////////

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "<p>Token exchange completed successfully</p><p>You may now close this window and return to the CLI application</p>")

		quit <- os.Interrupt
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	// Wait for OAuth callback to be hit, then shutdown HTTP server
	go func(server *http.Server, quit <-chan os.Signal, done chan<- bool) {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("error: %v", err)
		}
		close(done)
	}(server, quit, done)

	// Start HTTP server waiting for OAuth callback
	go func(server *http.Server) {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("error: %v", err)
		}
	}(server)

	// Open this URL to start the process of authentication
	authorizeUrl := url.URL{
		Scheme: "https",
		Host:   hub,
		Path:   "/oauth2/auth",
		RawQuery: url.Values{
			"client_id":             {clientId},
			"code_challenge":        {codeChallenge},
			"code_challenge_method": {"S256"},
			"redirect_uri":          {fmt.Sprintf("http://localhost:%d", port)},
			"response_type":         {"code"},
			"scope":                 {"openid email"},
			"state":                 {state},
		}.Encode(),
	}

	// Open web browser to authorize
	fmt.Printf("Opening web browser to initiate authentication...\n")
	open(authorizeUrl.String())

	// Wait for exchange to finish
	<-done

	return accessToken, accessTokenErr
}

// Sign into the Notehub account with browser-based OAuth2 flow
func authSignIn() error {

	// load config
	config, err := lib.GetConfig()
	if err != nil {
		return err
	}

	credentials := config.DefaultCredentials()

	// if signed in and it's an access token, then revoke it
	// we don't want to revoke a PAT because the user explicitly set an
	// expiration date on that token
	if credentials != nil && credentials.IsOAuthAccessToken() {
		if err := authRevokeAccessToken(context.Background(), credentials); err != nil {
			return err
		}
	}

	// initiate the browser-based OAuth2 login flow
	accessToken, err := initiateBrowserBasedLogin(config.Hub)
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
