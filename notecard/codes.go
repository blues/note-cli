// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
)

// codeTokenRegexp matches a Notecard error/status token such as "{io}" or
// "{cell-scan-wait}". Tokens are lowercase words separated by hyphens.
var codeTokenRegexp = regexp.MustCompile(`\{[a-z0-9][a-z0-9-]*\}`)

// codeDescriptions is the lazily-initialized lookup table of code token to
// description, keyed by the full token including braces (e.g. "{io}"). A nil
// value means it has not been loaded yet; an empty (non-nil) map means loading
// was attempted but produced nothing (e.g. offline), so we don't retry.
var codeDescriptions map[string]string

// codesSchemaFilename is the name of the code-definitions schema in the
// notecard-schema repository.
const codesSchemaFilename = "notecard.codes.json"

// parseCodeDescriptions parses the notecard.codes.json schema into a lookup
// table of code token (with braces) to human-readable description.
func parseCodeDescriptions(data []byte) map[string]string {
	descriptions := map[string]string{}

	var doc struct {
		Defs map[string]struct {
			Const       string `json:"const"`
			Description string `json:"description"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return descriptions
	}

	for _, def := range doc.Defs {
		if def.Const == "" || def.Description == "" {
			continue
		}
		descriptions[def.Const] = def.Description
	}

	return descriptions
}

// codesSchemaURL derives the URL of notecard.codes.json from the API schema.
// The codes file lives alongside the per-request schemas that the API schema
// references, so we take the base directory of one of those $refs and append
// the codes filename. This tracks upstream if the schema location ever moves,
// and avoids hardcoding a URL that may differ from the release the API schema
// itself points at.
func codesSchemaURL(schemaURL string, verbose bool) (string, error) {
	reader, err := loadOrFetchSchema(schemaURL, verbose)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	var mainSchema map[string]interface{}
	if err := json.Unmarshal(data, &mainSchema); err != nil {
		return "", err
	}
	refs := extractRefs(mainSchema, schemaURL)
	for _, ref := range refs {
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[:idx+1] + codesSchemaFilename, nil
		}
	}
	return "", fmt.Errorf("no referenced schema found from which to derive %s location", codesSchemaFilename)
}

// loadCodeDescriptions fetches the code definitions through the same
// fetch-and-cache pipeline used for schema validation and parses them into a
// lookup table. It is loaded once per run; a failure to fetch (e.g. offline
// with a cold cache) simply disables hints rather than surfacing an error.
func loadCodeDescriptions(schemaURL string, verbose bool) map[string]string {
	if codeDescriptions != nil {
		return codeDescriptions
	}
	codeDescriptions = map[string]string{}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return codeDescriptions
	}

	codesURL, err := codesSchemaURL(schemaURL, verbose)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "*** unable to locate %s: %v ***\n", codesSchemaFilename, err)
		}
		return codeDescriptions
	}

	reader, err := loadOrFetchSchema(codesURL, verbose)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "*** unable to load %s: %v ***\n", codesSchemaFilename, err)
		}
		return codeDescriptions
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return codeDescriptions
	}

	codeDescriptions = parseCodeDescriptions(data)
	return codeDescriptions
}

// explainCodesWith scans text for Notecard error/status tokens and returns a
// hint line for each recognized, distinct token in the order encountered.
// Unrecognized tokens are ignored so that arbitrary "{...}" text never
// produces noise.
func explainCodesWith(text string, descriptions map[string]string) []string {
	if len(descriptions) == 0 {
		return nil
	}

	var hints []string
	seen := map[string]bool{}
	for _, token := range codeTokenRegexp.FindAllString(text, -1) {
		if seen[token] {
			continue
		}
		seen[token] = true
		if desc, ok := descriptions[token]; ok {
			hints = append(hints, fmt.Sprintf("%s %s", token, desc))
		}
	}
	return hints
}

// explainCodes loads the code definitions and returns hints for any recognized
// tokens found in text.
func explainCodes(text, schemaURL string, verbose bool) []string {
	return explainCodesWith(text, loadCodeDescriptions(schemaURL, verbose))
}

// printCodeHints writes human-readable explanations for any recognized
// Notecard error/status tokens found in text. Hints are written to stderr so
// they never pollute the JSON response on stdout, and color auto-disables on
// non-TTY output.
func printCodeHints(text, schemaURL string, verbose bool) {
	hints := explainCodes(text, schemaURL, verbose)
	if len(hints) == 0 {
		return
	}
	label := color.New(color.FgYellow, color.Bold).Sprint("hint:")
	for _, hint := range hints {
		// Colorize the leading token, leaving the description plain.
		parts := strings.SplitN(hint, " ", 2)
		token := color.CyanString(parts[0])
		desc := ""
		if len(parts) > 1 {
			desc = parts[1]
		}
		fmt.Fprintf(os.Stderr, "%s %s %s\n", label, token, desc)
	}
}
