// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills Storage, which is where a project's skills live.
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
//     What we choose is the source, which the service records alongside it, and which is
//     the skill's name as everyone else knows it: its path, such as index.md.
//
//   - Uploading never replaces.  Adding a skill that already exists creates a second
//     upload with the same source and a newer name, so after adding we delete the ones it
//     supersedes.  That ordering matters: a failed upload leaves the old one in place.

package main

import (
	"fmt"
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

// skillsStorageStore stores one skill in the project under its name, replacing whatever
// the project held under that name before.  superseded names the uploads it replaces,
// from the caller's own read of the project.  The new upload is added before any of them
// is removed, so that a failure part-way leaves the project holding what it held before.
func skillsStorageStore(name string, contents []byte, kinds string, superseded []string) error {
	add := map[string]any{
		"req":     "hub.app.upload.add",
		"type":    skillsUploadType,
		"name":    name,
		"tags":    kinds,
		"payload": contents,
	}
	_, err := skillsRequest(add)

	// The service names an upload by its source and the second in which it arrived, so a
	// skill stored again within a second of its last upload collides with that upload and
	// is refused.  Only a name the project already holds can collide, and the next second
	// gives the upload a name of its own.
	if err != nil && len(superseded) != 0 {
		time.Sleep(time.Second)
		_, err = skillsRequest(add)
	}
	if err != nil {
		return err
	}
	for _, upload := range superseded {
		if err = skillsStorageRemove(upload); err != nil {
			return err
		}
	}
	return nil
}

// skillsStorageNames returns the names of every upload stored under each skill's name,
// current and superseded alike, which is everything a replacement or a removal has to
// account for
func skillsStorageNames(uploads []skillsUpload) (names map[string][]string) {
	names = map[string][]string{}
	for _, upload := range uploads {
		names[upload.Source] = append(names[upload.Source], upload.Name)
	}
	return
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
