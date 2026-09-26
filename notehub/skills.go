// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills, which is everything done by 'notehub skills': reading and managing the skills a
// project holds.
//
// A Notehub project knows the mechanics of the data flowing through it, but not what the
// product is, what its fields mean, or what anyone would want to ask about it.  Its skills
// are that knowledge, written down as a small set of Markdown files stored in the project
// itself, where any agent with read access can find it.  They are written by training the
// project, which an AI agent does by following the notehub-project-train skill at
// https://notehub.md, and this mode is how a person reads them, edits them, and moves them
// in and out of the project.
//
// Every command here talks to the project and keeps nothing between runs: a local file is
// only a copy that somebody is reading or editing.  'show' and 'list' read the project,
// 'get' copies skills out of it into files, 'set' stores files into it, and 'delete'
// removes skills from it.  'set' never removes anything - only 'delete' does - so a file
// missing from somebody's directory never takes a skill out of a project.
//
// 'notehub skills' with no command writes the project's whole skill set to stdout as one
// assembled document, because its output is meant for whoever or whatever asked to read
// the skills.  Help meant for a person is displayed by 'notehub skills --help', and never
// by the bare command.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The variables into which this mode's switches are parsed
var (
	flagSkillsDryRun bool
	flagSkillsForce  bool
)

// skillsSwitches returns the switches that are specific to this mode.  These are defined
// here, rather than alongside the switches used to interact with Notehub, so that
// everything belonging to this mode stays in one place, but they are part of the same
// table and so they are registered, validated, and documented in exactly the same way.
func skillsSwitches() []*cliSwitch {
	return []*cliSwitch{
		{Name: "dry-run", Target: &flagSkillsDryRun, Group: "skills", Modes: []string{modeSkills},
			Usage: "with set, say what would change in the project without changing it"},
		{Name: "force", Target: &flagSkillsForce, Group: "skills", Modes: []string{modeSkills},
			Usage: "with get, overwrite local files that differ; with delete all, remove them all"},
	}
}

// skillsCommands returns the commands accepted by this mode
func skillsCommands() []cliCommand {
	return []cliCommand{
		{Name: "show", Args: "[name|all]", Summary: "write one skill, or the whole set as one linked document, to stdout",
			Run: skillsShowCommand},
		{Name: "list", Summary: "list the skills the project holds",
			Run: skillsListCommand},
		{Name: "get", Args: "<name|all> [path]", Summary: "save a skill to a file, or every skill into a directory",
			Run: skillsGetCommand},
		{Name: "set", Args: "<path> [name]", Summary: "store a file as a skill, or every skill in a directory",
			Run: skillsSetCommand},
		{Name: "delete", Args: "<name|all>", Summary: "remove a skill, or every skill, from the project",
			Run: skillsDeleteCommand},
	}
}

// runSkills is the handler for 'notehub skills'
func runSkills(config *lib.ConfigSettings) error {

	// With no command, show what the project holds, assembled into one document.  That
	// is what somebody asking about a project's skills is asking for, and it is the one
	// output of this mode worth piping somewhere.
	args := flag.Args()
	if len(args) == 0 {
		return skillsShowCommand(config, []string{skillsAll})
	}

	// A skill's name where a command would go means to show it, because that is the only
	// thing it could mean and it is what everybody types.  A command word always wins:
	// a project whose skills collide with one is read with an explicit 'show'.
	if !strings.EqualFold(args[0], modeHelp) && skillsCommandNamed(args[0]) == nil {
		args = append([]string{"show"}, args...)
	}

	// Run the specified command
	return cliDispatch(cliModeNamed(modeSkills), config, args)

}

// skillsCommandNamed returns the named command of this mode, or nil if there is no such
// command
func skillsCommandNamed(name string) *cliCommand {
	commands := skillsCommands()
	for i := range commands {
		if strings.EqualFold(name, commands[i].Name) {
			return &commands[i]
		}
	}
	return nil
}

// skillsGetCommand saves one skill to a file, or every skill into a directory.  A local
// file that already holds something different is left alone unless --force says
// otherwise, because it may hold edits that nobody has stored yet.
func skillsGetCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s %s get <name|all> [path]", cliName, modeSkills)
	}

	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no skills yet", project)
	}

	// Which skills, and the file each one goes to: every skill into a directory, which is
	// the current one unless another is named, or one skill into a file, a directory, or
	// stdout
	targets := map[string]string{}
	if strings.EqualFold(args[0], skillsAll) {
		dir := "."
		if len(args) == 2 {
			dir = args[1]
		}
		for source := range current {
			if targets[source], err = skillsPathWithin(dir, source); err != nil {
				return err
			}
		}
	} else {
		upload, resolveErr := skillsResolve(current, args[0])
		if resolveErr != nil {
			return resolveErr
		}
		filename := ""
		switch {
		case len(args) == 1:
			filename, err = skillsPathWithin(".", upload.Source)
		case args[1] == "-":
			contents, readErr := skillsStorageRead(upload)
			if readErr != nil {
				return readErr
			}
			_, err = os.Stdout.Write(contents)
			return err
		default:
			filename = args[1]
			if info, statErr := os.Stat(filename); statErr == nil && info.IsDir() {
				filename, err = skillsPathWithin(filename, upload.Source)
			}
		}
		if err != nil {
			return err
		}
		targets[upload.Source] = filename
	}

	// Save them, in the order a reader would meet them
	kept := 0
	for _, source := range skillsOrder(current, nil) {
		filename, wanted := targets[source]
		if !wanted {
			continue
		}
		contents, readErr := skillsStorageRead(current[source])
		if readErr != nil {
			return fmt.Errorf("%s: %s", source, readErr)
		}
		state, saveErr := skillsSaveFile(filename, contents, flagSkillsForce)
		if saveErr != nil {
			return fmt.Errorf("%s: %s", source, saveErr)
		}
		if state == "kept" {
			kept++
		}
		label := source
		if filepath.ToSlash(filepath.Clean(filename)) != source {
			label += " -> " + filename
		}
		fmt.Printf("  %-9s %s\n", state, label)
	}
	if kept != 0 {
		return fmt.Errorf("%d local file(s) hold something different from the project's copy and were kept - use --force to overwrite them", kept)
	}
	return nil

}

// skillsSetCommand stores a file as a skill, or every skill in a directory.  A skill whose
// contents and kinds the project already holds is left as it is, so setting a whole
// directory stores only what changed; and set never removes anything, because a file
// missing from a directory is not a decision to remove a skill.
func skillsSetCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s %s set <path> [name]", cliName, modeSkills)
	}
	info, err := os.Stat(args[0])
	if err != nil {
		return err
	}

	// What to store, by the name each is stored under: a directory's skills are named by
	// their paths within it, and a file by its own name unless another is given
	files := map[string][]byte{}
	if info.IsDir() {
		if len(args) == 2 {
			return fmt.Errorf("the skills in a directory are named by their paths within it, so 'set <directory>' takes no name")
		}
		if files, err = skillsReadDir(args[0]); err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("%s holds no skills - a skill is a %s file", args[0], skillsExt)
		}
	} else {
		name := filepath.Base(args[0])
		if len(args) == 2 {
			name = args[1]
		}
		contents, readErr := os.ReadFile(args[0])
		if readErr != nil {
			return readErr
		}
		files[name] = contents
	}

	// Every name and every kind must be acceptable before anything is stored
	names := skillsSortedNames(files)
	kinds := map[string]string{}
	width := 0
	for _, name := range names {
		if err = skillsCheckName(name); err != nil {
			return err
		}
		if kinds[name], err = skillsKinds(files[name]); err != nil {
			return fmt.Errorf("%s: %s", name, err)
		}
		width = max(width, len(name))
	}

	// What the project holds now
	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	stored := skillsStorageNames(uploads)

	// Store each one that differs from what the project holds
	changed, unchanged := 0, 0
	for _, name := range names {
		state := "new"
		if upload, have := current[name]; have {
			state = "replaced"
			held, readErr := skillsStorageRead(upload)
			if readErr != nil {
				return fmt.Errorf("%s: %s", name, readErr)
			}
			if bytes.Equal(held, files[name]) && upload.Tags == kinds[name] && len(stored[name]) == 1 {
				state = "unchanged"
			}
		}
		fmt.Printf("  %-9s %-*s  %s\n", state, width, name, skillsKindsLabel(kinds[name]))
		if state == "unchanged" {
			unchanged++
			continue
		}
		changed++
		if flagSkillsDryRun {
			continue
		}
		if err = skillsStorageStore(name, files[name], kinds[name], stored[name]); err != nil {
			return fmt.Errorf("%s: %s", name, err)
		}
	}

	// What happened, and what set does not do
	if flagSkillsDryRun {
		fmt.Printf("\n%d skill(s) would be stored in %s, and %d are unchanged.  Nothing was changed (--dry-run).\n",
			changed, project, unchanged)
	} else {
		fmt.Printf("\n%d skill(s) stored in %s, and %d are unchanged.\n", changed, project, unchanged)
	}
	if info.IsDir() {
		others := []string{}
		for _, source := range skillsOrder(current, nil) {
			if _, present := files[source]; !present {
				others = append(others, source)
			}
		}
		if len(others) != 0 {
			fmt.Printf("%d skill(s) in the project are not in %s, and were left as they are: %s\n",
				len(others), args[0], strings.Join(others, ", "))
		}
	}
	return nil

}

// skillsDeleteCommand removes a skill from the project - every upload stored under its
// name - or, with 'all' and --force, every skill the project holds
func skillsDeleteCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s %s delete <name|all>", cliName, modeSkills)
	}

	uploads, err := skillsStorageQuery(false)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no skills", project)
	}
	stored := skillsStorageNames(uploads)

	// Which skills: every one, which is only done when asked for twice over, or one
	sources := []string{}
	if strings.EqualFold(args[0], skillsAll) {
		sources = skillsOrder(current, nil)
		if !flagSkillsForce {
			return fmt.Errorf("this would remove all %d skills from %s (%s) - add --force to remove them",
				len(sources), project, strings.Join(sources, ", "))
		}
	} else {
		upload, resolveErr := skillsResolve(current, args[0])
		if resolveErr != nil {
			return resolveErr
		}
		sources = append(sources, upload.Source)
	}

	// Remove every upload stored under each one's name
	for _, source := range sources {
		for _, name := range stored[source] {
			if err = skillsStorageRemove(name); err != nil {
				return fmt.Errorf("%s: %s", source, err)
			}
		}
		fmt.Printf("  removed   %s\n", source)
	}
	fmt.Printf("\n%d skill(s) removed from %s.\n", len(sources), project)
	return nil

}

// skillsProject returns the project whose skills these are, which every command needs
func skillsProject() (project string, err error) {
	if flagApp != "" {
		return flagApp, nil
	}
	if flagProduct != "" {
		return flagProduct, nil
	}
	return "", fmt.Errorf("specify the project with --project or --product")
}
