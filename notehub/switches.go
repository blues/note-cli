// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The switches understood by this CLI.  Each declares the modes it may be used in; only
// those of the mode being run are registered, a switch used in the wrong mode is named as
// such, and help is generated from the same table.  To add a switch, add its variable and
// a row below.  A name may be defined once per set of modes, with its own usage and group
// in each, so long as no mode gets it twice.

package main

import (
	"strings"

	"github.com/blues/note-cli/lib/cli"
)

// The variables into which the switches are parsed
var (
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
	flagOverwrite   bool
	flagOut         string
	flagTrace       bool
	flagSignIn      bool
	flagSignInAgent string
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
	flagAgentChat   bool
)

// notehubGroups returns the help groups in display order; empty groups aren't shown
func notehubGroups() []cli.Group {
	return []cli.Group{
		{Name: cli.GroupGeneral, Description: "General Options"},
		{Name: "auth", Description: "Authentication & Session"},
		{Name: "scope", Description: "Project & Device Scope"},
		{Name: "vars", Description: "Environment Variables"},
		{Name: "request", Description: "API Request Options"},
		{Name: "output", Description: "Output Options"},
		{Name: "notefile", Description: "Notefiles"},
		{Name: "firmware", Description: "Firmware"},
		{Name: "files", Description: "Files"},
		{Name: "skills", Description: "Skills"},
		{Name: "other", Description: "Other Options"},
	}
}

// notehubSwitches returns every switch, in any mode
func notehubSwitches() (switches []*cli.Switch) {
	for _, more := range [][]*cli.Switch{defaultSwitches(), authSwitches(), apiSwitches(), firmwareSwitches(), filesSwitches(), skillsSwitches()} {
		switches = append(switches, more...)
	}
	return switches
}

// The mode lists used by the switches below
var (
	inDefault       = []string{modeDefault}
	inDefaultAndApi = []string{modeDefault, modeApi}
	inProjectModes  = []string{modeDefault, modeSkills, modeApi, modeFirmware, modeFiles}
	inAnyMode       = []string{cli.AnyMode}
)

// defaultSwitches returns the switches of the default mode, and those shared by every mode
func defaultSwitches() []*cli.Switch {
	return []*cli.Switch{

		// General Options, the only ones shown when there is nothing to do.  The framework
		// provides --help and --help-agent; lib registers --hub for every note CLI.
		{Name: "hub", External: true, ValueType: "hub", Group: cli.GroupGeneral, Modes: inAnyMode,
			Usage: "set currently active Notehub domain"},

		// Authentication & Session.  Logging in and out are modes; these are the spellings
		// that predate them, which keep working but aren't shown.
		{Name: "signin", Target: &flagSignIn, Group: "auth", Modes: inDefault, Hidden: true,
			Usage: "sign into hub in your browser, as 'notehub login' does"},
		{Name: "signin-token", Target: &flagSignInToken, Group: "auth", Modes: inDefault, Hidden: true,
			Usage: "sign in with a personal access `token`, as 'notehub login --with-token' does"},
		{Name: "signout", Target: &flagSignOut, Group: "auth", Modes: inDefault, Hidden: true,
			Usage: "sign out of hub, as 'notehub logout' does"},
		{Name: "token", Target: &flagToken, Group: "auth", Modes: inDefault, Hidden: true,
			Usage: "print the token in use, as 'notehub auth token' does"},

		// Project & Device Scope
		{Name: "projects", Target: &flagProjects, Group: "scope", Modes: inDefault,
			Usage: "list the projects you belong to, each with its products, as JSON"},
		{Name: "project", Target: &flagApp, Group: "scope", Modes: inProjectModes,
			Usage: "the project to act on, by its `projectUID`, as in app:…"},
		{Name: "product", Target: &flagProduct, Group: "scope", Modes: inProjectModes,
			Usage: "the project to act on, by one of its `productUID`s, as in com.example:sensor"},
		{Name: "device", Target: &flagDevice, Group: "scope", Modes: inDefaultAndApi,
			Usage: "the device a request concerns, by its `deviceUID`, as in dev:864475040518622"},
		{Name: "scope", Target: &flagScope, Group: "scope", Modes: inDefault,
			Usage: "the `devices` or fleets an operation concerns: dev:xx, fleet:xx, a fleet's name, @fleet for its devices, @file of them, or devices or fleets for all; comma-separated"},
		{Name: "provision", Target: &flagProvision, Group: "scope", Modes: inDefault,
			Usage: "provision the devices in --scope into --product, with --sn as the serial number"},
		{Name: "sn", Target: &flagSn, Group: "scope", Modes: inDefault,
			Usage: "the `serial` number to give the device being provisioned"},

		// Environment Variables
		{Name: "get-vars", Target: &flagVarsGet, Group: "vars", Modes: inDefault,
			Usage: "get the environment variables of the devices or fleets in --scope, as JSON"},
		{Name: "set-vars", Target: &flagVarsSet, Group: "vars", Modes: inDefault,
			Usage: "set environment variables on the devices or fleets in --scope from a `json` template, or @file; a value of - removes one"},

		// API Request Options
		{Name: "req", Target: &flagReq, Group: "request", Modes: inDefault,
			Usage: "a `request` in the Notecard's JSON form, sent to the hub for --device, or @file; the same as a bare request as the last argument"},
		{Name: "pretty", Target: &flagPretty, Group: "request", Modes: inDefault,
			Usage: "pretty print JSON output"},
		{Name: "json", Target: &flagJson, Group: "request", Modes: inDefault,
			Usage: "keep only the JSON lines of a response"},
		{Name: "verbose", Target: &flagVerbose, Group: "request", Modes: inProjectModes,
			Usage: "display requests and responses as they go by"},

		// Upload switches kept for scripts that predate the firmware and files modes.
		{Name: "upload", Target: &flagUpload, Group: "files", Modes: inDefault, Hidden: true,
			Usage: "the `filename` to upload"},
		{Name: "type", Target: &flagType, Group: "files", Modes: inDefault, Hidden: true,
			Usage: "the upload `type`, such as firmware"},
		{Name: "tags", Target: &flagTags, Group: "files", Modes: inDefault, Hidden: true,
			Usage: "`tags` to attach to the upload"},
		{Name: "notes", Target: &flagNotes, Group: "files", Modes: inDefault, Hidden: true,
			Usage: "`notes` to attach to the upload"},
		{Name: "overwrite", Target: &flagOverwrite, Group: "files", Modes: inDefault, Hidden: true,
			Usage: "use the exact upload filename and overwrite it on the service"},
		{Name: "out", Target: &flagOut, Group: "output", Modes: inDefault, Hidden: true,
			Usage: "write the request or upload response to a `filename`"},

		// Notefiles
		{Name: "explore", Target: &flagExplore, Group: "notefile", Modes: inDefault,
			Usage: "list the notefiles the hub holds for --device, and their notes; with --scope, for each device in it"},
		{Name: "reserved", Target: &flagReserved, Group: "notefile", Modes: inDefault,
			Usage: "when exploring, include the reserved notefiles too"},
		// 'notehub api' is the way to send requests by hand now; this still works
		{Name: "trace", Target: &flagTrace, Group: "notefile", Modes: inDefault, Hidden: true,
			Usage: "enter trace mode to interactively send requests to notehub"},

		// Other Options
		{Name: "agent-chat", Target: &flagAgentChat, Group: "other", Modes: inDefault,
			Usage: "fetch Notehub interaction guidance for agents from https://chat.notehub.io"},
		{Name: "version", Target: &flagVersion, Group: "other", Modes: inDefault,
			Usage: "print the version of this CLI"},
	}
}

// normalizeScope takes a value given to the wrong one of --project and --product as
// meant for the other: a projectUID begins with "app:", and a productUID never does
func normalizeScope() {
	switch {
	case flagApp != "" && !isProjectUID(flagApp) && flagProduct == "":
		flagApp, flagProduct = "", flagApp
	case flagProduct != "" && isProjectUID(flagProduct) && flagApp == "":
		flagApp, flagProduct = flagProduct, ""
	case flagApp != "" && !isProjectUID(flagApp) && flagProduct != "" && isProjectUID(flagProduct):
		flagApp, flagProduct = flagProduct, flagApp
	}
}

// isProjectUID reports whether a value is a projectUID rather than a productUID
func isProjectUID(uid string) bool {
	return strings.HasPrefix(uid, "app:")
}
