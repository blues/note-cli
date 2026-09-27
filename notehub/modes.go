// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The modes understood by this CLI.  A mode is an optional first-argument keyword that
// selects which switches apply and how the remaining arguments are read.  To add one, add
// it to the table below, write its handler, and list it in the Modes of each switch it
// accepts (see switches.go).

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The name of each mode, as typed on the command line
const (
	// modeDefault is the CLI's original behavior, and what runs when no mode is given
	modeDefault = "default"

	// modeSignIn is --signin, for those who type it without the hyphens
	modeSignIn = "signin"

	// modeSkills reads and manages the skills a project holds
	modeSkills = "skills"

	// modeHelp is a hidden alias of --help
	modeHelp = "help"
)

// cliModes returns every mode, in the order help displays them
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
				Detail: "With no command, 'notehub skills' writes the whole set to stdout as one\n" +
					"document, index first, with cross-references turned into links.  A skill's\n" +
					"name in place of a command shows just that skill.  'get' and 'set' copy\n" +
					"skills between the project and local files, one file or a directory at a\n" +
					"time, and 'get <name> -' writes one to stdout.  'set' never removes a\n" +
					"skill; only 'delete' and 'restore' do.\n\n" +
					"'backup' saves every skill into a zip file, each at its path, and 'restore'\n" +
					"makes the project hold exactly what a zip file holds.  --dry-run says what\n" +
					"set, rename, delete, backup or restore would do, without doing it.\n\n" +
					"A skill's kinds are stored as its tags, taken from the 'kind:', 'kinds:' or\n" +
					"'tags:' line of its front matter.  To change them, edit that line and set\n" +
					"the file again.  A backup writes the kinds a skill is stored under into its\n" +
					"front matter when they aren't already there, so a restore keeps them.\n\n" +
					"To write a project's skills, point any AI agent at https://notehub.md and\n" +
					"ask it to train the project.",
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
