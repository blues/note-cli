// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The set used by these tests, written the way a trained project writes one: a front
// matter block, one title, headings beneath it, and cross-references in prose
var testSkillBodies = map[string]string{
	"index.md": `---
kind: index
---

# Radnote

## Training status

Product context: product.md. Field decoder: notefiles.md (partial).
See product.md, Ukraine use case, for the documented scope, and
operations.md, Historical question scope, Population and identity, and Scenarios and lifecycle.
Audiences are in product.md, Audiences and presentation.
`,
	"product.md": `---
kind: product
---

# Radnote product context

## Ukraine use case

Text.

## Audiences and presentation ` + "—" + ` incomplete

Text.
`,
	"operations.md": `---
kind: population
---

# Operational rules

## Historical question scope

Text.

## Population and identity

Text.

## Scenarios and lifecycle

Text.
`,
	"notefiles.md": `# Notefile roster

## ` + "`_air.qo`" + `

Text.
`,
}

// testSkillSet builds the uploads and bodies that the assembly works from
func testSkillSet() (current map[string]skillsUpload, bodies map[string][]byte) {
	current = map[string]skillsUpload{}
	bodies = map[string][]byte{}
	for source, body := range testSkillBodies {
		current[source] = skillsUpload{Name: source + "$1", Source: source, Tags: "kind", Length: len(body)}
		bodies[source] = []byte(body)
	}
	return
}

// The index is read first and everything else in the order it is first mentioned there,
// so that following a reference always moves forward through the document
func TestSkillsOrder(t *testing.T) {
	current, bodies := testSkillSet()
	sources := skillsOrder(current, bodies["index.md"])
	expected := []string{"index.md", "product.md", "notefiles.md", "operations.md"}
	if !slices.Equal(sources, expected) {
		t.Errorf("order is %v, expected %v", sources, expected)
	}

	// With nothing to order by, and with no index, the order is still stable
	sources = skillsOrder(current, nil)
	expected = []string{"index.md", "notefiles.md", "operations.md", "product.md"}
	if !slices.Equal(sources, expected) {
		t.Errorf("order without an index is %v, expected %v", sources, expected)
	}
}

// A skill is named as it was read, with or without its extension and in any case
func TestSkillsResolve(t *testing.T) {
	current, _ := testSkillSet()
	for _, name := range []string{"product.md", "product", "PRODUCT.MD", " product.md "} {
		upload, err := skillsResolve(current, name)
		if err != nil || upload.Source != "product.md" {
			t.Errorf("'%s' resolved to '%s' (%v), expected product.md", name, upload.Source, err)
		}
	}
	if _, err := skillsResolve(current, "nosuch"); err == nil {
		t.Errorf("a name that is not a skill must be an error naming the ones that are")
	} else if !strings.Contains(err.Error(), "product.md") {
		t.Errorf("the error must list what the project holds: %s", err)
	}
}

// Every cross-reference becomes a link, every link resolves to an anchor that exists, and
// nothing else about the stored text changes
func TestSkillsAssemble(t *testing.T) {

	current, bodies := testSkillSet()
	assembled := skillsAssemble("test:project", current, bodies)

	// Every link points at an anchor that the document defines
	anchors := map[string]bool{}
	for _, match := range regexp.MustCompile(`<a id="([^"]+)"></a>`).FindAllStringSubmatch(assembled, -1) {
		if anchors[match[1]] {
			t.Errorf("anchor '%s' is defined twice, so a link to it is ambiguous", match[1])
		}
		anchors[match[1]] = true
	}
	targets := regexp.MustCompile(`\]\(#([^)]+)\)`).FindAllStringSubmatch(assembled, -1)
	if len(targets) == 0 {
		t.Fatalf("the assembled document has no links")
	}
	for _, match := range targets {
		if !anchors[match[1]] {
			t.Errorf("link to '#%s' has no anchor in the document", match[1])
		}
	}

	// A reference to a file, however it is punctuated, and a heading cited after one
	for _, expected := range []string{
		"[product.md](#product-md).",
		"[notefiles.md](#notefiles-md) (partial)",
		"[product.md](#product-md), [Ukraine use case](#product-md--ukraine-use-case),",
		"[Historical question scope](#operations-md--historical-question-scope)",
		"[Audiences and presentation](#product-md--audiences-and-presentation-incomplete)",
	} {
		if !strings.Contains(assembled, expected) {
			t.Errorf("expected the assembled document to contain: %s", expected)
		}
	}

	// A list of headings after one filename links each of them
	for _, heading := range []string{"population-and-identity", "scenarios-and-lifecycle"} {
		if !strings.Contains(assembled, "](#operations-md--"+heading+")") {
			t.Errorf("a heading listed after a filename must be linked: %s", heading)
		}
	}

	// One title, each document beneath it, and nothing promoted past the top
	if count := strings.Count(assembled, "\n# "); count != 0 {
		t.Errorf("the assembled document has %d headings competing with its title", count)
	}
	if !strings.HasPrefix(assembled, "# Skills for test:project\n") {
		t.Errorf("the assembled document must open with its title")
	}
	for _, section := range []string{"## Radnote\n", "## Radnote product context\n", "## Notefile roster\n"} {
		if !strings.Contains(assembled, "\n"+section) {
			t.Errorf("each document is a section of its own: %s", section)
		}
	}

	// The stored sentences are the stored sentences
	if !strings.Contains(assembled, "for the documented scope, and") {
		t.Errorf("assembly must not reword what was stored")
	}
	if !strings.Contains(assembled, "kind: index") {
		t.Errorf("a document's front matter is part of what it says")
	}

}

// A heading is cited by its name, whatever qualifier the heading itself carries
func TestSkillsHeadingNames(t *testing.T) {
	names := skillsHeadingNames("Audiences and presentation — incomplete")
	if !slices.Contains(names, "Audiences and presentation") {
		t.Errorf("a qualified heading must be citable by its name alone: %v", names)
	}
	names = skillsHeadingNames("`_air.qo`")
	if !slices.Contains(names, "_air.qo") {
		t.Errorf("a heading naming a Notefile must be citable without its backquotes: %v", names)
	}
}

// A filename inside a code span, a link, or a longer word is not a cross-reference
func TestSkillsLinkLeavesAlone(t *testing.T) {

	current, bodies := testSkillSet()
	bodies["index.md"] = []byte("# Index\n\n## Uses\n\n" +
		"A span `product.md` stays, [product.md](https://example.com/product.md) stays,\n" +
		"notproduct.md is not ours, and product.md links.\n")
	assembled := skillsAssemble("test:project", current, bodies)

	if !strings.Contains(assembled, "A span `product.md` stays") {
		t.Errorf("a filename in a code span is an example, not a reference")
	}
	if !strings.Contains(assembled, "[product.md](https://example.com/product.md) stays") {
		t.Errorf("a filename already in a link must be left alone")
	}
	if strings.Contains(assembled, "not[product.md]") || strings.Contains(assembled, "[notproduct.md]") {
		t.Errorf("a filename that is part of a longer name is not a reference")
	}
	if !strings.Contains(assembled, "and [product.md](#product-md) links.") {
		t.Errorf("an ordinary reference must still be linked")
	}

}
