// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cli

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
)

// The switches of the test tool
var (
	testProject string
	testVerbose bool
)

// testTool is a small tool with a default mode, a mode with commands, and a hidden mode
func testTool() *Tool {
	nothing := func(*lib.ConfigSettings) error { return nil }
	return &Tool{
		Name:  "tool",
		Title: "a tool for testing",
		Modes: []*Mode{
			{Name: ModeDefault, Summary: "do the usual thing", Args: "[request]", Run: nothing,
				Examples: []Example{{"Do the usual thing to a project", "tool --project app:1"}}},
			{Name: "things", Summary: "manage things", Args: "<command>", Run: nothing,
				Commands: []Command{{Name: "list", Summary: "list the things", Run: func(*lib.ConfigSettings, []string) error { return nil }}}},
			{Name: "secret", Summary: "a mode that isn't listed", Hidden: true, Run: nothing},
		},
		Switches: []*Switch{
			{Name: "hub", External: true, ValueType: "hub", Group: GroupGeneral, Modes: []string{AnyMode}, Usage: "the hub"},
			{Name: "project", Target: &testProject, Group: "scope", Modes: []string{ModeDefault, "things"}, Usage: "the `projectUID` to act on"},
			{Name: "verbose", Target: &testVerbose, Group: "scope", Modes: []string{ModeDefault}, Usage: "say more"},
		},
		Groups: []Group{{GroupGeneral, "General Options"}, {"scope", "Scope"}},
	}
}

// Defining a tool adds --help and the help mode, and checks the tables
func TestDefine(t *testing.T) {
	Define(testTool())
	if s := Switches()[0]; s.Name != Help || s.Group != GroupGeneral {
		t.Errorf("the first switch is %+v, expected --help among the general options", s)
	}
	if mode := ModeNamed(Help); mode == nil || !mode.Hidden {
		t.Errorf("the help mode is %+v, expected a hidden one", mode)
	}
	if names := ModeNames(); !slices.Equal(names, []string{ModeDefault, "things"}) {
		t.Errorf("the visible modes are %v, expected the default and 'things'", names)
	}
	if Name() != "tool" {
		t.Errorf("the tool is named %q", Name())
	}
}

// A mistake in the tables is a programming error, reported when the tool is defined
func TestDefineRefusesBadTables(t *testing.T) {
	bad := testTool()
	bad.Switches = append(bad.Switches, &Switch{Name: "lost", Target: &testVerbose, Group: "nowhere", Modes: []string{ModeDefault}})
	bad.Switches = append(bad.Switches, &Switch{Name: "verbose", Target: &testVerbose, Group: "scope", Modes: []string{ModeDefault}})
	defer func() {
		message, _ := recover().(string)
		for _, expected := range []string{"--lost is in group 'nowhere'", "--verbose is defined twice for the 'default' mode"} {
			if !strings.Contains(message, expected) {
				t.Errorf("defining a bad tool reported %q, expected it to say %q", message, expected)
			}
		}
		Define(testTool())
	}()
	Define(bad)
	t.Errorf("defining a bad tool didn't panic")
}

// The first argument names a mode, unless it is a switch or a request
func TestExtractMode(t *testing.T) {
	Define(testTool())
	for _, test := range []struct {
		args      []string
		mode      string
		remaining []string
	}{
		{[]string{"things", "list"}, "things", []string{"list"}},
		{[]string{"--things", "list"}, "things", []string{"list"}},
		{[]string{"THINGS"}, "things", []string{}},
		{[]string{"secret"}, "secret", []string{}},
		{[]string{"--help"}, ModeDefault, []string{"--help"}},
		{[]string{"--project", "app:1"}, ModeDefault, []string{"--project", "app:1"}},
		{[]string{`{"req":"hub.app.get"}`}, ModeDefault, []string{`{"req":"hub.app.get"}`}},
		{[]string{}, ModeDefault, []string{}},
	} {
		mode, remaining := ExtractMode(test.args)
		if mode.Name != test.mode || !slices.Equal(remaining, test.remaining) {
			t.Errorf("%v: got mode '%s' and %v, expected '%s' and %v", test.args, mode.Name, remaining, test.mode, test.remaining)
		}
	}
}

// A switch used in the wrong mode is named as such, with where it may be used
func TestValidateSwitches(t *testing.T) {
	Define(testTool())
	things := ModeNamed("things")
	if err := ValidateSwitches(things, []string{"--project", "app:1", "list"}); err != nil {
		t.Errorf("--project is available in 'things', but: %s", err)
	}
	err := ValidateSwitches(things, []string{"--verbose", "list"})
	if err == nil || !strings.Contains(err.Error(), "not available in 'things' mode (it is available in: default)") {
		t.Errorf("--verbose isn't available in 'things', but: %v", err)
	}
	if err := ValidateSwitches(things, []string{"--unknown"}); err != nil {
		t.Errorf("an unknown switch is the flag package's to report, but: %s", err)
	}
}

// Short help shows the modes, the general options and the examples; full help adds every
// switch of the mode and its detail
func TestPrintHelp(t *testing.T) {
	Define(testTool())
	short := captureStdout(t, func() { PrintHelp(ModeNamed(ModeDefault), false) })
	for _, expected := range []string{
		"tool - a tool for testing\n", "USAGE: tool [options] [request]\n", "tool <mode> [options] [arguments]\n",
		"Modes:\n", "  things", "General Options:\n", "--help", "--hub (hub)", "Examples:\n  # Do the usual thing to a project\n  tool --project app:1\n",
		"https://dev.blues.io/tools-and-sdks/tool-cli\n",
	} {
		if !strings.Contains(short, expected) {
			t.Errorf("short help lacks %q:\n%s", expected, short)
		}
	}
	for _, unexpected := range []string{"secret", "--project (projectUID)", "Scope:"} {
		if strings.Contains(short, unexpected) {
			t.Errorf("short help shows %q:\n%s", unexpected, short)
		}
	}
	full := captureStdout(t, func() { PrintHelp(ModeNamed(ModeDefault), true) })
	for _, expected := range []string{"Scope:\n", "--project (projectUID)", "the projectUID to act on", "--verbose "} {
		if !strings.Contains(full, expected) {
			t.Errorf("full help lacks %q:\n%s", expected, full)
		}
	}
	things := captureStdout(t, func() { PrintHelp(ModeNamed("things"), true) })
	for _, expected := range []string{"tool things - manage things\n", "USAGE: tool things [options] <command>\n", "Commands:\n", "  list", "--project (projectUID)"} {
		if !strings.Contains(things, expected) {
			t.Errorf("mode help lacks %q:\n%s", expected, things)
		}
	}
	if strings.Contains(things, "Modes:") || strings.Contains(things, "--verbose") {
		t.Errorf("mode help lists the modes or a switch of another mode:\n%s", things)
	}
}

// A bare word where a request goes is a mode or a command out of place
func TestCheckNotAMode(t *testing.T) {
	Define(testTool())
	for word, expected := range map[string]string{
		"things": "is a mode, and a mode must be the first thing on the command line, as in: tool things [options] <command>",
		"list":   "is a command of the things mode, as in: tool things list",
		"other":  "is neither a request nor a mode",
	} {
		err := CheckNotAMode(word)
		if err == nil || !strings.Contains(err.Error(), expected) {
			t.Errorf("'%s': got %v, expected it to say %q", word, err, expected)
		}
	}
	if err := CheckNotAMode(`{"req":"hub.app.get"}`); err != nil {
		t.Errorf("a request is not a bare word, but: %s", err)
	}
}

// captureStdout returns what fn writes to stdout
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = saved
	out, _ := io.ReadAll(r)
	return string(out)
}
