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

	// modeTrain runs a training session: it emits the protocol that turns an AI harness
	// into the trainer, and the harness does the rest
	modeTrain = "train"

	// modeSkills manages what a training run produced
	modeSkills = "skills"

	// modeHelp displays help.  Help is displayed with -help, and this keyword is a
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
				Name:    modeTrain,
				Summary: "train this project to understand its physical product",
				Detail: "'notehub train' emits the training protocol: the instructions that turn an\n" +
					"AI harness into the trainer.  Run it inside Claude, Codex or any other\n" +
					"competent harness and it will interview you about your product and write\n" +
					"down what it learns.  The output is Markdown for the harness to read, not\n" +
					"for you - what you will see is the conversation it starts.",
				Run: runTrain,
			},
			{
				Name:     modeSkills,
				Summary:  "inspect and manage what training produced",
				Args:     "[command]",
				Commands: skillsCommands(),
				Detail: "With no command, 'notehub skills' shows the working copy and what is\n" +
					"pending.  To run a training session, use 'notehub train'.",
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
