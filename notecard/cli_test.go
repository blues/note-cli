// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib/cli"
)

// TestMain defines the tool, as main does
func TestMain(m *testing.M) {
	cli.Define(notecardTool())
	os.Exit(m.Run())
}

// Short help shows how to get going: the general options, the examples and the saved
// settings' switches; full help shows every switch, each with its value named
func TestHelp(t *testing.T) {
	short := captureStdout(t, func() { cli.PrintHelp(cli.ModeNamed(cli.ModeDefault), false) })
	for _, expected := range []string{
		"notecard - ", `USAGE: notecard [options] ['{"req":"..."}']`, "General Options:\n", "--help", "--interface (interface)",
		"--port (port)", "--portconfig (speed)", "Examples:\n  # ", `notecard '{"req":"card.version"}'`,
	} {
		if !strings.Contains(short, expected) {
			t.Errorf("short help lacks %q:\n%s", expected, short)
		}
	}
	for _, unexpected := range []string{"Modes:", "<mode>", "\n  --req ", "\n  --info "} {
		if strings.Contains(short, unexpected) {
			t.Errorf("short help shows %q:\n%s", unexpected, short)
		}
	}
	full := captureStdout(t, func() { cli.PrintHelp(cli.ModeNamed(cli.ModeDefault), true) })
	for _, s := range cli.Switches() {
		if !strings.Contains(full, "\n  --"+s.Name+" ") {
			t.Errorf("full help lacks --%s", s.Name)
		}
	}
	for _, group := range cli.Groups() {
		if !strings.Contains(full, "\n"+group.Description+":\n") {
			t.Errorf("full help lacks the %s group", group.Description)
		}
	}
	for _, unexpected := range []string{"(string)", "(int)", "--watch"} {
		if strings.Contains(full, unexpected) {
			t.Errorf("full help shows %q", unexpected)
		}
	}
	if !strings.Contains(full, "NOTE_PORT") {
		t.Errorf("full help doesn't mention the environment variables")
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
