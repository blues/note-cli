// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Every command line that predates modes must parse as the default mode, unchanged
func TestExtractMode(t *testing.T) {
	tests := []struct {
		args      []string
		mode      string
		remaining []string
	}{
		// Command lines that predate modes
		{[]string{}, modeDefault, []string{}},
		{[]string{"-version"}, modeDefault, []string{"-version"}},
		{[]string{"-product", "net.ozzie.ray:t", "-explore"}, modeDefault, []string{"-product", "net.ozzie.ray:t", "-explore"}},
		{[]string{`{"req":"hub.app.get"}`}, modeDefault, []string{`{"req":"hub.app.get"}`}},
		{[]string{"@request.json"}, modeDefault, []string{"@request.json"}},

		// Naming the default mode is the same as not naming a mode at all
		{[]string{"default"}, modeDefault, []string{}},
		{[]string{"default", "-version"}, modeDefault, []string{"-version"}},
		{[]string{"default", "--version"}, modeDefault, []string{"--version"}},

		// Mode keywords
		{[]string{"signin"}, modeSignIn, []string{}},
		{[]string{"SignIn", "--hub", "api.notefile.net"}, modeSignIn, []string{"--hub", "api.notefile.net"}},
		{[]string{"netcat"}, modeNetcat, []string{}},
		{[]string{"NetCat", "--hub", "api.notefile.net"}, modeNetcat, []string{"--hub", "api.notefile.net"}},
		{[]string{"skills"}, modeSkills, []string{}},
		{[]string{"SKILLS", "-project", "app:1"}, modeSkills, []string{"-project", "app:1"}},
		{[]string{"skills", "--project=app:1"}, modeSkills, []string{"--project=app:1"}},
		{[]string{"help", "skills"}, modeHelp, []string{"skills"}},

		// A mode typed as a switch
		{[]string{"-skills"}, modeSkills, []string{}},
		{[]string{"--skills"}, modeSkills, []string{}},
		{[]string{"--SKILLS"}, modeSkills, []string{}},
		{[]string{"--skills", "get", "all", "-project", "app:1"}, modeSkills, []string{"get", "all", "-project", "app:1"}},

		// A switch is always a switch, even when a mode has the same name
		{[]string{"--help"}, modeDefault, []string{"--help"}},
		{[]string{"-help"}, modeDefault, []string{"-help"}},
		{[]string{"--signin"}, modeDefault, []string{"--signin"}},
		{[]string{"-signin"}, modeDefault, []string{"-signin"}},
		{[]string{"--verbose"}, modeDefault, []string{"--verbose"}},

		// A hyphenated word that isn't a mode is left for the flag package to diagnose
		{[]string{"--notamode"}, modeDefault, []string{"--notamode"}},
		{[]string{"--skills=yes"}, modeDefault, []string{"--skills=yes"}},

		// A mode keyword is recognized only in the first position
		{[]string{"-pretty", "skills"}, modeDefault, []string{"-pretty", "skills"}},
		{[]string{"-pretty", "--skills"}, modeDefault, []string{"-pretty", "--skills"}},

		// A word that isn't a mode is left alone for the mode to interpret
		{[]string{"notamode"}, modeDefault, []string{"notamode"}},
	}
	for _, test := range tests {
		mode, remaining := cliExtractMode(test.args)
		if mode.Name != test.mode {
			t.Errorf("%v: mode is '%s', expected '%s'", test.args, mode.Name, test.mode)
		}
		if !slices.Equal(remaining, test.remaining) {
			t.Errorf("%v: remaining args are %v, expected %v", test.args, remaining, test.remaining)
		}
	}
}

// Switches must be found the way the flag package finds them
func TestScanSwitchNames(t *testing.T) {
	tests := []struct {
		args  []string
		names []string
	}{
		{[]string{"-project", "app:1", "-verbose", "-upload", "f.bin"}, []string{"project", "verbose", "upload"}},
		{[]string{"-project=app:1", "-pretty"}, []string{"project", "pretty"}},
		{[]string{"--project", "app:1", "--pretty"}, []string{"project", "pretty"}},
		{[]string{"--project=app:1", "-pretty", "--verbose=false"}, []string{"project", "pretty", "verbose"}},
		{[]string{"--project", "-pretty", "--verbose"}, []string{"project", "verbose"}},
		{[]string{"-pretty", `{"req":"hub.app.get"}`}, []string{"pretty"}},
		{[]string{`{"req":"hub.app.get"}`, "-pretty"}, nil},
		{[]string{"--", "-pretty"}, nil},
		{[]string{}, nil},
	}
	for _, test := range tests {
		names := cliScanSwitchNames(test.args)
		if !slices.Equal(names, test.names) {
			t.Errorf("%v: switches are %v, expected %v", test.args, names, test.names)
		}
	}
}

// A switch is accepted only in the modes in which it is defined to be available
func TestValidateSwitches(t *testing.T) {
	tests := []struct {
		mode     string
		args     []string
		accepted bool
	}{
		{modeDefault, []string{"-upload", "f.bin"}, true},
		{modeDefault, []string{"-project", "app:1", "-pretty"}, true},
		{modeSkills, []string{"-project", "app:1"}, true},
		{modeSkills, []string{"-product", "net.ozzie.ray:t"}, true},
		{modeSkills, []string{"-hub", "api.notefile.net"}, true},
		{modeSkills, []string{"-upload", "f.bin"}, false},
		{modeSkills, []string{"-project", "app:1", "-verbose"}, true},
		{modeSkills, []string{"-explore"}, false},
		{modeSkills, []string{"-scope", "dev:1"}, false},
		{modeSkills, []string{"--project=app:1", "-verbose"}, true},
		{modeSkills, []string{"--hub", "api.notefile.net"}, true},
		{modeSkills, []string{"--upload=f.bin"}, false},
		{modeSkills, []string{"--explore"}, false},
		{modeSignIn, []string{"--hub", "api.notefile.net"}, true},
		{modeSignIn, []string{"--signin"}, false},
		{modeSignIn, []string{"--signin-token", "pat"}, false},
		{modeSignIn, []string{"--project", "app:1"}, false},
		{modeNetcat, []string{"--hub", "api.notefile.net"}, true},
		{modeNetcat, []string{"--signin"}, false},
		{modeNetcat, []string{"--project", "app:1"}, false},
		{modeNetcat, []string{"--verbose"}, false},

		// The switches of the skills mode belong to it alone
		{modeSkills, []string{"--force"}, true},
		{modeSkills, []string{"--dry-run", "--project", "app:1"}, true},
		{modeDefault, []string{"--force"}, false},
		{modeDefault, []string{"--dry-run"}, false},

		// The general options are available no matter what mode is being run
		{modeSkills, []string{"-help"}, true},
		{modeDefault, []string{"-help"}, true},

		// An unrecognized switch is left for the flag package to report
		{modeSkills, []string{"-nosuchswitch"}, true},
	}
	for _, test := range tests {
		err := cliValidateSwitches(cliModeNamed(test.mode), test.args)
		if test.accepted && err != nil {
			t.Errorf("%s %v: unexpectedly rejected: %s", test.mode, test.args, err)
		}
		if !test.accepted && err == nil {
			t.Errorf("%s %v: unexpectedly accepted", test.mode, test.args)
		}
	}
}

// Both spellings must produce the same values and preserve the default mode's
// argument boundaries.  In particular, values are never rewritten as switches.
func TestParseSwitchSpellings(t *testing.T) {
	tests := []struct {
		args       []string
		project    string
		pretty     bool
		positional []string
	}{
		{[]string{"-project", "app:1", "-pretty"}, "app:1", true, nil},
		{[]string{"--project", "app:1", "--pretty"}, "app:1", true, nil},
		{[]string{"-project=app:1", "-pretty=false"}, "app:1", false, nil},
		{[]string{"--project=app:1", "--pretty=false"}, "app:1", false, nil},
		{[]string{"-project", "app:1", "--pretty"}, "app:1", true, nil},
		{[]string{"--project=app:1", "-project", "app:2", "--pretty", "-pretty=false"}, "app:2", false, nil},
		{[]string{"--project", "--pretty"}, "--pretty", false, nil},
		{[]string{"--project", "--"}, "--", false, nil},
		{[]string{"--pretty", "--", "-project", "app:1"}, "", true, []string{"-project", "app:1"}},
		{[]string{"@request.json", "--pretty"}, "", false, []string{"@request.json", "--pretty"}},
	}
	saved := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = saved })
	for _, test := range tests {
		flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)
		cliRegisterSwitches(cliModeNamed(modeDefault))
		if err := flag.CommandLine.Parse(test.args); err != nil {
			t.Errorf("%v: %s", test.args, err)
			continue
		}
		if flagApp != test.project || flagPretty != test.pretty || !slices.Equal(flag.Args(), test.positional) {
			t.Errorf("%v: got project=%q pretty=%v args=%v; expected project=%q pretty=%v args=%v",
				test.args, flagApp, flagPretty, flag.Args(), test.project, test.pretty, test.positional)
		}
	}
	flagApp = ""
	flagPretty = false
}

// A command's switches are accepted anywhere among its arguments.  A value that isn't a
// projectUID lands in --product, as cliNormalizeScope arranges.
func TestParseCommandSwitches(t *testing.T) {
	tests := []struct {
		args       []string
		positional []string
		project    string
		product    string
	}{
		{[]string{"-project", "app:1"}, nil, "app:1", ""},
		{[]string{"notefiles", "-project", "app:1"}, []string{"notefiles"}, "app:1", ""},
		{[]string{"-project", "app:1", "notefiles"}, []string{"notefiles"}, "app:1", ""},
		{[]string{"./dir", "-project=app:1"}, []string{"./dir"}, "app:1", ""},
		{[]string{"a", "-project", "app:1", "b"}, []string{"a", "b"}, "app:1", ""},
		{[]string{"--project", "app:1", "notefiles"}, []string{"notefiles"}, "app:1", ""},
		{[]string{"./dir", "--project=app:1"}, []string{"./dir"}, "app:1", ""},
		{[]string{"a", "--project", "app:1", "b"}, []string{"a", "b"}, "app:1", ""},
		{[]string{"-project", "app:1", "./dir", "--project=app:2"}, []string{"./dir"}, "app:2", ""},
		{[]string{"--project", "--verbose"}, nil, "", "--verbose"},
		{[]string{"--project", "--"}, nil, "", "--"},
		{[]string{"--", "-notaswitch"}, []string{"-notaswitch"}, "", ""},
		{[]string{"--project=app:1", "--", "--project=app:2"}, []string{"--project=app:2"}, "app:1", ""},
		{[]string{"--project", "net.ozzie.ray:t"}, nil, "", "net.ozzie.ray:t"},
		{[]string{"--product", "app:1"}, nil, "app:1", ""},
	}
	// Use a command line of our own; the test binary owns the real one
	saved := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	cliRegisterSwitches(cliModeNamed(modeSkills))
	t.Cleanup(func() { flag.CommandLine = saved })

	for _, test := range tests {
		flagApp, flagProduct = "", ""
		positional, err := cliParseCommandSwitches(cliModeNamed(modeSkills), test.args)
		if err != nil {
			t.Errorf("%v: %s", test.args, err)
			continue
		}
		if !slices.Equal(positional, test.positional) {
			t.Errorf("%v: positional args are %v, expected %v", test.args, positional, test.positional)
		}
		if flagApp != test.project || flagProduct != test.product {
			t.Errorf("%v: -project is %q and -product is %q, expected %q and %q", test.args, flagApp, flagProduct, test.project, test.product)
		}
	}
	flagApp, flagProduct = "", ""
}

// A bare word is a mistyped or misplaced mode keyword rather than a request
func TestIsBareWord(t *testing.T) {
	tests := []struct {
		arg  string
		bare bool
	}{
		{"skills", true},
		{"set-vars", true},
		{"a_1", true},
		{"", false},
		{`{"req":"hub.app.get"}`, false},
		{"@request.json", false},
		{"net.ozzie.ray:t", false},
		{"two words", false},
	}
	for _, test := range tests {
		if cliIsBareWord(test.arg) != test.bare {
			t.Errorf("'%s': expected bare word to be %v", test.arg, test.bare)
		}
	}
}

// Every switch in the table must be one the framework can register, validate and display
func TestSwitchDefinitions(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range cliSwitches() {
		if s.Name == "" {
			t.Errorf("a switch is defined with no name")
			continue
		}
		if seen[s.Name] {
			t.Errorf("-%s is defined more than once", s.Name)
		}
		seen[s.Name] = true
		if len(s.Modes) == 0 {
			t.Errorf("-%s is not available in any mode", s.Name)
		}
		for _, m := range s.Modes {
			if m != cliAnyMode && cliModeNamed(m) == nil {
				t.Errorf("-%s is available in '%s', which is not a mode", s.Name, m)
			}
		}
		if !slices.ContainsFunc(cliSwitchGroups(), func(g struct{ Name, Description string }) bool {
			return g.Name == s.Group
		}) {
			t.Errorf("-%s is in group '%s', which is not a group", s.Name, s.Group)
		}
		switch s.Target.(type) {
		case *bool, *string, *int:
			if s.External {
				t.Errorf("-%s is registered elsewhere and so must not have a target", s.Name)
			}
		case nil:
			if !s.External {
				t.Errorf("-%s has no target and so must be registered elsewhere", s.Name)
			}
		default:
			t.Errorf("-%s has an unsupported target type %T", s.Name, s.Target)
		}
	}
}

// The general options, which short help shows, must be available in every mode
func TestGeneralOptions(t *testing.T) {
	general := []string{}
	for _, s := range cliSwitches() {
		if s.Group == cliGroupGeneral {
			general = append(general, s.Name)
			for _, mode := range cliModes() {
				if !s.allowedIn(mode.Name) {
					t.Errorf("-%s is a general option and so must be available in every mode, including '%s'", s.Name, mode.Name)
				}
			}
		}
	}
	if !slices.Equal(general, []string{"help", "hub"}) {
		t.Errorf("the general options are %v, expected [help hub]", general)
	}
}

// A hidden mode still runs, but it isn't one of the modes that we describe
func TestHiddenModes(t *testing.T) {
	mode, remaining := cliExtractMode([]string{modeHelp, modeSkills})
	if mode.Name != modeHelp || !slices.Equal(remaining, []string{modeSkills}) {
		t.Errorf("'%s' is hidden and so must still be recognized as a mode", modeHelp)
	}
	if slices.Contains(cliModeNames(), modeHelp) {
		t.Errorf("'%s' is hidden and so must not be listed as one of the modes", modeHelp)
	}
	for _, m := range cliVisibleModes() {
		if m.Hidden {
			t.Errorf("'%s' is hidden and so must not be listed as one of the modes", m.Name)
		}
	}
}

// Every mode must be able to run, and the modes that the default mode's help
// describes must all exist
func TestModeDefinitions(t *testing.T) {
	seen := map[string]bool{}
	for _, mode := range cliModes() {
		if mode.Name == "" {
			t.Errorf("a mode is defined with no name")
			continue
		}
		if seen[mode.Name] {
			t.Errorf("'%s' is defined more than once", mode.Name)
		}
		seen[mode.Name] = true
		if mode.Summary == "" {
			t.Errorf("'%s' has no summary", mode.Name)
		}
		if mode.Run == nil {
			t.Errorf("'%s' has no handler", mode.Name)
		}
		for _, c := range mode.Commands {
			if c.Name == "" || c.Run == nil {
				t.Errorf("'%s' has a command with no name or no handler", mode.Name)
			}
		}
	}
	if !seen[modeDefault] {
		t.Errorf("there is no '%s' mode", modeDefault)
	}
}

// A skill's kinds become its upload's tags: comma-separated, without spaces, and never
// the reserved one
func TestSkillsKinds(t *testing.T) {
	tests := []struct {
		contents string
		kinds    string
		refused  bool
	}{
		{"---\nkind: glossary\n---\n\n# G\n", "glossary", false},
		{"---\nkind: schema, execution\n---\n\n# S\n", "schema,execution", false},
		{"---\nkinds: Schema,GLOSSARY\n---\n\n# S\n", "schema,glossary", false},

		// 'tags' is a synonym, and 'kind' wins when both are there
		{"---\ntags: glossary, schema\n---\n", "glossary,schema", false},
		{"---\ntags: [glossary, schema]\n---\n", "glossary,schema", false},
		{"---\nkind: glossary\ntags: other\n---\n", "glossary", false},
		{"---\ntags: publish\n---\n", "", true},
		{"---\ndescription: no kind here\n---\n\n# D\n", "", false},
		{"# no front matter at all\n", "", false},
		{"---\nkind: \"quoted\"\n---\n\n# Q\n", "quoted", false},
		{"---\nkind: glossary\n---\nkind: notthisone\n", "glossary", false},

		// Either form of YAML list
		{"---\nkind: [schema, glossary]\n---\n", "schema,glossary", false},
		{"---\nkind: [\"schema\", 'glossary']\n---\n", "schema,glossary", false},
		{"---\nkind:\n  - schema\n  - glossary\ndescription: after the list\n---\n", "schema,glossary", false},
		{"---\nkind:\n- schema\n- glossary\n---\n", "schema,glossary", false},

		// A line of kinds wrapped onto the next, a comment, and a line ending in CRLF
		{"---\nkind: product,usage,mission,\n  constraints\n---\n", "product,usage,mission,constraints", false},
		{"---\nkind: schema # what the fields are\n---\n", "schema", false},
		{"---\r\nkind: schema\r\n---\r\n", "schema", false},

		// Only the top-level field is read, so a slip or blank line elsewhere doesn't matter
		{"---\nmetadata:\n  kind: nested\ndescription: x\n---\n", "", false},
		{"---\nmetadata:\n  kind: nested\nkind: schema\n---\n", "schema", false},
		{"---\ndescription: Use when: writing notes\nkind: schema\n---\n", "schema", false},
		{"---\n\nkind: schema\n\n---\n", "schema", false},

		// An empty kind is no kind, and front matter that never closes is not front matter
		{"---\nkind:\ndescription: x\n---\n", "", false},
		{"---\nkind:\nkinds: glossary\n---\n", "glossary", false},
		{"---\nkind: glossary\n\n# never closed\n", "", false},

		// The service reserves this one, and a tag may not carry whitespace
		{"---\nkind: publish\n---\n\n# P\n", "", true},
		{"---\nkind: schema,publish\n---\n\n# P\n", "", true},
		{"---\nkind:\n  - schema\n  - publish\n---\n", "", true},
		{"---\nkind: two words\n---\n\n# W\n", "", true},

		// Anything that is not a word, or a list of them, is refused rather than stored
		{"---\nkind: [schema, glossary\n---\n", "", true},
		{"---\nkind: schema:v2\n---\n", "", true},
		{"---\nkind:schema\n---\n", "", true},
		{"---\nkind: 42\n---\n", "", true},
		{"---\nkind:\n  primary: schema\n---\n", "", true},
	}
	for _, test := range tests {
		kinds, err := skillsKinds([]byte(test.contents))
		if test.refused && err == nil {
			t.Errorf("%q: unexpectedly accepted as %q", test.contents, kinds)
			continue
		}
		if !test.refused && err != nil {
			t.Errorf("%q: unexpectedly refused: %s", test.contents, err)
			continue
		}
		if !test.refused && kinds != test.kinds {
			t.Errorf("%q: kinds are %q, expected %q", test.contents, kinds, test.kinds)
		}
	}
}

// Only Markdown uploads are skills
func TestIsSkill(t *testing.T) {
	tests := []struct {
		source string
		skill  bool
	}{
		{"glossary.md", true},
		{"references/schema.md", true},
		{"GLOSSARY.MD", true},
		{"model.bin", false},
		{"notes.txt", false},
		{"", false},
	}
	for _, test := range tests {
		upload := skillsUpload{Source: test.source}
		if upload.isSkill() != test.skill {
			t.Errorf("'%s': expected isSkill to be %v", test.source, test.skill)
		}
	}
}

// A skill's name must be a clean relative path, so a file has one name and none escapes
// its directory
func TestSkillsCheckName(t *testing.T) {
	for name, allowed := range map[string]bool{
		"index.md":            true,
		"references/field.md": true,
		"INDEX.MD":            true,
		"index":               false,
		"notes.txt":           false,
		"":                    false,
		"/index.md":           false,
		"../index.md":         false,
		"a/../index.md":       false,
		"./index.md":          false,
		"a//index.md":         false,
		`a\index.md`:          false,
	} {
		if err := skillsCheckName(name); (err == nil) != allowed {
			t.Errorf("'%s': allowed is %v, expected %v (%v)", name, err == nil, allowed, err)
		}
	}
}

// get keeps a local file that differs unless forced, since it may hold unstored edits
func TestSkillsSaveFile(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "nested", "index.md")
	steps := []struct {
		contents string
		force    bool
		state    string
		after    string
	}{
		{"one", false, "new", "one"},
		{"one", false, "unchanged", "one"},
		{"two", false, "kept", "one"},
		{"two", true, "updated", "two"},
	}
	for i, step := range steps {
		state, err := skillsSaveFile(filename, []byte(step.contents), step.force)
		if err != nil {
			t.Fatalf("step %d: %s", i, err)
		}
		after, _ := os.ReadFile(filename)
		if state != step.state || string(after) != step.after {
			t.Errorf("step %d: state %q with %q on disk, expected %q with %q", i, state, after, step.state, step.after)
		}
	}
}

// set on a directory stores its skills by relative path, skipping hidden and other files
func TestSkillsReadDir(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"index.md":           "i",
		"references/a.md":    "a",
		"notes.txt":          "not a skill",
		".hidden.md":         "hidden",
		".git/HEAD.md":       "hidden",
		"references/.tmp.md": "hidden",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0666); err != nil {
			t.Fatal(err)
		}
	}
	skills, err := skillsReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if names := skillsSortedNames(skills); !slices.Equal(names, []string{"index.md", "references/a.md"}) {
		t.Errorf("skills are %v, expected [index.md references/a.md]", names)
	}
}

// Replacing or removing a skill covers every upload under its name, superseded ones too
func TestSkillsStorageNames(t *testing.T) {
	names := skillsStorageNames([]skillsUpload{
		{Name: "index$2.md", Source: "index.md"},
		{Name: "index$1.md", Source: "index.md"},
		{Name: "product$1.md", Source: "product.md"},
	})
	if !slices.Equal(names["index.md"], []string{"index$2.md", "index$1.md"}) || !slices.Equal(names["product.md"], []string{"product$1.md"}) {
		t.Errorf("names are %v", names)
	}
}

// A projectUID begins with app: and a productUID never does, so a value given to the
// wrong one of --project and --product is taken as meant for the other
func TestNormalizeScope(t *testing.T) {
	tests := []struct{ app, product, wantApp, wantProduct string }{
		{"app:123", "", "app:123", ""},
		{"", "net.ozzie.ray:t", "", "net.ozzie.ray:t"},
		{"net.ozzie.ray:t", "", "", "net.ozzie.ray:t"},
		{"", "app:123", "app:123", ""},
		{"net.ozzie.ray:t", "app:123", "app:123", "net.ozzie.ray:t"},
		{"app:123", "net.ozzie.ray:t", "app:123", "net.ozzie.ray:t"},

		// One that can't be right is left alone when the other is already in place
		{"net.ozzie.ray:t", "net.ozzie.ray:u", "net.ozzie.ray:t", "net.ozzie.ray:u"},
		{"app:123", "app:456", "app:123", "app:456"},
		{"", "", "", ""},
	}
	defer func() { flagApp, flagProduct = "", "" }()
	for _, test := range tests {
		flagApp, flagProduct = test.app, test.product
		cliNormalizeScope()
		if flagApp != test.wantApp || flagProduct != test.wantProduct {
			t.Errorf("--project %q --product %q: became %q and %q, expected %q and %q",
				test.app, test.product, flagApp, flagProduct, test.wantApp, test.wantProduct)
		}
	}
}
