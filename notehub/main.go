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
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

// Exit codes
const exitOk = 0
const exitFail = 1

// Used by req.go
var flagApp string
var flagProduct string
var flagDevice string

// CLI Version - Set by ldflags during build/release
var version = "development"

// getFlagGroups returns the organized flag groups
func getFlagGroups() []lib.FlagGroup {
	return []lib.FlagGroup{
		{
			Name:        "auth",
			Description: "Authentication & Session",
			Flags: []*flag.Flag{
				lib.GetFlagByName("signin"),
				lib.GetFlagByName("signin-token"),
				lib.GetFlagByName("signout"),
				lib.GetFlagByName("token"),
			},
		},
		{
			Name:        "scope",
			Description: "Project & Device Scope",
			Flags: []*flag.Flag{
				lib.GetFlagByName("project"),
				lib.GetFlagByName("provision"),
				lib.GetFlagByName("product"),
				lib.GetFlagByName("device"),
				lib.GetFlagByName("scope"),
				lib.GetFlagByName("sn"),
			},
		},
		{
			Name:        "vars",
			Description: "Environment Variables",
			Flags: []*flag.Flag{
				lib.GetFlagByName("get-vars"),
				lib.GetFlagByName("set-vars"),
			},
		},
		{
			Name:        "request",
			Description: "API Request Options",
			Flags: []*flag.Flag{
				lib.GetFlagByName("req"),
				lib.GetFlagByName("pretty"),
				lib.GetFlagByName("json"),
				lib.GetFlagByName("verbose"),
			},
		},
		{
			Name:        "operations",
			Description: "Notefile Operations",
			Flags: []*flag.Flag{
				lib.GetFlagByName("upload"),
				lib.GetFlagByName("type"),
				lib.GetFlagByName("tags"),
				lib.GetFlagByName("notes"),
				lib.GetFlagByName("overwrite"),
				lib.GetFlagByName("out"),
			},
		},
		{
			Name:        "notefile",
			Description: "Notefile Management",
			Flags: []*flag.Flag{
				lib.GetFlagByName("explore"),
				lib.GetFlagByName("reserved"),
				lib.GetFlagByName("trace"),
			},
		},
		{
			Name:        "other",
			Description: "Other Options",
			Flags: []*flag.Flag{
				lib.GetFlagByName("version"),
			},
		},
	}
}

// open opens the specified URL in the default browser of the user.
func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

type AccessToken struct {
	Host        string
	Email       string
	AccessToken string
	ExpiresAt   time.Time
}

func login() (*AccessToken, error) {
	// these are configured on the OAuth Client within Hydra
	clientId := "notehub_cli"
	port := 58766

	// this is per-environment
	notehubHost := "scott.blues.tools"

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
				Host:   notehubHost,
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

		accessTokenString := tokenData["access_token"].(string)
		expiresIn := time.Duration(tokenData["expires_in"].(float64)) * time.Second

		///////////////////////////////////////////
		// Get user's information (specifically email)
		///////////////////////////////////////////

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/userinfo", notehubHost), nil)
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
			Host:        notehubHost,
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
		Host:   notehubHost,
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

// Main entry point
func main() {

	accessToken, err := login()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Access Token: %+v\n", accessToken)
	fmt.Printf("Exiting\n")
	os.Exit(0)

	// Override the default usage function to use our grouped format
	flag.Usage = func() {
		lib.PrintGroupedFlags(getFlagGroups(), "notehub")
	}

	// Process command line
	var flagReq string
	flag.StringVar(&flagReq, "req", "", "{json for device-like request}")
	var flagPretty bool
	flag.BoolVar(&flagPretty, "pretty", false, "pretty print json output")
	var flagJson bool
	flag.BoolVar(&flagJson, "json", false, "strip all non json lines from output")
	var flagUpload string
	flag.StringVar(&flagUpload, "upload", "", "filename to upload")
	var flagType string
	flag.StringVar(&flagType, "type", "", "indicate file type of image such as 'firmware'")
	var flagTags string
	flag.StringVar(&flagTags, "tags", "", "indicate tags to attach to uploaded image")
	var flagNotes string
	flag.StringVar(&flagNotes, "notes", "", "indicate notes to attach to uploaded image")
	var flagTrace bool
	flag.BoolVar(&flagTrace, "trace", false, "enter trace mode to interactively send requests to notehub")
	var flagOverwrite bool
	flag.BoolVar(&flagOverwrite, "overwrite", false, "use exact filename in upload and overwrite it on service")
	var flagOut string
	flag.StringVar(&flagOut, "out", "", "output filename")
	var flagSignIn bool
	flag.BoolVar(&flagSignIn, "signin", false, "sign-in to the notehub so that API requests may be made")
	var flagSignInToken string
	flag.StringVar(&flagSignInToken, "signin-token", "", "sign-in to the notehub with an explicit token")
	var flagSignOut bool
	flag.BoolVar(&flagSignOut, "signout", false, "sign out of the notehub")
	var flagToken bool
	flag.BoolVar(&flagToken, "token", false, "obtain the signed-in account's Authentication Token")
	var flagExplore bool
	flag.BoolVar(&flagExplore, "explore", false, "explore the contents of the device")
	var flagReserved bool
	flag.BoolVar(&flagReserved, "reserved", false, "when exploring, include reserved notefiles")
	var flagVerbose bool
	flag.BoolVar(&flagVerbose, "verbose", false, "display requests and responses")
	flag.StringVar(&flagApp, "project", "", "projectUID")
	flag.StringVar(&flagProduct, "product", "", "productUID")
	flag.StringVar(&flagDevice, "device", "", "deviceUID")
	var flagVersion bool
	flag.BoolVar(&flagVersion, "version", false, "print the current version of the CLI")
	var flagScope string
	flag.StringVar(&flagScope, "scope", "", "dev:xx or @fleet:xx or fleet:xx or @filename")
	var flagVarsGet bool
	flag.BoolVar(&flagVarsGet, "get-vars", false, "get environment vars")
	var flagVarsSet string
	flag.StringVar(&flagVarsSet, "set-vars", "", "set environment vars using a json template")
	var flagSn string
	flag.StringVar(&flagSn, "sn", "", "serial number")
	var flagProvision bool
	flag.BoolVar(&flagProvision, "provision", false, "provision devices")

	// Parse these flags and also the note tool config flags
	err = lib.FlagParse(false, true)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}

	// If no commands found, just show the config
	if len(os.Args) == 1 {
		lib.PrintGroupedFlags(getFlagGroups(), "notehub")
		lib.ConfigShow()
		os.Exit(exitOk)
	}

	// Process the sign-in request
	if flagSignIn {
		err = authSignIn()
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(exitFail)
		}
	}
	if flagSignInToken != "" {
		err = authSignInToken(flagSignInToken)
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(exitFail)
		}
	}
	if flagSignOut {
		err = authSignOut()
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(exitFail)
		}
	}

	// See if we did something
	didSomething := false

	// Display the token
	if flagToken {
		_, _, authenticated := lib.ConfigSignedIn()
		if !authenticated {
			fmt.Printf("please sign in using -signin\n")
			os.Exit(exitFail)
		}
		var token, username string
		username, token, err = authToken()
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(exitFail)
		} else {
			fmt.Printf("To issue HTTP API requests on behalf of %s place the token into the X-Session-Token header field\n", username)
			fmt.Printf("%s\n", token)
		}
		didSomething = true
	}

	// Create an output function that will be used during -req processing
	outq := make(chan string)
	go func() {
		for {
			fmt.Printf("%s", <-outq)
		}
	}()

	// Process the main part of the command line as a -req
	argsLeft := len(flag.Args())
	if argsLeft == 1 {
		flagReq = flag.Args()[0]
	} else if argsLeft != 0 {
		remainingArgs := strings.Join(flag.Args()[1:], " ")
		fmt.Printf("These switches must be placed on the command line prior to the request: %s\n", remainingArgs)
		os.Exit(exitFail)
	}

	// Process request starting with @ as a filename containing the request
	if strings.HasPrefix(flagReq, "@") {
		fn := strings.TrimPrefix(flagReq, "@")
		contents, err := os.ReadFile(fn)
		if err != nil {
			fmt.Printf("Can't read request file '%s': %s\n", fn, err)
			os.Exit(exitFail)
		}
		flagReq = string(contents)
	}

	// Process requests
	if flagReq != "" || flagUpload != "" {
		var rsp []byte
		rsp, err = reqHubV0JSON(flagVerbose, lib.ConfigAPIHub(), []byte(flagReq), flagUpload, flagType, flagTags, flagNotes, flagOverwrite, flagJson, nil)
		if err == nil {
			if flagOut == "" {
				if flagPretty {
					var rspo map[string]interface{}
					err = note.JSONUnmarshal(rsp, &rspo)
					if err != nil {
						fmt.Printf("%s", rsp)
					} else {
						rsp, _ = note.JSONMarshalIndent(rspo, "", "    ")
						fmt.Printf("%s", rsp)
					}
				} else {
					fmt.Printf("%s", rsp)
				}
			} else {
				outfile, err2 := os.Create(flagOut)
				if err2 != nil {
					fmt.Printf("Can't create output file: %s\n", err)
					os.Exit(exitFail)
				}
				outfile.Write(rsp)
				outfile.Close()
			}
			didSomething = true
		}
	}

	// Explore the contents of the device
	if err == nil && flagExplore && flagScope == "" {
		err = explore(flagReserved, flagVerbose, flagPretty)
		didSomething = true
	}

	// Enter trace mode
	if err == nil && flagTrace {
		err = trace()
		didSomething = true
	}

	if err == nil && flagVersion {
		fmt.Printf("Notehub CLI Version: %s\n", version)
		didSomething = true
	}

	// Determine the scope of a later request
	var scopeDevices, scopeFleets []string
	var appMetadata AppMetadata
	if err == nil && flagScope != "" {
		appMetadata, scopeDevices, scopeFleets, err = appGetScope(flagScope, flagVerbose)
		didSomething = true
		if err == nil {
			if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
				err = fmt.Errorf("'from' scope may include devices or fleets but not both")
				fmt.Printf("%d devices and %d fleets\n%v\n%v\n", len(scopeDevices), len(scopeFleets), scopeDevices, scopeFleets)
			}
			if len(scopeDevices) == 0 && len(scopeFleets) == 0 {
				err = fmt.Errorf("no devices or fleets found within the specified scope")
			}
		}
	}

	// Provision devices before doing get or set
	if err == nil && flagProvision {
		if flagScope == "" {
			err = fmt.Errorf("use -scope to specify device(s) to be provisioned")
		} else {
			if flagProduct == "" {
				err = fmt.Errorf("productUID must be specified")
			} else {
				if len(scopeDevices) != 0 {
					err = varsProvisionDevices(appMetadata, scopeDevices, flagProduct, flagSn, flagVerbose)
				} else {
					err = fmt.Errorf("no devices to provision")
				}
			}
		}
	}

	// Perform VarsGet actions based on scope
	if err == nil && flagScope != "" && flagVarsGet {
		var vars map[string]Vars
		var varsJSON []byte
		if len(scopeDevices) != 0 {
			vars, err = varsGetFromDevices(appMetadata, scopeDevices, flagVerbose)
		} else if len(scopeFleets) != 0 {
			vars, err = varsGetFromFleets(appMetadata, scopeFleets, flagVerbose)
		}
		if err == nil {
			if flagPretty {
				varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
			} else {
				varsJSON, err = note.JSONMarshal(vars)
			}
			if err == nil {
				fmt.Printf("%s\n", varsJSON)
			}
		}
	}

	// Perform VarsSet actions based on scope
	if err == nil && flagScope != "" && flagVarsSet != "" {
		template := Vars{}
		if strings.HasPrefix(flagVarsSet, "@") {
			var templateJSON []byte
			templateJSON, err = os.ReadFile(strings.TrimPrefix(flagVarsSet, "@"))
			if err == nil {
				err = note.JSONUnmarshal(templateJSON, &template)
			}
		} else {
			err = note.JSONUnmarshal([]byte(flagVarsSet), &template)
		}
		if err == nil {
			var vars map[string]Vars
			var varsJSON []byte
			if len(scopeDevices) != 0 {
				vars, err = varsSetFromDevices(appMetadata, scopeDevices, template, flagVerbose)
			} else if len(scopeFleets) != 0 {
				vars, err = varsSetFromFleets(appMetadata, scopeFleets, template, flagVerbose)
			}
			if err == nil {
				if flagPretty {
					varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
				} else {
					varsJSON, err = note.JSONMarshal(vars)
				}
				if err == nil {
					fmt.Printf("%s\n", varsJSON)
				}
			}
		}
	}

	// Explore the contents of the device
	if err == nil && len(scopeDevices) != 0 && flagExplore {
		didSomething = true
		for _, deviceUID := range scopeDevices {
			flagDevice = deviceUID
			err = explore(flagReserved, flagVerbose, flagPretty)
			if err != nil {
				break
			}
		}
	}

	// If we didn't do anything and we're just asking about an app, do it
	if err == nil && !didSomething && (flagApp != "" || flagProduct != "") {
		appMetadata, err = appGetMetadata(flagVerbose, flagVarsGet)
		if err == nil {
			var metaJSON []byte
			if flagPretty {
				metaJSON, err = note.JSONMarshalIndent(appMetadata, "", "    ")
			} else {
				metaJSON, err = note.JSONMarshal(appMetadata)
			}
			if err == nil {
				fmt.Printf("%s\n", metaJSON)
			}
		}
	}

	// Success
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}
	os.Exit(exitOk)

}
