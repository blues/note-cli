// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub skills backup' and 'restore': the whole set as one zip file, holding each
// skill at its path.  The kinds a skill is stored under are its tags, which a zip can't
// hold, so a backup writes them into each skill's front matter when they aren't already
// there, and a restore takes them from it, as 'set' does.

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
)

// skillsZipExt is what a backup is
const skillsZipExt = ".zip"

// skillsZipEntry is one skill as it goes into, or comes out of, a zip file
type skillsZipEntry struct {
	Name     string
	Contents []byte
	Modified time.Time
}

// skillsBackupCommand saves every skill into a zip file, each at its path and with the
// kinds it is stored under in its front matter
func skillsBackupCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s %s backup <path%s>", cliName, modeSkills, skillsZipExt)
	}
	filename := args[0]
	if err = skillsCheckZipName(filename); err != nil {
		return err
	}
	if _, statErr := os.Stat(filename); statErr == nil && !flagSkillsForce {
		return fmt.Errorf("%s already exists - add --force to overwrite it", filename)
	}

	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no skills yet", project)
	}

	// Every skill, in reading order.  One whose front matter doesn't declare the kinds
	// it is stored under, or declares them unreadably, has them written into it.
	sources := skillsOrder(current, nil)
	width := skillsWidth(sources)
	entries := []skillsZipEntry{}
	tagged := 0
	for _, source := range sources {
		upload := current[source]
		contents, readErr := skillsStorageRead(upload)
		if readErr != nil {
			return fmt.Errorf("%s: %s", source, readErr)
		}
		state := "saved"
		if kinds, kindsErr := skillsKinds(contents); kindsErr != nil || (upload.Tags != "" && kinds != upload.Tags) {
			contents = skillsWithKinds(contents, upload.Tags)
			state = "tagged"
			tagged++
		}
		when := time.Now()
		if stored := skillsUploadWhen(upload); stored != 0 {
			when = time.Unix(stored, 0)
		}
		entries = append(entries, skillsZipEntry{Name: source, Contents: contents, Modified: when})
		skillsReport(state, width, source, skillsKindsLabel(upload.Tags))
	}

	if !flagSkillsDryRun {
		comment := fmt.Sprintf("skills of %s, saved by '%s %s backup' on %s",
			project, cliName, modeSkills, time.Now().UTC().Format("2006-01-02 15:04 UTC"))
		if err = skillsWriteZip(filename, entries, comment); err != nil {
			return err
		}
	}
	skillsSummary("%d skill(s) %s to %s from %s.", len(entries), skillsVerb("saved", "would be saved"), filename, project)
	if tagged != 0 {
		fmt.Printf("%d had the kinds the project stores them under written into their front matter, so a restore keeps them.\n", tagged)
	}
	return nil

}

// skillsRestoreCommand makes the project hold exactly the skills in a zip file: each is
// stored, with the kinds its front matter declares, and every other skill is removed.
// The storing comes before the removing, so a failure part-way loses nothing.
func skillsRestoreCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s %s restore <path%s>", cliName, modeSkills, skillsZipExt)
	}
	filename := args[0]
	if err = skillsCheckZipName(filename); err != nil {
		return err
	}
	files, err := skillsReadZip(filename)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s holds no skills - a skill is a %s file", filename, skillsExt)
	}

	// What the project holds now
	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	stored := skillsStorageNames(uploads)

	// Store what the zip holds, then remove what it doesn't
	width := skillsWidth(append(skillsSortedNames(files), skillsOrder(current, nil)...))
	changed, unchanged, err := skillsStore(files, uploads, width)
	if err != nil {
		return err
	}
	removed := 0
	for _, source := range skillsOrder(current, nil) {
		if _, present := files[source]; present {
			continue
		}
		if !flagSkillsDryRun {
			for _, name := range stored[source] {
				if err = skillsStorageRemove(name); err != nil {
					return fmt.Errorf("%s: %s", source, err)
				}
			}
		}
		skillsReport("removed", width, source, "")
		removed++
	}
	skillsSummary("%d skill(s) %s in %s from %s, %d are unchanged, and %d %s.",
		changed, skillsVerb("stored", "would be stored"), project, filename, unchanged,
		removed, skillsVerb("removed", "would be removed"))
	return nil

}

// skillsCheckZipName refuses a backup's name unless it ends in .zip
func skillsCheckZipName(filename string) error {
	if !strings.HasSuffix(strings.ToLower(filename), skillsZipExt) {
		return fmt.Errorf("'%s' is not a backup's name, which ends in %s", filename, skillsZipExt)
	}
	return nil
}

// skillsWriteZip writes skills into a zip file, each at its path.  It writes to a
// temporary file beside it and renames that into place, so a failure leaves no partial
// archive, and no earlier one is lost.
func skillsWriteZip(filename string, entries []skillsZipEntry, comment string) (err error) {

	dir := filepath.Dir(filename)
	if err = os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(filename)+".*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	w := zip.NewWriter(tmp)
	if comment != "" {
		if err = w.SetComment(comment); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		f, createErr := w.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Deflate, Modified: entry.Modified})
		if createErr != nil {
			return createErr
		}
		if _, err = f.Write(entry.Contents); err != nil {
			return err
		}
	}
	if err = w.Close(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filename)

}

// skillsReadZip returns the skills in a zip file by their paths within it, ignoring
// hidden files and anything that isn't a skill, as reading a directory does
func skillsReadZip(filename string) (skills map[string][]byte, err error) {

	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	skills = map[string][]byte{}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))
		if skillsHidden(name) || !strings.HasSuffix(strings.ToLower(name), skillsExt) {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return nil, fmt.Errorf("%s: %s", f.Name, openErr)
		}
		contents, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("%s: %s", f.Name, readErr)
		}
		skills[name] = contents
	}

	return skills, nil

}

// skillsHidden reports whether any part of a path begins with a dot, as a hidden file or
// directory does
func skillsHidden(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}
