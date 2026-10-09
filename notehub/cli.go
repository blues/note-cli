// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// This CLI as the shared command line framework sees it: its name, modes, switches and
// help groups.  The modes are defined in modes.go and the switches in switches.go.

package main

import (
	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
)

// The name of this CLI as typed on the command line
const cliName = "notehub"

// notehubTool describes this CLI to the framework.  Main defines it before anything else,
// and so do the tests.
func notehubTool() *cli.Tool {
	return &cli.Tool{
		Name:          cliName,
		Title:         "the command line for Notehub",
		Modes:         notehubModes(),
		Switches:      notehubSwitches(),
		Groups:        notehubGroups(),
		NotehubConfig: true,
		AfterParse:    normalizeScope,
	}
}

// apiAccessPreamble is what every mode needs, said where the modes are listed and in the
// help of the modes that provide it
func apiAccessPreamble() string {
	return "This tool requires Notehub API access, granted interactively through a browser\n" +
		"using '" + cliName + " login'. Agents can sign-in in headless by using either:\n" +
		"  '" + agentLoginForms()[0] + "' starts a prompts-disabled login flow\n" +
		"  '" + lib.ConfigTokenEnvVar + "' environment variable contains a Notehub Personal Access Token"
}

// agentLoginForms are the ways an agent is meant to sign in, as help shows them wherever
// API access is explained
func agentLoginForms() []string {
	return []string{
		cliName + ` login --name "DotBot" --json`,
		`echo "$SECRET" | ` + cliName + ` login --with-token --json`,
	}
}
