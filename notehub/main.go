// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

// Exit codes
const exitOk = 0
const exitFail = 1

// CLI Version - Set by ldflags during build/release
var version = "development"

// withCreds validates credentials and then calls the provided function
func withCreds(credentials *lib.ConfigCreds, fn func() error) error {
	if err := credentials.Validate(); err != nil {
		config, _ := lib.GetConfig()
		fmt.Printf("invalid credentials for %s: %s\n", config.Hub, err)
		return fmt.Errorf("please use 'notehub -signin' or 'notehub -signin-token' to sign into Notehub")
	}
	return fn()
}

// Main entry point
func main() {

	// The first argument on the command line may optionally be a mode keyword, which
	// determines which switches are available and how the remaining arguments are
	// interpreted.  When no mode keyword is present we run in default mode, which is
	// the behavior of this CLI as it was before modes were introduced.
	mode, args := cliExtractMode(os.Args[1:])

	// Rewrite the command line with the mode keyword removed, so that the flag
	// package sees exactly the command line that it would have seen had modes never
	// existed.  This is what makes 'notehub default ...' identical to 'notehub ...',
	// and it keeps the config processing within lib unchanged.
	os.Args = append([]string{os.Args[0]}, args...)

	// Register the switches available in this mode, and only those switches, and use
	// a usage message describing this mode
	cliRegisterSwitches(mode)
	flag.Usage = func() {
		cliPrintHelp(mode, true)
	}

	// Diagnose a switch that exists but that is not available in this mode, which the
	// flag package would otherwise report as simply being undefined
	if err := cliValidateSwitches(mode, args); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}

	// Parse these flags and also the note tool config flags
	err := lib.FlagParse(false, true)
	if err != nil {
		fmt.Printf("flags: %s\n", err)
		os.Exit(exitFail)
	}

	// after flags are parsed, get the resulting configuration
	config, err := lib.GetConfig()
	if err != nil {
		fmt.Printf("config: %s\n", err)
		os.Exit(exitFail)
	}

	// Display everything available in this mode, if that's all that was asked for
	if flagHelp {
		cliPrintHelp(mode, true)
		os.Exit(exitOk)
	}

	// Run the mode
	if err = mode.Run(config); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}
	os.Exit(exitOk)

}

// runDefault is the handler for the default mode, which is what we run when no mode
// keyword is specified on the command line
func runDefault(config *lib.ConfigSettings) (err error) {

	// If no commands found, just show where to go next along with the config
	if len(os.Args) == 1 {
		cliPrintHelp(cliModeNamed(modeDefault), false)
		config.Print()
		os.Exit(exitOk)
	}

	// Process the interactive sign-in
	if flagSignIn {
		err = authSignIn()
		if err != nil {
			fmt.Printf("sign-in: %s\n", err)
			os.Exit(exitFail)
		}
	}

	// Process the sign-in with explicit personal access token
	if flagSignInToken != "" {
		err = authSignInToken(flagSignInToken)
		if err != nil {
			fmt.Printf("sign-in-token: %s\n", err)
			os.Exit(exitFail)
		}
	}

	// Get the current API credentials
	credentials := config.DefaultCredentials()

	// Process the sign-out
	if flagSignOut {
		if err := config.RemoveDefaultCredentials(); err != nil {
			fmt.Printf("sign-out: %s\n", err)
			os.Exit(exitFail)
		}
		os.Exit(exitOk)
	}

	// Report whether we are signed in, with an exit code that says so.  This is the
	// question that an agent asks before doing anything else, so it is answered with a
	// single line and at most one small request to the hub
	if flagWhoAmI {
		status := authWhoAmI(config.Hub, credentials, time.Now())
		authWhoAmIPrint(status, flagJson)
		if !status.SignedIn {
			os.Exit(exitFail)
		}
		os.Exit(exitOk)
	}

	// Display the token
	if flagToken {
		if credentials == nil {
			fmt.Printf("please sign in using -signin or -signin-token\n")
			os.Exit(exitFail)
		}

		fmt.Printf("%s\n", credentials.Token)
		os.Exit(exitOk)
	}

	// Past this point, we need valid credentials, so validate them here

	// See if we did something
	didSomething := false

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
		// The only non-switch argument accepted in this mode is a device-like request,
		// which is either JSON or an @filename.  A bare word in that position is far
		// more likely to be a misplaced or misspelled mode keyword than a request, so
		// say so rather than sending it to the service as a request that can't succeed.
		if err = cliCheckNotAMode(flag.Args()[0]); err != nil {
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
		err = withCreds(credentials, func() (err error) {
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
		err = withCreds(credentials, func() (err error) {
			var rsp []byte
			rsp, err = reqHubV0JSON(flagVerbose, lib.ConfigAPIHub(), []byte(flagReq), flagUpload, flagType, flagTags, flagNotes, flagOverwrite, flagJson, nil)
			if err != nil {
				return err
			}
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
				var outfile *os.File
				outfile, err = os.Create(flagOut)
				if err != nil {
					return err
				}
				outfile.Write(rsp)
				outfile.Close()
			}
			return nil
		})
	}

	// Explore the contents of the device
	if err == nil && flagExplore && flagScope == "" {
		didSomething = true
		err = withCreds(credentials, func() error {
			return explore(flagReserved, flagVerbose, flagPretty)
		})
	}

	// Enter trace mode
	if err == nil && flagTrace {
		didSomething = true
		err = withCreds(credentials, func() error {
			return trace()
		})
	}

	// Determine the scope of a later request
	var scopeDevices, scopeFleets []string
	var appMetadata AppMetadata
	if err == nil && flagScope != "" {
		didSomething = true
		err = withCreds(credentials, func() (err error) {
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
		err = withCreds(credentials, func() error {
			if flagScope == "" {
				return fmt.Errorf("use -scope to specify device(s) to be provisioned")
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
		err = withCreds(credentials, func() (err error) {
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
		err = withCreds(credentials, func() (err error) {
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
		err = withCreds(credentials, func() (err error) {
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
		err = withCreds(credentials, func() (err error) {
			appMetadata, err = appGetMetadata(flagVerbose, flagVarsGet)
			if err != nil {
				return err
			}
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
