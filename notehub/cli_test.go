// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"slices"
	"testing"
)

// The mode keyword is additive, so every command line that was legal before modes
// existed must still be parsed as the default mode with its arguments untouched
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

		// Mode keywords
		{[]string{"skills"}, modeSkills, []string{}},
		{[]string{"SKILLS", "-project", "app:1"}, modeSkills, []string{"-project", "app:1"}},
		{[]string{"help", "skills"}, modeHelp, []string{"skills"}},

		// A mode keyword is recognized only in the first position
		{[]string{"-pretty", "skills"}, modeDefault, []string{"-pretty", "skills"}},

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

// The switches on a command line must be identified in the same way that the flag
// package identifies them, because that is what the mode check is performed against
func TestScanSwitchNames(t *testing.T) {
	tests := []struct {
		args  []string
		names []string
	}{
		{[]string{"-project", "app:1", "-verbose", "-upload", "f.bin"}, []string{"project", "verbose", "upload"}},
		{[]string{"-project=app:1", "-pretty"}, []string{"project", "pretty"}},
		{[]string{"--project", "app:1", "--pretty"}, []string{"project", "pretty"}},
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
		{modeSkills, []string{"-project", "app:1", "-verbose"}, false},
		{modeSkills, []string{"-explore"}, false},

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

// Every switch must be defined in a way that the framework can register, validate
// and display, which is checked here so that a mistake in the table is caught at
// test time rather than when the switch is first used
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

// The general options are what the short form of help displays, so they must be
// exactly the switches that belong to the CLI rather than to any one mode
func TestGeneralOptions(t *testing.T) {
	general := []string{}
	for _, s := range cliSwitches() {
		if s.Group == cliGroupGeneral {
			general = append(general, s.Name)
			if !s.allowedIn(modeDefault) || !s.allowedIn(modeSkills) {
				t.Errorf("-%s is a general option and so must be available in every mode", s.Name)
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
