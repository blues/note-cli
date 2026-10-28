// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub files': the files a project holds, as uploads of a type, by default the user
// data type.  The mode works, and 'notehub files --help' describes it, but it isn't
// listed among the modes until the hub's documentation covers it.

package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// Switches used in this mode, and those shared with the firmware mode
var (
	flagFilesType        string
	flagUploadsTags      string
	flagUploadsNotes     string
	flagUploadsNoReplace bool
)

// The modes in which these switches may be used
var inFiles = []string{modeFiles}

// filesSwitches returns the switches of this mode
func filesSwitches() []*cli.Switch {
	return []*cli.Switch{
		{Name: "type", Target: &flagFilesType, Default: "data", Group: "files", Modes: inFiles, Hidden: true,
			Usage: "the upload `type` to manage: data, or another kind the hub stores"},
		{Name: "tags", Target: &flagUploadsTags, Group: "files", Modes: inFiles,
			Usage: "`tags` to store a file under, comma-separated"},
		{Name: "notes", Target: &flagUploadsNotes, Group: "files", Modes: inFiles,
			Usage: "`notes` to store with a file, or @file for a file's contents"},
		{Name: "no-replace", Target: &flagUploadsNoReplace, Group: "files", Modes: inFiles,
			Usage: "refuse to replace a file the project already holds under the same name"},
		{Name: "dry-run", Target: &flagSkillsDryRun, Group: "files", Modes: inFiles,
			Usage: "say what set or delete would do, without doing it"},
		{Name: "force", Target: &flagSkillsForce, Group: "files", Modes: inFiles,
			Usage: "let get overwrite a local file that differs, and delete all remove every file"},
		{Name: "json", Target: &flagJson, Group: "output", Modes: inFiles,
			Usage: "write the listing or metadata as JSON"},
	}
}

// filesExamples returns the examples shown in this mode's help
func filesExamples() []cli.Example {
	return []cli.Example{
		{What: "List the files the project holds", Command: cliName + " files --project <projectUID>"},
		{What: "Download one, verified against its md5", Command: cliName + " files get model.tflite --project <projectUID>"},
		{What: "Upload one, with notes, skipping it when the project already holds those bytes", Command: cliName + ` files set ./model.tflite --notes "trained 2026-10-01" --project <projectUID>`},
		{What: "Upload every file in a directory", Command: cliName + " files set ./assets --project <projectUID>"},
		{What: "Remove one", Command: cliName + " files delete model.tflite --project <projectUID>"},
	}
}

// filesCommands returns the commands accepted by this mode
func filesCommands() []cli.Command {
	return []cli.Command{
		{Name: "list", Summary: "list the files the project holds, by name, size, date, tags and notes",
			Run: filesListCommand},
		{Name: "show", Args: "<name>", Summary: "write one file's metadata, as the hub stores it",
			Run: filesShowCommand},
		{Name: "get", Args: "<name|all> [path|-]", Summary: "download a file to a file or stdout, or every file into a directory",
			Run: filesGetCommand},
		{Name: "set", Args: "<path> [name]", Summary: "upload a file, or every file in a directory, skipping those the project holds already",
			Run: filesSetCommand},
		{Name: "delete", Args: "<name|all>", Summary: "remove a file, or every file, from the project",
			Run: filesDeleteCommand},
	}
}

// runFiles is the handler for 'notehub files'.  With a project and no command it lists
// the project's files; with nothing at all, the help is shown.
func runFiles(config *lib.ConfigSettings) error {
	args := flag.Args()
	if len(args) == 0 {
		if flagApp == "" && flagProduct == "" {
			cli.PrintHelp(cli.ModeNamed(modeFiles), false)
			return nil
		}
		return cli.Dispatch(cli.ModeNamed(modeFiles), config, []string{"list"})
	}
	if !strings.EqualFold(args[0], modeHelp) && cli.CommandNamed(filesCommands(), args[0]) == nil {
		args = append([]string{"show"}, args...)
	}
	return cli.Dispatch(cli.ModeNamed(modeFiles), config, args)
}

// filesScope is the uploads this mode manages: the project's, of the type given
func filesScope() (uploadsScope, string, error) {
	project, err := skillsProject()
	if err != nil {
		return uploadsScope{}, "", err
	}
	return uploadsScope{Type: flagFilesType}, project, nil
}

// filesListCommand lists what the project holds
func filesListCommand(config *lib.ConfigSettings, args []string) error {
	scope, project, err := filesScope()
	if err != nil {
		return err
	}
	if len(args) != 0 {
		return fmt.Errorf("'%s files list' takes no arguments", cliName)
	}
	uploads, err := uploadsQuery(scope, false)
	if err != nil {
		return err
	}
	current, superseded := uploadsCurrent(uploads)
	sources := uploadsSources(current)
	if flagJson {
		items := []notehub.UploadMetadata{}
		for _, source := range sources {
			items = append(items, current[source])
		}
		return filesWriteJSON(items)
	}
	if len(current) == 0 {
		fmt.Printf("%s holds no %s files.\n", project, scope.Type)
		return nil
	}
	width := skillsWidth(sources)
	fmt.Printf("project: %s (%s files)\n\n", project, scope.Type)
	total := 0
	for _, source := range sources {
		upload := current[source]
		total += upload.Length
		line := fmt.Sprintf("  %*s%9d  %-16s %-12s %s", -(width + 2), source, upload.Length, filesWhen(upload), upload.Tags, uploadsFirstLine(upload.Notes))
		fmt.Println(strings.TrimRight(line, " "))
	}
	fmt.Printf("\n%d file(s), %d bytes.\n", len(sources), total)
	count := 0
	for _, names := range superseded {
		count += len(names)
	}
	if count != 0 {
		fmt.Printf("%d superseded upload(s) are also stored, and are not part of the set.\n", count)
	}
	return nil
}

// filesWhen is how a stored file's date is displayed in a listing
func filesWhen(upload notehub.UploadMetadata) string {
	when := uploadsWhen(upload)
	if when == 0 {
		return ""
	}
	return time.Unix(when, 0).Format("2006-01-02 15:04")
}

// filesWriteJSON writes something as JSON, indented with --pretty
func filesWriteJSON(value any) error {
	var out []byte
	var err error
	if flagPretty {
		out, err = note.JSONMarshalIndent(value, "", "    ")
	} else {
		out, err = note.JSONMarshal(value)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

// filesShowCommand writes one file's metadata
func filesShowCommand(config *lib.ConfigSettings, args []string) error {
	scope, project, err := filesScope()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s files show <name>", cliName)
	}
	uploads, err := uploadsQuery(scope, false)
	if err != nil {
		return err
	}
	current, _ := uploadsCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no %s files", project, scope.Type)
	}
	upload, err := uploadsResolve(current, args[0])
	if err != nil {
		return err
	}
	if flagJson {
		return filesWriteJSON(upload)
	}
	fmt.Printf("name:      %s\n", upload.Source)
	fmt.Printf("stored as: %s\n", upload.Name)
	fmt.Printf("type:      %s\n", upload.FileType)
	fmt.Printf("length:    %d\n", upload.Length)
	fmt.Printf("md5:       %s\n", upload.MD5)
	fmt.Printf("created:   %s\n", filesWhen(notehub.UploadMetadata{Created: upload.Created}))
	if upload.Modified != 0 && upload.Modified != upload.Created {
		fmt.Printf("modified:  %s\n", filesWhen(upload))
	}
	if upload.Tags != "" {
		fmt.Printf("tags:      %s\n", upload.Tags)
	}
	if upload.Contains != "" {
		fmt.Printf("contains:  %s\n", upload.Contains)
	}
	if len(upload.Info) != 0 {
		info, _ := note.JSONMarshal(upload.Info)
		fmt.Printf("info:      %s\n", info)
	}
	if upload.Notes != "" {
		fmt.Printf("notes:\n%s\n", upload.Notes)
	}
	return nil
}

// filesGetCommand downloads a file to a file or stdout, or every file into a directory.
// A local file that differs is kept unless --force is given, as with skills.
func filesGetCommand(config *lib.ConfigSettings, args []string) error {
	scope, project, err := filesScope()
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s files get <name|all> [path|-]", cliName)
	}
	uploads, err := uploadsQuery(scope, false)
	if err != nil {
		return err
	}
	current, _ := uploadsCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no %s files", project, scope.Type)
	}

	// Where each goes
	targets := map[string]string{}
	if strings.EqualFold(args[0], skillsAll) {
		dir := "."
		if len(args) == 2 {
			dir = args[1]
		}
		if dir == "-" {
			return fmt.Errorf("stdout can take one file, not all of them - name one, or give a directory")
		}
		for source := range current {
			if targets[source], err = skillsPathWithin(dir, source); err != nil {
				return err
			}
		}
	} else {
		upload, resolveErr := uploadsResolve(current, args[0])
		if resolveErr != nil {
			return resolveErr
		}
		filename := ""
		switch {
		case len(args) == 1:
			filename, err = skillsPathWithin(".", upload.Source)
		case args[1] == "-":
			contents, readErr := uploadsRead(scope, upload)
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

	kept := 0
	for _, source := range uploadsSources(current) {
		filename, wanted := targets[source]
		if !wanted {
			continue
		}
		contents, readErr := uploadsRead(scope, current[source])
		if readErr != nil {
			return readErr
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

// filesSetCommand uploads a file, or every file in a directory.  One the project already
// holds, byte for byte, is skipped; one it holds under the same name with other bytes is
// replaced, unless --no-replace refuses that.  Nothing is ever removed.
func filesSetCommand(config *lib.ConfigSettings, args []string) error {
	scope, project, err := filesScope()
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s files set <path> [name]", cliName)
	}
	info, err := os.Stat(args[0])
	if err != nil {
		return err
	}
	notes, err := uploadsNotes()
	if err != nil {
		return err
	}

	// The files to store, by name
	files := map[string][]byte{}
	if info.IsDir() {
		if len(args) == 2 {
			return fmt.Errorf("the files in a directory are named by their paths within it, so 'set <directory>' takes no name")
		}
		if files, err = filesReadDir(args[0]); err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("%s holds no files", args[0])
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
	names := skillsSortedNames(files)
	for _, name := range names {
		if err = skillsCheckPath(name); err != nil {
			return err
		}
	}

	// What the project holds now, and what becomes of each file
	uploads, err := uploadsQuery(scope, false)
	if err != nil {
		return err
	}
	current, _ := uploadsCurrent(uploads)
	stored := uploadsNames(uploads)
	width := skillsWidth(names)
	changed, unchanged := 0, 0
	for _, name := range names {
		state := "new"
		if upload, have := current[name]; have {
			state = "replaced"
			if strings.EqualFold(upload.MD5, uploadsMD5(files[name])) && len(stored[name]) == 1 {
				state = "unchanged"
			}
		}
		if state == "replaced" && flagUploadsNoReplace {
			return fmt.Errorf("%s already holds %s, with other bytes - without --no-replace it would be replaced", project, name)
		}
		skillsReport(state, width, name, fmt.Sprintf("%d bytes", len(files[name])))
		if state == "unchanged" {
			unchanged++
			continue
		}
		changed++
		if flagSkillsDryRun {
			continue
		}
		if _, err = uploadsStore(scope, name, files[name], flagUploadsTags, notes, stored[name]); err != nil {
			return fmt.Errorf("%s: %s", name, err)
		}
	}
	skillsSummary("%d file(s) %s in %s, and %d are unchanged.", changed, skillsVerb("stored", "would be stored"), project, unchanged)
	return nil
}

// filesReadDir returns the files in a directory by relative slash-separated path, ignoring
// hidden files and directories
func filesReadDir(dir string) (files map[string][]byte, err error) {
	files = map[string][]byte{}
	err = filepath.WalkDir(dir, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name != dir && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, name)
		if relErr != nil {
			return relErr
		}
		contents, readErr := os.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		files[filepath.ToSlash(rel)] = contents
		return nil
	})
	return
}

// skillsCheckPath refuses a name an upload can't be stored under: it must be a clean,
// relative, slash-separated path
func skillsCheckPath(name string) error {
	clean := filepath.ToSlash(filepath.Clean(name))
	if name == "" || clean != name || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, `\`) {
		return fmt.Errorf("'%s' is not a name a file may have, which is a relative path written plainly", name)
	}
	return nil
}

// filesDeleteCommand removes a file, or with 'all' and --force, every file
func filesDeleteCommand(config *lib.ConfigSettings, args []string) error {
	scope, project, err := filesScope()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s files delete <name|all>", cliName)
	}
	uploads, err := uploadsQuery(scope, false)
	if err != nil {
		return err
	}
	current, _ := uploadsCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no %s files", project, scope.Type)
	}
	stored := uploadsNames(uploads)
	sources := []string{}
	if strings.EqualFold(args[0], skillsAll) {
		sources = uploadsSources(current)
		if !flagSkillsForce && !flagSkillsDryRun {
			return fmt.Errorf("this would remove all %d files from %s (%s) - add --force to remove them",
				len(sources), project, strings.Join(sources, ", "))
		}
	} else {
		upload, resolveErr := uploadsResolve(current, args[0])
		if resolveErr != nil {
			return resolveErr
		}
		sources = append(sources, upload.Source)
	}
	for _, source := range sources {
		if !flagSkillsDryRun {
			for _, name := range stored[source] {
				if err = uploadsRemove(scope, name); err != nil {
					return fmt.Errorf("%s: %s", source, err)
				}
			}
		}
		fmt.Printf("  removed   %s\n", source)
	}
	skillsSummary("%d file(s) %s from %s.", len(sources), skillsVerb("removed", "would be removed"), project)
	return nil
}
