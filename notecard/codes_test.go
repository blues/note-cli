package main

import (
	"strings"
	"testing"
)

// fixtureCodesJSON is a representative slice of notecard.codes.json used to
// keep these tests hermetic (no network, no embedded copy).
const fixtureCodesJSON = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"$id": "https://raw.githubusercontent.com/blues/notecard-schema/master/notecard.codes.json",
	"title": "Notecard Error and Status Code Definitions",
	"$defs": {
		"network": { "const": "{network}", "description": "General wireless connectivity error." },
		"host-unreachable": { "const": "{host-unreachable}", "description": "The host is unreachable." },
		"template-incompatible": { "const": "{template-incompatible}", "description": "Operation is incompatible with the Notefile template." },
		"connected": { "const": "{connected}", "description": "Notecard is connected to the cellular network." },
		"cell-registered": { "const": "{cell-registered}", "description": "Wireless service successfully registered." },
		"missing-desc": { "const": "{missing-desc}" }
	}
}`

func TestParseCodeDescriptions(t *testing.T) {
	descriptions := parseCodeDescriptions([]byte(fixtureCodesJSON))

	if got := descriptions["{network}"]; got != "General wireless connectivity error." {
		t.Errorf("{network} = %q, want the network description", got)
	}
	// Entries missing a description must be skipped, not stored empty.
	if _, ok := descriptions["{missing-desc}"]; ok {
		t.Error("{missing-desc} should be skipped because it has no description")
	}
	// Every stored key must be a braced token with a non-empty description.
	for token, desc := range descriptions {
		if !strings.HasPrefix(token, "{") || !strings.HasSuffix(token, "}") {
			t.Errorf("code key %q is not a braced token", token)
		}
		if desc == "" {
			t.Errorf("code %q has an empty description", token)
		}
	}
}

func TestParseCodeDescriptionsMalformed(t *testing.T) {
	// Malformed JSON must disable hints, never panic or error out.
	if got := parseCodeDescriptions([]byte(`not json`)); len(got) != 0 {
		t.Errorf("malformed input should yield no descriptions, got %v", got)
	}
}

func TestExplainCodesWith(t *testing.T) {
	descriptions := parseCodeDescriptions([]byte(fixtureCodesJSON))

	tests := []struct {
		name string
		text string
		// wantTokens are the tokens (with braces) we expect to appear, in
		// order, in the returned hints. nil means we expect no hints.
		wantTokens []string
	}{
		{
			name: "promoted error string with req prefix",
			// This is the shape note-go produces: "<req>: <rsp.Err>"
			text:       `note.template: template mismatch {template-incompatible}`,
			wantTokens: []string{"{template-incompatible}"},
		},
		{
			name:       "multiple distinct tokens",
			text:       `note.add: cannot reach notehub {network} {host-unreachable}`,
			wantTokens: []string{"{network}", "{host-unreachable}"},
		},
		{
			name:       "duplicate tokens deduplicated",
			text:       `{network} something {network} again`,
			wantTokens: []string{"{network}"},
		},
		{
			name:       "status codes in a successful response",
			text:       `{"status":"{connected}{cell-registered}"}`,
			wantTokens: []string{"{connected}", "{cell-registered}"},
		},
		{
			name: "undocumented token ignored",
			// {io} is a real note-go error qualifier but is not (yet)
			// present in notecard.codes.json, so it must be skipped.
			text:       `{io} is undocumented but {network} is documented`,
			wantTokens: []string{"{network}"},
		},
		{
			name:       "no tokens at all",
			text:       `everything is fine`,
			wantTokens: nil,
		},
		{
			name:       "empty string",
			text:       ``,
			wantTokens: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := explainCodesWith(tt.text, descriptions)
			if len(hints) != len(tt.wantTokens) {
				t.Fatalf("got %d hints %v, want %d for tokens %v",
					len(hints), hints, len(tt.wantTokens), tt.wantTokens)
			}
			for i, token := range tt.wantTokens {
				if !strings.HasPrefix(hints[i], token+" ") {
					t.Errorf("hint %d = %q, want it to start with %q and include a description", i, hints[i], token)
				}
				if strings.TrimSpace(strings.TrimPrefix(hints[i], token)) == "" {
					t.Errorf("hint %d = %q has no description", i, hints[i])
				}
			}
		})
	}
}

func TestExplainCodesWithNoDescriptions(t *testing.T) {
	// With no descriptions loaded (e.g. offline), scanning is a no-op.
	if got := explainCodesWith(`{network} {io}`, nil); got != nil {
		t.Errorf("expected no hints with nil descriptions, got %v", got)
	}
}
