// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blues/note-cli/lib"
)

// The credentials netcat relays with
var netcatTestCreds = &lib.ConfigCreds{User: "someone@example.com", Token: "ory_at_netcat"}

// netcatTestRequest is what a hub handler saw of a relayed request
type netcatTestRequest struct {
	Method        string
	RequestURI    string
	Host          string
	Header        http.Header
	ContentLength int64
	Body          string
}

// netcatTestHub is a hub reached over TLS, which records what it is sent and answers as
// told.  It returns the hub's host and the transport that trusts it.
func netcatTestHub(t *testing.T, seen *netcatTestRequest, answer http.HandlerFunc) (hub string, transport http.RoundTripper) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("hub reading body: %s", err)
		}
		if seen != nil {
			*seen = netcatTestRequest{r.Method, r.RequestURI, r.Host, r.Header.Clone(), r.ContentLength, string(body)}
		}
		if answer != nil {
			answer(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server.Listener.Addr().String(), netcatTransport(server.Client().Transport.(*http.Transport))
}

// netcatTestRelay relays a request and parses what was written to stdout
func netcatTestRelay(t *testing.T, request string, hub string, transport http.RoundTripper, credentials *lib.ConfigCreds) (exitCode int, rsp *http.Response, body string, stderr string) {
	t.Helper()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	exitCode = netcatRelay(strings.NewReader(request), out, errOut, hub, credentials, transport)
	stderr = errOut.String()
	var err error
	rsp, err = http.ReadResponse(bufio.NewReader(bytes.NewReader(out.Bytes())), nil)
	if err != nil {
		t.Fatalf("stdout isn't an HTTP response: %s\n%s", err, out.String())
	}
	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("reading the relayed body: %s\n%s", err, out.String())
	}
	body = string(bodyBytes)
	return
}

// A request is sent to the hub with our credentials in place of whatever the caller
// supplied, to the hub's host rather than the one named, with its path and query as given,
// and without the headers that describe the caller's connection.  The hub's answer comes
// back whole, and the exit code says the hub answered.
func TestNetcatRelaysRequest(t *testing.T) {
	seen := &netcatTestRequest{}
	hub, transport := netcatTestHub(t, seen, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Hub", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	})
	request := "POST /req?product=net.ozzie.ray%3At&device=dev%3A1 HTTP/1.1\r\n" +
		"Host: somewhere.else\r\n" +
		"Authorization: Bearer callers-token\r\n" +
		"X-Session-Token: callers-session\r\n" +
		"Connection: close, X-Hop\r\n" +
		"X-Hop: 1\r\n" +
		"Expect: 100-continue\r\n" +
		"Content-Type: application/json\r\n" +
		"X-Kept: yes\r\n" +
		"Content-Length: 21\r\n" +
		"\r\n" +
		`{"req":"hub.app.get"}`
	exitCode, rsp, body, stderr := netcatTestRelay(t, request, hub, transport, netcatTestCreds)
	if exitCode != exitOk {
		t.Fatalf("exit code %d, expected %d since the hub answered; stderr %q", exitCode, exitOk, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr %q, expected nothing", stderr)
	}

	// What the hub saw
	if seen.Method != "POST" || seen.RequestURI != "/req?product=net.ozzie.ray%3At&device=dev%3A1" {
		t.Errorf("hub saw %s %s, expected the request line as given", seen.Method, seen.RequestURI)
	}
	if seen.Host != hub {
		t.Errorf("hub saw host %q, expected its own, %q", seen.Host, hub)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer "+netcatTestCreds.Token {
		t.Errorf("hub saw authorization %q, expected our token", got)
	}
	for _, name := range []string{"X-Session-Token", "X-Hop", "Expect"} {
		if _, present := seen.Header[name]; present {
			t.Errorf("hub saw %s, which should have been dropped", name)
		}
	}
	if got := seen.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("hub saw content type %q, expected the caller's", got)
	}
	if got := seen.Header.Get("X-Kept"); got != "yes" {
		t.Errorf("hub saw X-Kept %q, expected the caller's", got)
	}
	if got := seen.Header.Get("User-Agent"); got != netcatUserAgent {
		t.Errorf("hub saw user agent %q, expected %q when the caller named none", got, netcatUserAgent)
	}
	if seen.ContentLength != 21 || seen.Body != `{"req":"hub.app.get"}` {
		t.Errorf("hub saw a body of %d bytes, %q, expected the caller's 21", seen.ContentLength, seen.Body)
	}

	// What the caller got
	if rsp.StatusCode != http.StatusCreated || rsp.Proto != "HTTP/1.1" {
		t.Errorf("caller got %s, expected HTTP/1.1 201", rsp.Status)
	}
	if rsp.Header.Get("X-Hub") != "yes" || rsp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("caller got headers %v, expected the hub's", rsp.Header)
	}
	if body != `{"ok":true}` {
		t.Errorf("caller got body %q, expected the hub's", body)
	}
}

// The hub's status is relayed as it is, with the exit code still saying the hub answered;
// a caller distinguishes the hub's 401 from ours by the exit code
func TestNetcatRelaysHubErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
		hub, transport := netcatTestHub(t, nil, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"err":"from the hub"}`))
		})
		exitCode, rsp, body, _ := netcatTestRelay(t, "GET /v1/projects HTTP/1.1\r\n\r\n", hub, transport, netcatTestCreds)
		if exitCode != exitOk {
			t.Errorf("hub status %d: exit code %d, expected %d since the hub answered", status, exitCode, exitOk)
			continue
		}
		if rsp.StatusCode != status || body != `{"err":"from the hub"}` {
			t.Errorf("hub status %d: caller got %s with body %q, expected the hub's", status, rsp.Status, body)
		}
	}
}

// A body that is neither Content-Length-delimited nor chunked is the rest of the input,
// and the caller may write the request by hand, with bare newlines and no Host
func TestNetcatLenientBody(t *testing.T) {
	seen := &netcatTestRequest{}
	hub, transport := netcatTestHub(t, seen, nil)
	request := "POST /req HTTP/1.1\nContent-Type: application/json\n\n{\"req\":\"hub.app.get\"}\n"
	exitCode, _, _, stderr := netcatTestRelay(t, request, hub, transport, netcatTestCreds)
	if exitCode != exitOk {
		t.Fatalf("exit code %d, expected %d since the hub answered; stderr %q", exitCode, exitOk, stderr)
	}
	if seen.Body != "{\"req\":\"hub.app.get\"}\n" || seen.ContentLength != int64(len(seen.Body)) {
		t.Errorf("hub saw a body of %d bytes, %q, expected the rest of the input with its length", seen.ContentLength, seen.Body)
	}
	if seen.Host != hub {
		t.Errorf("hub saw host %q, expected its own, %q, with no Host header given", seen.Host, hub)
	}
}

// A chunked request body reaches the hub whole, with its length
func TestNetcatChunkedRequest(t *testing.T) {
	seen := &netcatTestRequest{}
	hub, transport := netcatTestHub(t, seen, nil)
	request := "POST /req HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n" +
		"7\r\n{\"req\":\r\n" +
		"e\r\n\"hub.app.get\"}\r\n" +
		"0\r\n\r\n"
	exitCode, _, _, stderr := netcatTestRelay(t, request, hub, transport, netcatTestCreds)
	if exitCode != exitOk {
		t.Fatalf("exit code %d, expected %d since the hub answered; stderr %q", exitCode, exitOk, stderr)
	}
	if seen.Body != `{"req":"hub.app.get"}` || seen.ContentLength != 21 {
		t.Errorf("hub saw a body of %d bytes, %q, expected the chunks joined, with their length", seen.ContentLength, seen.Body)
	}
}

// A response the hub streams is streamed on, and arrives whole
func TestNetcatStreamedResponse(t *testing.T) {
	hub, transport := netcatTestHub(t, nil, func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for _, line := range []string{"{\"line\":1}\n", "{\"line\":2}\n", "{\"line\":3}\n"} {
			w.Write([]byte(line))
			flusher.Flush()
		}
	})
	exitCode, rsp, body, _ := netcatTestRelay(t, "GET /stream HTTP/1.1\r\n\r\n", hub, transport, netcatTestCreds)
	if exitCode != exitOk {
		t.Fatalf("exit code %d, expected %d since the hub answered", exitCode, exitOk)
	}
	if body != "{\"line\":1}\n{\"line\":2}\n{\"line\":3}\n" {
		t.Errorf("caller got body %q, expected every line", body)
	}
	if rsp.ContentLength != -1 {
		t.Errorf("caller got a content length of %d, expected the response to be streamed without one", rsp.ContentLength)
	}
}

// Compression is the caller's business: the hub is asked for it only when the caller
// asks, and what the hub sends is relayed as it is
func TestNetcatCompressionPassesThrough(t *testing.T) {
	seen := &netcatTestRequest{}
	hub, transport := netcatTestHub(t, seen, nil)
	for _, acceptEncoding := range []string{"", "gzip"} {
		request := "GET /v1/projects HTTP/1.1\r\n"
		if acceptEncoding != "" {
			request += "Accept-Encoding: " + acceptEncoding + "\r\n"
		}
		request += "\r\n"
		if exitCode, _, _, _ := netcatTestRelay(t, request, hub, transport, netcatTestCreds); exitCode != exitOk {
			t.Fatalf("exit code %d, expected %d since the hub answered", exitCode, exitOk)
		}
		if got := seen.Header.Get("Accept-Encoding"); got != acceptEncoding {
			t.Errorf("caller accepted %q, and the hub saw %q", acceptEncoding, got)
		}
	}
}

// The response is written in the caller's HTTP version
func TestNetcatMirrorsHTTPVersion(t *testing.T) {
	hub, transport := netcatTestHub(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	exitCode, rsp, body, _ := netcatTestRelay(t, "GET /v1/projects HTTP/1.0\r\n\r\n", hub, transport, netcatTestCreds)
	if exitCode != exitOk {
		t.Fatalf("exit code %d, expected %d since the hub answered", exitCode, exitOk)
	}
	if rsp.Proto != "HTTP/1.0" || body != "ok" {
		t.Errorf("caller got %s with body %q, expected HTTP/1.0 and the hub's body", rsp.Proto, body)
	}
}

// Without credentials the answer is a 401 of our own, before the request is read, and the
// exit code says the hub wasn't asked
func TestNetcatNotSignedIn(t *testing.T) {
	hub, transport := netcatTestHub(t, nil, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("hub was asked while not signed in")
	})
	exitCode, rsp, body, stderr := netcatTestRelay(t, "GET /v1/projects HTTP/1.1\r\n\r\n", hub, transport, nil)
	if exitCode != exitFail {
		t.Fatalf("exit code %d, expected %d since the response is our own", exitCode, exitFail)
	}
	if rsp.StatusCode != http.StatusUnauthorized || rsp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("caller got %s as %s, expected a JSON 401", rsp.Status, rsp.Header.Get("Content-Type"))
	}
	if !strings.Contains(body, `"err":"not signed in to `+hub) || !strings.Contains(body, authSignInAction) {
		t.Errorf("caller got body %q, expected the hub named and the sign-in action", body)
	}
	if !strings.Contains(stderr, "not signed in") {
		t.Errorf("stderr %q, expected the reason", stderr)
	}
}

// What can't be read as a request, including nothing at all, is answered with a 400 of
// our own
func TestNetcatBadRequest(t *testing.T) {
	hub, transport := netcatTestHub(t, nil, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("hub was asked with a request that couldn't be read")
	})
	tests := []struct{ request, reason string }{
		{"", "no request on stdin"},
		{"this is not http\r\n\r\n", "malformed HTTP"},
		{"POST /req HTTP/1.1\r\nContent-Length: 50\r\n\r\nshort", "reading body"},
	}
	for _, test := range tests {
		exitCode, rsp, body, stderr := netcatTestRelay(t, test.request, hub, transport, netcatTestCreds)
		if exitCode != exitFail {
			t.Errorf("%q: exit code %d, expected %d since the response is our own", test.request, exitCode, exitFail)
			continue
		}
		if rsp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"err":"bad request: `+test.reason) || !strings.Contains(stderr, test.reason) {
			t.Errorf("%q: caller got %s with body %q and stderr %q, expected a 400 saying %q", test.request, rsp.Status, body, stderr, test.reason)
		}
	}
}

// A hub that can't be reached is answered with a 502 of our own
func TestNetcatHubUnreachable(t *testing.T) {
	hub := "127.0.0.1:1"
	exitCode, rsp, body, stderr := netcatTestRelay(t, "GET /v1/projects HTTP/1.1\r\n\r\n", hub, netcatTransport(nil), netcatTestCreds)
	if exitCode != exitFail {
		t.Fatalf("exit code %d, expected %d since the response is our own", exitCode, exitFail)
	}
	if rsp.StatusCode != http.StatusBadGateway || !strings.Contains(body, `"err":"`+hub) || !strings.Contains(stderr, hub) {
		t.Errorf("caller got %s with body %q and stderr %q, expected a 502 naming the hub", rsp.Status, body, stderr)
	}
}
