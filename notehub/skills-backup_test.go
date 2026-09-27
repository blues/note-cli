// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// A backup writes the kinds a skill is stored under into its front matter, in place of
// the kind field it has or ahead of everything else, and leaves the rest of the file as
// it is, so that what a restore reads back is exactly those kinds
func TestSkillsWithKinds(t *testing.T) {
	tests := []struct {
		contents string
		kinds    string
		expected string
	}{
		// Front matter is added when there is none
		{"# T\n\nbody\n", "a,b", "---\nkind: a,b\n---\n\n# T\n\nbody\n"},
		{"", "a", "---\nkind: a\n---\n\n"},

		// The field comes first when the front matter lacks one
		{"---\ndescription: x\n---\n\n# T\n", "a,b", "---\nkind: a,b\ndescription: x\n---\n\n# T\n"},
		{"---\n---\n# T\n", "a", "---\nkind: a\n---\n# T\n"},

		// A kind field is replaced whatever its form, and whatever follows it stays
		{"---\nkind: old\ndescription: x\n---\n# T\n", "a,b", "---\nkind: a,b\ndescription: x\n---\n# T\n"},
		{"---\nKind: [old, older]\n---\n", "a", "---\nkind: a\n---\n"},
		{"---\ndescription: x\nkind:\n  - old\n  - older\nupdated: y\n---\n", "a,b", "---\ndescription: x\nkind: a,b\nupdated: y\n---\n"},
		{"---\nkind: old,\n  older\n---\n", "a", "---\nkind: a\n---\n"},
		{"---\nkind: old\n\n# about the next one\ndescription: x\n---\n", "a", "---\nkind: a\n\n# about the next one\ndescription: x\n---\n"},
		{"---\nkind: old\n---", "a", "---\nkind: a\n---"},

		// A field that kind takes precedence over is left as it is
		{"---\ntags: old\n---\n", "a", "---\nkind: a\ntags: old\n---\n"},

		// No kinds at all is an empty field
		{"---\nkind: old\n---\n", "", "---\nkind:\n---\n"},

		// The file's line ending and byte order mark are kept
		{"---\r\nkind: old\r\ndescription: x\r\n---\r\n# T\r\n", "a", "---\r\nkind: a\r\ndescription: x\r\n---\r\n# T\r\n"},
		{"# T\r\n", "a", "---\r\nkind: a\r\n---\r\n\r\n# T\r\n"},
		{"\ufeff---\nkind: old\n---\n", "a", "\ufeff---\nkind: a\n---\n"},
		{"\ufeff# T\n", "a", "\ufeff---\nkind: a\n---\n\n# T\n"},
	}
	for _, test := range tests {
		result := string(skillsWithKinds([]byte(test.contents), test.kinds))
		if result != test.expected {
			t.Errorf("%q with %q:\n     got %q\nexpected %q", test.contents, test.kinds, result, test.expected)
			continue
		}
		kinds, err := skillsKinds([]byte(result))
		if err != nil || kinds != test.kinds {
			t.Errorf("%q with %q: reads back as %q (%v)", test.contents, test.kinds, kinds, err)
		}
	}
}

// A backup's zip holds each skill at its path, and reading one back skips what isn't a
// skill, as reading a directory does
func TestSkillsZip(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "nested", "skills.zip")
	when := time.Date(2026, 9, 17, 14, 2, 0, 0, time.UTC)
	entries := []skillsZipEntry{
		{Name: "index.md", Contents: []byte("---\nkind: index\n---\n# I\n"), Modified: when},
		{Name: "references/a.md", Contents: []byte("a"), Modified: when},
		{Name: "notes.txt", Contents: []byte("not a skill"), Modified: when},
		{Name: ".hidden.md", Contents: []byte("hidden"), Modified: when},
		{Name: "__MACOSX/._index.md", Contents: []byte("hidden"), Modified: when},
		{Name: "./dotted.md", Contents: []byte("d"), Modified: when},
	}
	if err := skillsWriteZip(filename, entries, "a comment"); err != nil {
		t.Fatal(err)
	}
	skills, err := skillsReadZip(filename)
	if err != nil {
		t.Fatal(err)
	}
	if names := skillsSortedNames(skills); !slices.Equal(names, []string{"dotted.md", "index.md", "references/a.md"}) {
		t.Errorf("skills are %v, expected [dotted.md index.md references/a.md]", names)
	}
	if string(skills["index.md"]) != "---\nkind: index\n---\n# I\n" || string(skills["references/a.md"]) != "a" {
		t.Errorf("contents came back changed: %q", skills)
	}

	// Writing again replaces the file whole, and leaves no temporary file beside it
	if err := skillsWriteZip(filename, entries[:1], ""); err != nil {
		t.Fatal(err)
	}
	if skills, err = skillsReadZip(filename); err != nil || len(skills) != 1 {
		t.Errorf("the rewritten zip holds %v (%v), expected just index.md", skills, err)
	}
	if dir, _ := os.ReadDir(filepath.Dir(filename)); len(dir) != 1 {
		t.Errorf("the directory holds %d entries, expected just the zip", len(dir))
	}
}

// A backup is a zip file, and is named as one
func TestSkillsCheckZipName(t *testing.T) {
	for name, allowed := range map[string]bool{
		"skills.zip":   true,
		"a/b.ZIP":      true,
		"skills":       false,
		"skills.md":    false,
		"skills.zip.1": false,
		"":             false,
	} {
		if err := skillsCheckZipName(name); (err == nil) != allowed {
			t.Errorf("'%s': allowed is %v, expected %v (%v)", name, err == nil, allowed, err)
		}
	}
}
