// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
)

// Exit codes
const exitOk = 0
const exitFail = 1

// CLI Version - Set by ldflags during build/release
var version = "development"

// withCreds validates credentials and then calls the provided function.  The remedy for
// bad ones depends on where they came from: a token from the environment is the caller's
// to replace, since signing in wouldn't displace it.
func withCreds(config *lib.ConfigSettings, fn func() error) error {
	credentials := config.DefaultCredentials()
	if credentials == nil {
		return fmt.Errorf("not logged in: set %s, or run '%s login'", lib.ConfigTokenEnvVar, cliName)
	}
	if _, err := config.IntrospectCredentials(context.Background(), config.Hub); err != nil {
		if credentials.Source != "" {
			return fmt.Errorf("%s rejected the token in %s: %s", credentials.Hub, credentials.Source, err)
		}
		return fmt.Errorf("%s rejected the saved login: %s - run '%s login' to log in again", credentials.Hub, err, cliName)
	}
	return fn()
}

// Main entry point
func main() {

	// Describe this CLI to the framework, and find the mode keyword, which comes first
	cli.Define(notehubTool())
	mode, args := cli.Start(os.Args[1:])

	// An agent logging in is told what went wrong as JSON, from here on
	agentSignIn := cli.SwitchAmong(args, "json") &&
		(mode.Name == modeLogin || (mode.Name == modeAuth && strings.EqualFold(cli.FirstPositional(args), "login")))
	if agentSignIn {
		// Return parse errors for JSON reporting, without printing usage to stdout.
		flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)
		flag.CommandLine.Usage = func() {}
	}
	fail := func(err error) {
		if agentSignIn {
			authAgentFailure(os.Stdout, err)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		os.Exit(exitFail)
	}

	// Parse the switches, and get the resulting configuration
	config, err := cli.Parse(mode, args)
	if err != nil {
		fail(err)
	}

	// Run the mode, unless the login command has already reported what went wrong
	if err = mode.Run(config); err != nil {
		if agentSignIn && errors.Is(err, errAgentReported) {
			os.Exit(exitFail)
		}
		fail(err)
	}
	os.Exit(exitOk)

}

// runDefault is the handler for the default mode, run when no mode is given
func runDefault(config *lib.ConfigSettings) (err error) {

	// If no commands found, just show where to go next along with the config
	if len(os.Args) == 1 {
		cli.PrintHelp(cli.ModeNamed(modeDefault), false)
		cli.PrintSettings(config)
		os.Exit(exitOk)
	}

	// Agent guidance is public and needs no Notehub credentials.
	if flagAgentChat {
		return agentChat(os.Stdout, &http.Client{Timeout: 30 * time.Second})
	}

	// Process --signin and --signin-token, the spellings of 'notehub login' that predate
	// it.  As they always have, they and --signout make the hub given by --hub the active
	// one, which 'notehub login' and 'notehub logout' don't.
	if flagSignIn || flagSignInToken != "" || flagSignOut {
		config.AdoptHub()
	}
	if flagSignIn {
		if err = authSignIn(); err != nil {
			return fmt.Errorf("login: %w", err)
		}
	}
	if flagSignInToken != "" {
		if err = authSignInToken(flagSignInToken); err != nil {
			return fmt.Errorf("login: %w", err)
		}
	}

	// Process --signout, the spelling of 'notehub logout' that predates it
	if flagSignOut {
		if err = authLogout(config); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
		os.Exit(exitOk)
	}

	// Process --token, the spelling of 'notehub auth token' that predates it
	if flagToken {
		if err = authPrintToken(config); err != nil {
			return err
		}
		os.Exit(exitOk)
	}

	// Past this point, we need valid credentials, so validate them here

	// See if we did something
	didSomething := false

	// Process the main part of the command line as a --req
	argsLeft := len(flag.Args())
	if argsLeft == 1 {
		// A bare word here is a misplaced or misspelled mode, not a request
		if err = cli.CheckNotAMode(flag.Args()[0]); err != nil {
			return err
		}
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
	if err == nil && flagVersion {
		didSomething = true
		fmt.Printf("Notehub CLI Version: %s\n", version)
	}

	if err == nil && flagProjects {
		didSomething = true
		err = withCreds(config, func() (err error) {
			projects, err := appListProjects(flagVerbose)
			if err != nil {
				return err
			}
			var projectsJSON []byte
			if flagPretty {
				projectsJSON, err = note.JSONMarshalIndent(projects, "", "    ")
			} else {
				projectsJSON, err = note.JSONMarshal(projects)
			}
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", projectsJSON)
			return nil
		})
	}

	if flagReq != "" || flagUpload != "" {
		didSomething = true
		err = withCreds(config, func() (err error) {
			var rsp []byte
			rsp, err = reqHubV0JSON(flagVerbose, lib.ConfigAPIHub(), []byte(flagReq), flagUpload, flagType, flagTags, flagNotes, flagOverwrite, flagJson, nil)
			if err != nil {
				return err
			}
			if flagOut != "" {
				return os.WriteFile(flagOut, rsp, 0666)
			}
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
			return nil
		})
	}

	// Explore the contents of the device
	if err == nil && flagExplore && flagScope == "" {
		didSomething = true
		err = withCreds(config, func() error {
			return explore(flagReserved, flagVerbose, flagPretty)
		})
	}

	// Enter trace mode
	if err == nil && flagTrace {
		didSomething = true
		err = withCreds(config, func() error {
			return trace()
		})
	}

	// Determine the scope of a later request
	var scopeDevices, scopeFleets []string
	var appMetadata AppMetadata
	if err == nil && flagScope != "" {
		didSomething = true
		err = withCreds(config, func() (err error) {
			appMetadata, scopeDevices, scopeFleets, err = appGetScope(flagScope, flagVerbose)
			if err == nil {
				if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
					err = fmt.Errorf("'from' scope may include devices or fleets but not both")
					fmt.Printf("%d devices and %d fleets\n%v\n%v\n", len(scopeDevices), len(scopeFleets), scopeDevices, scopeFleets)
				}
				if len(scopeDevices) == 0 && len(scopeFleets) == 0 {
					err = fmt.Errorf("no devices or fleets found within the specified scope")
				}
			}
			return err
		})
	}

	// Provision devices before doing get or set
	if err == nil && flagProvision {
		didSomething = true
		err = withCreds(config, func() error {
			if flagScope == "" {
				return fmt.Errorf("use --scope to specify device(s) to be provisioned")
			}
			if flagProduct == "" {
				return fmt.Errorf("productUID must be specified")
			}
			if len(scopeDevices) != 0 {
				return varsProvisionDevices(appMetadata, scopeDevices, flagProduct, flagSn, flagVerbose)
			}
			return fmt.Errorf("no devices to provision")
		})
	}

	// Perform VarsGet actions based on scope
	if err == nil && flagScope != "" && flagVarsGet {
		didSomething = true
		err = withCreds(config, func() (err error) {
			var vars map[string]Vars
			var varsJSON []byte
			if len(scopeDevices) != 0 {
				vars, err = varsGetFromDevices(appMetadata, scopeDevices, flagVerbose)
			} else if len(scopeFleets) != 0 {
				vars, err = varsGetFromFleets(appMetadata, scopeFleets, flagVerbose)
			}
			if err != nil {
				return err
			}
			if flagPretty {
				varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
			} else {
				varsJSON, err = note.JSONMarshal(vars)
			}
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", varsJSON)
			return nil
		})
	}

	// Perform VarsSet actions based on scope
	if err == nil && flagScope != "" && flagVarsSet != "" {
		didSomething = true
		err = withCreds(config, func() (err error) {
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
			if err != nil {
				return err
			}
			var vars map[string]Vars
			var varsJSON []byte
			if len(scopeDevices) != 0 {
				vars, err = varsSetFromDevices(appMetadata, scopeDevices, template, flagVerbose)
			} else if len(scopeFleets) != 0 {
				vars, err = varsSetFromFleets(appMetadata, scopeFleets, template, flagVerbose)
			}
			if err != nil {
				return err
			}
			if flagPretty {
				varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
			} else {
				varsJSON, err = note.JSONMarshal(vars)
			}
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", varsJSON)
			return nil
		})
	}

	// Explore the contents of the device
	if err == nil && len(scopeDevices) != 0 && flagExplore {
		didSomething = true
		err = withCreds(config, func() (err error) {
			for _, deviceUID := range scopeDevices {
				flagDevice = deviceUID
				err = explore(flagReserved, flagVerbose, flagPretty)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	// If we didn't do anything and we're just asking about an app, do it
	if err == nil && !didSomething && (flagApp != "" || flagProduct != "") {
		didSomething = true
		err = withCreds(config, func() (err error) {
			appMetadata, err = appGetMetadata(flagVerbose, flagVarsGet)
			if err != nil {
				return err
			}
			appMetadata.App.Role = appGetRole(flagVerbose, appMetadata.App.UID)
			var metaJSON []byte
			if flagPretty {
				metaJSON, err = note.JSONMarshalIndent(appMetadata, "", "    ")
			} else {
				metaJSON, err = note.JSONMarshal(appMetadata)
			}
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", metaJSON)
			return nil
		})
	}

	// Done
	return err

}
