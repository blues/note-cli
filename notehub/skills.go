// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The skill builder, which is everything done by 'notehub skills'.
//
// A skill is a Markdown document that teaches an agent how to do one kind of job with
// Notehub: which interface to use, the vocabulary it expects, the procedure, and how
// to verify the result.  The documents live in the skills directory as ordinary
// Markdown files and are compiled into the binary, so a skill is written and reviewed
// as Markdown rather than as Go.
//
// 'notehub skills' emits the overview, which introduces Notehub's surface area and
// indexes the skills that are available.  'notehub skills <name>' emits one of them.
// Help meant for a person is displayed by 'notehub skills -help', and never by the
// bare command, because the bare command's output belongs to the agent that asked for
// it.
//
// This mode has its own switches and its own commands, neither of which have anything
// to do with the switches used to interact with Notehub directly.  The switches it
// does share with the default mode are -project and -product, which are marked as
// being available in this mode in switches.go.

package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The skills themselves.  To add one, add a Markdown file to the skills directory
// with front matter naming it and describing it, and it becomes a command of its own.
//
//go:embed skills/*.md
var skillsFS embed.FS

// skillsDir is where the skills live, and skillsOverview is the one of them that the
// bare 'notehub skills' emits rather than it being a command
const skillsDir = "skills"
const skillsOverview = "overview"

// skillsSkill is one Markdown document that teaches an agent how to do something
type skillsSkill struct {
	// Name is how the skill is named on the command line, from its filename
	Name string
	// Description is the one-line summary from the document's front matter
	Description string
	// Body is the document with its front matter removed
	Body string
}

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

// skillsCommands returns the commands accepted by the skill builder, which are the
// skills themselves
func skillsCommands() []cliCommand {
	commands := []cliCommand{}
	for _, skill := range skillsAll() {
		if skill.Name == skillsOverview {
			continue
		}
		name := skill.Name
		commands = append(commands, cliCommand{
			Name:    name,
			Summary: skill.Description,
			Run: func(config *lib.ConfigSettings, args []string) error {
				return skillsEmit(name)
			},
		})
	}
	return commands
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

// skillsDescribe is the default action of 'notehub skills', which emits the overview
// followed by an index of the skills that are available.  The index is generated
// rather than written down, so that it can't drift from what is actually installed.
func skillsDescribe(config *lib.ConfigSettings) error {

	if err := skillsEmit(skillsOverview); err != nil {
		return err
	}

	fmt.Printf("\n## Skills\n\n")
	for _, skill := range skillsAll() {
		if skill.Name == skillsOverview {
			continue
		}
		fmt.Printf("- `%s %s %s` - %s\n", cliName, modeSkills, skill.Name, skill.Description)
	}

	return nil

}

// skillsEmit displays the named skill
func skillsEmit(name string) error {
	for _, skill := range skillsAll() {
		if skill.Name == name {
			fmt.Printf("%s\n", strings.TrimRight(skill.Body, "\n"))
			return nil
		}
	}
	return fmt.Errorf("there is no '%s' skill", name)
}

// skillsAll returns every skill, in the order in which they are displayed
func skillsAll() []skillsSkill {

	if skillsTable != nil {
		return skillsTable
	}

	entries, err := fs.ReadDir(skillsFS, skillsDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		filename := entry.Name()
		if !strings.HasSuffix(filename, ".md") {
			continue
		}
		contents, err := fs.ReadFile(skillsFS, skillsDir+"/"+filename)
		if err != nil {
			continue
		}
		description, body := skillsParseFrontMatter(string(contents))
		skillsTable = append(skillsTable, skillsSkill{
			Name:        strings.TrimSuffix(filename, ".md"),
			Description: description,
			Body:        body,
		})
	}

	return skillsTable

}

var skillsTable []skillsSkill

// skillsParseFrontMatter separates a skill's front matter from its body, returning the
// description from the front matter along with the body.  The front matter is what
// makes a skill self-describing, and it is removed before the skill is emitted because
// it is metadata about the document rather than part of what it teaches.
func skillsParseFrontMatter(contents string) (description string, body string) {

	body = contents

	const fence = "---\n"
	if !strings.HasPrefix(body, fence) {
		return
	}
	end := strings.Index(body[len(fence):], "\n"+strings.TrimSuffix(fence, "\n"))
	if end < 0 {
		return
	}
	frontMatter := body[len(fence) : len(fence)+end]
	body = strings.TrimLeft(body[len(fence)+end+len(fence)+1:], "\n")

	for _, line := range strings.Split(frontMatter, "\n") {
		if field, value, found := strings.Cut(line, ":"); found && strings.TrimSpace(field) == "description" {
			description = strings.TrimSpace(value)
		}
	}

	return

}
