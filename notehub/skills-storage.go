// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Skills storage selects the public V1 Skills API or the original V0 upload API.
// Commands retain the metadata they read so V1 mutations can use its revision.
// V0 replacements still add a new upload before deleting superseded uploads.

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-go/note"
)

// Set true and rebuild to use the original V0 Skills implementation.
// False uses V1 exclusively; errors never fall back to V0.
const useV0SkillApis = true

var skillsClient = &http.Client{Timeout: 2 * time.Minute}

const (
	// skillsUploadType is the upload type that skills are stored as
	skillsUploadType = "skill"

	// skillsReservedTag is a tag the service reserves, so no kind may use it
	skillsReservedTag = "publish"
)

// skillsUpload is the shared command model. V0 uses an opaque Name and logical Source;
// V1 uses the logical name for both and carries a revision and resolved project.
type skillsUpload struct {
	Name     string `json:"name,omitempty"`   // opaque for V0; the logical name for V1
	Source   string `json:"source,omitempty"` // what we chose: the skill's path
	Tags     string `json:"tags,omitempty"`
	Length   int    `json:"length,omitempty"`
	Created  int64  `json:"created,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Text     string `json:"text,omitempty"`    // the contents, when the query asked for them
	Payload  []byte `json:"payload,omitempty"` // non-nil when content is loaded, including empty content
	Revision string `json:"-"`
	Project  string `json:"-"`
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

	query := url.Values{}
	if flagApp != "" {
		query.Set("app", flagApp)
	} else {
		query.Set("product", flagProduct)
	}
	httpRsp, err := apiSend(skillsClient, &apiRequest{Method: http.MethodPost,
		URL: &url.URL{Path: "/req", RawQuery: query.Encode()}, Header: http.Header{}, Body: requestJSON})
	if err != nil {
		return
	}
	if err = apiFailure(httpRsp); err != nil {
		return
	}
	if err = note.JSONUnmarshal(httpRsp.Body, &rsp); err != nil {
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
	if !useV0SkillApis {
		return skillsV1Query(contents)
	}

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

// skillsStorageRead uses already-loaded content, otherwise a conditional V1 GET
// or a V0 range read.
func skillsStorageRead(upload skillsUpload) (contents []byte, err error) {
	if !useV0SkillApis {
		if upload.Payload != nil {
			return upload.Payload, nil
		}
		return skillsV1Read(upload)
	}

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

// skillsStorageStore conditionally replaces one V1 skill, or adds a V0 upload
// before removing the superseded uploads.
func skillsStorageStore(name string, contents []byte, kinds string, superseded []skillsUpload) error {
	if !useV0SkillApis {
		return skillsV1Store(name, contents, kinds, superseded)
	}
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

// skillsStorageVersions retains every backing upload for V0 and the current revision
// for V1, so a later write never refreshes away a conflict.
func skillsStorageVersions(uploads []skillsUpload) (names map[string][]skillsUpload) {
	names = map[string][]skillsUpload{}
	for _, upload := range uploads {
		names[upload.Source] = append(names[upload.Source], upload)
	}
	return
}

// skillsStorageRemove deletes an opaque V0 upload or a logical V1 skill at its revision.
func skillsStorageRemove(upload skillsUpload) error {
	if !useV0SkillApis {
		return skillsV1Remove(upload)
	}
	_, err := skillsRequest(map[string]any{
		"req":  "hub.app.upload.delete",
		"type": skillsUploadType,
		"name": upload.Name,
	})
	return err
}

// skillsStorageSelection reads either one named skill or the complete set.
func skillsStorageSelection(name string) ([]skillsUpload, error) {
	if useV0SkillApis || strings.EqualFold(name, skillsAll) {
		return skillsStorageQuery(true)
	}
	return skillsV1QueryName(name)
}
