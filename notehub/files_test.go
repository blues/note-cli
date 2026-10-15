// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
)

// fakeRequest is one request a fake hub saw
type fakeRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
	Header http.Header
	Req    map[string]any // the JSON of a v0 request
}

// fakeHub is a hub that records what it is sent and answers as its handler says
type fakeHub struct {
	seen   []fakeRequest
	handle func(r fakeRequest, w http.ResponseWriter)
}

// newFakeHub starts a fake hub, points the CLI at it, and gives it a token
func newFakeHub(t *testing.T) *fakeHub {
	t.Helper()
	hub := &fakeHub{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen := fakeRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: body, Header: r.Header.Clone()}
		if r.URL.Path == "/req" {
			seen.Req = map[string]any{}
			note.JSONUnmarshal(body, &seen.Req)
		}
		hub.seen = append(hub.seen, seen)
		hub.handle(seen, w)
	}))
	t.Cleanup(server.Close)
	savedBase := apiBaseURL
	apiBaseURL = func() string { return server.URL }
	t.Cleanup(func() { apiBaseURL = savedBase })
	if _, err := lib.GetConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lib.ConfigTokenEnvVar, "api_key_test")
	return hub
}

// calls returns the requests seen as "METHOD path" or "req name" lines, for a check
func (hub *fakeHub) calls() []string {
	calls := []string{}
	for _, r := range hub.seen {
		if r.Req != nil {
			calls = append(calls, fmt.Sprint(r.Req["req"]))
			continue
		}
		calls = append(calls, r.Method+" "+r.Path)
	}
	return calls
}

// uploadsTestReset clears the switches the files and firmware modes read
func uploadsTestReset(t *testing.T) {
	t.Helper()
	saved := []any{flagFilesType, flagUploadsTags, flagUploadsNotes, flagUploadsNoReplace, flagSkillsDryRun, flagSkillsForce, flagApp, flagProduct, flagJson, flagPretty, flagVerbose,
		flagFirmwareType, flagFirmwareVersion, flagFirmwareInfo, flagFirmwareTags, flagFirmwareSNs, flagFirmwareSKUs, flagScope}
	t.Cleanup(func() {
		flagFilesType, flagUploadsTags, flagUploadsNotes, flagUploadsNoReplace = saved[0].(string), saved[1].(string), saved[2].(string), saved[3].(bool)
		flagSkillsDryRun, flagSkillsForce, flagApp, flagProduct = saved[4].(bool), saved[5].(bool), saved[6].(string), saved[7].(string)
		flagJson, flagPretty, flagVerbose = saved[8].(bool), saved[9].(bool), saved[10].(bool)
		flagFirmwareType, flagFirmwareVersion, flagFirmwareInfo = saved[11].(string), saved[12].(string), saved[13].(string)
		flagFirmwareTags, flagFirmwareSNs, flagFirmwareSKUs, flagScope = saved[14].(cli.StringList), saved[15].(cli.StringList), saved[16].(cli.StringList), saved[17].(string)
	})
	flagFilesType, flagUploadsTags, flagUploadsNotes, flagUploadsNoReplace = "data", "", "", false
	flagSkillsDryRun, flagSkillsForce, flagApp, flagProduct = false, false, "app:1", ""
	flagJson, flagPretty, flagVerbose = false, false, false
	flagFirmwareType, flagFirmwareVersion, flagFirmwareInfo = "host", "", ""
	flagFirmwareTags, flagFirmwareSNs, flagFirmwareSKUs, flagScope = nil, nil, nil, ""
}

// fakeInt reads a number from decoded JSON, whichever way it was decoded
func fakeInt(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

// A fake hub's store of data files, answering the v0 upload calls
type fakeFiles struct {
	uploads  []map[string]any
	contents map[string][]byte
}

func (store *fakeFiles) handle(r fakeRequest, w http.ResponseWriter) {
	answer := func(value any) {
		out, _ := note.JSONMarshal(value)
		w.Write(out)
	}
	switch r.Req["req"] {
	case "hub.app.upload.query":
		answer(map[string]any{"uploads": store.uploads})
	case "hub.app.upload.get":
		name := r.Req["name"].(string)
		offset, length := fakeInt(r.Req["offset"]), fakeInt(r.Req["length"])
		answer(map[string]any{"name": name, "payload": store.contents[name][offset : offset+length]})
	case "hub.app.upload.add":
		name := r.Req["name"].(string)
		stored := name + "$20260101000001"
		answer(map[string]any{"name": stored, "source": name})
	case "hub.app.upload.delete":
		answer(map[string]any{})
	default:
		w.WriteHeader(http.StatusBadRequest)
		answer(map[string]any{"err": "unexpected request"})
	}
}

// The files mode lists, downloads, uploads and deletes a project's data files through
// the v0 upload calls, scoped to the project, skipping what the project already holds
func TestFilesMode(t *testing.T) {
	uploadsTestReset(t)
	hub := newFakeHub(t)
	store := &fakeFiles{
		uploads: []map[string]any{{"name": "model.tflite$20260101000000", "source": "model.tflite", "length": 5, "md5": uploadsMD5([]byte("hello")),
			"created": 1767225600, "tags": "ml", "notes": "first line\nmore", "type": "data"}},
		contents: map[string][]byte{"model.tflite$20260101000000": []byte("hello")},
	}
	hub.handle = store.handle

	// list, scoped to the project and the type
	out := captureStdout(t, func() {
		if err := filesListCommand(nil, nil); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "model.tflite") || !strings.Contains(out, "first line") || strings.Contains(out, "more") {
		t.Errorf("list wrote %q", out)
	}
	if last := hub.seen[len(hub.seen)-1]; last.Query.Get("app") != "app:1" || last.Req["type"] != "data" {
		t.Errorf("list asked with query %v and type %v", last.Query, last.Req["type"])
	}

	// get, verified against the md5, into a directory
	dir := t.TempDir()
	out = captureStdout(t, func() {
		if err := filesGetCommand(nil, []string{"model.tflite", dir}); err != nil {
			t.Error(err)
		}
	})
	if got, _ := os.ReadFile(filepath.Join(dir, "model.tflite")); string(got) != "hello" || !strings.Contains(out, "new") {
		t.Errorf("get wrote %q and said %q", got, out)
	}

	// set: the same bytes are skipped, other bytes replace, --no-replace refuses
	same := filepath.Join(dir, "model.tflite")
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := filesSetCommand(nil, []string{same}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "unchanged") || slices.Contains(hub.calls(), "hub.app.upload.add") {
		t.Errorf("setting the same bytes wrote %q and called %v", out, hub.calls())
	}
	os.WriteFile(same, []byte("hello!"), 0666)
	flagUploadsNoReplace = true
	if err := filesSetCommand(nil, []string{same}); err == nil || !strings.Contains(err.Error(), "--no-replace") {
		t.Errorf("with --no-replace got %v", err)
	}
	flagUploadsNoReplace = false
	flagUploadsNotes = "second"
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := filesSetCommand(nil, []string{same}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "replaced") || !slices.Equal(hub.calls(), []string{"hub.app.upload.query", "hub.app.upload.add", "hub.app.upload.delete"}) {
		t.Errorf("replacing wrote %q and called %v", out, hub.calls())
	}
	if add := hub.seen[1].Req; add["name"] != "model.tflite" || add["filenotes"] != "second" || add["payload"] == nil {
		t.Errorf("the add request was %v", add)
	}
	if del := hub.seen[2].Req; del["name"] != "model.tflite$20260101000000" {
		t.Errorf("the delete request was %v", del)
	}

	// delete
	hub.seen = nil
	out = captureStdout(t, func() {
		if err := filesDeleteCommand(nil, []string{"model.tflite"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "removed") || !slices.Equal(hub.calls(), []string{"hub.app.upload.query", "hub.app.upload.delete"}) {
		t.Errorf("delete wrote %q and called %v", out, hub.calls())
	}
}

// The files mode works and has help, but isn't listed among the modes
func TestFilesModeHidden(t *testing.T) {
	if mode := cli.ModeNamed(modeFiles); mode == nil || !mode.Hidden {
		t.Fatalf("the files mode must exist and be hidden")
	}
	if slices.Contains(cli.ModeNames(), modeFiles) {
		t.Errorf("the files mode is listed among the modes")
	}
	help := captureStdout(t, func() { cli.PrintHelp(cli.ModeNamed(modeFiles), true) })
	for _, shown := range []string{"list", "show", "get <name|all> [path|-]", "set <path> [name]", "delete <name|all>", "--notes", "--no-replace"} {
		if !strings.Contains(help, shown) {
			t.Errorf("files help doesn't mention '%s'", shown)
		}
	}
	if strings.Contains(help, "--type") {
		t.Errorf("files help shows --type, which is hidden")
	}
}
