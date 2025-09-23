// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/blues/note-cli/lib"
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
