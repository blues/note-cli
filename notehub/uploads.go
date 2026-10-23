// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The hub's uploads, through the v0 upload calls, which take every kind the hub stores:
// a project's files and host firmware, and the hub's own notecard firmware, which no
// project holds.  The 'files' mode is built on this, and the 'firmware' mode uses it for
// Notecard catalog operations. Uploads use v1 with an explicit or default project. The hub
// stores an upload under a name of its own, the name given with the moment it arrived,
// and keeps the name given as the upload's source; adding never replaces, so replacing
// means adding and then removing what the addition supersedes.

package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// uploadsScope says which uploads: of which type, and a project's or the hub's own
type uploadsScope struct {
	Type   string
	Global bool
}

// uploadsResponse is what a v0 upload call answers: an error, a listing, or the metadata
// of one upload, which a range read carries the bytes in
type uploadsResponse struct {
	Err     string                   `json:"err,omitempty"`
	Uploads []notehub.UploadMetadata `json:"uploads,omitempty"`
	notehub.UploadMetadata
}

// uploadsClient is how the hub is reached, which a test may replace
var uploadsClient = &http.Client{}

// uploadsChunk is how much of an upload one range read asks for
const uploadsChunk = 512 * 1024

// uploadsRequest performs one v0 upload call: a project's, scoped by --project or
// --product, or the hub's own for a global type, which no project scopes
func uploadsRequest(scope uploadsScope, verb string, request map[string]any) (rsp uploadsResponse, err error) {
	request["req"] = "hub.app.upload." + verb
	if scope.Global {
		request["req"] = "hub.upload." + verb
	}
	request["type"] = scope.Type
	body, err := note.JSONMarshal(request)
	if err != nil {
		return
	}
	query := url.Values{}
	if !scope.Global {
		if flagApp != "" {
			query.Set("app", flagApp)
		} else if flagProduct != "" {
			query.Set("product", flagProduct)
		}
	}
	httpRsp, err := apiSend(uploadsClient, &apiRequest{Method: http.MethodPost, URL: &url.URL{Path: "/req", RawQuery: query.Encode()}, Header: http.Header{}, Body: body})
	if err != nil {
		return
	}
	if err = note.JSONUnmarshal(httpRsp.Body, &rsp); err != nil {
		return rsp, fmt.Errorf("the hub's answer can't be read: %w", err)
	}
	if rsp.Err != "" {
		return rsp, errors.New(rsp.Err)
	}
	return rsp, apiFailure(httpRsp)
}

// uploadsQuery returns the uploads of a scope, newest first within each source, with
// their contents if asked
func uploadsQuery(scope uploadsScope, contents bool) ([]notehub.UploadMetadata, error) {
	request := map[string]any{}
	if contents {
		request["full"] = true
	}
	rsp, err := uploadsRequest(scope, "query", request)
	if err != nil {
		return nil, err
	}
	uploads := rsp.Uploads
	sort.SliceStable(uploads, func(i, j int) bool {
		if uploads[i].Source != uploads[j].Source {
			return uploads[i].Source < uploads[j].Source
		}
		return uploadsWhen(uploads[i]) > uploadsWhen(uploads[j])
	})
	return uploads, nil
}

// uploadsWhen is when an upload was stored
func uploadsWhen(upload notehub.UploadMetadata) int64 {
	if upload.Modified > upload.Created {
		return upload.Modified
	}
	return upload.Created
}

// uploadsCurrent returns the current upload for each source, and the names of the older
// ones it supersedes
func uploadsCurrent(uploads []notehub.UploadMetadata) (current map[string]notehub.UploadMetadata, superseded map[string][]string) {
	current = map[string]notehub.UploadMetadata{}
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

// uploadsNames returns the names of every upload stored under each source
func uploadsNames(uploads []notehub.UploadMetadata) map[string][]string {
	names := map[string][]string{}
	for _, upload := range uploads {
		names[upload.Source] = append(names[upload.Source], upload.Name)
	}
	return names
}

// uploadsSources returns the sources, sorted
func uploadsSources(current map[string]notehub.UploadMetadata) []string {
	sources := []string{}
	for source := range current {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

// uploadsResolve returns the upload a name refers to, by its source or by the name the
// hub stored it under, ignoring case
func uploadsResolve(current map[string]notehub.UploadMetadata, name string) (notehub.UploadMetadata, error) {
	wanted := strings.TrimSpace(name)
	for source, upload := range current {
		if strings.EqualFold(source, wanted) || strings.EqualFold(upload.Name, wanted) {
			return upload, nil
		}
	}
	return notehub.UploadMetadata{}, fmt.Errorf("'%s' is not one of them - they are: %s", name, strings.Join(uploadsSources(current), ", "))
}

// uploadsRead returns an upload's contents: those the query included, or else read as
// ranges, a chunk at a time, and checked against its md5
func uploadsRead(scope uploadsScope, upload notehub.UploadMetadata) ([]byte, error) {
	contents := []byte{}
	switch {
	case upload.Payload != nil:
		contents = *upload.Payload
	case upload.Text != nil:
		contents = []byte(*upload.Text)
	default:
		for offset := 0; offset < upload.Length; offset += uploadsChunk {
			length := min(uploadsChunk, upload.Length-offset)
			rsp, err := uploadsRequest(scope, "get", map[string]any{"name": upload.Name, "offset": offset, "length": length})
			if err != nil {
				return nil, err
			}
			if rsp.Payload == nil || len(*rsp.Payload) != length {
				got := 0
				if rsp.Payload != nil {
					got = len(*rsp.Payload)
				}
				return nil, fmt.Errorf("%s: read %d of %d bytes at %d", upload.Name, got, length, offset)
			}
			contents = append(contents, *rsp.Payload...)
		}
	}
	if upload.MD5 != "" && !strings.EqualFold(uploadsMD5(contents), upload.MD5) {
		return nil, fmt.Errorf("%s: the bytes read don't match the hub's md5 of them", upload.Name)
	}
	return contents, nil
}

// uploadsMD5 is the md5 of some contents, as the hub records it
func uploadsMD5(contents []byte) string {
	sum := md5.Sum(contents)
	return hex.EncodeToString(sum[:])
}

// uploadsStore stores contents under a name, with tags and notes, and then removes the
// uploads it supersedes, in that order so that a failure leaves the old one in place.
// It returns the upload as the hub recorded it.
func uploadsStore(scope uploadsScope, name string, contents []byte, tags string, notes string, superseded []string) (notehub.UploadMetadata, error) {
	add := map[string]any{"name": name, "payload": contents}
	if tags != "" {
		add["tags"] = tags
	}
	if notes != "" {
		add["filenotes"] = notes
	}
	rsp, err := uploadsRequest(scope, "add", add)

	// Upload names include the second they arrived, so storing one again within a second
	// of its last upload collides with it; the next second gives it its own name
	if err != nil && len(superseded) != 0 {
		time.Sleep(time.Second)
		rsp, err = uploadsRequest(scope, "add", add)
	}
	if err != nil {
		return rsp.UploadMetadata, err
	}
	for _, old := range superseded {
		if err = uploadsRemove(scope, old); err != nil {
			return rsp.UploadMetadata, err
		}
	}
	return rsp.UploadMetadata, nil
}

// uploadsRemove deletes one upload by the name the hub stored it under
func uploadsRemove(scope uploadsScope, name string) error {
	_, err := uploadsRequest(scope, "delete", map[string]any{"name": name})
	return err
}

// uploadsNotes returns the notes given with --notes: the text itself, or a file's
// contents for @file
func uploadsNotes() (string, error) {
	if !strings.HasPrefix(flagUploadsNotes, "@") {
		return flagUploadsNotes, nil
	}
	contents, err := os.ReadFile(flagUploadsNotes[1:])
	if err != nil {
		return "", fmt.Errorf("--notes: %w", err)
	}
	return strings.TrimRight(string(contents), "\n"), nil
}

// uploadsFirstLine is the first line of some notes, for a listing
func uploadsFirstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(line)
}
