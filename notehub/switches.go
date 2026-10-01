// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The switches understood by this CLI.  Each declares the modes it may be used in; only
// those of the mode being run are registered, a switch used in the wrong mode is named as
// such, and help is generated from the same table.  To add a switch, add its variable and
// a row below.

package main

import "strings"

// The variables into which the switches are parsed
var (
	flagHelp        bool
	flagApp         string
	flagProduct     string
	flagDevice      string
	flagReq         string
	flagPretty      bool
	flagJson        bool
	flagUpload      string
	flagType        string
	flagTags        string
	flagNotes       string
	flagTrace       bool
	flagOverwrite   bool
	flagOut         string
	flagSignIn      bool
	flagSignInAgent bool
	flagSignInToken string
	flagSignOut     bool
	flagWhoAmI      bool
	flagToken       bool
	flagExplore     bool
	flagReserved    bool
	flagVerbose     bool
	flagVersion     bool
	flagScope       string
	flagVarsGet     bool
	flagVarsSet     string
	flagSn          string
	flagProvision   bool
	flagProjects    bool
)

// cliSwitchGroups returns the help groups in display order; empty groups aren't shown
func cliSwitchGroups() []struct{ Name, Description string } {
	return []struct{ Name, Description string }{
		{cliGroupGeneral, "General Options"},
		{"auth", "Authentication & Session"},
		{"scope", "Project & Device Scope"},
		{"vars", "Environment Variables"},
		{"request", "API Request Options"},
		{"operations", "Notefile Operations"},
		{"notefile", "Notefile Management"},
		{"skills", "Skills"},
		{"other", "Other Options"},
	}
}

// cliSwitches returns every switch, in any mode
func cliSwitches() []*cliSwitch {
	if cliSwitchTable == nil {
		cliSwitchTable = append(notehubSwitches(), skillsSwitches()...)
	}
	return cliSwitchTable
}

var cliSwitchTable []*cliSwitch

// The mode lists used by the switches below
var (
	inDefault          = []string{modeDefault}
	inDefaultAndSkills = []string{modeDefault, modeSkills}
	inAnyMode          = []string{cliAnyMode}
)

// notehubSwitches returns the switches used to interact with Notehub
func notehubSwitches() []*cliSwitch {
	return []*cliSwitch{

		// General Options, the only ones shown when there is nothing to do
		{Name: "help", Target: &flagHelp, Group: cliGroupGeneral, Modes: inAnyMode,
			Usage: "display all of the options available in this mode"},

		// lib registers --hub for every note CLI
		{Name: "hub", External: true, ValueType: "string", Group: cliGroupGeneral, Modes: inAnyMode,
			Usage: "set Notehub domain"},

		// Authentication & Session
		{Name: "signin", Target: &flagSignIn, Group: "auth", Modes: inDefault,
			Usage: "sign-in to the notehub so that API requests may be made"},
		{Name: "signin-agent", Target: &flagSignInAgent, Group: "auth", Modes: inDefault,
			Usage: "sign in to Notehub from your agent"},
		{Name: "signin-token", Target: &flagSignInToken, Group: "auth", Modes: inDefault,
			Usage: "sign-in to the notehub with an explicit token"},
		{Name: "signout", Target: &flagSignOut, Group: "auth", Modes: inDefault,
			Usage: "sign out of the notehub"},
		{Name: "whoami", Target: &flagWhoAmI, Group: "auth", Modes: inDefault,
			Usage: "report whether signed in to the configured hub, and as whom (exit 0 if so)"},
		{Name: "token", Target: &flagToken, Group: "auth", Modes: inDefault,
			Usage: "obtain the signed-in account's Authentication Token"},

		// Project & Device Scope
		{Name: "projects", Target: &flagProjects, Group: "scope", Modes: inDefault,
			Usage: "list all projects"},
		{Name: "project", Target: &flagApp, Group: "scope", Modes: inDefaultAndSkills,
			Usage: "projectUID"},
		{Name: "provision", Target: &flagProvision, Group: "scope", Modes: inDefault,
			Usage: "provision devices"},
		{Name: "product", Target: &flagProduct, Group: "scope", Modes: inDefaultAndSkills,
			Usage: "productUID"},
		{Name: "device", Target: &flagDevice, Group: "scope", Modes: inDefault,
			Usage: "deviceUID"},
		{Name: "scope", Target: &flagScope, Group: "scope", Modes: inDefault,
			Usage: "dev:xx or @fleet:xx or fleet:xx or @filename"},
		{Name: "sn", Target: &flagSn, Group: "scope", Modes: inDefault,
			Usage: "serial number"},

		// Environment Variables
		{Name: "get-vars", Target: &flagVarsGet, Group: "vars", Modes: inDefault,
			Usage: "get environment vars"},
		{Name: "set-vars", Target: &flagVarsSet, Group: "vars", Modes: inDefault,
			Usage: "set environment vars using a json template"},

		// API Request Options
		{Name: "req", Target: &flagReq, Group: "request", Modes: inDefault,
			Usage: "{json for device-like request}"},
		{Name: "pretty", Target: &flagPretty, Group: "request", Modes: inDefault,
			Usage: "pretty print json output"},
		{Name: "json", Target: &flagJson, Group: "request", Modes: inDefault,
			Usage: "strip all non json lines from output"},
		{Name: "verbose", Target: &flagVerbose, Group: "request", Modes: inDefaultAndSkills,
			Usage: "display requests and responses"},

		// Notefile Operations
		{Name: "upload", Target: &flagUpload, Group: "operations", Modes: inDefault,
			Usage: "filename to upload"},
		{Name: "type", Target: &flagType, Group: "operations", Modes: inDefault,
			Usage: "indicate file type of image such as 'firmware'"},
		{Name: "tags", Target: &flagTags, Group: "operations", Modes: inDefault,
			Usage: "indicate tags to attach to uploaded image"},
		{Name: "notes", Target: &flagNotes, Group: "operations", Modes: inDefault,
			Usage: "indicate notes to attach to uploaded image"},
		{Name: "overwrite", Target: &flagOverwrite, Group: "operations", Modes: inDefault,
			Usage: "use exact filename in upload and overwrite it on service"},
		{Name: "out", Target: &flagOut, Group: "operations", Modes: inDefault,
			Usage: "output filename"},

		// Notefile Management
		{Name: "explore", Target: &flagExplore, Group: "notefile", Modes: inDefault,
			Usage: "explore the contents of the device"},
		{Name: "reserved", Target: &flagReserved, Group: "notefile", Modes: inDefault,
			Usage: "when exploring, include reserved notefiles"},
		{Name: "trace", Target: &flagTrace, Group: "notefile", Modes: inDefault,
			Usage: "enter trace mode to interactively send requests to notehub"},

		// Other Options
		{Name: "version", Target: &flagVersion, Group: "other", Modes: inDefault,
			Usage: "print the current version of the CLI"},
	}
}

// cliNormalizeScope takes a value given to the wrong one of --project and --product as
// meant for the other: a projectUID begins with "app:", and a productUID never does
func cliNormalizeScope() {
	switch {
	case flagApp != "" && !cliIsProjectUID(flagApp) && flagProduct == "":
		flagApp, flagProduct = "", flagApp
	case flagProduct != "" && cliIsProjectUID(flagProduct) && flagApp == "":
		flagApp, flagProduct = flagProduct, ""
	case flagApp != "" && !cliIsProjectUID(flagApp) && flagProduct != "" && cliIsProjectUID(flagProduct):
		flagApp, flagProduct = flagProduct, flagApp
	}
}

// cliIsProjectUID reports whether a value is a projectUID rather than a productUID
func cliIsProjectUID(uid string) bool {
	return strings.HasPrefix(uid, "app:")
}
