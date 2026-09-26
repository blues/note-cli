// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub skills list' and 'show': reading what a project holds.  'show all' assembles
// the set into one document, turning each cross-reference ("see product.md, Vocabulary")
// into a link to that skill's section and heading.

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
)

// skillsAll means every skill rather than one; a skill named all.md is still 'all.md'
const skillsAll = "all"

// skillsIndex is the skill a reader starts from, always placed first
const skillsIndex = "index" + skillsExt

// skillsListCommand displays what the project holds
func skillsListCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	if len(args) != 0 {
		return fmt.Errorf("'%s %s list' takes no arguments", cliName, modeSkills)
	}

	uploads, err := skillsStorageQuery(false)
	if err != nil {
		return err
	}
	current, superseded := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		fmt.Printf("%s holds no skills yet.\n", project)
		return nil
	}

	// Align the names
	sources := skillsOrder(current, nil)
	width := 0
	for _, source := range sources {
		if len(source) > width {
			width = len(source)
		}
	}

	fmt.Printf("project: %s\n\n", project)
	total := 0
	for _, source := range sources {
		upload := current[source]
		total += upload.Length
		line := fmt.Sprintf("  %*s%7d  %-16s %s", -(width + 2), source, upload.Length,
			skillsWhen(upload), upload.Tags)
		fmt.Println(strings.TrimRight(line, " "))
	}
	fmt.Printf("\n%d skill(s), %d bytes.  Read one with '%s %s show <name>', or the whole set with '%s %s'.\n",
		len(sources), total, cliName, modeSkills, cliName, modeSkills)

	// Superseded uploads are still stored, so say how many
	count := 0
	for _, names := range superseded {
		count += len(names)
	}
	if count != 0 {
		fmt.Printf("%d superseded upload(s) are also stored, and are not part of the set.\n", count)
	}

	return nil

}

// skillsShowCommand writes one skill, or the whole set as one document, to stdout
func skillsShowCommand(config *lib.ConfigSettings, args []string) error {

	project, err := skillsProject()
	if err != nil {
		return err
	}
	name := skillsAll
	switch len(args) {
	case 0:
	case 1:
		name = args[0]
	default:
		return fmt.Errorf("'%s %s show' takes one skill name, or 'all'", cliName, modeSkills)
	}

	// Every skill, with contents, in one request
	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)
	if len(current) == 0 {
		return fmt.Errorf("%s holds no skills yet", project)
	}

	// One skill, exactly as stored
	if !strings.EqualFold(name, skillsAll) {
		upload, resolveErr := skillsResolve(current, name)
		if resolveErr != nil {
			return resolveErr
		}
		contents, readErr := skillsStorageRead(upload)
		if readErr != nil {
			return readErr
		}
		os.Stdout.Write(contents)
		if len(contents) != 0 && !strings.HasSuffix(string(contents), "\n") {
			fmt.Println()
		}
		return nil
	}

	// The whole set, assembled
	bodies := map[string][]byte{}
	for source, upload := range current {
		contents, readErr := skillsStorageRead(upload)
		if readErr != nil {
			return readErr
		}
		bodies[source] = contents
	}
	fmt.Print(skillsAssemble(project, current, bodies))
	return nil

}

// skillsResolve returns the skill a name refers to, ignoring case and an omitted .md
func skillsResolve(current map[string]skillsUpload, name string) (upload skillsUpload, err error) {

	wanted := strings.ToLower(strings.TrimSpace(name))
	if wanted == "" {
		return upload, fmt.Errorf("name a skill, or 'all' for the whole set")
	}
	if !strings.HasSuffix(wanted, skillsExt) {
		wanted += skillsExt
	}
	for source, candidate := range current {
		if strings.ToLower(source) == wanted {
			return candidate, nil
		}
	}

	return upload, fmt.Errorf("'%s' is not one of this project's skills - it holds: %s",
		name, strings.Join(skillsOrder(current, nil), ", "))

}

// skillsOrder returns the sources in reading order: the index first, then the others in
// the order the index first mentions them, then any it never mentions
func skillsOrder(current map[string]skillsUpload, index []byte) (sources []string) {

	rest := []string{}
	for source := range current {
		if strings.EqualFold(source, skillsIndex) {
			continue
		}
		rest = append(rest, source)
	}
	sort.Strings(rest)

	if _, have := current[skillsIndex]; have {
		sources = append(sources, skillsIndex)
	}

	// Order by first mention in the index
	if len(index) != 0 {
		text := string(index)
		sort.SliceStable(rest, func(i, j int) bool {
			at, bt := strings.Index(text, rest[i]), strings.Index(text, rest[j])
			switch {
			case at == bt:
				return false
			case at < 0:
				return false
			case bt < 0:
				return true
			}
			return at < bt
		})
	}

	return append(sources, rest...)

}

// skillsWhen is how a stored skill's date is displayed in a listing
func skillsWhen(upload skillsUpload) string {
	when := skillsUploadWhen(upload)
	if when == 0 {
		return ""
	}
	return time.Unix(when, 0).Format("2006-01-02 15:04")
}

// skillsDoc is one skill prepared for assembly
type skillsDoc struct {
	source   string            // the skill's filename
	upload   skillsUpload      // what the project holds
	title    string            // from its first heading
	front    string            // its front matter, as stored
	body     string            // what follows the front matter and title
	anchor   string            // its section's anchor
	headings map[string]string // each heading, as cited, to its anchor
}

// skillsAssemble turns the whole set into one document: a section per skill, an anchor on
// every heading, and a link on every cross-reference, with the text itself unchanged
func skillsAssemble(project string, current map[string]skillsUpload, bodies map[string][]byte) string {

	sources := skillsOrder(current, bodies[skillsIndex])

	// Prepare every skill first, since a link may point at a later one
	docs := map[string]*skillsDoc{}
	ordered := []*skillsDoc{}
	for _, source := range sources {
		doc := skillsPrepare(source, current[source], string(bodies[source]))
		docs[source] = doc
		ordered = append(ordered, doc)
	}

	out := &strings.Builder{}
	fmt.Fprintf(out, "# Skills for %s\n\n", project)
	fmt.Fprintf(out, "*%d documents, assembled from the project by `%s %s show all` on %s.  Each is",
		len(ordered), cliName, modeSkills, time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintf(out, " stored separately and appears here as stored, with its headings one level lower")
	fmt.Fprintf(out, " and its cross-references linked.*\n\n")

	fmt.Fprintf(out, "## Contents\n\n")
	for _, doc := range ordered {
		entry := fmt.Sprintf("- [%s](#%s) - `%s`", doc.title, doc.anchor, doc.source)
		if doc.upload.Tags != "" {
			entry += ", " + doc.upload.Tags
		}
		fmt.Fprintf(out, "%s\n", entry)
	}

	for _, doc := range ordered {
		fmt.Fprintf(out, "\n---\n\n")
		fmt.Fprintf(out, "<a id=\"%s\"></a>\n\n", doc.anchor)
		fmt.Fprintf(out, "## %s\n\n", doc.title)
		about := []string{"`" + doc.source + "`"}
		if label := skillsKindsLabel(doc.upload.Tags); label != "" {
			about = append(about, label)
		}
		if when := skillsWhen(doc.upload); when != "" {
			about = append(about, when)
		}
		about = append(about, fmt.Sprintf("%d bytes", doc.upload.Length))
		fmt.Fprintf(out, "*%s*\n\n", strings.Join(about, " | "))
		if doc.front != "" {
			fmt.Fprintf(out, "```yaml\n%s\n```\n\n", doc.front)
		}
		fmt.Fprintf(out, "%s\n", skillsLink(doc, docs))
	}

	return out.String()

}

// skillsKindsLabel describes an upload's kinds, or returns "" if it has none
func skillsKindsLabel(tags string) string {
	if tags == "" {
		return ""
	}
	return "kinds: " + strings.ReplaceAll(tags, ",", ", ")
}

// skillsPrepare splits a skill into title, front matter and body, and records where each
// of its headings will land
func skillsPrepare(source string, upload skillsUpload, contents string) *skillsDoc {

	doc := &skillsDoc{
		source:   source,
		upload:   upload,
		anchor:   skillsSlug(source),
		headings: map[string]string{},
	}

	front, rest := skillsFrontMatter(contents)
	doc.front = front

	// The first heading becomes the section title; the others drop a level and gain anchors
	lines := strings.Split(rest, "\n")
	body := []string{}
	fenced := false
	for _, line := range lines {
		if skillsFence(line) {
			fenced = !fenced
			body = append(body, line)
			continue
		}
		level, text := skillsHeading(line, fenced)
		if level == 0 {
			body = append(body, line)
			continue
		}
		if level == 1 && doc.title == "" && len(body) == 0 {
			doc.title = text
			continue
		}
		anchor := doc.anchor + "--" + skillsSlug(text)
		for _, cited := range skillsHeadingNames(text) {
			if _, have := doc.headings[cited]; !have {
				doc.headings[cited] = anchor
			}
		}
		body = append(body, fmt.Sprintf("<a id=\"%s\"></a>\n\n%s %s", anchor,
			strings.Repeat("#", level+1), text))
	}
	if doc.title == "" {
		doc.title = source
	}
	doc.body = strings.Trim(strings.Join(body, "\n"), "\n")

	return doc

}

// skillsHeadingNames returns the forms a heading may be cited by: with or without a
// trailing qualifier such as " - incomplete", and with or without backquotes
func skillsHeadingNames(text string) (names []string) {

	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	add(text)
	add(strings.ReplaceAll(text, "`", ""))
	for _, dash := range []string{" — ", " - ", " – "} {
		if cut := strings.Index(text, dash); cut > 0 {
			add(text[:cut])
			add(strings.ReplaceAll(text[:cut], "`", ""))
		}
	}

	return

}

// skillsFrontMatter separates a skill's front matter from the rest of it
func skillsFrontMatter(contents string) (front string, rest string) {
	contents = strings.TrimLeft(contents, "\ufeff")
	if !strings.HasPrefix(contents, "---\n") {
		return "", contents
	}
	if end := strings.Index(contents[4:], "\n---"); end >= 0 {
		front = strings.Trim(contents[4:4+end], "\n")
		rest = contents[4+end+4:]
		return front, strings.TrimLeft(rest, "\n")
	}
	return "", contents
}

// skillsFence reports whether a line opens or closes a fenced code block
func skillsFence(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// skillsHeading returns a line's heading level and text, or zero if it is not a heading
func skillsHeading(line string, fenced bool) (level int, text string) {
	if fenced {
		return 0, ""
	}
	trimmed := strings.TrimLeft(line, " ")
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 5 || level == len(trimmed) || trimmed[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(strings.TrimRight(trimmed[level+1:], "#"))
}

// skillsSlug turns a name into an anchor
func skillsSlug(text string) string {
	out := &strings.Builder{}
	dash := false
	for _, c := range strings.ToLower(text) {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			out.WriteRune(c)
			dash = false
		default:
			if !dash && out.Len() != 0 {
				out.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

// skillsLink turns a skill's cross-references into links: a filename to that skill's
// section, and any headings cited after it to theirs.  The text itself is unchanged.
func skillsLink(doc *skillsDoc, docs map[string]*skillsDoc) string {

	lines := strings.Split(doc.body, "\n")
	fenced := false
	for i, line := range lines {
		if skillsFence(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		// Only outside code spans
		parts := strings.Split(line, "`")
		for p := 0; p < len(parts); p += 2 {
			parts[p] = skillsLinkText(parts[p], docs)
		}
		lines[i] = strings.Join(parts, "`")
	}

	return strings.Join(lines, "\n")

}

// skillsLinkText links every cross-reference in one span of ordinary prose
func skillsLinkText(text string, docs map[string]*skillsDoc) string {

	out := &strings.Builder{}
	for at := 0; at < len(text); {

		// The next filename that names one of the set's skills
		start, end, target := skillsNextRef(text, at, docs)
		if target == nil {
			out.WriteString(text[at:])
			break
		}
		out.WriteString(text[at:start])
		fmt.Fprintf(out, "[%s](#%s)", text[start:end], target.anchor)
		at = end

		// Headings cited after it, however many are listed
		for {
			separator, heading, anchor, next := skillsNextHeading(text, at, target)
			if anchor == "" {
				break
			}
			fmt.Fprintf(out, "%s[%s](#%s)", separator, heading, anchor)
			at = next
		}

	}
	return out.String()

}

// skillsNextRef finds the next filename in text that names one of the set's skills and
// isn't already inside a link
func skillsNextRef(text string, from int, docs map[string]*skillsDoc) (start int, end int, target *skillsDoc) {

	for at := from; at < len(text); at++ {

		next := strings.Index(text[at:], skillsExt)
		if next < 0 {
			break
		}
		end = at + next + len(skillsExt)
		at = end - 1

		// Back up to the start of the name
		start = end - len(skillsExt)
		for start > 0 && skillsNameByte(text[start-1]) {
			start--
		}
		source := text[start:end]

		// A name inside a link, or one that is not one of ours, is left alone
		if start > 0 && (text[start-1] == '[' || text[start-1] == '(' || text[start-1] == '/') {
			continue
		}
		// A name running into a longer word isn't a reference, but one ending a sentence is
		if end < len(text) {
			switch next := text[end]; {
			case next == ']':
				continue
			case next == '.' && end+1 < len(text) && skillsNameByte(text[end+1]):
				continue
			case next != '.' && skillsNameByte(next):
				continue
			}
		}
		for name, doc := range docs {
			if strings.EqualFold(name, source) {
				return start, end, doc
			}
		}

	}

	return 0, 0, nil

}

// skillsNameByte reports whether a byte can appear within a skill's filename
func skillsNameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '_' || c == '-' || c == '.'
}

// skillsNextHeading matches a heading of the cited skill where the citation continues, as
// in ", Vocabulary", and returns its anchor
func skillsNextHeading(text string, at int, target *skillsDoc) (separator string, heading string, anchor string, next int) {

	for _, candidate := range []string{", and ", ", ", " and "} {
		if !strings.HasPrefix(text[at:], candidate) {
			continue
		}
		rest := text[at+len(candidate):]

		// Longest match first, so a heading containing a separator isn't split on it
		longest := ""
		for cited := range target.headings {
			if len(cited) > len(longest) && strings.HasPrefix(rest, cited) {
				longest = cited
			}
		}
		if longest == "" {
			continue
		}
		return candidate, longest, target.headings[longest], at + len(candidate) + len(longest)
	}

	return "", "", "", at

}
