// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills storage is the command model over the Skills API. Commands retain the
// metadata they read so that every mutation is conditional on its revision.

package main

import (
	"net/http"
	"strings"
	"time"
)

var skillsClient = &http.Client{Timeout: 2 * time.Minute}

// skillsReservedTag is a tag the service reserves, so no kind may use it
const skillsReservedTag = "publish"

// skillsUpload is one stored skill. Name and Source are both its logical name;
// Revision and the resolved Project make later writes conditional.
type skillsUpload struct {
	Name     string `json:"name,omitempty"`
	Source   string `json:"source,omitempty"` // the skill's path
	Tags     string `json:"tags,omitempty"`
	Length   int    `json:"length,omitempty"`
	Created  int64  `json:"created,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Payload  []byte `json:"payload,omitempty"` // non-nil when content is loaded, including empty content
	Revision string `json:"-"`
	Project  string `json:"-"`
}

// skillsStorageQuery returns the project's stored skills, with their contents if asked
func skillsStorageQuery(contents bool) ([]skillsUpload, error) {
	return skillsV1Query(contents)
}

// skillsUploadWhen is when a skill was stored
func skillsUploadWhen(upload skillsUpload) int64 {
	if upload.Modified > upload.Created {
		return upload.Modified
	}
	return upload.Created
}

// skillsStorageCurrent indexes skills by name
func skillsStorageCurrent(uploads []skillsUpload) map[string]skillsUpload {
	current := map[string]skillsUpload{}
	for _, upload := range uploads {
		current[upload.Source] = upload
	}
	return current
}

// skillsStorageRead uses already-loaded content, otherwise a conditional GET.
func skillsStorageRead(upload skillsUpload) ([]byte, error) {
	if upload.Payload != nil {
		return upload.Payload, nil
	}
	return skillsV1Read(upload)
}

// skillsStorageStore creates a skill, or conditionally replaces previous when given.
func skillsStorageStore(name string, contents []byte, kinds string, previous *skillsUpload) error {
	return skillsV1Store(name, contents, kinds, previous)
}

// skillsStorageRemove deletes a skill at its revision.
func skillsStorageRemove(upload skillsUpload) error {
	return skillsV1Remove(upload)
}

// skillsStorageSelection reads either one named skill or the complete set.
func skillsStorageSelection(name string) ([]skillsUpload, error) {
	if strings.EqualFold(name, skillsAll) {
		return skillsStorageQuery(true)
	}
	return skillsV1QueryName(name)
}
