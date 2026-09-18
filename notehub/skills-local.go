// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The local working copy of a project's skills.
//
// Teaching happens locally.  Every project being taught has a working copy in a known
// place, which is where the agent writes and where the person is free to edit, rename,
// delete, and experiment.  Nothing reaches the project until it is pushed, so a teaching
// session can be argued with, undone, and slept on.
//
// Alongside the working copy is a baseline, which is what the project held the last time
// it was pulled or pushed.  Comparing the two is what makes it possible to say, at any
// moment, exactly what would change in the project if it were pushed right now.

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
)

// The layout of the working copy
const (
	// skillsDirName is the directory, within the note tool's config directory, that
	// holds the working copy of every project being taught
	skillsDirName = "skills"

	// skillsBaselineDir holds what the project itself last held, so that the working
	// copy can be compared against it
	skillsBaselineDir = ".baseline"

	// skillsPulledFile records when the baseline was last known to match the project
	skillsPulledFile = ".pulled"

	// skillsProjectFile records the project's human-readable name, when it is known,
	// so that a backup can be named after it without going to the network
	skillsProjectFile = ".project"

	// skillsExt is what a skill is
	skillsExt = ".md"
)

// skillsChange is one difference between the working copy and the project
type skillsChange struct {
	Name  string
	State string // "new", "modified" or "deleted"
}

// skillsLocalDir returns the working copy for a project, creating it if it is new
func skillsLocalDir(project string) (dir string, err error) {
	dir = filepath.Join(lib.ConfigDir(), skillsDirName, skillsSafeName(project))
	err = os.MkdirAll(filepath.Join(dir, skillsBaselineDir), 0777)
	return
}

// skillsBaselinePath is where the baseline lives within a working copy
func skillsBaselinePath(dir string) string {
	return filepath.Join(dir, skillsBaselineDir)
}

// skillsSafeName makes a project identifier usable as a directory name, on Windows as
// well as everywhere else
func skillsSafeName(project string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:/\<>|?*"`, r) {
			return '-'
		}
		return r
	}, project)
}

// skillsReadDir returns the skills within a directory, by their slash-separated path
// relative to it, ignoring anything that isn't a skill so that the person may keep notes
// of their own alongside them
func skillsReadDir(dir string) (skills map[string][]byte, err error) {

	skills = map[string][]byte{}

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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), skillsExt) {
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
		skills[filepath.ToSlash(rel)] = contents
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return map[string][]byte{}, nil
	}

	return

}

// skillsWriteFile writes one skill into a directory, creating whatever the path needs
func skillsWriteFile(dir string, name string, contents []byte) error {
	path, err := skillsPathWithin(dir, name)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0666)
}

// skillsPathWithin resolves a skill's name to a path, and refuses any name that would
// land outside the directory it belongs in
func skillsPathWithin(dir string, name string) (path string, err error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("'%s' is not a name a skill may have", name)
	}
	return filepath.Join(dir, clean), nil
}

// skillsReplaceDir makes a directory hold exactly the given skills, and returns what it
// had to change to do so
func skillsReplaceDir(dir string, skills map[string][]byte) (changes []skillsChange, err error) {

	had, err := skillsReadDir(dir)
	if err != nil {
		return
	}

	for _, name := range skillsSortedNames(skills) {
		was, existed := had[name]
		if existed && bytes.Equal(was, skills[name]) {
			continue
		}
		if err = skillsWriteFile(dir, name, skills[name]); err != nil {
			return
		}
		if existed {
			changes = append(changes, skillsChange{name, "modified"})
		} else {
			changes = append(changes, skillsChange{name, "new"})
		}
	}

	for _, name := range skillsSortedNames(had) {
		if _, present := skills[name]; present {
			continue
		}
		path, pathErr := skillsPathWithin(dir, name)
		if pathErr != nil {
			return nil, pathErr
		}
		if err = os.Remove(path); err != nil {
			return
		}
		changes = append(changes, skillsChange{name, "deleted"})
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return

}

// skillsPending returns what would change in the project if the working copy were pushed
// right now, in the order in which it is displayed
func skillsPending(dir string) (changes []skillsChange, err error) {

	working, err := skillsReadDir(dir)
	if err != nil {
		return
	}
	baseline, err := skillsReadDir(filepath.Join(dir, skillsBaselineDir))
	if err != nil {
		return
	}

	for name, contents := range working {
		was, existed := baseline[name]
		switch {
		case !existed:
			changes = append(changes, skillsChange{name, "new"})
		case !bytes.Equal(contents, was):
			changes = append(changes, skillsChange{name, "modified"})
		}
	}
	for name := range baseline {
		if _, present := working[name]; !present {
			changes = append(changes, skillsChange{name, "deleted"})
		}
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return

}

// skillsBaselineNote reads one of the small records kept alongside the baseline
func skillsBaselineNote(dir string, name string) string {
	contents, err := os.ReadFile(filepath.Join(dir, skillsBaselineDir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}

// skillsProjectLabel is the project's human-readable name if we have learned it, and
// otherwise the identifier that was used to reach it
func skillsProjectLabel(dir string, project string) string {
	if label := skillsBaselineNote(dir, skillsProjectFile); label != "" {
		return label
	}
	return project
}

// skillsBackupPath is where a backup goes when the person doesn't say: onto the desktop,
// named for the project and the moment, so that a folder of them reads as a history
func skillsBackupPath(dir string, project string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	into := filepath.Join(home, "Desktop")
	if info, err := os.Stat(into); err != nil || !info.IsDir() {
		into = home
	}
	name := fmt.Sprintf("skills - %s - %s.zip",
		skillsSafeName(skillsProjectLabel(dir, project)), time.Now().Format("2006-01-02 1504"))
	return filepath.Join(into, name)
}

// skillsBackup writes the working copy to a zip file, and returns where it went.  The
// baseline is deliberately left out: a backup is of the teaching, not of the sync state.
func skillsBackup(dir string, filename string) (count int, err error) {

	skills, err := skillsReadDir(dir)
	if err != nil {
		return
	}

	file, err := os.Create(filename)
	if err != nil {
		return
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	for _, name := range skillsSortedNames(skills) {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.Modified = time.Now()
		if info, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); statErr == nil {
			header.Modified = info.ModTime()
		}
		var entry io.Writer
		entry, err = archive.CreateHeader(header)
		if err != nil {
			archive.Close()
			return
		}
		if _, err = entry.Write(skills[name]); err != nil {
			archive.Close()
			return
		}
		count++
	}

	err = archive.Close()
	return

}

// skillsRestore replaces the working copy with the contents of a backup, leaving the
// baseline alone so that what was restored can then be compared against the project
func skillsRestore(dir string, filename string) (restored []skillsChange, err error) {

	archive, err := zip.OpenReader(filename)
	if err != nil {
		return
	}
	defer archive.Close()

	from := map[string][]byte{}
	for _, entry := range archive.File {
		name := path.Clean(entry.Name)
		if entry.FileInfo().IsDir() || !strings.HasSuffix(name, skillsExt) {
			continue
		}
		if _, err = skillsPathWithin(dir, name); err != nil {
			return
		}
		var reader io.ReadCloser
		reader, err = entry.Open()
		if err != nil {
			return
		}
		contents, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		from[name] = contents
	}
	if len(from) == 0 {
		return nil, fmt.Errorf("%s contains no skills", filename)
	}

	return skillsReplaceDir(dir, from)

}

// skillsSortedNames returns the names within a set of skills, in order
func skillsSortedNames(skills map[string][]byte) (names []string) {
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return
}

// skillsKinds returns the kinds of knowledge a skill holds, taken from the "kind" field
// of its front matter and stored as the upload's tags.
//
// The kind is what lets an agent load only the knowledge a question needs, so a skill
// without one is still stored, but it is invisible to anything selecting by kind.
func skillsKinds(contents []byte) (kinds string, err error) {

	field := skillsFrontMatterField(contents, "kind")
	if field == "" {
		field = skillsFrontMatterField(contents, "kinds")
	}
	if field == "" {
		return "", nil
	}

	// Tags are comma-separated without whitespace, and lowercase so that a kind reads
	// the same way wherever it was written
	list := []string{}
	for _, kind := range strings.Split(field, ",") {
		kind = strings.ToLower(strings.TrimSpace(kind))
		if kind == "" {
			continue
		}
		if kind == skillsReservedTag {
			return "", fmt.Errorf("'%s' is reserved and can't be used as a kind", skillsReservedTag)
		}
		if strings.ContainsAny(kind, ` ,"'`) {
			return "", fmt.Errorf("'%s' is not a kind a skill may have", kind)
		}
		list = append(list, kind)
	}

	return strings.Join(list, ","), nil

}

// skillsFrontMatterField returns one field of a skill's YAML front matter, which is the
// block between a "---" on the very first line and the next "---"
func skillsFrontMatterField(contents []byte, field string) string {

	lines := strings.Split(string(contents), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), field) {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}

	return ""

}
