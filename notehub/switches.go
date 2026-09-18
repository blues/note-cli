// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The switches understood by this CLI, and the variables into which they are parsed.
//
// Every switch declares the modes in which it may be used.  Only the switches
// available in the mode being run are registered with the flag package, and a switch
// used in the wrong mode is diagnosed by name rather than simply being rejected as
// undefined.  Help for a mode is generated from these same definitions, so it always
// describes exactly what that mode accepts.
//
// To add a switch, add its variable and a row to the table below.  To make an
// existing switch available in another mode, add that mode to the switch's Modes.

package main

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
	flagSignInToken string
	flagSignOut     bool
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

// cliSwitchGroups returns the groups in which switches are displayed in help, in the
// order in which they are displayed.  A group with no switches in the mode being
// displayed is not shown at all.
func cliSwitchGroups() []struct{ Name, Description string } {
	return []struct{ Name, Description string }{
		{cliGroupGeneral, "General Options"},
		{"auth", "Authentication & Session"},
		{"scope", "Project & Device Scope"},
		{"vars", "Environment Variables"},
		{"request", "API Request Options"},
		{"operations", "Notefile Operations"},
		{"notefile", "Notefile Management"},
		{"skills", "Skill Builder"},
		{"other", "Other Options"},
	}
}

// cliSwitches returns the definition of every switch understood by this CLI,
// regardless of mode
func cliSwitches() []*cliSwitch {
	if cliSwitchTable == nil {
		cliSwitchTable = append(notehubSwitches(), skillsSwitches()...)
	}
	return cliSwitchTable
}

var cliSwitchTable []*cliSwitch

// Convenience definitions of the mode lists used by the switches below
var (
	// inDefault is a switch that is available only in the CLI's historical behavior
	inDefault = []string{modeDefault}

	// inDefaultAndSkills is a switch that is available to the skill builder as well
	inDefaultAndSkills = []string{modeDefault, modeSkills}

	// inAnyMode is a switch that is available no matter what mode is being run
	inAnyMode = []string{cliAnyMode}
)

// notehubSwitches returns the switches used to interact with Notehub
func notehubSwitches() []*cliSwitch {
	return []*cliSwitch{

		// General Options, which are the switches that belong to the CLI itself
		// rather than to any one mode, and which are the only ones displayed when we
		// are invoked with nothing to do
		{Name: "help", Target: &flagHelp, Group: cliGroupGeneral, Modes: inAnyMode,
			Usage: "display all of the options available in this mode"},

		// The hub is registered with the flag package by lib, on behalf of every note
		// CLI, and so it is general to this CLI rather than belonging to any one mode
		{Name: "hub", External: true, ValueType: "string", Group: cliGroupGeneral, Modes: inAnyMode,
			Usage: "set Notehub domain"},

		// Authentication & Session
		{Name: "signin", Target: &flagSignIn, Group: "auth", Modes: inDefault,
			Usage: "sign-in to the notehub so that API requests may be made"},
		{Name: "signin-token", Target: &flagSignInToken, Group: "auth", Modes: inDefault,
			Usage: "sign-in to the notehub with an explicit token"},
		{Name: "signout", Target: &flagSignOut, Group: "auth", Modes: inDefault,
			Usage: "sign out of the notehub"},
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
