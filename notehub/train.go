// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Training, which is everything done by 'notehub train'.
//
// A Notehub project knows the mechanics of the data flowing through it and nothing about
// the product that produced it.  Nobody asks a question about their data; they ask about
// their product, in their own words, and the gap between the two is what this closes.
//
// The CLI does not do the training.  The harness does.  'notehub train' emits the
// protocol - the instructions that turn Claude, Codex, or any other competent harness
// into the trainer - and the harness then interviews the person, writes what it learns
// into a working copy, and stores it in the project with 'notehub skills push'.

package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The protocol that turns a harness into the trainer
//
//go:embed skills/train.md
var trainProtocol string

// runTrain is the handler for 'notehub train'.  Its output is written to be read by an
// agent rather than by a person, which is why it is emitted rather than summarized.
func runTrain(config *lib.ConfigSettings) error {
	fmt.Printf("%s\n", strings.TrimRight(trainProtocol, "\n"))
	return nil
}
