// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills are stored as project uploads of type "skill", through the v0 /req API.  The
// service gives each upload an opaque name and records the skill's path as its source.
// Adding never replaces, so storing a skill adds a new upload and then removes the older
// ones, in that order so that a failure leaves the old one in place.

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

	// skillsReservedTag is a tag the service reserves, so no kind may use it
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

// isSkill reports whether an upload is a skill, which is a Markdown file
func (upload skillsUpload) isSkill() bool {
	return upload.Source != "" && strings.HasSuffix(strings.ToLower(upload.Source), skillsExt)
}

// skillsRequest performs one v0 upload request, scoped by --project or --product
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

// skillsStorageQuery returns the project's stored skills, newest first within each
// source, with their contents if asked
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

	// Newest first within each source, so the first one seen is current
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

// skillsStorageCurrent returns the current upload for each source, and the names of the
// older ones it supersedes
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

// skillsStorageRead returns a stored skill's contents, reading them as a range when the
// query didn't include them
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

// skillsStorageStore stores a skill under its name, replacing the uploads named in
// superseded.  It adds before removing, so a failure leaves the old upload in place.
func skillsStorageStore(name string, contents []byte, kinds string, superseded []string) error {
	add := map[string]any{
		"req":     "hub.app.upload.add",
		"type":    skillsUploadType,
		"name":    name,
		"tags":    kinds,
		"payload": contents,
	}
	_, err := skillsRequest(add)

	// Upload names include the second they arrived, so storing a skill again within a
	// second of its last upload collides with it; the next second gives it its own name
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
// superseded ones included
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
