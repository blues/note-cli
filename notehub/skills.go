// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The teaching tool, which is everything done by 'notehub skills'.
//
// A Notehub project knows the mechanics of the data flowing through it, but not what the
// product is, what its fields mean, or what anyone would want to ask about it.  This mode
// exists so that a developer, working through an AI harness, can write that knowledge
// down as a small set of Markdown files stored in the project itself, where any agent
// with read access can later find it.
//
// The CLI is not the teacher.  The harness is.  'notehub skills' emits the protocol that
// turns a harness into the teacher, and the remaining commands move Markdown in and out
// of the project's Skills Storage.  Everything else a harness needs - listing projects,
// sampling events, reading schemas - it already does through the CLI's default mode or
// the HTTP API directly, so none of it is duplicated here.
//
// 'notehub skills' with no command emits the protocol, because its output is meant for
// the agent that asked for it.  Help meant for a person is displayed by
// 'notehub skills -help', and never by the bare command.

package main

import (
	"flag"
	"fmt"

	"github.com/blues/note-cli/lib"
)

// The variables into which this mode's switches are parsed go here, alongside the
// switch definitions below

// skillsSwitches returns the switches that are specific to the teaching tool.  These are
// defined here, rather than alongside the switches used to interact with Notehub, so that
// everything belonging to the teaching tool stays in one place, but they are part of the
// same table and so they are registered, validated, and documented in exactly the same
// way.
func skillsSwitches() []*cliSwitch {
	return []*cliSwitch{
		// For example:
		// {Name: "name", Target: &flagSkillName, Group: "skills", Modes: []string{modeSkills},
		//     Usage: "name of the skill"},
	}
}

// skillsCommands returns the commands accepted by the teaching tool.  They are built
// around the working copy: teaching happens there, and nothing reaches the project until
// it is pushed.
func skillsCommands() []cliCommand {
	return []cliCommand{
		{Name: "status", Summary: "show the working copy and what is pending",
			Run: skillsStatusCommand},
		{Name: "pull", Summary: "refresh the working copy from the project",
			Run: skillsPullCommand},
		{Name: "push", Summary: "upload new and changed skills to the project",
			Run: skillsPushCommand},
		{Name: "delete", Args: "<name>", Summary: "remove one skill from the project",
			Run: skillsDeleteCommand},
		{Name: "backup", Args: "[file]", Summary: "save the working copy as a zip file",
			Run: skillsBackupCommand},
		{Name: "restore", Args: "<file>", Summary: "replace the working copy from a zip file",
			Run: skillsRestoreCommand},
	}
}

// runSkills is the handler for 'notehub skills'
func runSkills(config *lib.ConfigSettings) error {

	// With no command, show where things stand.  Running a training session is
	// 'notehub train'; this mode is for inspecting and managing what it produced.
	args := flag.Args()
	if len(args) == 0 {
		return skillsStatusCommand(config, nil)
	}

	// Run the specified command
	return cliDispatch(cliModeNamed(modeSkills), config, args)

}

// skillsStatusCommand says where the working copy is and what would change in the project
// if it were pushed.  This is what the person is shown before anything is uploaded.
func skillsStatusCommand(config *lib.ConfigSettings, args []string) error {

	project, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}

	fmt.Printf("project:      %s\n", project)
	fmt.Printf("working copy: %s\n", dir)
	if pulled := skillsBaselineNote(dir, skillsPulledFile); pulled != "" {
		fmt.Printf("last synced:  %s\n", pulled)
	} else {
		fmt.Printf("last synced:  never\n")
	}

	changes, err := skillsPending(dir)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Printf("\nNo pending changes.\n")
		return nil
	}

	// What kind of knowledge each pending skill holds
	working, err := skillsReadDir(dir)
	if err != nil {
		return err
	}
	kinds := map[string]string{}
	problems := map[string]string{}
	width := 0
	for _, change := range changes {
		kind, kindErr := skillsKinds(working[change.Name])
		switch {
		case kindErr != nil:
			problems[change.Name] = kindErr.Error()
			continue
		case change.State == "deleted":
			kind = ""
		case kind == "":
			kind = "(no kind)"
		}
		kinds[change.Name] = kind
		if len(kind) > width {
			width = len(kind)
		}
	}

	fmt.Printf("\nPending changes:\n")
	for _, change := range changes {
		fmt.Printf("  %-9s %*s%s\n", change.State, -(width + 2), kinds[change.Name], change.Name)
	}

	// A kind that would be refused on upload is worth knowing about before pushing
	if len(problems) != 0 {
		fmt.Printf("\nProblems:\n")
		for _, change := range changes {
			if problem, found := problems[change.Name]; found {
				fmt.Printf("  %s: %s\n", change.Name, problem)
			}
		}
	}

	fmt.Printf("\nUpload the new and changed skills with '%s %s push'.\n", cliName, modeSkills)
	for _, change := range changes {
		if change.State == "deleted" {
			fmt.Printf("A skill deleted here stays in the project until it is removed with '%s %s delete <name>'.\n",
				cliName, modeSkills)
			break
		}
	}

	return nil

}

// skillsPullCommand refreshes the working copy from the project
func skillsPullCommand(config *lib.ConfigSettings, args []string) error {

	project, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}

	// Pulling over unpushed work would throw it away without asking
	changes, err := skillsPending(dir)
	if err != nil {
		return err
	}
	if len(changes) != 0 {
		return fmt.Errorf("the working copy has %d pending change(s), which pulling would overwrite - push them, or save them with '%s %s backup', and then pull",
			len(changes), cliName, modeSkills)
	}

	return skillsStoragePull(project, dir)

}

// skillsPushCommand uploads the new and changed skills to the project
func skillsPushCommand(config *lib.ConfigSettings, args []string) error {

	project, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}

	changes, err := skillsPending(dir)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Printf("No pending changes.\n")
		return nil
	}

	return skillsStoragePush(project, dir, changes)

}

// skillsDeleteCommand removes a skill from the project, which is kept separate from
// pushing so that a skill is never removed from a project merely because a file went
// missing from somebody's working copy
func skillsDeleteCommand(config *lib.ConfigSettings, args []string) error {

	project, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s %s delete <name>", cliName, modeSkills)
	}

	return skillsStorageDelete(project, dir, args[0])

}

// skillsBackupCommand saves the working copy as a zip file
func skillsBackupCommand(config *lib.ConfigSettings, args []string) error {

	project, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}

	filename := ""
	switch len(args) {
	case 0:
		filename = skillsBackupPath(dir, project)
	case 1:
		filename = args[0]
	default:
		return fmt.Errorf("usage: %s %s backup [file]", cliName, modeSkills)
	}

	count, err := skillsBackup(dir, filename)
	if err != nil {
		return err
	}
	fmt.Printf("%d skill(s) saved to %s\n", count, filename)
	return nil

}

// skillsRestoreCommand replaces the working copy with the contents of a backup
func skillsRestoreCommand(config *lib.ConfigSettings, args []string) error {

	_, dir, err := skillsWorkingCopy()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s %s restore <file>", cliName, modeSkills)
	}

	restored, err := skillsRestore(dir, args[0])
	if err != nil {
		return err
	}
	if len(restored) == 0 {
		fmt.Printf("The working copy already matches %s.\n", args[0])
		return nil
	}

	fmt.Printf("Restored into %s:\n", dir)
	for _, change := range restored {
		fmt.Printf("  %-9s %s\n", change.State, change.Name)
	}
	fmt.Printf("\nNothing has reached the project yet - see '%s %s status'.\n", cliName, modeSkills)
	return nil

}

// skillsWorkingCopy returns the project being taught along with its working copy
func skillsWorkingCopy() (project string, dir string, err error) {
	project, err = skillsProject()
	if err != nil {
		return
	}
	dir, err = skillsLocalDir(project)
	return
}

// skillsProject returns the project being taught, which every command needs
func skillsProject() (project string, err error) {
	if flagApp != "" {
		return flagApp, nil
	}
	if flagProduct != "" {
		return flagProduct, nil
	}
	return "", fmt.Errorf("specify the project being taught with -project or -product")
}
