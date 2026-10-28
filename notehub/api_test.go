// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
)

// apiTestReset clears every switch the api mode reads, and restores them afterwards
func apiTestReset(t *testing.T) {
	t.Helper()
	saved := []any{flagApiInput, flagApiFields, flagApiRaw, flagApiHeaders, flagApiInclude, flagApiPaginate, flagApiSpec, flagApp, flagProduct, flagDevice, flagPretty, flagVerbose}
	t.Cleanup(func() {
		flagApiInput, flagApiFields, flagApiRaw, flagApiHeaders = saved[0].(string), saved[1].(cli.StringList), saved[2].(cli.StringList), saved[3].(cli.StringList)
		flagApiInclude, flagApiPaginate, flagApiSpec = saved[4].(bool), saved[5].(bool), saved[6].(bool)
		flagApp, flagProduct, flagDevice, flagPretty, flagVerbose = saved[7].(string), saved[8].(string), saved[9].(string), saved[10].(bool), saved[11].(bool)
	})
	flagApiInput, flagApiFields, flagApiRaw, flagApiHeaders = "", nil, nil, nil
	flagApiInclude, flagApiPaginate, flagApiSpec = false, false, false
	flagApp, flagProduct, flagDevice, flagPretty, flagVerbose = "", "", "", false, false
}

// A request is [METHOD] <path>, with the method in any case and the path given a slash
func TestApiRequestLine(t *testing.T) {
	for _, test := range []struct {
		args         []string
		method, path string
		refused      bool
	}{
		{[]string{"/v1/projects"}, "", "/v1/projects", false},
		{[]string{"v1/projects"}, "", "/v1/projects", false},
		{[]string{"GET", "/v1/projects"}, "GET", "/v1/projects", false},
		{[]string{"post", "/req"}, "POST", "/req", false},
		{[]string{"delete", "/v1/projects/app:1/devices/dev:2"}, "DELETE", "/v1/projects/app:1/devices/dev:2", false},
		{[]string{}, "", "", true},
		{[]string{"FETCH", "/v1/projects"}, "", "", true},
		{[]string{"GET", "/v1/projects", "extra"}, "", "", true},
		{[]string{"https://api.notefile.net/v1/projects"}, "", "", true},
	} {
		method, path, err := apiRequestLine(test.args)
		if test.refused {
			if err == nil {
				t.Errorf("%v: accepted as %s %s", test.args, method, path)
			}
			continue
		}
		if err != nil || method != test.method || path != test.path {
			t.Errorf("%v: got %s %s (%v), want %s %s", test.args, method, path, err, test.method, test.path)
		}
	}
}

// Placeholders are filled from the scope switches, in our spelling and the spec's, and
// one with nothing to fill it, or an unknown one, is refused
func TestApiFillPlaceholders(t *testing.T) {
	apiTestReset(t)
	flagApp, flagDevice = "app:1", "dev:2"
	for path, want := range map[string]string{
		"/v1/projects/{project}/devices/{device}":         "/v1/projects/app:1/devices/dev:2",
		"/v1/projects/{projectOrProductUID}":              "/v1/projects/app:1",
		"/v1/projects/{projectUID}/devices/{deviceUID}/x": "/v1/projects/app:1/devices/dev:2/x",
		"/v1/projects":                       "/v1/projects",
		"/req?app={project}&device={device}": "/req?app=app:1&device=dev:2",
	} {
		if got, err := apiFillPlaceholders(path); err != nil || got != want {
			t.Errorf("%s: got %s (%v), want %s", path, got, err, want)
		}
	}
	for _, path := range []string{"/v1/products/{product}/x", "/v1/{fleet}"} {
		if got, err := apiFillPlaceholders(path); err == nil {
			t.Errorf("%s: filled as %s, expected a refusal", path, got)
		}
	}

	// A project is named by either UID
	flagApp, flagProduct = "", "com.example:product"
	if got, _ := apiFillPlaceholders("/v1/projects/{project}/x/{product}"); got != "/v1/projects/com.example:product/x/com.example:product" {
		t.Errorf("with only --product, got %s", got)
	}
}

// --field values are typed as gh types them; --raw-field values stay strings
func TestApiTypedValue(t *testing.T) {
	file := filepath.Join(t.TempDir(), "value.txt")
	os.WriteFile(file, []byte("from a file"), 0666)
	for value, want := range map[string]any{
		"true":     true,
		"false":    false,
		"null":     nil,
		"42":       json.Number("42"),
		"-1.5":     json.Number("-1.5"),
		"1e3":      json.Number("1e3"),
		"007":      "007",
		"4ever":    "4ever",
		"hot":      "hot",
		`"quoted"`: `"quoted"`,
		"":         "",
		"@" + file: "from a file",
		"@-":       "from stdin",
	} {
		got, err := apiTypedValue(value, strings.NewReader("from stdin"))
		if err != nil || got != want {
			t.Errorf("%q: got %#v (%v), want %#v", value, got, err, want)
		}
	}
}

// apiTestServer is a hub that records what it was sent and answers as told
type apiTestServer struct {
	server *httptest.Server
	method string
	path   string
	query  string
	header http.Header
	body   string
}

func newApiTestServer(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) *apiTestServer {
	t.Helper()
	seen := &apiTestServer{}
	seen.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.method, seen.path, seen.query, seen.header, seen.body = r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), string(body)
		answer(w, r)
	}))
	t.Cleanup(seen.server.Close)
	savedBase := apiBaseURL
	apiBaseURL = func() string { return seen.server.URL }
	t.Cleanup(func() { apiBaseURL = savedBase })
	if _, err := lib.GetConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lib.ConfigTokenEnvVar, "api_key_test")
	return seen
}

func apiTestRun(t *testing.T, args []string, stdin string) (out string, err error) {
	t.Helper()
	buffer := &bytes.Buffer{}
	err = apiCommand(args, strings.NewReader(stdin), buffer, http.DefaultClient)
	return buffer.String(), err
}

// A GET carries our token, its parameters in the query string, and the body comes back
// whole with a newline after it.  The method is named, since parameters alone make a POST.
func TestApiGet(t *testing.T) {
	apiTestReset(t)
	hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[]}`))
	})
	flagApiFields = cli.StringList{"pageSize=5", "archived=false"}
	flagApiRaw = cli.StringList{"label=7"}
	out, err := apiTestRun(t, []string{"GET", "/v1/projects"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if hub.method != "GET" || hub.path != "/v1/projects" || hub.query != "archived=false&label=7&pageSize=5" {
		t.Errorf("hub saw %s %s?%s", hub.method, hub.path, hub.query)
	}
	if hub.header.Get("Authorization") != "Bearer api_key_test" || hub.header.Get("User-Agent") != apiUserAgent || hub.body != "" {
		t.Errorf("hub saw headers %v and body %q", hub.header, hub.body)
	}
	if out != "{\"projects\":[]}\n" {
		t.Errorf("wrote %q", out)
	}
}

// Parameters without a method make a POST with a typed JSON body; a method with --input
// sends the file, or stdin, as it is
func TestApiBodies(t *testing.T) {
	apiTestReset(t)
	hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	flagApiFields = cli.StringList{"count=3", "on=true", "name=sensor"}
	if _, err := apiTestRun(t, []string{"/v1/things"}, ""); err != nil {
		t.Fatal(err)
	}
	if hub.method != "POST" || hub.header.Get("Content-Type") != "application/json" || hub.body != `{"count":3,"name":"sensor","on":true}` {
		t.Errorf("hub saw %s with %s body %q", hub.method, hub.header.Get("Content-Type"), hub.body)
	}

	flagApiFields, flagApiInput = nil, "-"
	if _, err := apiTestRun(t, []string{"PUT", "/v1/things/1"}, `{"raw":"body"}`); err != nil {
		t.Fatal(err)
	}
	if hub.method != "PUT" || hub.body != `{"raw":"body"}` {
		t.Errorf("hub saw %s with body %q", hub.method, hub.body)
	}

	file := filepath.Join(t.TempDir(), "body.json")
	os.WriteFile(file, []byte(`{"from":"file"}`), 0666)
	flagApiInput = file
	flagApiHeaders = cli.StringList{"Content-Type: text/plain", "X-Test: yes"}
	if _, err := apiTestRun(t, []string{"/v1/things"}, ""); err != nil {
		t.Fatal(err)
	}
	if hub.method != "POST" || hub.body != `{"from":"file"}` || hub.header.Get("Content-Type") != "text/plain" || hub.header.Get("X-Test") != "yes" {
		t.Errorf("hub saw %s with %s body %q and headers %v", hub.method, hub.header.Get("Content-Type"), hub.body, hub.header)
	}

	flagApiFields = cli.StringList{"extra=1"}
	if _, err := apiTestRun(t, []string{"/v1/things"}, ""); err == nil {
		t.Errorf("--input with --field was accepted")
	}
}

// --include writes the status line and headers; --pretty indents JSON; an error status
// is written and is an error too, with the hub's reason
func TestApiOutput(t *testing.T) {
	apiTestReset(t)
	hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"err":"no such thing"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"a":1}`))
	})
	flagApiInclude = true
	out, err := apiTestRun(t, []string{"/v1/things"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "HTTP/1.1 201 Created\nContent-Length: 7\nContent-Type: application/json\n") || !strings.HasSuffix(out, "\n\n{\"a\":1}\n") {
		t.Errorf("wrote %q", out)
	}

	flagApiInclude, flagPretty = false, true
	if out, err = apiTestRun(t, []string{"/v1/things"}, ""); err != nil || out != "{\n    \"a\": 1\n}\n" {
		t.Errorf("with --pretty wrote %q (%v)", out, err)
	}

	flagPretty = false
	out, err = apiTestRun(t, []string{"/missing"}, "")
	if err == nil || err.Error() != "HTTP 404 Not Found: no such thing" || out != "{\"err\":\"no such thing\"}\n" {
		t.Errorf("for an error status got %v and wrote %q", err, out)
	}
	_ = hub
}

// A response the hub streams is written as it arrives, and arrives whole, with the
// status line and headers first when asked; one with an error status is still explained
func TestApiStreams(t *testing.T) {
	apiTestReset(t)
	hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"err":"upstream"}`))
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher := w.(http.Flusher)
		for _, line := range []string{"{\"line\":1}\n", "{\"line\":2}\n", "{\"line\":3}"} {
			w.Write([]byte(line))
			flusher.Flush()
		}
	})
	out, err := apiTestRun(t, []string{"/stream"}, "")
	if err != nil || out != "{\"line\":1}\n{\"line\":2}\n{\"line\":3}\n" {
		t.Errorf("streamed %q (%v)", out, err)
	}
	flagApiInclude = true
	if out, err = apiTestRun(t, []string{"/stream"}, ""); err != nil || !strings.HasPrefix(out, "HTTP/1.1 200 OK\nContent-Type: application/x-ndjson\n") || !strings.HasSuffix(out, "\n\n{\"line\":1}\n{\"line\":2}\n{\"line\":3}\n") {
		t.Errorf("with --include streamed %q (%v)", out, err)
	}
	flagApiInclude = false
	out, err = apiTestRun(t, []string{"/broken"}, "")
	if err == nil || err.Error() != "HTTP 502 Bad Gateway: upstream" || out != "{\"err\":\"upstream\"}\n" {
		t.Errorf("an error status while streaming got %v and wrote %q", err, out)
	}
	_ = hub
}

// Modification cursors retain their full precision, including timestamp ties.
// Both JSON metadata and headers (including array bodies) drive continuation.
func TestApiPaginate(t *testing.T) {
	for _, test := range []struct {
		name, start, query string
		headers            bool
		rawCursor          bool
	}{
		{name: "JSON from beginning", start: "0"},
		{name: "headers and arrays", start: "0", headers: true},
		{name: "timestamp", start: "1793145600123456", query: "&cursor=1793145600123456"},
		{name: "resume exact cursor", start: "1793145600123456.9007199254740993", query: "&cursor=1793145600123456.9007199254740993&pageNum=1", headers: true},
		{name: "resume raw field", start: "1793145600123456.9007199254740993", rawCursor: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			apiTestReset(t)
			cursors := []string{test.start, "1793145600123456.9007199254740994", "1793145600123456.9007199254740995"}
			calls := 0
			hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				index := calls
				calls++
				if index >= len(cursors) {
					t.Error("requested a page after has_more=false")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				query := r.URL.Query()
				if query.Get("cursor") != cursors[index] || query.Has("pageNum") || query.Get("pageSize") != "1" || query.Get("deviceUID") != "dev:1" || query.Get("test") != "yes" {
					t.Errorf("page %d: unexpected query %v", index, query)
				}
				more := index < len(cursors)-1
				next := cursors[index]
				if more {
					next = cursors[index+1]
				}
				if test.headers {
					w.Header().Set("X-Next-Cursor", next)
					w.Header().Set("X-Has-More", fmt.Sprint(more))
					fmt.Fprintf(w, `[{"id":%d,"deleted":true}]`, index)
				} else {
					fmt.Fprintf(w, `{"events":[{"id":%d,"deleted":true}],"next_cursor":%q,"has_more":%t}`, index, next, more)
				}
			})
			flagApiPaginate, flagApp = true, "app:1"
			flagApiFields = cli.StringList{"pageSize=1"}
			flagApiRaw = cli.StringList{"deviceUID=dev:1"}
			if test.rawCursor {
				flagApiRaw = append(flagApiRaw, "cursor="+test.start)
			}
			flagApiInclude = test.headers
			out, err := apiTestRun(t, []string{"GET", "/v1/projects/{project}/events?test=yes" + test.query}, "")
			if err != nil || calls != 3 || hub.path != "/v1/projects/app:1/events" {
				t.Fatalf("calls=%d path=%s error=%v", calls, hub.path, err)
			}
			if strings.Count(out, `"deleted":true`) != 3 || !strings.Contains(out, `"id":2`) {
				t.Errorf("page bodies were not preserved: %q", out)
			}
			if test.headers && strings.Count(out, "X-Next-Cursor:") != 3 {
				t.Errorf("--include lost cursor headers: %q", out)
			}
		})
	}
}

func TestApiPaginateFailures(t *testing.T) {
	for _, test := range []struct {
		name, body, want string
		header           http.Header
	}{
		{name: "missing cursor", body: `{"has_more":true}`, want: "missing next_cursor"},
		{name: "missing continuation", body: `{"next_cursor":"1.2"}`, want: "missing has_more"},
		{name: "invalid JSON", body: `{`, want: "expected modification cursor metadata"},
		{name: "numeric cursor", body: `{"next_cursor":1793145600123456,"has_more":true}`, want: "expected modification cursor metadata"},
		{name: "invalid continuation", body: `{"next_cursor":"1.2","has_more":"true"}`, want: "expected modification cursor metadata"},
		{name: "unsupported endpoint", body: `[]`, want: "expected modification cursor metadata"},
		{name: "repeated cursor", body: `{"next_cursor":"0","has_more":true}`, want: "repeated a modification cursor"},
		{name: "invalid header", body: `[]`, header: http.Header{"X-Next-Cursor": {"1.2"}, "X-Has-More": {"maybe"}}, want: "invalid X-Has-More"},
	} {
		t.Run(test.name, func(t *testing.T) {
			apiTestReset(t)
			calls := 0
			newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 1 {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				for key, values := range test.header {
					w.Header()[key] = values
				}
				io.WriteString(w, test.body)
			})
			flagApiPaginate = true
			out, err := apiTestRun(t, []string{"/v1/projects/app:1/events"}, "")
			if err == nil || !strings.Contains(err.Error(), test.want) || calls != 1 {
				t.Fatalf("calls=%d error=%v, want %q", calls, err, test.want)
			}
			if out != test.body+"\n" {
				t.Errorf("response body lost: %q", out)
			}
		})
	}
}

func TestApiPaginateStops(t *testing.T) {
	for _, test := range []struct {
		name   string
		bodies []string
		status int
		want   string
	}{
		{name: "empty terminal page", bodies: []string{`{"events":[],"next_cursor":"0","has_more":false}`}},
		{name: "cursor cycle", bodies: []string{`{"next_cursor":"1.2","has_more":true}`, `{"next_cursor":"1.3","has_more":true}`, `{"next_cursor":"1.2","has_more":true}`}, want: "repeated a modification cursor"},
		{name: "HTTP error on next page", bodies: []string{`{"next_cursor":"1.2","has_more":true}`, `{"err":"rate limit"}`}, status: http.StatusTooManyRequests, want: "HTTP 429 Too Many Requests: rate limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			apiTestReset(t)
			calls := 0
			newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				index := calls
				calls++
				if index >= len(test.bodies) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if index == len(test.bodies)-1 && test.status != 0 {
					w.WriteHeader(test.status)
				}
				io.WriteString(w, test.bodies[index])
			})
			flagApiPaginate = true
			out, err := apiTestRun(t, []string{"/v1/projects/app:1/events"}, "")
			if (test.want == "" && err != nil) || (test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want))) || calls != len(test.bodies) {
				t.Fatalf("calls=%d error=%v, want %q", calls, err, test.want)
			}
			if out != strings.Join(test.bodies, "\n")+"\n" {
				t.Errorf("response bodies lost: %q", out)
			}
		})
	}
}

func TestApiPaginateRejectsConflictingRequests(t *testing.T) {
	apiTestReset(t)
	flagApiPaginate = true
	for _, args := range [][]string{
		{"POST", "/v1/projects/app:1/events"},
		{"GET", "/v1/projects/app:1/events?pageNum=2"},
		{"GET", "/v1/projects/app:1/events?cursor=0&pageNum=2"},
	} {
		if _, err := apiTestRun(t, args, ""); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

// --spec writes the spec from the first place it is found, and says where it looked when
// it isn't
func TestApiSpec(t *testing.T) {
	apiTestReset(t)
	served := true
	hub := newApiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.yaml" && served {
			w.Header().Set("Content-Type", "application/yaml")
			w.Write([]byte("openapi: 3.0.0"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
	})
	savedURLs := apiSpecURLs
	apiSpecURLs = func() []string { return []string{hub.server.URL + "/nope", hub.server.URL + "/openapi.yaml"} }
	t.Cleanup(func() { apiSpecURLs = savedURLs })
	flagApiSpec = true
	if out, err := apiTestRun(t, nil, ""); err != nil || out != "openapi: 3.0.0\n" {
		t.Errorf("wrote %q (%v)", out, err)
	}
	served = false
	if _, err := apiTestRun(t, nil, ""); err == nil || !strings.Contains(err.Error(), "/openapi.yaml") {
		t.Errorf("without a spec got %v, expected an error naming where it looked", err)
	}
	if _, err := apiTestRun(t, []string{"/v1/projects"}, ""); err == nil {
		t.Errorf("--spec with a request was accepted")
	}
}
