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

// skillsWithKinds returns a skill declaring the given kinds in its front matter, as
// 'kind: a,b': in place of the kind field it has, or first in the front matter it has, or
// in front matter added for it.  Everything else in the file is left as it is, so a skill
// that comes back from a backup reads as it did, with its kinds.
func skillsWithKinds(contents []byte, kinds string) []byte {

	// The file's own byte order mark and line ending are kept
	text := string(contents)
	bom := ""
	if strings.HasPrefix(text, "\ufeff") {
		bom, text = "\ufeff", strings.TrimPrefix(text, "\ufeff")
	}
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	field := strings.TrimRight("kind: "+kinds, " ")

	// Without front matter, the field is all of it
	open, close := skillsFrontMatterSpan(text)
	if open < 0 {
		return []byte(bom + "---" + eol + field + eol + "---" + eol + eol + text)
	}

	// Within it, the field replaces the one there, or comes first
	front := []string{}
	if inner := strings.TrimSuffix(strings.TrimSuffix(text[open:close], "\n"), "\r"); inner != "" {
		front = strings.Split(inner, "\n")
		for i := range front {
			front[i] = strings.TrimRight(front[i], "\r")
		}
	}
	start, end := skillsFrontMatterField(front, "kind")
	if start < 0 {
		front = append([]string{field}, front...)
	} else {
		// The blank and comment lines after its value belong to whatever follows
		for end > start+1 {
			if last := strings.TrimSpace(front[end-1]); last != "" && !strings.HasPrefix(last, "#") {
				break
			}
			end--
		}
		front = append(append(append([]string{}, front[:start]...), field), front[end:]...)
	}

	return []byte(bom + text[:open] + strings.Join(front, eol) + eol + text[close:])

}

// skillsFrontMatterSpan returns where a skill's front matter lies within it: the offset
// just past its opening fence line, and the offset of its closing fence line, or -1 and -1
// if it has none
func skillsFrontMatterSpan(text string) (open int, close int) {
	lines := strings.SplitAfter(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return -1, -1
	}
	open = len(lines[0])
	close = open
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return open, close
		}
		close += len(line)
	}
	return -1, -1
}

// skillsFrontMatterValue returns one top-level field of a skill's YAML front matter, or
// nil if there is none.  Only that field is parsed, so a mistake elsewhere in the front
// matter can't make it unreadable.
func skillsFrontMatterValue(contents []byte, field string) (value any, err error) {

	front := skillsFrontMatterLines(contents)
	start, end := skillsFrontMatterField(front, field)
	if start < 0 {
		return nil, nil
	}

	// To YAML, "kind:x" is a word rather than a field
	name, rest, _ := strings.Cut(front[start], ":")
	name = strings.TrimSpace(name)
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return nil, fmt.Errorf("'%s' in the front matter needs a space after its colon, as in '%s: %s'", name, name, rest)
	}

	entry := map[string]any{}
	if err = yaml.Unmarshal([]byte(strings.Join(front[start:end], "\n")), &entry); err != nil {
		return nil, fmt.Errorf("'%s' in the front matter can't be read: %s", name, err)
	}
	return entry[name], nil

}

// skillsFrontMatterLines returns the lines of a skill's front matter, without its fences
// and line endings, or nil if it has none
func skillsFrontMatterLines(contents []byte) []string {
	lines := strings.Split(strings.TrimPrefix(string(contents), "\ufeff"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i]
		}
	}
	return nil
}

// skillsFrontMatterField finds a top-level field among the lines of a skill's front
// matter, returning the index of its line and of the line after its value, which runs on
// over indented, blank and comment lines and a list flush with it.  Both are -1 when the
// field isn't there.
func skillsFrontMatterField(front []string, field string) (start int, end int) {
	for i, line := range front {
		// A top-level field starts at the beginning of its line
		name, _, isField := strings.Cut(line, ":")
		if !isField || line[0] == ' ' || line[0] == '\t' || !strings.EqualFold(strings.TrimSpace(name), field) {
			continue
		}
		end = i + 1
		for end < len(front) {
			next := front[end]
			if next != "" && next[0] != ' ' && next[0] != '\t' && next[0] != '#' &&
				next != "-" && !strings.HasPrefix(next, "- ") {
				break
			}
			end++
		}
		return i, end
	}
	return -1, -1
}
