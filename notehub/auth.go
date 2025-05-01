// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/notehub"
	terminal "golang.org/x/term"
)

// Sign into the notehub account with a token
func authSignInToken(token string) (err error) {

	// Sign out
	_, _, authenticated := lib.ConfigSignedIn()
	if authenticated {
		authSignOut()
	}

	// Print hub if not the default
	if lib.Config.Hub != "" {
		fmt.Printf("notehub %s\n", lib.Config.Hub)
	}

	// Extract the token and save it
	var creds lib.ConfigCreds
	creds.Token = token
	creds.User = "(token)"
	if lib.Config.HubCreds == nil {
		lib.Config.HubCreds = map[string]lib.ConfigCreds{}
	}
	hub := lib.Config.Hub
	if hub == "" {
		hub = notehub.DefaultAPIService
	}
	lib.Config.HubCreds[hub] = creds
	err = lib.ConfigWrite()
	if err != nil {
		return
	}

	// Done
	fmt.Printf("signed in successfully with token\n")
	return

}

// Sign into the notehub account
func authSignIn() (err error) {

	// Sign out
	_, _, authenticated := lib.ConfigSignedIn()
	if authenticated {
		authSignOut()
	}

	// Print banner
	fmt.Printf("%s", banner())

	// Print hub if not the default
	if lib.Config.Hub != "" {
		fmt.Printf("notehub %s\n", lib.Config.Hub)
	}

	// Read the account
	var username string
	for username == "" {
		fmt.Printf("account email@address.com > ")
		scanner := bufio.NewScanner(os.Stdin)
		ok := scanner.Scan()
		if ok {
			username = strings.TrimRight(scanner.Text(), "\r\n")
		}
	}

	// Read the password
	var password string
	for password == "" {
		fmt.Printf("account password > ")
		var pw []byte
		pw, err = terminal.ReadPassword(int(os.Stdin.Fd()))
		fmt.Printf("\n")
		if err != nil {
			return
		}
		password = string(pw)
	}

	// Do the sign-in HTTP request
	req := map[string]interface{}{}
	req["username"] = username
	req["password"] = password
	reqJSON, err2 := json.Marshal(req)
	if err2 != nil {
		err = err2
		return
	}
	httpURL := "https://" + lib.ConfigAPIHub() + "/auth/login"
	httpReq, err2 := http.NewRequest("POST", httpURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		err = err2
		return
	}
	httpReq.Header.Set("User-Agent", "notehub-client")
	httpReq.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{}
	httpRsp, err2 := httpClient.Do(httpReq)
	if err2 != nil {
		err = err2
		return
	}
	if httpRsp.StatusCode == http.StatusUnauthorized || httpRsp.StatusCode == http.StatusBadRequest {
		err = fmt.Errorf("unrecognized username or password")
		return
	}
	if httpRsp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %d", httpRsp.StatusCode)
		return
	}
	rspJSON, err2 := io.ReadAll(httpRsp.Body)
	if err2 != nil {
		err = err2
		return
	}
	rsp := map[string]interface{}{}
	err = json.Unmarshal(rspJSON, &rsp)
	if err != nil {
		err = fmt.Errorf("%s: '%s'", err, string(rspJSON))
		return
	}
	token := ""
	if rsp["session_token"] != nil {
		token = rsp["session_token"].(string)
	}
	if token == "" {
		err = fmt.Errorf("%s authentication error", lib.ConfigAPIHub())
		return
	}

	// Extract the token and save it
	var creds lib.ConfigCreds
	creds.Token = token
	creds.User = username
	if lib.Config.HubCreds == nil {
		lib.Config.HubCreds = map[string]lib.ConfigCreds{}
	}
	hub := lib.Config.Hub
	if hub == "" {
		hub = notehub.DefaultAPIService
	}
	lib.Config.HubCreds[hub] = creds
	err = lib.ConfigWrite()
	if err != nil {
		return
	}

	// Done
	fmt.Printf("signed in successfully as %s\n", username)
	return

}

// Sign out of the API
func authSignOut() (err error) {

	// Exit if not signed in
	user, token, authenticated := lib.ConfigSignedIn()
	if !authenticated {
		err = fmt.Errorf("not currently signed in")
		return
	}

	// Get the token, and clear it
	if lib.Config.HubCreds != nil {
		hub := lib.Config.Hub
		if hub == "" {
			delete(lib.Config.HubCreds, "")
			hub = notehub.DefaultAPIService
		}
		delete(lib.Config.HubCreds, hub)
	}
	err = lib.ConfigWrite()
	if err != nil {
		return
	}

	// Hit the logout endpoint in the API to revoke the session
	httpURL := "https://" + lib.ConfigAPIHub() + "/auth/logout"
	httpReq, err2 := http.NewRequest("POST", httpURL, bytes.NewBuffer([]byte{}))
	if err != nil {
		err = err2
		return
	}
	httpReq.Header.Set("User-Agent", "notehub-client")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Session-Token", token)
	httpClient := &http.Client{}
	httpRsp, err2 := httpClient.Do(httpReq)
	if err2 != nil {
		err = err2
		return
	}
	if httpRsp.StatusCode == http.StatusUnauthorized || httpRsp.StatusCode == http.StatusBadRequest {
		err = fmt.Errorf("user is not signed in")
		return
	}
	rspJSON, err2 := io.ReadAll(httpRsp.Body)
	if err2 != nil {
		err = err2
		return
	}

	response := string(rspJSON)
	if response == "" {
		fmt.Printf("%s signed out successfully\n", user)
	} else {
		fmt.Printf("%s signed out successfully: %s\n", user, response)
	}
	return
}

// Get the token for use in the API
func authToken() (user string, token string, err error) {
	var authenticated bool
	user, token, authenticated = lib.ConfigSignedIn()
	if !authenticated {
		err = fmt.Errorf("not currently signed in")
		return
	}
	return
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
