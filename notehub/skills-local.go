// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills as local files, and the kinds a skill's front matter declares

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// skillsExt is what a skill is
const skillsExt = ".md"

// skillsCheckName refuses a name a skill can't be stored under: it must be a clean,
// relative, slash-separated path ending in .md
func skillsCheckName(name string) error {
	clean := path.Clean(name)
	switch {
	case !strings.HasSuffix(strings.ToLower(name), skillsExt):
		return fmt.Errorf("'%s' is not a skill's name, which ends in %s", name, skillsExt)
	case clean != name || path.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, `\`):
		return fmt.Errorf("'%s' is not a name a skill may have, which is a relative path written plainly", name)
	}
	return nil
}

// skillsSaveFile writes a skill to a local file and says what it did, keeping a file that
// differs unless force is set
func skillsSaveFile(filename string, contents []byte, force bool) (state string, err error) {
	existing, readErr := os.ReadFile(filename)
	switch {
	case readErr == nil && bytes.Equal(existing, contents):
		return "unchanged", nil
	case readErr == nil && !force:
		return "kept", nil
	case readErr != nil && !os.IsNotExist(readErr):
		return "", readErr
	}
	state = "new"
	if readErr == nil {
		state = "updated"
	}
	if err = os.MkdirAll(filepath.Dir(filename), 0777); err != nil {
		return "", err
	}
	return state, os.WriteFile(filename, contents, 0666)
}

// skillsReadDir returns the skills in a directory by relative slash-separated path,
// ignoring hidden files and anything that isn't a skill
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

// skillsPathWithin resolves a skill's name to a path within dir, refusing any that escape it
func skillsPathWithin(dir string, name string) (path string, err error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("'%s' is not a name a skill may have", name)
	}
	return filepath.Join(dir, clean), nil
}

// skillsSortedNames returns the skills' names, sorted
func skillsSortedNames(skills map[string][]byte) (names []string) {
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return
}

// skillsKindRE is what a kind must be: a single lowercase word
var skillsKindRE = regexp.MustCompile(`^[a-z0-9_-]+$`)

// skillsKinds returns a skill's kinds, which are stored as its upload's tags, from the
// "kind", "kinds" or "tags" field of its front matter.  Each of these means the same:
//
//	kind: notefiles,derivation
//	kind: [notefiles, derivation]
//	kind:
//	  - notefiles
//	  - derivation
//
// Anything else is refused rather than stored mangled.
func skillsKinds(contents []byte) (kinds string, err error) {

	// Tags are lowercase and comma-separated without spaces.  A later field is read only
	// if the earlier ones hold none.
	list := []string{}
	for _, field := range []string{"kind", "kinds", "tags"} {
		value, fieldErr := skillsFrontMatterValue(contents, field)
		if fieldErr != nil {
			return "", fieldErr
		}
		items := []any{value}
		if values, isList := value.([]any); isList {
			items = values
		}
		for _, item := range items {
			words, isString := item.(string)
			if item != nil && !isString {
				return "", fmt.Errorf("'%s' must be a word, a comma-separated line of words, or a list of them", field)
			}
			for _, kind := range strings.Split(words, ",") {
				kind = strings.ToLower(strings.TrimSpace(kind))
				switch {
				case kind == "":
					continue
				case kind == skillsReservedTag:
					return "", fmt.Errorf("'%s' is reserved and can't be used as a kind", skillsReservedTag)
				case !skillsKindRE.MatchString(kind):
					return "", fmt.Errorf("'%s' is not a kind a skill may have, which is one word of letters, digits, hyphens and underscores", kind)
				}
				list = append(list, kind)
			}
		}
		if len(list) != 0 {
			break
		}
	}

	return strings.Join(list, ","), nil

}

// skillsFrontMatterValue returns one top-level field of a skill's YAML front matter, or
// nil if there is none.  Only that field is parsed, so a mistake elsewhere in the front
// matter can't make it unreadable.
func skillsFrontMatterValue(contents []byte, field string) (value any, err error) {

	// The front matter, if there is any
	lines := strings.Split(strings.TrimPrefix(string(contents), "\ufeff"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	front := []string{}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			front = lines[1:i]
			break
		}
	}

	for i, line := range front {

		// A top-level field starts at the beginning of its line
		name, rest, isField := strings.Cut(line, ":")
		if !isField || line[0] == ' ' || line[0] == '\t' || !strings.EqualFold(strings.TrimSpace(name), field) {
			continue
		}
		name = strings.TrimSpace(name)

		// To YAML, "kind:x" is a word rather than a field
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			return nil, fmt.Errorf("'%s' in the front matter needs a space after its colon, as in '%s: %s'", name, name, rest)
		}

		// Its value runs on over indented, blank and comment lines, and a list flush with it
		end := i + 1
		for end < len(front) {
			next := front[end]
			if next != "" && next[0] != ' ' && next[0] != '\t' && next[0] != '#' &&
				next != "-" && !strings.HasPrefix(next, "- ") {
				break
			}
			end++
		}

		entry := map[string]any{}
		if err = yaml.Unmarshal([]byte(strings.Join(front[i:end], "\n")), &entry); err != nil {
			return nil, fmt.Errorf("'%s' in the front matter can't be read: %s", name, err)
		}
		return entry[name], nil

	}

	return nil, nil

}
