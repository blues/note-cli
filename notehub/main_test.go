// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blues/note-cli/lib"
)

// Run main in a subprocess because it exits. The transport stays local while the
// real startup, argument parsing, retry handling, and diagnostic output are exercised.
func TestMainRateLimitLogging(t *testing.T) {
	if os.Getenv("NOTEHUB_TEST_RATE_LIMIT_MAIN") == "1" {
		separator := slices.Index(os.Args, "--")
		if separator < 0 {
			t.Fatal("missing CLI arguments")
		}
		os.Args = append([]string{cliName}, os.Args[separator+1:]...)
		flag.CommandLine = flag.NewFlagSet(cliName, flag.ContinueOnError)
		calls := 0
		http.DefaultTransport = authStatusTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests",
					Header: headerOf("X-RateLimit-Limit", "1", "X-RateLimit-Period", "1ms"), Body: http.NoBody,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(`{"projects":[]}`)),
			}, nil
		})
		main()
		return
	}
	for _, test := range []struct {
		name    string
		args    []string
		verbose bool
	}{
		{"before path", []string{"api", "--verbose", "GET", "/v1/projects"}, true},
		{"after path", []string{"api", "GET", "/v1/projects", "--verbose"}, true},
		{"disabled after path", []string{"api", "--verbose", "GET", "/v1/projects", "--verbose=false"}, false},
		{"quiet by default", []string{"api", "GET", "/v1/projects"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestMainRateLimitLogging$", "--"}, test.args...)
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			dir := t.TempDir()
			cmd.Env = append(os.Environ(), "NOTEHUB_TEST_RATE_LIMIT_MAIN=1", "XDG_CONFIG_HOME="+dir, "APPDATA="+dir, lib.ConfigTokenEnvVar+"=api_key_test")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("CLI failed: %v\n%s", err, &stderr)
			}
			if stdout.String() != "{\"projects\":[]}\n" {
				t.Fatalf("stdout=%q", &stdout)
			}
			wantMessages := 0
			if test.verbose {
				wantMessages = 1
			}
			if got := strings.Count(stderr.String(), "then retry 1 of 6"); got != wantMessages {
				t.Fatalf("verbose=%v, retry messages=%d, stderr=%q", test.verbose, got, &stderr)
			}
			if !test.verbose && stderr.Len() != 0 {
				t.Fatalf("unexpected diagnostics: %q", &stderr)
			}
		})
	}
}
