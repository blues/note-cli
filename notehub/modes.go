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

	// modeSignInAgent is --signin-agent without the hyphens
	modeSignInAgent = "signin-agent"

	// modeNetcat sends an HTTP request from stdin to the hub, like netcat for HTTPS
	modeNetcat = "netcat"

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
				Name:    modeSignInAgent,
				Summary: "sign in to Notehub from your agent",
				Detail: "'notehub signin-agent' writes the sign-in URL, progress, and result as\n" +
					"JSON to stdout when polling is enabled. With localhost authentication,\n" +
					"it opens your browser, the same as 'notehub signin'.",
				Run: runSignInAgent,
			},
			{
				Name:    modeNetcat,
				Summary: "send an HTTPS request from stdin to Notehub and response to stdout",
				Args:    "< request.http > response.http",
				Detail: "'notehub netcat' works like netcat for HTTPS: it reads a complete HTTP/1.1\n" +
					"request from stdin, sends it over TLS to the configured Notehub destination,\n" +
					"and writes the complete HTTP response, including its status line, headers,\n" +
					"and body, to stdout.  Diagnostics go to stderr.\n\n" +
					"TLS is always enabled, and the server's certificate is verified.  Notehub\n" +
					"credentials from --signin are used, with no Authorization header needed.\n\n" +
					"The exit code is 0 when the hub answered (whatever its status) and 1 when the\n" +
					"response is from the Notehub. The response status is 401 when you are not\n" +
					"signed in, 400 when the request can't be read, and 502 if hub is unreachable.",
				Run: runNetcat,
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
