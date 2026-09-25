// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills Storage, which is where a project's taught skills live.
//
// Unlike everything else this CLI reaches, skills are stored through the older "v0" API,
// which is a JSON request posted to /req.  Skills are kept as project uploads, and a few
// properties of that API shape everything here:
//
//   - The upload type is "skill", which is the project's skill store and holds nothing
//     else.  Each upload is tagged with the kinds of knowledge it carries, so that a
//     reader can load only the kinds a question needs.
//
//   - A query may ask for the contents as well, and returns the whole set in one
//     transaction.
//
//   - The name under which an upload is stored is assigned by the service, and is opaque.
//     What we choose is the source, which the service records alongside it, and which we
//     use as the skill's path within the working copy.
//
//   - Uploading never replaces.  Adding a skill that already exists creates a second
//     upload with the same source and a newer name, so after adding we delete the ones it
//     supersedes.  That ordering matters: a failed upload leaves the old one in place.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

const (
	// skillsUploadType is the upload type that skills are stored as
	skillsUploadType = "skill"

	// skillsReservedTag is the one tag that means something to the service already,
	// and so is the one tag a skill may not claim as a kind
	skillsReservedTag = "publish"
)

// skillsUpload is one stored skill, as the upload API describes it
type skillsUpload struct {
	Name     string `json:"name,omitempty"`   // assigned by the service, and opaque
	Source   string `json:"source,omitempty"` // what we chose: the skill's path
	Tags     string `json:"tags,omitempty"`
	Length   int    `json:"length,omitempty"`
	Created  int64  `json:"created,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Text     string `json:"text,omitempty"`    // the contents, when the query asked for them
	Payload  []byte `json:"payload,omitempty"` // the contents of a range read
}

// skillsUploadResponse is what the upload API replies with
type skillsUploadResponse struct {
	Err     string         `json:"err,omitempty"`
	Uploads []skillsUpload `json:"uploads,omitempty"`
	Name    string         `json:"name,omitempty"`
	Source  string         `json:"source,omitempty"`
	Payload []byte         `json:"payload,omitempty"` // a range read returns bytes here
}

// isSkill reports whether an upload is one of this project's skills.  The store holds
// nothing else, so this is a guard rather than a filter: a skill is Markdown, and its
// tags say what kind of knowledge it holds rather than that it is a skill at all.
func (upload skillsUpload) isSkill() bool {
	return upload.Source != "" && strings.HasSuffix(strings.ToLower(upload.Source), skillsExt)
}

// skillsRequest performs one v0 upload request.  The project is already on the URL, put
// there from --project or --product, which is how every other v0 request in this CLI is
// scoped.
func skillsRequest(request map[string]any) (rsp skillsUploadResponse, err error) {

	requestJSON, err := note.JSONMarshal(request)
	if err != nil {
		return
	}

	err = reqHubV0(flagVerbose, lib.ConfigAPIHub(), requestJSON, "", "", "", "", false, false, nil, &rsp)
	if err != nil {
		return
	}
	if rsp.Err != "" {
		err = fmt.Errorf("%s", rsp.Err)
	}

	return

}

// skillsStorageQuery returns the project's stored skills, newest first for any source
// that has more than one, and with their contents when they are asked for.  Retrieving
// every skill, contents included, is a single transaction.
func skillsStorageQuery(contents bool) (uploads []skillsUpload, err error) {

	request := map[string]any{
		"req":  "hub.app.upload.query",
		"type": skillsUploadType,
	}
	if contents {
		request["full"] = true
	}

	rsp, err := skillsRequest(request)
	if err != nil {
		return
	}

	for _, upload := range rsp.Uploads {
		if upload.isSkill() {
			uploads = append(uploads, upload)
		}
	}

	// Newest first, so that the first upload seen for a source is the current one
	sort.SliceStable(uploads, func(i, j int) bool {
		if uploads[i].Source != uploads[j].Source {
			return uploads[i].Source < uploads[j].Source
		}
		return skillsUploadWhen(uploads[i]) > skillsUploadWhen(uploads[j])
	})

	return

}

// skillsUploadWhen is when an upload was stored
func skillsUploadWhen(upload skillsUpload) int64 {
	if upload.Modified > upload.Created {
		return upload.Modified
	}
	return upload.Created
}

// skillsStorageCurrent reduces the stored skills to the current one for each source,
// along with the names of the uploads that each of them supersedes
func skillsStorageCurrent(uploads []skillsUpload) (current map[string]skillsUpload, superseded map[string][]string) {
	current = map[string]skillsUpload{}
	superseded = map[string][]string{}
	for _, upload := range uploads {
		if _, have := current[upload.Source]; have {
			superseded[upload.Source] = append(superseded[upload.Source], upload.Name)
			continue
		}
		current[upload.Source] = upload
	}
	return
}

// skillsStorageRead returns the contents of one stored skill.
//
// A query that asked for contents has already brought them back, for the whole project in
// one transaction, and that is the path every caller takes.  An upload that arrives
// without its text - one read from a query that did not ask, or a skill stored empty - is
// read as a range instead, so that a caller never has to know which kind of query it holds.
func skillsStorageRead(upload skillsUpload) (contents []byte, err error) {

	if upload.Text != "" {
		return []byte(upload.Text), nil
	}
	if upload.Payload != nil || upload.Length == 0 {
		return upload.Payload, nil
	}

	rsp, err := skillsRequest(map[string]any{
		"req":    "hub.app.upload.get",
		"type":   skillsUploadType,
		"name":   upload.Name,
		"offset": 0,
		"length": upload.Length,
	})
	if err != nil {
		return nil, err
	}
	if len(rsp.Payload) != upload.Length {
		return nil, fmt.Errorf("%s: read %d of %d bytes", upload.Source, len(rsp.Payload), upload.Length)
	}

	return rsp.Payload, nil

}

// skillsStoragePull replaces the working copy and its baseline with what the project
// holds
func skillsStoragePull(project string, dir string) error {

	uploads, err := skillsStorageQuery(true)
	if err != nil {
		return err
	}
	current, _ := skillsStorageCurrent(uploads)

	skills := map[string][]byte{}
	for source, upload := range current {
		contents, readErr := skillsStorageRead(upload)
		if readErr != nil {
			return readErr
		}
		if contents == nil {
			contents = []byte{}
		}
		skills[source] = contents
	}

	changes, err := skillsReplaceDir(dir, skills)
	if err != nil {
		return err
	}
	if _, err = skillsReplaceDir(skillsBaselinePath(dir), skills); err != nil {
		return err
	}
	skillsRecordPull(dir, project)

	if len(skills) == 0 {
		fmt.Printf("This project holds no skills yet.\n")
	} else {
		fmt.Printf("%d skill(s) in %s\n", len(skills), dir)
	}
	for _, change := range changes {
		fmt.Printf("  %-9s %s\n", change.State, change.Name)
	}

	return nil

}

// skillsStoragePush uploads the new and changed skills, and then removes the uploads
// that they supersede
func skillsStoragePush(project string, dir string, changes []skillsChange) error {

	working, err := skillsReadDir(dir)
	if err != nil {
		return err
	}

	uploads, err := skillsStorageQuery(false)
	if err != nil {
		return err
	}
	_, superseded := skillsStorageCurrent(uploads)
	for _, upload := range uploads {
		superseded[upload.Source] = append(superseded[upload.Source], upload.Name)
	}

	pushed := 0
	for _, change := range changes {
		if change.State == "deleted" {
			continue
		}

		// Add the skill before removing what it replaces, so that a failure here
		// leaves the project holding what it held before
		kinds, kindErr := skillsKinds(working[change.Name])
		if kindErr != nil {
			return fmt.Errorf("%s: %s", change.Name, kindErr)
		}
		_, err = skillsRequest(map[string]any{
			"req":     "hub.app.upload.add",
			"type":    skillsUploadType,
			"name":    change.Name,
			"tags":    kinds,
			"payload": working[change.Name],
		})
		if err != nil {
			return fmt.Errorf("%s: %s", change.Name, err)
		}
		fmt.Printf("  %-9s %-12s %s\n", change.State, kinds, change.Name)
		pushed++

		for _, name := range superseded[change.Name] {
			if err = skillsStorageRemove(name); err != nil {
				return fmt.Errorf("%s: %s", change.Name, err)
			}
		}

		// The baseline now matches the project for this one skill
		if err = skillsWriteFile(skillsBaselinePath(dir), change.Name, working[change.Name]); err != nil {
			return err
		}
	}

	skillsRecordPull(dir, project)
	fmt.Printf("\n%d skill(s) uploaded to %s.\n", pushed, project)

	for _, change := range changes {
		if change.State == "deleted" {
			fmt.Printf("%s is still in the project - remove it with '%s %s delete %s'.\n",
				change.Name, cliName, modeSkills, change.Name)
		}
	}

	return nil

}

// skillsStorageDelete removes a skill from the project, and from the working copy
func skillsStorageDelete(project string, dir string, name string) error {

	uploads, err := skillsStorageQuery(false)
	if err != nil {
		return err
	}

	removed := 0
	for _, upload := range uploads {
		if upload.Source != name {
			continue
		}
		if err = skillsStorageRemove(upload.Name); err != nil {
			return err
		}
		removed++
	}
	if removed == 0 {
		return fmt.Errorf("this project has no skill named '%s'", name)
	}

	// Whatever the project no longer holds, the baseline must not claim it holds
	if path, pathErr := skillsPathWithin(skillsBaselinePath(dir), name); pathErr == nil {
		os.Remove(path)
	}
	if path, pathErr := skillsPathWithin(dir, name); pathErr == nil {
		os.Remove(path)
	}

	fmt.Printf("%s removed from %s\n", name, project)
	return nil

}

// skillsStorageRemove deletes one upload by the name the service assigned it
func skillsStorageRemove(name string) error {
	_, err := skillsRequest(map[string]any{
		"req":  "hub.app.upload.delete",
		"type": skillsUploadType,
		"name": name,
	})
	return err
}

// skillsRecordPull records that the baseline now matches the project.
//
// It deliberately does not go looking for the project's human-readable name.  The v0
// request that carries it, hub.app.get, returns the project's entire configuration,
// which includes route credentials and other secrets, and nothing that merely wants to
// name a backup file should be handling those.
func skillsRecordPull(dir string, project string) {
	os.WriteFile(filepath.Join(skillsBaselinePath(dir), skillsPulledFile),
		[]byte(time.Now().UTC().Format(time.RFC3339)), 0666)
}
