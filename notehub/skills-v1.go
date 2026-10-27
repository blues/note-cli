// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// These small wire types keep the CLI independent of an unreleased SDK. HTTP,
// authentication, hub selection and verbose logging use the existing API transport.
type skillsV1Document struct {
	Name      string    `json:"name"`
	Kinds     []string  `json:"kinds"`
	SizeBytes int       `json:"size_bytes"`
	Revision  string    `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   *string   `json:"content,omitempty"`
}

type skillsV1Collection struct {
	Project  string             `json:"project_uid"`
	Revision string             `json:"set_revision"`
	Total    int                `json:"total"`
	Complete bool               `json:"complete"`
	Skills   []skillsV1Document `json:"skills"`
}

const (
	skillsV1MaxContentBytes = 1024 * 1024
	skillsV1MaxRequestBytes = 2*skillsV1MaxContentBytes + 64*1024
)

type skillsV1Write struct {
	Content string   `json:"content"`
	Kinds   []string `json:"kinds"`
}

func skillsV1WriteBody(content []byte, kinds string) skillsV1Write {
	// Always send [] for no kinds; omission would preserve the old kinds.
	tags := []string{}
	if kinds != "" {
		tags = strings.Split(kinds, ",")
	}
	return skillsV1Write{string(content), tags}
}

func skillsV1Request(project, method, name string, query url.Values, headers http.Header, body, result any) (*apiResponse, error) {
	if project == "" {
		var err error
		project, err = skillsProject()
		if err != nil {
			return nil, err
		}
	}
	// Keep the logical path and its escaped form together. Encoding a whole name
	// as one segment preserves slashes, literal percent signs and punctuation.
	path := "/v1/projects/" + project + "/skills"
	escaped := "/v1/projects/" + url.PathEscape(project) + "/skills"
	if name != "" {
		path += "/" + name
		escaped += "/" + url.PathEscape(name)
	}
	endpoint := &url.URL{Path: path, RawPath: escaped, RawQuery: query.Encode()}
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Accept", "application/json")
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	rsp, err := apiSend(skillsClient, &apiRequest{Method: method, URL: endpoint, Header: headers, Body: encoded})
	if err != nil {
		return rsp, err
	}
	if err = apiFailure(rsp); err != nil {
		if rsp.StatusCode == http.StatusPreconditionFailed || rsp.StatusCode == http.StatusConflict {
			return rsp, fmt.Errorf("%w; reread the skills and review your changes before retrying", err)
		}
		return rsp, err
	}
	if (method == http.MethodGet && rsp.StatusCode != http.StatusOK) ||
		(method == http.MethodPut && rsp.StatusCode != http.StatusOK && rsp.StatusCode != http.StatusCreated) ||
		(method == http.MethodDelete && rsp.StatusCode != http.StatusNoContent) {
		return rsp, fmt.Errorf("unexpected Skills response: HTTP %s", rsp.Status)
	}
	if result != nil {
		if err = json.Unmarshal(rsp.Body, result); err != nil {
			return rsp, fmt.Errorf("the hub's Skills response can't be read: %w", err)
		}
	}
	return rsp, nil
}

func (doc skillsV1Document) upload(project string, content bool) (skillsUpload, error) {
	if doc.Name == "" || doc.Revision == "" || doc.SizeBytes < 0 {
		return skillsUpload{}, fmt.Errorf("incomplete Skills metadata for %q", doc.Name)
	}
	upload := skillsUpload{Name: doc.Name, Source: doc.Name, Tags: strings.Join(doc.Kinds, ","),
		Length: doc.SizeBytes, Created: doc.CreatedAt.Unix(), Modified: doc.UpdatedAt.Unix(),
		Revision: doc.Revision, Project: project}
	if content {
		if doc.Content == nil || !utf8.ValidString(*doc.Content) || len(*doc.Content) != doc.SizeBytes {
			return skillsUpload{}, fmt.Errorf("%s: missing or inconsistent skill content", doc.Name)
		}
		upload.Payload = []byte(*doc.Content) // Non-nil for a valid empty document.
	}
	return upload, nil
}

func skillsV1Query(content bool) ([]skillsUpload, error) {
	if !content {
		uploads, _, err := skillsV1Catalog()
		return uploads, err
	}
	var set skillsV1Collection
	_, err := skillsV1Request("", http.MethodGet, "", url.Values{"includeContent": {"true"}}, nil, nil, &set)
	if err != nil {
		return nil, err
	}
	if !set.Complete || set.Project == "" || set.Revision == "" || set.Total != len(set.Skills) {
		return nil, fmt.Errorf("the hub returned an incomplete skill set")
	}
	uploads := make([]skillsUpload, 0, len(set.Skills))
	for _, doc := range set.Skills {
		upload, err := doc.upload(set.Project, true)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

// The metadata catalog is complete in one response; there are no page parameters.
func skillsV1Catalog() ([]skillsUpload, skillsV1Collection, error) {
	var set skillsV1Collection
	if _, err := skillsV1Request("", http.MethodGet, "", nil, nil, nil, &set); err != nil {
		return nil, set, err
	}
	if set.Project == "" || set.Revision == "" || set.Total != len(set.Skills) {
		return nil, set, fmt.Errorf("the hub returned an incomplete Skills catalog")
	}
	uploads := make([]skillsUpload, 0, len(set.Skills))
	for _, doc := range set.Skills {
		upload, err := doc.upload(set.Project, false)
		if err != nil {
			return nil, set, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, set, nil
}

// A named show/get also uses one request. Shorthand is expanded locally; V1
// resolves case-insensitive names without requiring a preceding catalog read.
func skillsV1QueryName(name string) ([]skillsUpload, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name a skill, or 'all' for the whole set")
	}
	if !strings.HasSuffix(strings.ToLower(name), skillsExt) {
		name += skillsExt
	}
	var doc skillsV1Document
	if _, err := skillsV1Request("", http.MethodGet, name, nil, nil, nil, &doc); err != nil {
		return nil, err
	}
	upload, err := doc.upload("", true)
	if err != nil {
		return nil, err
	}
	return []skillsUpload{upload}, nil
}

func skillsV1Read(upload skillsUpload) ([]byte, error) {
	if upload.Revision == "" {
		return nil, fmt.Errorf("%s: missing revision; reread the skill", upload.Source)
	}
	var doc skillsV1Document
	headers := http.Header{"If-Match": {strconv.Quote(upload.Revision)}}
	if _, err := skillsV1Request(upload.Project, http.MethodGet, upload.Source, nil, headers, nil, &doc); err != nil {
		return nil, err
	}
	if doc.Revision != upload.Revision || doc.Name != upload.Source {
		return nil, fmt.Errorf("%s changed while reading; retry the command", upload.Source)
	}
	read, err := doc.upload(upload.Project, true)
	return read.Payload, err
}

func skillsV1Store(name string, content []byte, kinds string, previous *skillsUpload) error {
	if err := skillsV1ValidateWrite(name, content, kinds); err != nil {
		return err
	}
	headers := http.Header{"If-None-Match": {"*"}}
	project := ""
	if previous != nil {
		if previous.Revision == "" || !strings.EqualFold(name, previous.Source) {
			return fmt.Errorf("%s: missing or inconsistent revision; reread the skill", name)
		}
		project = previous.Project
		headers = http.Header{"If-Match": {strconv.Quote(previous.Revision)}}
	}
	body := skillsV1WriteBody(content, kinds)
	var result struct {
		Outcome string           `json:"outcome"`
		Skill   skillsV1Document `json:"skill"`
	}
	if _, err := skillsV1Request(project, http.MethodPut, name, nil, headers, body, &result); err != nil {
		return err
	}
	if _, err := result.Skill.upload(project, true); err != nil {
		return err
	}
	if (result.Outcome != "created" && result.Outcome != "updated" && result.Outcome != "unchanged") ||
		result.Skill.Name != name || *result.Skill.Content != string(content) || strings.Join(result.Skill.Kinds, ",") != kinds {
		return fmt.Errorf("%s: unexpected write result; reread the skill before continuing", name)
	}
	return nil
}

func skillsV1Remove(upload skillsUpload) error {
	if upload.Revision == "" {
		return fmt.Errorf("%s: missing revision; reread the skill", upload.Source)
	}
	_, err := skillsV1Request(upload.Project, http.MethodDelete, upload.Source, nil,
		http.Header{"If-Match": {strconv.Quote(upload.Revision)}}, nil, nil)
	return err
}

// Validate all local files before the first write, including the service's limits.
func skillsV1ValidateWrite(name string, content []byte, kinds string) error {
	if err := skillsCheckName(name); err != nil {
		return err
	}
	if len(name) > 512 || !utf8.ValidString(name) || strings.Contains(name, ":") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s: name must be valid UTF-8, at most 512 bytes, without colons or control characters", name)
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimSpace(part) != part {
			return fmt.Errorf("%s: name segments cannot have surrounding whitespace", name)
		}
	}
	if len(content) > skillsV1MaxContentBytes || !utf8.Valid(content) {
		return fmt.Errorf("%s: content must be valid UTF-8 and at most 1 MiB", name)
	}
	if kinds != "" {
		tags := strings.Split(kinds, ",")
		if len(tags) > 32 {
			return fmt.Errorf("%s: at most 32 kinds are allowed", name)
		}
		for _, tag := range tags {
			if len(tag) > 64 || !skillsKindRE.MatchString(tag) || tag == skillsReservedTag {
				return fmt.Errorf("%s: invalid kind %q (at most 64 lowercase letters, digits, hyphens or underscores; publish is reserved)", name, tag)
			}
		}
	}
	encoded, err := json.Marshal(skillsV1WriteBody(content, kinds))
	if err != nil {
		return err
	}
	if len(encoded) > skillsV1MaxRequestBytes {
		return fmt.Errorf("%s: encoded JSON exceeds 2 MiB plus 64 KiB", name)
	}
	return nil
}
