// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package main

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
)

// Existing publish scripts must still upload through the default mode, preserving
// the original upload metadata, overwrite behavior and response output.
func TestLegacyUploads(t *testing.T) {
	for _, test := range []struct {
		name, prefix, scope, uid, ftype   string
		equals, metadata, output, missing bool
	}{
		{name: "publish script", prefix: "-", scope: "product", uid: "com.example:radnote", ftype: "firmware"},
		{name: "double hyphens", prefix: "--", scope: "project", uid: "app:1", ftype: "firmware"},
		{name: "equals and metadata", prefix: "--", scope: "product", uid: "com.example:radnote", ftype: "data", equals: true, metadata: true},
		{name: "Notecard", prefix: "-", ftype: "notecard"},
		{name: "inferred type", prefix: "-", scope: "project", uid: "app:1"},
		{name: "response file", prefix: "-", scope: "project", uid: "app:1", ftype: "firmware", output: true},
		{name: "missing file", prefix: "-", scope: "product", uid: "com.example:radnote", ftype: "firmware", missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			uploadsTestReset(t)
			savedFlags, savedArgs := flag.CommandLine, os.Args
			flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
			t.Cleanup(func() { flag.CommandLine, os.Args = savedFlags, savedArgs })

			dir := t.TempDir()
			path := filepath.Join(dir, "radnote-3.2.2-20261020114042.bin")
			contents := []byte("firmware\x00\x01\xff")
			if !test.missing {
				if err := os.WriteFile(path, contents, 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{}
			add := func(name, value string) {
				if test.equals {
					args = append(args, test.prefix+name+"="+value)
				} else {
					args = append(args, test.prefix+name, value)
				}
			}
			if test.scope != "" {
				add(test.scope, test.uid)
			}
			if test.ftype != "" {
				add("type", test.ftype)
			}
			add("upload", path)
			if test.metadata {
				add("tags", "stable,radnote")
				// Legacy notes are literal text, even when they start with @.
				add("notes", "@release notes")
				args = append(args, test.prefix+"overwrite")
			}
			outPath := filepath.Join(dir, "response.json")
			if test.output {
				add("out", outPath)
			}
			mode, rest := cli.Start(args)
			t.Cleanup(func() {
				// Clear this mode's switches so later tests do not inherit the upload.
				flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
				cli.RegisterSwitches(mode)
			})
			if mode.Name != modeDefault {
				t.Fatalf("legacy upload selected %s mode", mode.Name)
			}
			if err := cli.ValidateSwitches(mode, rest); err != nil {
				t.Fatal(err)
			}
			if err := flag.CommandLine.Parse(rest); err != nil {
				t.Fatal(err)
			}
			normalizeScope()
			if _, err := lib.GetConfig(); err != nil {
				t.Fatal(err)
			}

			t.Setenv(lib.ConfigTokenEnvVar, "api_key_test")
			savedClient, savedDo := http.DefaultClient, authenticatedDo
			t.Cleanup(func() { http.DefaultClient, authenticatedDo = savedClient, savedDo })
			http.DefaultClient = &http.Client{Transport: authStatusTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/userinfo" {
					t.Fatalf("unexpected credential request: %s", r.URL.Path)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"email":"test@example.com"}`))}, nil
			})}
			const response = "{\"upload\":\"stored.bin\"}\n"
			calls := 0
			authenticatedDo = func(_ *http.Client, r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/req" {
					t.Errorf("upload request = %s %s", r.Method, r.URL.Path)
				}
				want := map[string]string{"upload": filepath.Base(path)}
				if test.scope != "" {
					key := test.scope
					if key == "project" {
						key = "app"
					}
					want[key] = test.uid
				}
				if test.ftype != "" {
					want["type"] = test.ftype
				}
				if test.metadata {
					want["tags"], want["filenotes"], want["overwrite"] = "stable,radnote", "@release notes", "true"
				}
				query := r.URL.Query()
				if len(query) != len(want) {
					t.Errorf("query = %v, want %v", query, want)
				}
				for key, value := range want {
					// The legacy /req endpoint takes quoted query values.
					if got := query.Get(key); got != `"`+value+`"` {
						t.Errorf("%s = %q, want quoted %q", key, got, value)
					}
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(body, contents) {
					t.Errorf("upload body = %q, err = %v", body, err)
				}
				if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response))}, nil
			}
			var err error
			out := captureStdout(t, func() {
				err = mode.Run(&lib.ConfigSettings{Hub: "api.notefile.net"})
			})
			if test.missing {
				if !os.IsNotExist(err) || calls != 0 {
					t.Fatalf("missing file: err = %v, upload calls = %d", err, calls)
				}
				return
			}
			if err != nil || calls != 1 {
				t.Fatalf("upload: err = %v, calls = %d", err, calls)
			}
			if test.output {
				saved, readErr := os.ReadFile(outPath)
				if readErr != nil || string(saved) != response || out != "" {
					t.Errorf("response file = %q, err = %v, stdout = %q", saved, readErr, out)
				}
			} else if out != response {
				t.Errorf("stdout = %q, want %q", out, response)
			}
		})
	}
}
