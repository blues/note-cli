// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A small HTTP contract fixture exercises commands through the real auth/transport
// and JSON paths. It rejects V0 and enforces the public API's write preconditions.
type fakeSkills struct {
	hub       *fakeHub
	docs      map[string]skillsV1Document
	version   int
	oversized bool
	before    func(fakeRequest)
}

func newFakeSkills(t *testing.T) *fakeSkills {
	t.Helper()
	if useV0SkillApis {
		t.Skip("V1 command tests; run with useV0SkillApis = false")
	}
	uploadsTestReset(t)
	s := &fakeSkills{hub: newFakeHub(t), docs: map[string]skillsV1Document{}}
	s.hub.handle = s.handle
	t.Cleanup(func() {
		for _, req := range s.hub.seen {
			if !strings.HasPrefix(req.Path, "/v1/projects/") || !strings.Contains(req.Path, "/skills") {
				t.Errorf("Skills used a non-V1 endpoint: %s", req.Path)
			}
			if req.Header.Get("Authorization") != "Bearer api_key_test" {
				t.Error("Skills did not use the configured token")
			}
		}
	})
	return s
}

func (s *fakeSkills) put(name, content string, kinds ...string) {
	s.version++
	if kinds == nil {
		kinds = []string{}
	}
	s.docs[name] = skillsV1Document{Name: name, Content: &content, Kinds: kinds,
		SizeBytes: len(content), Revision: fmt.Sprintf("%064x", s.version),
		CreatedAt: time.Unix(100, 0).UTC(), UpdatedAt: time.Unix(int64(100+s.version), 0).UTC()}
}

func (s *fakeSkills) handle(r fakeRequest, w http.ResponseWriter) {
	if s.before != nil {
		s.before(r)
	}
	w.Header().Set("Content-Type", "application/json")
	answer := func(status int, value any) {
		w.WriteHeader(status)
		if value != nil {
			_ = json.NewEncoder(w).Encode(value)
		}
	}
	fail := func(status int, message string) {
		answer(status, map[string]any{"err": message, "code": status, "status": http.StatusText(status), "request": r.Path, "details": map[string]any{"reason": "test_error"}})
	}
	if r.Method == http.MethodGet && (strings.HasSuffix(r.Path, "/skills") || strings.HasSuffix(r.Path, "/skills/list")) {
		full := strings.HasSuffix(r.Path, "/skills")
		if full && s.oversized {
			fail(http.StatusRequestEntityTooLarge, "use ListSkills and GetSkill")
			return
		}
		names := make([]string, 0, len(s.docs))
		for name := range s.docs {
			names = append(names, name)
		}
		sort.Strings(names)
		page := skillsV1Collection{Project: "app:1", Revision: strconv.Itoa(s.version),
			Total: len(names), Complete: full, Skills: []skillsV1Document{}}
		for _, name := range names {
			doc := s.docs[name]
			if !full {
				doc.Content = nil
			}
			page.Skills = append(page.Skills, doc)
		}
		answer(http.StatusOK, page)
		return
	}
	if !strings.HasSuffix(r.Path, "/skills/by-name") {
		fail(http.StatusNotFound, "unknown endpoint")
		return
	}
	name, key := r.Query.Get("name"), ""
	var old skillsV1Document
	for candidate, doc := range s.docs {
		if strings.EqualFold(candidate, name) {
			old, key = doc, candidate
			break
		}
	}
	if match := r.Header.Get("If-Match"); match != "" && (key == "" || match != strconv.Quote(old.Revision)) {
		fail(http.StatusPreconditionFailed, "stale revision")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if key == "" {
			fail(http.StatusNotFound, "not found")
			return
		}
		answer(http.StatusOK, old)
	case http.MethodPut:
		if key != "" && r.Header.Get("If-None-Match") == "*" {
			fail(http.StatusPreconditionFailed, "already exists")
			return
		}
		if r.Header.Get("If-Match") == "" && r.Header.Get("If-None-Match") != "*" {
			fail(http.StatusPreconditionRequired, "missing precondition")
			return
		}
		var body struct {
			Content *string  `json:"content"`
			Kinds   []string `json:"kinds"`
		}
		if json.Unmarshal(r.Body, &body) != nil || body.Content == nil || body.Kinds == nil {
			fail(http.StatusBadRequest, "content and explicit kinds required by this CLI")
			return
		}
		status, outcome := http.StatusCreated, "created"
		if key != "" {
			status, outcome = http.StatusOK, "updated"
			if key == name && *old.Content == *body.Content && slices.Equal(old.Kinds, body.Kinds) {
				answer(status, map[string]any{"outcome": "unchanged", "skill": old})
				return
			}
			delete(s.docs, key)
		}
		s.put(name, *body.Content, body.Kinds...)
		answer(status, map[string]any{"outcome": outcome, "skill": s.docs[name]})
	case http.MethodDelete:
		if r.Header.Get("If-Match") == "" {
			fail(http.StatusPreconditionRequired, "missing precondition")
			return
		}
		delete(s.docs, key)
		s.version++
		answer(http.StatusNoContent, nil)
	default:
		fail(http.StatusMethodNotAllowed, "unexpected method")
	}
}

func skillsTestCommand(t *testing.T, run func() error) string {
	t.Helper()
	var err error
	out := captureStdout(t, func() { err = run() })
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSkillsV1Reads(t *testing.T) {
	s := newFakeSkills(t)
	if _, err := skillsStorageSelection(" "); err == nil || len(s.hub.seen) != 0 {
		t.Fatalf("empty name must fail without an HTTP request: %v", err)
	}
	name, content := "references/a +#雪.md", "\ufeff# Café\r\n\r\nExact text.\r\n"
	s.put("INDEX.md", "# Index\nSee references/a +#雪.md.\n", "index")
	s.put(name, content, "reference")
	s.put("empty.md", "")
	flagApp, flagProduct = "", "product:com.example:test"
	out := skillsTestCommand(t, func() error { return skillsListCommand(nil, nil) })
	if !strings.Contains(out, "3 skill(s)") || !strings.Contains(out, name) || len(s.hub.seen) != 1 {
		t.Fatalf("listing: %s; calls: %v", out, s.hub.calls())
	}
	if s.hub.seen[0].Path != "/v1/projects/product:com.example:test/skills/list" {
		t.Fatal("catalog did not use the selected product")
	}
	s.hub.seen = nil
	out = skillsTestCommand(t, func() error { return skillsGetCommand(nil, []string{name, "-"}) })
	if out != content || len(s.hub.seen) != 1 {
		t.Fatalf("single read: %q; calls: %v", out, s.hub.calls())
	}
	last := s.hub.seen[0]
	if last.Query.Get("name") != name || last.Header.Get("If-Match") != "" {
		t.Fatalf("single read lost name or revision: %+v", last)
	}
	s.hub.seen = nil
	out = skillsTestCommand(t, func() error { return skillsShowCommand(nil, []string{"index"}) })
	if out != *s.docs["INDEX.md"].Content {
		t.Fatalf("case-insensitive shorthand: %q", out)
	}
	s.hub.seen = nil
	out = skillsTestCommand(t, func() error { return skillsShowCommand(nil, []string{"all"}) })
	if !strings.Contains(out, "3 documents") || !strings.Contains(out, "INDEX.md") || len(s.hub.seen) != 1 {
		t.Fatalf("full set: %q; calls: %v", out, s.hub.calls())
	}
	s.hub.seen = nil
	dir := t.TempDir()
	skillsTestCommand(t, func() error { return skillsGetCommand(nil, []string{"all", dir}) })
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil || string(got) != content || len(s.hub.seen) != 1 {
		t.Fatalf("get all: %q, %v; calls: %v", got, err, s.hub.calls())
	}
	if got, err = os.ReadFile(filepath.Join(dir, "empty.md")); err != nil || len(got) != 0 {
		t.Fatalf("empty document: %q, %v", got, err)
	}
}

func TestSkillsV1Set(t *testing.T) {
	s := newFakeSkills(t)
	file := filepath.Join(t.TempDir(), "index.md")
	content := "\ufeff---\r\nkind: index\r\n---\r\n# Exact 雪\r\n"
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	skillsTestCommand(t, func() error { return skillsSetCommand(nil, []string{file}) })
	if s.hub.seen[1].Header.Get("If-None-Match") != "*" || *s.docs["index.md"].Content != content {
		t.Fatal("create did not preserve content or require absence")
	}
	old := s.docs["index.md"].Revision
	s.hub.seen = nil
	out := skillsTestCommand(t, func() error { return skillsSetCommand(nil, []string{file}) })
	if !strings.Contains(out, "unchanged") || s.docs["index.md"].Revision != old || len(s.hub.seen) != 2 || s.hub.seen[1].Header.Get("If-Match") != strconv.Quote(old) {
		t.Fatalf("no-op must still check revision and authorization: %s, %v", out, s.hub.calls())
	}
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.hub.seen = nil
	skillsTestCommand(t, func() error { return skillsSetCommand(nil, []string{file}) })
	if *s.docs["index.md"].Content != "" || len(s.docs["index.md"].Kinds) != 0 || !bytes.Contains(s.hub.seen[1].Body, []byte(`"kinds":[]`)) {
		t.Fatalf("empty content/kinds were not cleared: %s", s.hub.seen[1].Body)
	}
	// Directory updates retain unmentioned documents and change display casing safely.
	s.put("keep.md", "keep")
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("case change"), 0600)
	skillsTestCommand(t, func() error { return skillsSetCommand(nil, []string{dir}) })
	if _, oldName := s.docs["index.md"]; oldName || len(s.docs) != 2 || *s.docs["INDEX.md"].Content != "case change" {
		t.Fatalf("directory update: %v", s.docs)
	}
}

func TestSkillsV1Rename(t *testing.T) {
	t.Run("case only", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("index.md", "# Same", "index")
		old := s.docs["index.md"].Revision
		skillsTestCommand(t, func() error { return skillsRenameCommand(nil, []string{"index", "INDEX"}) })
		if len(s.docs) != 1 || s.docs["INDEX.md"].Name == "" || len(s.hub.seen) != 2 || s.hub.seen[1].Header.Get("If-Match") != strconv.Quote(old) {
			t.Fatalf("case-only rename must be one conditional PUT: %v", s.hub.calls())
		}
	})
	t.Run("force replacement", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("old.md", "source", "source")
		s.put("new.md", "destination", "destination")
		source, destination := s.docs["old.md"].Revision, s.docs["new.md"].Revision
		if err := skillsRenameCommand(nil, []string{"old", "new"}); err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("missing force: %v", err)
		}
		s.hub.seen = nil
		flagSkillsForce = true
		skillsTestCommand(t, func() error { return skillsRenameCommand(nil, []string{"old", "new"}) })
		if len(s.docs) != 1 || *s.docs["new.md"].Content != "source" || len(s.hub.seen) != 3 || s.hub.seen[1].Header.Get("If-Match") != strconv.Quote(destination) || s.hub.seen[2].Header.Get("If-Match") != strconv.Quote(source) {
			t.Fatalf("rename did not retain both original revisions: %v", s.hub.calls())
		}
	})
	t.Run("source edited after destination created", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("old.md", "original")
		s.before = func(r fakeRequest) {
			if r.Method == http.MethodDelete {
				s.put("old.md", "concurrent edit")
			}
		}
		err := skillsRenameCommand(nil, []string{"old", "new"})
		if err == nil || !strings.Contains(err.Error(), "both names may remain") || len(s.docs) != 2 || *s.docs["old.md"].Content != "concurrent edit" || *s.docs["new.md"].Content != "original" || len(s.hub.seen) != 3 {
			t.Fatalf("rename conflict lost data or retried: %v; %v", err, s.hub.calls())
		}
	})
}

func TestSkillsV1BackupRestoreAndDelete(t *testing.T) {
	s := newFakeSkills(t)
	s.put("INDEX.md", "# Index\n", "index")
	s.put("references/a.md", "---\nkind: guide\n---\n# Guide\n", "guide")
	s.put("cleared.md", "---\nkinds: [stale]\ntags: older\n---\n# No effective kinds\n")
	archive := filepath.Join(t.TempDir(), "skills.zip")
	skillsTestCommand(t, func() error { return skillsBackupCommand(nil, []string{archive}) })
	files, err := skillsReadZip(archive)
	if err != nil || len(files) != 3 || len(s.hub.seen) != 1 {
		t.Fatalf("backup: %v, %v", files, err)
	}
	s.docs = map[string]skillsV1Document{}
	s.put("index.md", "old spelling and content")
	s.put("remove.md", "obsolete")
	s.hub.seen = nil
	skillsTestCommand(t, func() error { return skillsRestoreCommand(nil, []string{archive}) })
	if len(s.docs) != 3 || s.docs["INDEX.md"].Name == "" || *s.docs["INDEX.md"].Content != string(files["INDEX.md"]) || !slices.Equal(s.docs["INDEX.md"].Kinds, []string{"index"}) || len(s.docs["cleared.md"].Kinds) != 0 {
		t.Fatalf("restore lost names, content or effective kinds: %v", s.docs)
	}
	deleted := false
	for _, r := range s.hub.seen {
		if r.Method == http.MethodDelete {
			deleted = true
			if r.Query.Get("name") != "remove.md" {
				t.Fatalf("restore deleted a case-only replacement: %s", r.Query)
			}
		}
		if deleted && r.Method == http.MethodPut {
			t.Fatal("restore removed skills before writes finished")
		}
	}
	if err = skillsDeleteCommand(nil, []string{"all"}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("delete-all lost force guard: %v", err)
	}
	skillsTestCommand(t, func() error { return skillsDeleteCommand(nil, []string{"references/a"}) })
	flagSkillsForce = true
	skillsTestCommand(t, func() error { return skillsDeleteCommand(nil, []string{"all"}) })
	if len(s.docs) != 0 {
		t.Fatalf("delete all: %v", s.docs)
	}
}

func TestSkillsV1ConflictsAndFailures(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusPreconditionFailed, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			s := newFakeSkills(t)
			s.put("index.md", "original")
			uploads, err := skillsStorageQuery(true)
			if err != nil {
				t.Fatal(err)
			}
			s.hub.seen = nil
			s.hub.handle = func(r fakeRequest, w http.ResponseWriter) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"err":"denied by test","code":%d,"status":%q}`, status, http.StatusText(status))))
			}
			_, _, err = skillsStore(map[string][]byte{"index.md": []byte("replacement")}, uploads, 8)
			if err == nil || !strings.Contains(err.Error(), "denied by test") || len(s.hub.seen) != 1 || s.hub.seen[0].Method != http.MethodPut {
				t.Fatalf("failed write must stop without retry/fallback: %v, %v", err, s.hub.calls())
			}
		})
	}
	t.Run("create raced", func(t *testing.T) {
		s := newFakeSkills(t)
		s.before = func(r fakeRequest) {
			if r.Method == http.MethodPut {
				s.put("index.md", "other writer")
			}
		}
		err := skillsStorageStore("index.md", []byte("mine"), "", nil)
		if err == nil || !strings.Contains(err.Error(), "412") || len(s.hub.seen) != 1 || *s.docs["index.md"].Content != "other writer" {
			t.Fatalf("create race: %v", err)
		}
	})
	t.Run("stale read", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("empty.md", "")
		uploads, _ := skillsStorageQuery(false)
		s.put("empty.md", "other writer")
		if _, err := skillsStorageRead(uploads[0]); err == nil || !strings.Contains(err.Error(), "412") {
			t.Fatalf("empty metadata read bypassed revision: %v", err)
		}
	})
	t.Run("restore stops before deletes", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("keep.md", "existing")
		archive := filepath.Join(t.TempDir(), "skills.zip")
		if err := skillsWriteZip(archive, []skillsZipEntry{{Name: "new.md", Contents: []byte("new")}}, ""); err != nil {
			t.Fatal(err)
		}
		s.before = func(r fakeRequest) {
			if r.Method == http.MethodPut {
				s.put("new.md", "concurrent")
			}
		}
		if err := skillsRestoreCommand(nil, []string{archive}); err == nil || !strings.Contains(err.Error(), "412") {
			t.Fatalf("restore conflict: %v", err)
		}
		for _, r := range s.hub.seen {
			if r.Method == http.MethodDelete {
				t.Fatal("restore deleted despite failed write")
			}
		}
	})
	t.Run("restore verifies final set", func(t *testing.T) {
		s := newFakeSkills(t)
		archive := filepath.Join(t.TempDir(), "skills.zip")
		if err := skillsWriteZip(archive, []skillsZipEntry{{Name: "new.md", Contents: []byte("new")}}, ""); err != nil {
			t.Fatal(err)
		}
		reads := 0
		s.before = func(r fakeRequest) {
			if r.Method == http.MethodGet && strings.HasSuffix(r.Path, "/skills") {
				reads++
				if reads == 2 {
					s.put("new.md", "concurrent edit after restore's PUT")
				}
			}
		}
		var err error
		_ = captureStdout(t, func() { err = skillsRestoreCommand(nil, []string{archive}) })
		if err == nil || !strings.Contains(err.Error(), "final verification failed") {
			t.Fatalf("restore silently accepted concurrent edit: %v", err)
		}
	})
	t.Run("invalid write response stops rename", func(t *testing.T) {
		s := newFakeSkills(t)
		s.put("old.md", "original")
		s.hub.handle = func(r fakeRequest, w http.ResponseWriter) {
			if r.Method == http.MethodPut {
				_, _ = w.Write([]byte(`{"outcome":"created","skill":{"name":"new.md"}}`))
				return
			}
			s.handle(r, w)
		}
		if err := skillsRenameCommand(nil, []string{"old", "new"}); err == nil {
			t.Fatal("rename accepted incomplete write response")
		}
		if len(s.hub.seen) != 2 || s.docs["old.md"].Name == "" {
			t.Fatalf("rename deleted source without confirming destination: %v", s.hub.calls())
		}
	})
}

func TestSkillsV1OversizedSet(t *testing.T) {
	s := newFakeSkills(t)
	s.put("a.md", "one")
	s.put("b.md", "two")
	s.oversized = true
	if _, err := skillsStorageQuery(true); err == nil || !strings.Contains(err.Error(), "413") {
		t.Fatalf("oversized set was accepted: %v", err)
	}
	if len(s.hub.seen) != 1 {
		t.Fatal("a full-set read must not silently become multiple requests")
	}
	s.hub.seen = nil
	catalog, err := skillsStorageQuery(false)
	if err != nil || len(catalog) != 2 || len(s.hub.seen) != 1 || len(s.hub.seen[0].Query) != 0 {
		t.Fatalf("catalog must remain one complete read: %v, %v", catalog, err)
	}
	s.hub.seen = nil
	out := skillsTestCommand(t, func() error { return skillsGetCommand(nil, []string{"a", "-"}) })
	if out != "one" || len(s.hub.seen) != 1 {
		t.Fatalf("individual read: %q; %v", out, s.hub.calls())
	}
}

func TestSkillsV1DryRunsAndValidation(t *testing.T) {
	s := newFakeSkills(t)
	s.put("index.md", "original")
	file := filepath.Join(t.TempDir(), "new.md")
	_ = os.WriteFile(file, []byte("new"), 0600)
	archive := filepath.Join(t.TempDir(), "skills.zip")
	_ = skillsWriteZip(archive, []skillsZipEntry{{Name: "new.md", Contents: []byte("new")}}, "")
	flagSkillsDryRun = true
	for _, run := range []func() error{
		func() error { return skillsSetCommand(nil, []string{file}) },
		func() error { return skillsRenameCommand(nil, []string{"index", "new"}) },
		func() error { return skillsDeleteCommand(nil, []string{"all"}) },
		func() error { return skillsRestoreCommand(nil, []string{archive}) },
		func() error { return skillsBackupCommand(nil, []string{archive + ".zip"}) },
	} {
		skillsTestCommand(t, run)
	}
	for _, r := range s.hub.seen {
		if r.Method != http.MethodGet {
			t.Fatalf("dry run mutated: %s", r.Method)
		}
	}
	if _, err := os.Stat(archive + ".zip"); !os.IsNotExist(err) {
		t.Fatal("dry-run backup wrote a file")
	}
	flagSkillsDryRun = false
	s.hub.seen = nil
	for _, invalid := range []map[string][]byte{
		{"a.md": []byte("valid"), "z.md": {0xff}},
		{"a.md": []byte("valid"), "Z.md": []byte("upper"), "z.md": []byte("lower")},
		{"a.md": []byte("valid"), "bad:name.md": []byte("invalid")},
		{"a.md": []byte("valid"), "z.md": []byte("---\nkind: " + strings.Repeat("a", 65) + "\n---\n")},
		{"a.md": []byte("valid"), "z.md": bytes.Repeat([]byte("x"), 1024*1024+1)},
		{"a.md": []byte("valid"), "z.md": bytes.Repeat([]byte{1}, skillsV1MaxContentBytes)},
	} {
		if _, _, err := skillsStore(invalid, nil, 10); err == nil {
			t.Fatal("invalid set was accepted")
		}
		if len(s.hub.seen) != 0 {
			t.Fatal("validation happened after a write")
		}
	}
}

// This test is run with a Go source overlay setting the compile-time switch true.
// It covers the retained V0 query/read/add/delete flow without changing the checkout.
func TestSkillsV0Selected(t *testing.T) {
	if !useV0SkillApis {
		t.Skip("V0 selection test; run with useV0SkillApis = true")
	}
	uploadsTestReset(t)
	hub := newFakeHub(t)
	hub.handle = func(r fakeRequest, w http.ResponseWriter) {
		var body any = map[string]any{}
		switch r.Req["req"] {
		case "hub.app.upload.query":
			body = skillsUploadResponse{Uploads: []skillsUpload{{Name: "index$old.md", Source: "index.md", Length: 3}}}
		case "hub.app.upload.get":
			body = skillsUploadResponse{Payload: []byte("old")}
		case "hub.app.upload.add":
			body = skillsUploadResponse{Name: "index$new.md"}
		case "hub.app.upload.delete":
		default:
			t.Errorf("unexpected V0 request: %+v", r)
		}
		_ = json.NewEncoder(w).Encode(body)
	}
	uploads, err := skillsStorageQuery(false)
	if err != nil || len(uploads) != 1 {
		t.Fatalf("query: %v, %v", uploads, err)
	}
	content, err := skillsStorageRead(uploads[0])
	if err != nil || string(content) != "old" {
		t.Fatalf("read: %q, %v", content, err)
	}
	if err = skillsStorageStore("index.md", []byte("new"), "index", uploads); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hub.calls(), []string{"hub.app.upload.query", "hub.app.upload.get", "hub.app.upload.add", "hub.app.upload.delete"}) {
		t.Fatalf("V0 selection: %v", hub.calls())
	}
	for _, r := range hub.seen {
		if r.Path != "/req" || r.Query.Get("app") != "app:1" || r.Req["type"] != "skill" {
			t.Fatalf("incorrect V0 scope: %+v", r)
		}
	}
}
