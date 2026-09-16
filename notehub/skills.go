// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The skill builder, which is everything done by 'notehub skills'.
//
// This mode has its own switches and its own commands, neither of which have
// anything to do with the switches used to interact with Notehub directly.  The
// switches that the mode does share with the default mode are -project and -product,
// which are marked as being available in this mode in switches.go.
//
// 'notehub skills' with no command performs this mode's own default action, which is
// to describe the skill builder in Markdown for an agent to read.  Help meant for a
// person is displayed by 'notehub skills -help', and never by the bare command.

package main

import (
	"flag"
	"fmt"

	"github.com/blues/note-cli/lib"
)

// The variables into which this mode's switches are parsed go here, alongside the
// switch definitions below

// skillsSwitches returns the switches that are specific to the skill builder.  These
// are defined here, rather than alongside the switches used to interact with Notehub,
// so that everything belonging to the skill builder stays in one place, but they are
// part of the same table and so they are registered, validated, and documented in
// exactly the same way.
func skillsSwitches() []*cliSwitch {
	return []*cliSwitch{
		// For example:
		// {Name: "name", Target: &flagSkillName, Group: "skills", Modes: []string{modeSkills},
		//     Usage: "name of the skill"},
	}
}

// skillsCommands returns the commands accepted by the skill builder
func skillsCommands() []cliCommand {
	return []cliCommand{
		// For example:
		// {Name: "list", Summary: "list the skills in the project", Run: skillsList},
	}
}

// runSkills is the handler for 'notehub skills'
func runSkills(config *lib.ConfigSettings) error {

	// With no command, perform this mode's default action rather than displaying
	// help, which is displayed only by -help
	args := flag.Args()
	if len(args) == 0 {
		return skillsDescribe(config)
	}

	// Run the specified command
	return cliDispatch(cliModeNamed(modeSkills), config, args)

}

// skillsDescribe is the default action of 'notehub skills', which describes the skill
// builder in Markdown for an agent to read
func skillsDescribe(config *lib.ConfigSettings) error {
	return fmt.Errorf("the skill builder is not yet implemented")
}
