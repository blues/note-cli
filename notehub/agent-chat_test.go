// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
)

type agentChatTransport func(*http.Request) (*http.Response, error)

func (fn agentChatTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestAgentChat(t *testing.T) {
	t.Setenv(lib.ConfigTokenEnvVar, "api_key_must_not_be_sent")
	for _, test := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"markdown", http.StatusOK, "# Guide\n\nFollow this link.", nil},
		{"html", http.StatusOK, "<!doctype html><p>Agent guide</p>\n", nil},
		{"not found", http.StatusNotFound, "missing", nil},
		{"server error", http.StatusServiceUnavailable, "unavailable", nil},
		{"network error", 0, "", errors.New("connection failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: agentChatTransport(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != agentChatURL {
					t.Errorf("requested %s %s", req.Method, req.URL)
				}
				if req.Header.Get("Authorization") != "" {
					t.Error("sent credentials when fetching public guidance")
				}
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{
					StatusCode: test.status,
					Status:     fmt.Sprintf("%d %s", test.status, http.StatusText(test.status)),
					Body:       io.NopCloser(strings.NewReader(test.body)),
					Header:     make(http.Header),
				}, nil
			})}
			var out bytes.Buffer
			err := agentChat(&out, client)
			if test.err != nil || test.status >= 400 {
				if err == nil || out.Len() != 0 {
					t.Fatalf("failure returned %v and stdout %q", err, out.String())
				}
				return
			}
			if err != nil || out.String() != test.body {
				t.Errorf("got %q (%v), want the exact response %q", out.String(), err, test.body)
			}
		})
	}
}
