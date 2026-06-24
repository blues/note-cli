// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"github.com/note-cli/notehub/cmd"
)

// version is set by GoReleaser via ldflags: -X main.version={{.Version}}
var version = "development"

func main() {
	cmd.Version = version
	cmd.Execute()
}
