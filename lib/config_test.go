// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"flag"
	"io"
	"testing"
)

// Configuration-only invocations must save identically for both flag spellings.
// Operations and positional arguments must not accidentally make an override persist.
func TestConfigFlagsOnly(t *testing.T) {
	tests := []struct {
		name string
		args []string
		save bool
	}{
		{"legacy", []string{"-hub", "example.com"}, true},
		{"modern", []string{"--hub", "example.com"}, true},
		{"legacy equals", []string{"-hub=example.com"}, true},
		{"modern equals", []string{"--hub=example.com"}, true},
		{"mixed", []string{"-interface", "serial", "--port=/dev/example", "-portconfig=9600"}, true},
		{"legacy notecard", []string{"-interface", "serial", "-port", "/dev/example", "-portconfig", "9600"}, true},
		{"modern notecard", []string{"--interface", "serial", "--port", "/dev/example", "--portconfig", "9600"}, true},
		{"repeated", []string{"--hub=first.example", "-hub", "second.example"}, true},
		{"dash value", []string{"--port", "-"}, true},
		{"negative value", []string{"--portconfig", "-1"}, true},
		{"flag-like value", []string{"--port", "--version"}, true},
		{"terminator as value", []string{"--port", "--"}, true},
		{"empty", nil, false},
		{"terminator only", []string{"--"}, false},
		{"trailing terminator", []string{"--hub=example.com", "--"}, true},
		{"legacy operation", []string{"-hub", "example.com", "-version"}, false},
		{"modern operation", []string{"--hub=example.com", "--version"}, false},
		{"false operation", []string{"--version=false", "--hub", "example.com"}, false},
		{"operation only", []string{"--version"}, false},
		{"request", []string{"--hub", "example.com", `{"req":"hub.app.get"}`}, false},
		{"command", []string{"--hub=example.com", "pull"}, false},
		{"flags after positional", []string{"pull", "--hub", "example.com"}, false},
		{"terminated positional", []string{"--hub=example.com", "--", "--version"}, false},
		{"terminated config flag", []string{"--", "--hub=example.com"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet("config", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.String("hub", "", "")
			flags.String("interface", "", "")
			flags.String("port", "", "")
			flags.Int("portconfig", 0, "")
			flags.Bool("version", false, "")
			if err := flags.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			if got := configFlagsOnly(flags); got != test.save {
				t.Errorf("%v: save is %v, expected %v", test.args, got, test.save)
			}
		})
	}
}
