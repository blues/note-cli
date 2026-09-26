// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub skills': read and manage the skills a project holds, which are Markdown files
// stored in the project.  Every command works on the project directly, with no local state.

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

// Switches used only in this mode
var (
	flagSkillsDryRun bool
	flagSkillsForce  bool
)

// skillsSwitches returns the switches used only in this mode
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

	// With no command, show the whole set as one document
	args := flag.Args()
	if len(args) == 0 {
		return skillsShowCommand(config, []string{skillsAll})
	}

	// A skill's name where a command would go means show it, but a command word wins
	if !strings.EqualFold(args[0], modeHelp) && skillsCommandNamed(args[0]) == nil {
		args = append([]string{"show"}, args...)
	}

	return cliDispatch(cliModeNamed(modeSkills), config, args)

}

// skillsCommandNamed returns the named command, or nil
func skillsCommandNamed(name string) *cliCommand {
	commands := skillsCommands()
	for i := range commands {
		if strings.EqualFold(name, commands[i].Name) {
			return &commands[i]
		}
	}
	return nil
}

// skillsGetCommand saves a skill to a file, or every skill into a directory.  A local file
// that differs from the project's copy may hold unstored edits, so it is kept unless
// --force is given.
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

	// Where each skill goes: every skill into a directory, or one into a file, a directory
	// or stdout
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

	// Save them in reading order
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

// skillsSetCommand stores a file as a skill, or every skill in a directory.  Skills the
// project already holds unchanged are skipped, and nothing is ever removed.
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

	// The files to store, by skill name: a directory's by their paths within it, and a
	// file by its own name unless another is given
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

	// Check every name and kind before storing anything
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

	// Store each one that differs
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
		line := fmt.Sprintf("  %-9s %-*s  %s", state, width, name, skillsKindsLabel(kinds[name]))
		fmt.Println(strings.TrimRight(line, " "))
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

	// Summarize, naming any skills the directory left untouched
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

// skillsDeleteCommand removes a skill, or with 'all' and --force, every skill
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

	// Which skills; removing them all requires --force
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

	// Remove every upload stored under each name, superseded ones included
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

// skillsProject returns the project named by --project or --product
func skillsProject() (project string, err error) {
	if flagApp != "" {
		return flagApp, nil
	}
	if flagProduct != "" {
		return flagProduct, nil
	}
	return "", fmt.Errorf("specify the project with --project or --product")
}
