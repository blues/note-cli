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
			Usage: "say what set, rename, delete, backup or restore would do, without doing it"},
		{Name: "force", Target: &flagSkillsForce, Group: "skills", Modes: []string{modeSkills},
			Usage: "let get or backup overwrite a local file, rename replace a skill, and delete all remove every skill"},
	}
}

// skillsCommands returns the commands accepted by this mode
func skillsCommands() []cliCommand {
	return []cliCommand{
		{Name: "show", Args: "[name|all]", Summary: "write one skill, or the whole set as one linked document, to stdout",
			Run: skillsShowCommand},
		{Name: "list", Summary: "list the skills the project holds",
			Run: skillsListCommand},
		{Name: "get", Args: "<name|all> [path|-]", Summary: "save a skill to a file or stdout, or every skill into a directory",
			Run: skillsGetCommand},
		{Name: "set", Args: "<path> [name]", Summary: "store a file as a skill, or every skill in a directory",
			Run: skillsSetCommand},
		{Name: "rename", Args: "<name> <newname>", Summary: "store a skill under another name, and remove the old one",
			Run: skillsRenameCommand},
		{Name: "delete", Args: "<name|all>", Summary: "remove a skill, or every skill, from the project",
			Run: skillsDeleteCommand},
		{Name: "backup", Args: "<path.zip>", Summary: "save every skill, with its kinds, into a zip file",
			Run: skillsBackupCommand},
		{Name: "restore", Args: "<path.zip>", Summary: "make the project hold exactly the skills in a zip file",
			Run: skillsRestoreCommand},
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
		return fmt.Errorf("usage: %s %s get <name|all> [path|-]", cliName, modeSkills)
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
		if dir == "-" {
			return fmt.Errorf("stdout can take one skill, not all of them - name one, or give a directory")
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

	// What the project holds now
	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}

	// Store each one that differs
	changed, unchanged, err := skillsStore(files, uploads, skillsWidth(skillsSortedNames(files)))
	if err != nil {
		return err
	}
	skillsSummary("%d skill(s) %s in %s, and %d are unchanged.",
		changed, skillsVerb("stored", "would be stored"), project, unchanged)

	// Name any skills the directory left untouched
	if info.IsDir() {
		current, _ := skillsStorageCurrent(uploads)
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

// skillsStore stores files as skills, each under its name, and reports each as new,
// replaced or unchanged.  Every name and kind is checked before anything is sent, a skill
// the project already holds unchanged is skipped, and with --dry-run nothing is sent.
func skillsStore(files map[string][]byte, uploads []skillsUpload, width int) (changed int, unchanged int, err error) {

	// Check every name and kind before storing anything
	names := skillsSortedNames(files)
	kinds := map[string]string{}
	for _, name := range names {
		if err = skillsCheckName(name); err != nil {
			return
		}
		if kinds[name], err = skillsKinds(files[name]); err != nil {
			return 0, 0, fmt.Errorf("%s: %s", name, err)
		}
	}

	// Store each one that differs from what the project holds
	current, _ := skillsStorageCurrent(uploads)
	stored := skillsStorageNames(uploads)
	for _, name := range names {
		state := "new"
		if upload, have := current[name]; have {
			state = "replaced"
			held, readErr := skillsStorageRead(upload)
			if readErr != nil {
				return changed, unchanged, fmt.Errorf("%s: %s", name, readErr)
			}
			if bytes.Equal(held, files[name]) && upload.Tags == kinds[name] && len(stored[name]) == 1 {
				state = "unchanged"
			}
		}
		skillsReport(state, width, name, skillsKindsLabel(kinds[name]))
		if state == "unchanged" {
			unchanged++
			continue
		}
		changed++
		if flagSkillsDryRun {
			continue
		}
		if err = skillsStorageStore(name, files[name], kinds[name], stored[name]); err != nil {
			return changed, unchanged, fmt.Errorf("%s: %s", name, err)
		}
	}

	return

}

// skillsRenameCommand stores a skill under another name and then removes it from under
// the old one, in that order, so a failure leaves it in place under its old name
func skillsRenameCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: %s %s rename <name> <newname>", cliName, modeSkills)
	}

	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no skills", project)
	}
	stored := skillsStorageNames(uploads)

	// The skill, and the name it is to have, which ends in .md like any other
	upload, err := skillsResolve(current, args[0])
	if err != nil {
		return err
	}
	newName := args[1]
	if !strings.HasSuffix(strings.ToLower(newName), skillsExt) {
		newName += skillsExt
	}
	if err = skillsCheckName(newName); err != nil {
		return err
	}
	if newName == upload.Source {
		return fmt.Errorf("%s is already named that", upload.Source)
	}

	// A skill already under the new name, or under one differing from it only in case,
	// is replaced only with --force
	replacing := []string{}
	for source := range current {
		if source != upload.Source && strings.EqualFold(source, newName) {
			if !flagSkillsForce {
				return fmt.Errorf("%s already holds %s - add --force to replace it", project, source)
			}
			replacing = append(replacing, stored[source]...)
		}
	}

	contents, err := skillsStorageRead(upload)
	if err != nil {
		return err
	}
	if !flagSkillsDryRun {
		if err = skillsStorageStore(newName, contents, upload.Tags, replacing); err != nil {
			return fmt.Errorf("%s: %s", newName, err)
		}
		for _, name := range stored[upload.Source] {
			if err = skillsStorageRemove(name); err != nil {
				return fmt.Errorf("%s: %s", upload.Source, err)
			}
		}
	}
	fmt.Printf("  renamed   %s -> %s\n", upload.Source, newName)
	skillsSummary("%s %s %s in %s.", upload.Source, skillsVerb("is now", "would become"), newName, project)

	// The other skills may still refer to it by its old name
	mentions := []string{}
	for _, source := range skillsOrder(current, nil) {
		if source == upload.Source {
			continue
		}
		other, readErr := skillsStorageRead(current[source])
		if readErr == nil && bytes.Contains(bytes.ToLower(other), []byte(strings.ToLower(upload.Source))) {
			mentions = append(mentions, source)
		}
	}
	if len(mentions) != 0 {
		fmt.Printf("%d skill(s) still refer to %s, and may need editing: %s\n",
			len(mentions), upload.Source, strings.Join(mentions, ", "))
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

	// Which skills; removing them all requires --force, unless nothing is to be changed
	sources := []string{}
	if strings.EqualFold(args[0], skillsAll) {
		sources = skillsOrder(current, nil)
		if !flagSkillsForce && !flagSkillsDryRun {
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
		if !flagSkillsDryRun {
			for _, name := range stored[source] {
				if err = skillsStorageRemove(name); err != nil {
					return fmt.Errorf("%s: %s", source, err)
				}
			}
		}
		fmt.Printf("  removed   %s\n", source)
	}
	skillsSummary("%d skill(s) %s from %s.", len(sources), skillsVerb("removed", "would be removed"), project)
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

// skillsReport prints one line of a command's report: what became of one skill
func skillsReport(state string, width int, name string, note string) {
	line := fmt.Sprintf("  %-9s %-*s  %s", state, width, name, note)
	fmt.Println(strings.TrimRight(line, " "))
}

// skillsWidth is the width of the longest of the names, for aligning a report
func skillsWidth(names []string) (width int) {
	for _, name := range names {
		width = max(width, len(name))
	}
	return
}

// skillsVerb is the wording for what a command did, or in a dry run would have done
func skillsVerb(did string, would string) string {
	if flagSkillsDryRun {
		return would
	}
	return did
}

// skillsSummary prints a command's closing line, adding that nothing was changed when the
// run was a dry one
func skillsSummary(format string, args ...any) {
	fmt.Printf("\n"+format, args...)
	if flagSkillsDryRun {
		fmt.Print("  Nothing was changed (--dry-run).")
	}
	fmt.Println()
}
