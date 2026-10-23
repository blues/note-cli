// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
)

// Uploads detect the binary's product before requiring a project. Only Notecard
// uploads default to Airnote; explicit project, product and type choices take priority.
func TestFirmwareUploadSelection(t *testing.T) {
	const notecard = "binary firmware::info:{\"product\":\"Notecard\",\"ver_major\":11,\"ver_minor\":4,\"ver_patch\":1,\"ver_build\":5602}\x00"
	const host = "firmware::info:{\"product\":\"Airnote\",\"ver_major\":1}"
	for _, test := range []struct {
		name, verb, contents string
		flags                []string
		project, ftype       string
		version              string
		wantError            string
		dryRun               bool
	}{
		{name: "detected Notecard", contents: notecard, project: firmwareNotecardProject, ftype: firmwareNotecard},
		{name: "set alias", verb: "set", contents: notecard, project: firmwareNotecardProject, ftype: firmwareNotecard},
		{name: "explicit project", contents: notecard, flags: []string{"--project", "app:chosen"}, project: "app:chosen", ftype: firmwareNotecard},
		{name: "explicit product", contents: notecard, flags: []string{"--product", "com.example:chosen"}, project: "com.example:chosen", ftype: firmwareNotecard},
		{name: "explicit Notecard type", contents: "firmware", flags: []string{"--type", "notecard"}, project: firmwareNotecardProject, ftype: firmwareNotecard},
		{name: "explicit host type", contents: notecard, flags: []string{"--type", "host", "--project", "app:chosen"}, project: "app:chosen", ftype: firmwareHost},
		{name: "explicit host needs project", contents: notecard, flags: []string{"--type", "host"}, wantError: "specify the project"},
		{name: "host needs project", contents: host, wantError: "specify the project"},
		{name: "host with project", contents: host, flags: []string{"--project", "app:chosen"}, project: "app:chosen", ftype: firmwareHost},
		{name: "host version", contents: host, flags: []string{"--version", "2.3.4", "--project", "app:chosen"}, project: "app:chosen", ftype: firmwareHost, version: "2.3.4"},
		{name: "missing metadata", contents: "notecard firmware", wantError: "specify the project"},
		{name: "malformed metadata", contents: "firmware::info:{\"product\":\"Notecard\",", wantError: "specify the project"},
		{name: "detected Notecard rejects version", contents: notecard, flags: []string{"--version", "1.0"}, wantError: "--version doesn't apply"},
		{name: "dry run", contents: notecard, flags: []string{"--dry-run"}, project: firmwareNotecardProject, ftype: firmwareNotecard, dryRun: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			uploadsTestReset(t)
			savedFlags := flag.CommandLine
			flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
			t.Cleanup(func() { flag.CommandLine = savedFlags })
			mode := cli.ModeNamed(modeFirmware)
			cli.RegisterSwitches(mode)

			// Neither the path nor the optional upload name determines the type.
			local := filepath.Join(t.TempDir(), "notecard-u5.bin")
			if err := os.WriteFile(local, []byte(test.contents), 0600); err != nil {
				t.Fatal(err)
			}
			hub := newFakeHub(t)
			hub.handle = func(r fakeRequest, w http.ResponseWriter) {
				switch {
				case r.Req["req"] == "hub.upload.query":
					w.Write([]byte(`{"uploads":[]}`))
				case r.Method == http.MethodGet:
					w.Write([]byte(`[]`))
				case r.Method == http.MethodPut:
					w.Write([]byte(`{"filename":"renamed$1.bin","version":"11.4.1.5602"}`))
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}
			verb := test.verb
			if verb == "" {
				verb = "upload"
			}
			args := append([]string{verb, local, "renamed.bin", "--notes", "test build"}, test.flags...)
			var err error
			out := captureStdout(t, func() { err = cli.Dispatch(mode, nil, args) })
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("got %v, expected %q", err, test.wantError)
				}
				if len(hub.seen) != 0 {
					t.Fatalf("invalid upload called %v", hub.calls())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			calls := []string{"GET /v1/projects/" + test.project + "/firmware"}
			if test.ftype == firmwareNotecard {
				calls[0] = "hub.upload.query"
			}
			if !test.dryRun {
				calls = append(calls, "PUT /v1/projects/"+test.project+"/firmware/"+test.ftype+"/renamed.bin")
			}
			if !slices.Equal(hub.calls(), calls) {
				t.Fatalf("called %v, want %v", hub.calls(), calls)
			}
			if test.ftype == firmwareNotecard && hub.seen[0].Req["type"] != firmwareNotecard {
				t.Errorf("queried %v, expected Notecard firmware", hub.seen[0].Req)
			}
			if !strings.Contains(out, test.project) {
				t.Errorf("upload did not report its project: %q", out)
			}
			if test.dryRun {
				if !strings.Contains(out, "Nothing was changed") {
					t.Errorf("dry run wrote %q", out)
				}
				return
			}
			put := hub.seen[1]
			if string(put.Body) != test.contents || put.Header.Get("Content-Type") != "application/octet-stream" || put.Query.Get("notes") != "test build" {
				t.Errorf("upload did not send the binary and notes unchanged")
			}
			if put.Query.Get("version") != test.version {
				t.Errorf("upload version = %q, want %q", put.Query.Get("version"), test.version)
			}
		})
	}
}

// Replacing Notecard firmware uploads through v1 before removing the old global
// upload. Identical bytes, --no-replace and an upload failure all keep the old one.
func TestFirmwareNotecardReplacement(t *testing.T) {
	for _, state := range []string{"replaced", "unchanged", "no-replace", "failed"} {
		t.Run(state, func(t *testing.T) {
			uploadsTestReset(t)
			flagApp = ""
			flagUploadsNoReplace = state == "no-replace"
			contents := []byte(`firmware::info:{"product":"Notecard","ver_major":11,"ver_minor":4,"ver_patch":1,"ver_build":5602}`)
			local := filepath.Join(t.TempDir(), "notecard.bin")
			if err := os.WriteFile(local, contents, 0600); err != nil {
				t.Fatal(err)
			}
			hub := newFakeHub(t)
			hub.handle = func(r fakeRequest, w http.ResponseWriter) {
				switch {
				case r.Req["req"] == "hub.upload.query":
					md5 := "old"
					if state == "unchanged" {
						md5 = uploadsMD5(contents)
					}
					out, _ := note.JSONMarshal(map[string]any{"uploads": []map[string]any{{
						"name": "notecard$1.bin", "source": "notecard.bin", "md5": md5,
						"firmware": map[string]any{"ver_major": 11, "ver_minor": 4, "ver_patch": 1, "ver_build": 5602},
					}}})
					w.Write(out)
				case r.Method == http.MethodPut:
					if state == "failed" {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(`{"err":"upload failed"}`))
						return
					}
					w.Write([]byte(`{"filename":"notecard$2.bin","version":"11.4.1.5602"}`))
				case r.Req["req"] == "hub.upload.delete":
					if r.Req["name"] != "notecard$1.bin" || r.Req["type"] != firmwareNotecard {
						t.Errorf("deleted unexpected firmware: %v", r.Req)
					}
					w.Write([]byte(`{}`))
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}
			var err error
			out := captureStdout(t, func() { err = firmwareUploadCommand(nil, []string{local}) })
			wantError := state == "no-replace" || state == "failed"
			if (err != nil) != wantError {
				t.Fatalf("got %v, expected error: %v", err, wantError)
			}
			calls := []string{"hub.upload.query"}
			if state == "replaced" || state == "failed" {
				calls = append(calls, "PUT /v1/projects/"+firmwareNotecardProject+"/firmware/notecard/notecard.bin")
			}
			if state == "replaced" {
				calls = append(calls, "hub.upload.delete")
			}
			if !slices.Equal(hub.calls(), calls) {
				t.Errorf("called %v, want %v", hub.calls(), calls)
			}
			if !wantError && !strings.Contains(out, state) {
				t.Errorf("upload wrote %q, want %q", out, state)
			}
		})
	}
}

// A stored filename carries the hub's stamp, and a Blues-style binary its version
func TestFirmwareNames(t *testing.T) {
	for stored, original := range map[string]string{
		"app$20260324190911.bin": "app.bin",
		"app.bin":                "app.bin",
		"notecard-u5-8.1.3.17044$20241220135610.bin": "notecard-u5-8.1.3.17044.bin",
	} {
		if got := firmwareOriginal(stored); got != original {
			t.Errorf("%s: got %s, want %s", stored, got, original)
		}
	}
	for contents, version := range map[string]string{
		"junk firmware::info:{\"ver_major\":1,\"ver_minor\":2,\"ver_patch\":3,\"ver_build\":0} junk": "1.2.3",
		"firmware::info:{\"ver_major\":1,\"ver_minor\":2,\"ver_patch\":3,\"ver_build\":44}\x00":      "1.2.3.44",
		"firmware::info:{\"version\":\"v7-rc1\"}":                                                    "v7-rc1",
		"firmware::info:not json": "",
		"no info at all":          "",
	} {
		if got := firmwareEmbeddedVersion([]byte(contents)); got != version {
			t.Errorf("%q: got %q, want %q", contents, got, version)
		}
	}
}

// Versions are the same with or without trailing zero numbers, as the hub writes them
func TestFirmwareSameVersion(t *testing.T) {
	for _, test := range []struct {
		a, b string
		same bool
	}{
		{"7.1.3", "7.1.3.0", true}, {"7.1.3.0", "7.1.3", true}, {"1.2.3.44", "1.2.3.44", true}, {"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false}, {"1.2.3.1", "1.2.3", false}, {"", "", true}, {"", "1.0", false}, {"v7-rc1", "v7-rc1", true}, {"v7-rc1", "v7-rc2", false},
	} {
		if got := firmwareSameVersion(test.a, test.b); got != test.same {
			t.Errorf("%q vs %q: got %v, want %v", test.a, test.b, got, test.same)
		}
	}
}

// A firmware is named by its stored filename, or by its original name or version when
// that names just one
func TestFirmwareResolve(t *testing.T) {
	items := []firmwareItem{
		{Filename: "app$1.bin", Version: "1.0.0"},
		{Filename: "app$2.bin", Version: "1.1.0"},
		{Filename: "other$3.bin", Version: "1.0.0"},
	}
	for name, want := range map[string]string{"app$2.bin": "app$2.bin", "APP$2.BIN": "app$2.bin", "1.1.0": "app$2.bin", "other.bin": "other$3.bin"} {
		if item, err := firmwareResolve(items, name); err != nil || item.Filename != want {
			t.Errorf("%s: got %s (%v), want %s", name, item.Filename, err, want)
		}
	}
	for name, mention := range map[string]string{"app.bin": "more than one", "1.0.0": "more than one", "nosuch": "not one of"} {
		if _, err := firmwareResolve(items, name); err == nil || !strings.Contains(err.Error(), mention) {
			t.Errorf("%s: got %v, expected an error saying '%s'", name, err, mention)
		}
	}
}

// A fake hub's host firmware, answering the v1 firmware calls
type fakeFirmware struct {
	items   []firmwareItem
	bytes   map[string][]byte
	version string // what an upload is recorded as, when none is given
}

func (store *fakeFirmware) handle(r fakeRequest, w http.ResponseWriter) {
	answer := func(value any) {
		out, _ := note.JSONMarshal(value)
		w.Write(out)
	}
	switch {
	case r.Method == "GET" && r.Path == "/v1/projects/app:1/firmware":
		answer(store.items)
	case r.Method == "PUT" && strings.HasPrefix(r.Path, "/v1/projects/app:1/firmware/host/"):
		name := strings.TrimPrefix(r.Path, "/v1/projects/app:1/firmware/host/")
		version := r.Query.Get("version")
		if version == "" {
			version = store.version
		}
		answer(firmwareItem{Filename: name + "$20260101000001", Version: version, MD5: uploadsMD5(r.Body), Notes: r.Query.Get("notes")})
	case r.Method == "GET" && strings.HasPrefix(r.Path, "/v1/projects/app:1/firmware/host/"):
		w.Write(store.bytes[strings.TrimPrefix(r.Path, "/v1/projects/app:1/firmware/host/")])
	case r.Method == "POST" && strings.HasPrefix(r.Path, "/v1/projects/app:1/firmware/host/"):
		answer(map[string]any{})
	case r.Method == "DELETE" && strings.HasPrefix(r.Path, "/v1/projects/app:1/firmware/host/"):
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
		answer(map[string]any{"err": "unexpected " + r.Method + " " + r.Path})
	}
}

// Host firmware is listed, uploaded, downloaded, edited and deleted through the v1 calls,
// with an upload skipped when the project holds its bytes and replacing one of the same
// name and version, new one up before old one down
func TestFirmwareHost(t *testing.T) {
	uploadsTestReset(t)
	hub := newFakeHub(t)
	old := []byte("firmware::info:{\"ver_major\":1,\"ver_minor\":2,\"ver_patch\":3} old")
	store := &fakeFirmware{
		items: []firmwareItem{{Filename: "app$20260101000000.bin", Version: "1.2.3", MD5: uploadsMD5(old), Created: "2026-01-01T00:00:00Z", Notes: "first\nmore"}},
		bytes: map[string][]byte{"app$20260101000000.bin": old},
	}
	hub.handle = store.handle

	out := captureStdout(t, func() {
		if err := firmwareListCommand(nil, nil); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "app$20260101000000.bin") || !strings.Contains(out, "1.2.3") || !strings.Contains(out, "first") || strings.Contains(out, "more") {
		t.Errorf("list wrote %q", out)
	}
	if q := hub.seen[0].Query; q.Get("firmwareType") != "host" || q.Get("sortOrder") != "desc" {
		t.Errorf("list asked with %v", q)
	}

	// get, named as uploaded, verified against the md5
	dir := t.TempDir()
	out = captureStdout(t, func() {
		if err := firmwareGetCommand(nil, []string{"1.2.3", dir}); err != nil {
			t.Error(err)
		}
	})
	if got, _ := os.ReadFile(filepath.Join(dir, "app.bin")); string(got) != string(old) || !strings.Contains(out, "app.bin") {
		t.Errorf("get wrote %q and said %q", got, out)
	}
	store.bytes["app$20260101000000.bin"] = []byte("corrupt")
	if err := firmwareGetCommand(nil, []string{"1.2.3", dir}); err == nil || !strings.Contains(err.Error(), "md5") {
		t.Errorf("a download that doesn't match its md5 got %v", err)
	}
	store.bytes["app$20260101000000.bin"] = old

	// upload: the same bytes are skipped; the same name and version, with other bytes, is
	// replaced after the upload; --no-replace refuses; a new version is simply new
	local := filepath.Join(dir, "app.bin")
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := firmwareUploadCommand(nil, []string{local}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "unchanged") || len(hub.seen) != 1 {
		t.Errorf("uploading the same bytes wrote %q after %v", out, hub.calls())
	}
	os.WriteFile(local, append(old, []byte(" rebuilt")...), 0666)
	flagUploadsNoReplace = true
	if err := firmwareUploadCommand(nil, []string{local}); err == nil || !strings.Contains(err.Error(), "--no-replace") {
		t.Errorf("with --no-replace got %v", err)
	}
	flagUploadsNoReplace = false
	flagUploadsNotes = "rebuilt"
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := firmwareUploadCommand(nil, []string{local}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "replaced") || !slices.Equal(hub.calls(), []string{"GET /v1/projects/app:1/firmware", "PUT /v1/projects/app:1/firmware/host/app.bin", "DELETE /v1/projects/app:1/firmware/host/app$20260101000000.bin"}) {
		t.Errorf("replacing wrote %q and called %v", out, hub.calls())
	}
	if put := hub.seen[1]; put.Query.Get("version") != "" || put.Query.Get("notes") != "rebuilt" || string(put.Body) != string(old)+" rebuilt" {
		t.Errorf("the upload was %v with body %q", put.Query, put.Body)
	}
	os.WriteFile(local, []byte("firmware::info:{\"ver_major\":1,\"ver_minor\":3,\"ver_patch\":0}"), 0666)
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := firmwareUploadCommand(nil, []string{local}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "new") || !strings.Contains(out, "1.3.0") || len(hub.seen) != 2 {
		t.Errorf("a new version wrote %q after %v", out, hub.calls())
	}

	// --version is passed through, and --dry-run sends nothing
	flagFirmwareVersion, flagSkillsDryRun = "9.9.9", true
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := firmwareUploadCommand(nil, []string{local}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "9.9.9") || !strings.Contains(out, "Nothing was changed") || len(hub.seen) != 1 {
		t.Errorf("a dry run wrote %q after %v", out, hub.calls())
	}
	flagFirmwareVersion, flagSkillsDryRun = "", false

	// edit and delete
	flagUploadsNotes, flagFirmwareInfo = "", `{"ticket":42}`
	hub.seen = nil
	if err := firmwareEditCommand(nil, []string{"1.2.3"}); err != nil {
		t.Error(err)
	}
	if post := hub.seen[1]; post.Method != "POST" || string(post.Body) != `{"info":{"ticket":42}}` {
		t.Errorf("edit sent %s %s with %s", post.Method, post.Path, post.Body)
	}
	flagFirmwareInfo = ""
	if err := firmwareEditCommand(nil, []string{"1.2.3"}); err == nil {
		t.Errorf("an edit with nothing to change was accepted")
	}
	hub.seen = nil
	if err := firmwareDeleteCommand(nil, []string{"1.2.3"}); err != nil {
		t.Error(err)
	}
	if !slices.Equal(hub.calls(), []string{"GET /v1/projects/app:1/firmware", "DELETE /v1/projects/app:1/firmware/host/app$20260101000000.bin"}) {
		t.Errorf("delete called %v", hub.calls())
	}
}

// Updates are requested per target that --scope names, with fleets resolved by name,
// and status is gathered across every page
func TestFirmwareDfu(t *testing.T) {
	uploadsTestReset(t)
	hub := newFakeHub(t)
	store := &fakeFirmware{items: []firmwareItem{{Filename: "app$1.bin", Version: "1.2.3"}}}
	hub.handle = func(r fakeRequest, w http.ResponseWriter) {
		answer := func(value any) {
			out, _ := note.JSONMarshal(value)
			w.Write(out)
		}
		switch {
		case r.Path == "/v1/projects/app:1/fleets":
			answer(map[string]any{"fleets": []map[string]any{{"uid": "fleet:1", "label": "Field Units"}, {"uid": "fleet:2", "label": "Lab"}}})
		case strings.HasPrefix(r.Path, "/v1/projects/app:1/dfu/host/update") || strings.HasPrefix(r.Path, "/v1/projects/app:1/dfu/host/cancel"):
			answer(map[string]any{})
		case r.Path == "/v1/projects/app:1/dfu/host/status":
			page := r.Query.Get("pageNum")
			answer(map[string]any{"has_more": page == "1", "devices": []map[string]any{{
				"device_uid": "dev:" + page, "dfu_in_progress": page == "1", "current": map[string]any{"version": "1.2.2"},
				"status": map[string]any{"requested_version": "1.2.3", "current_version": "1.2.2", "initiated": "2026-10-08T12:00:00Z",
					"updates": []map[string]any{{"phase": "downloading", "datetime": "2026-10-08T12:05:00Z", "description": "40%"}}},
			}}})
		default:
			store.handle(r, w)
		}
	}

	// Without a scope an update is refused; all needs --force
	if err := firmwareUpdateCommand(nil, []string{"1.2.3"}); err == nil || !strings.Contains(err.Error(), "--scope") {
		t.Errorf("without --scope got %v", err)
	}
	flagScope = "all"
	if err := firmwareUpdateCommand(nil, []string{"1.2.3"}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("--scope all without --force got %v", err)
	}

	// Devices in one call, each fleet in its own, narrowed by --tag
	flagScope, flagFirmwareTags = "dev:1, dev:2, Field Units, fleet:9", cli.StringList{"beta"}
	hub.seen = nil
	out := captureStdout(t, func() {
		if err := firmwareUpdateCommand(nil, []string{"1.2.3"}); err != nil {
			t.Error(err)
		}
	})
	calls := []string{}
	for _, r := range hub.seen {
		if strings.Contains(r.Path, "/dfu/") {
			calls = append(calls, r.Query.Encode()+" "+string(r.Body))
		}
	}
	if !slices.Equal(calls, []string{
		`deviceUID=dev%3A1&deviceUID=dev%3A2&tag=beta {"filename":"app$1.bin"}`,
		`fleetUID=fleet%3A1&tag=beta {"filename":"app$1.bin"}`,
		`fleetUID=fleet%3A9&tag=beta {"filename":"app$1.bin"}`,
	}) {
		t.Errorf("update made these DFU calls: %v", calls)
	}
	if !strings.Contains(out, "3 target(s)") {
		t.Errorf("update wrote %q", out)
	}
	if _, _, err := firmwareScopeItems("app:1", "Nowhere"); err == nil {
		t.Errorf("a fleet name that matches nothing was accepted")
	}

	// cancel sends an empty request; status walks every page
	flagScope, flagFirmwareTags = "fleet:1", nil
	hub.seen = nil
	if err := firmwareCancelCommand(nil, nil); err != nil {
		t.Error(err)
	}
	if last := hub.seen[len(hub.seen)-1]; !strings.HasSuffix(last.Path, "/cancel") || string(last.Body) != "{}" {
		t.Errorf("cancel sent %s with %s", last.Path, last.Body)
	}
	flagScope = ""
	out = captureStdout(t, func() {
		if err := firmwareStatusCommand(nil, nil); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "dev:1") || !strings.Contains(out, "dev:2") || !strings.Contains(out, "update to 1.2.3 under way: downloading at") || !strings.Contains(out, "40%") {
		t.Errorf("status wrote %q", out)
	}
	flagJson = true
	out = captureStdout(t, func() {
		if err := firmwareStatusCommand(nil, nil); err != nil {
			t.Error(err)
		}
	})
	var report struct {
		Devices []dfuDevice `json:"devices"`
	}
	if err := note.JSONUnmarshal([]byte(out), &report); err != nil || len(report.Devices) != 2 || report.Devices[0].DeviceUID != "dev:1" || report.Devices[1].DeviceUID != "dev:2" {
		t.Errorf("status --json wrote %q (%v)", out, err)
	}
}

// Notecard firmware is the hub's own: listed and fetched without a project through the
// v0 calls, never edited, and applied to devices only in a named project
func TestFirmwareNotecard(t *testing.T) {
	uploadsTestReset(t)
	flagFirmwareType, flagApp = "notecard", ""
	hub := newFakeHub(t)
	hub.handle = func(r fakeRequest, w http.ResponseWriter) {
		out, _ := note.JSONMarshal(map[string]any{"uploads": []map[string]any{{
			"name": "notecard-u5-8.1.3.17044$20241220135610.bin", "source": "notecard-u5-8.1.3.17044.bin", "length": 4, "created": 1734700570, "tags": "publish",
			"firmware": map[string]any{"ver_major": 8, "ver_minor": 1, "ver_patch": 3, "ver_build": 17044, "target": "u5", "org": "Blues"},
		}}})
		w.Write(out)
	}
	out := captureStdout(t, func() {
		if err := firmwareListCommand(nil, nil); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "notecard-u5-8.1.3.17044$20241220135610.bin") || !strings.Contains(out, "8.1.3.17044") {
		t.Errorf("list wrote %q", out)
	}
	if r := hub.seen[0]; r.Req["req"] != "hub.upload.query" || r.Req["type"] != "notecard" || len(r.Query) != 0 {
		t.Errorf("list asked %v with query %v, expected the hub's own uploads with no project", r.Req, r.Query)
	}
	out = captureStdout(t, func() {
		if err := firmwareShowCommand(nil, []string{"8.1.3.17044"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "target:       u5") || !strings.Contains(out, "published:    true") {
		t.Errorf("show wrote %q", out)
	}
	flagFirmwareVersion = "1.0"
	local := filepath.Join(t.TempDir(), "x.bin")
	if err := os.WriteFile(local, []byte("firmware"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := firmwareUploadCommand(nil, []string{local}); err == nil || !strings.Contains(err.Error(), "--version") {
		t.Errorf("upload --version on notecard firmware got %v", err)
	}
	flagFirmwareVersion = ""
	if err := firmwareEditCommand(nil, []string{"8.1.3.17044"}); err == nil || !strings.Contains(err.Error(), "can't be edited") {
		t.Errorf("edit of notecard firmware got %v", err)
	}
	flagScope = "dev:1"
	if err := firmwareUpdateCommand(nil, []string{"8.1.3.17044"}); err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("a notecard update without a project got %v", err)
	}
}
