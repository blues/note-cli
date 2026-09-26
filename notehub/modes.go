// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The modes understood by this CLI.  A mode is an optional keyword that appears as
// the first argument on the command line and that determines which switches are
// available and how the remaining arguments are interpreted.
//
// To add a mode, add it to the table below, write its handler, and add its name to
// the Modes list of each switch that the mode should accept (see switches.go).

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The name of each mode, as typed on the command line
const (
	// modeDefault is the behavior of this CLI as it was before modes were introduced.
	// It is what we run when no mode keyword is specified, and specifying it
	// explicitly is identical to not specifying a mode at all.
	modeDefault = "default"

	// modeSignIn signs in to Notehub in the browser, exactly as --signin does in the
	// default mode.  The switch is the documented form; the mode exists because typing
	// 'notehub signin' without the hyphens is a common mistake, and it should simply work.
	modeSignIn = "signin"

	// modeSkills reads and manages the skills a project holds
	modeSkills = "skills"

	// modeHelp displays help.  Help is displayed with --help, and this keyword is a
	// hidden alias of it for those who type it out of habit.
	modeHelp = "help"
)

// cliModes returns every mode understood by this CLI, in the order in which they are
// displayed in help
func cliModes() []*cliMode {
	if cliModeTable == nil {
		cliModeTable = []*cliMode{
			{
				Name:    modeDefault,
				Summary: "interact with Notehub (requests, uploads, env vars, provisioning)",
				Run:     runDefault,
			},
			{
				Name:    modeSignIn,
				Summary: "sign in to Notehub in your browser",
				Detail: "'notehub signin' opens your browser to sign in to Notehub, and saves the\n" +
					"credentials for the requests that follow.  To sign in with a personal\n" +
					"access token instead, use 'notehub --signin-token', and to see whether you\n" +
					"are signed in, and as whom, use 'notehub --whoami'.",
				Run: runSignIn,
			},
			{
				Name:     modeSkills,
				Summary:  "read and manage the skills a project holds",
				Args:     "[command]",
				Commands: skillsCommands(),
				Detail: "With no command, 'notehub skills' writes everything the project holds to\n" +
					"stdout as one document, with the index first and every cross-reference\n" +
					"turned into a link, and a skill's name where a command would go shows\n" +
					"that one skill.  'get' and 'set' move skills between the project and local\n" +
					"files - one file, or a whole directory at a time - and 'set' never removes\n" +
					"anything: only 'delete' does.\n\n" +
					"A project's skills are written by training it: point any AI agent at\n" +
					"https://notehub.md and ask it to train the project.",
				Run: runSkills,
			},
			{
				Name:    modeHelp,
				Summary: "display help for this CLI or for one of its modes",
				Args:    "[mode]",
				Hidden:  true,
				Run:     runHelp,
			},
		}
	}
	return cliModeTable
}

var cliModeTable []*cliMode

// runHelp is the handler for 'notehub help [mode]'
func runHelp(config *lib.ConfigSettings) error {

	// With no argument, display the top-level help
	args := flag.Args()
	if len(args) == 0 {
		cliPrintHelp(cliModeNamed(modeDefault), true)
		return nil
	}

	// Display help for the specified mode
	mode := cliModeNamed(args[0])
	if mode == nil {
		return fmt.Errorf("there is no '%s' mode - the modes are: %s", args[0], strings.Join(cliModeNames(), ", "))
	}
	cliPrintHelp(mode, true)
	return nil

}
