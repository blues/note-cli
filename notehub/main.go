// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

	// If no commands found, just show the config
	if len(os.Args) == 1 {
		lib.PrintGroupedFlags(getFlagGroups(), "notehub")
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

	// Exit
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}
	os.Exit(exitOk)

}
