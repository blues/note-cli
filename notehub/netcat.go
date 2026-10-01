// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub netcat' works like netcat for HTTPS: it reads a complete HTTP/1.1 request from
// stdin, sends it over TLS to the configured Notehub destination, and writes the complete
// HTTP response, including its status line, headers and body, to stdout.  Notehub
// credentials from --signin are used for the request.  This lets a program send
// authenticated requests to Notehub without holding a token of its own.
//
// Stdout is always a complete HTTP response, with the status in its first line.  The exit
// code is 0 when the hub answered, whatever its status, and 1 when the response was made
// here instead: 401 when not signed in, 400 when the request can't be read, and 502 when
// the hub can't be reached.  Diagnostics go to stderr, never stdout.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
)

// netcatUserAgent is sent to the hub when the caller named none, as the CLI's other
// requests do
const netcatUserAgent = "notehub-client"

// netcatHopByHop are the headers that describe a connection rather than a request or a
// response (RFC 7230 section 6.1), and so don't survive being relayed
var netcatHopByHop = []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}

// netcatReplaced are the request headers that are ours to supply: the credentials, the
// length of the body as we send it, and Expect, since with the whole body in hand there
// is nothing to withhold until the hub says to continue
var netcatReplaced = []string{"Authorization", "X-Session-Token", "Content-Length", "Expect"}

// runNetcat is the handler for 'notehub netcat'.  Whatever happens, stdout gets an HTTP
// response, so the exit is taken here rather than left to main, which would print an
// error there.
func runNetcat(config *lib.ConfigSettings) error {
	if args := flag.Args(); len(args) != 0 {
		reason := fmt.Sprintf("'%s %s' takes no arguments, but was given: %s", cliName, modeNetcat, strings.Join(args, " "))
		os.Exit(netcatFail(os.Stdout, os.Stderr, http.StatusBadRequest, reason))
	}
	exitCode := netcatRelay(os.Stdin, os.Stdout, os.Stderr, lib.ConfigAPIHub(), config.DefaultCredentials(), netcatTransport(nil))
	os.Exit(exitCode)
	return nil
}

// netcatRelay reads one HTTP request from in and writes the hub's response to out, in
// the caller's HTTP version, streaming the body.  The hub is a host, reached over https
// through transport, whose certificate the transport verifies.  Diagnostics go to
// errOut.  The exit code is as described at the top of this file.
func netcatRelay(in io.Reader, out io.Writer, errOut io.Writer, hub string, credentials *lib.ConfigCreds, transport http.RoundTripper) (exitCode int) {

	// Without credentials there is nothing we could add to the request, so it isn't read
	if credentials == nil {
		reason := fmt.Sprintf("not signed in to %s - run '%s'", hub, authSignInAction)
		return netcatFail(out, errOut, http.StatusUnauthorized, reason)
	}

	// The request, with its body in memory
	req, body, err := netcatReadRequest(bufio.NewReader(in))
	if err != nil {
		return netcatFail(out, errOut, http.StatusBadRequest, fmt.Sprintf("bad request: %s", err))
	}

	// Send it to the hub as ourselves.  Redirects, if any, are the caller's to follow,
	// so the transport is used directly rather than through a client.
	hubReq, err := netcatHubRequest(req, body, hub, credentials)
	if err != nil {
		return netcatFail(out, errOut, http.StatusBadRequest, fmt.Sprintf("bad request: %s", err))
	}
	rsp, err := transport.RoundTrip(hubReq)
	if err != nil {
		return netcatFail(out, errOut, http.StatusBadGateway, fmt.Sprintf("%s: %s", hub, err))
	}

	// Relay the response.  Writing it frames the body as the caller's HTTP version
	// allows, from the length and encoding the hub used, and closes the body.
	netcatStripHopByHop(rsp.Header)
	rsp.ProtoMajor, rsp.ProtoMinor = req.ProtoMajor, req.ProtoMinor
	if err := rsp.Write(out); err != nil {
		fmt.Fprintf(errOut, "%s: writing response: %s\n", cliName, err)
		return exitFail
	}
	return exitOk

}

// netcatReadRequest reads a request and the whole of its body.  A body that is neither
// Content-Length-delimited nor chunked, which the standard takes to be no body at all,
// is taken to be the rest of the input, since a request written by hand often omits
// Content-Length.
func netcatReadRequest(reader *bufio.Reader) (req *http.Request, body []byte, err error) {
	req, err = http.ReadRequest(reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("no request on stdin")
		}
		return
	}
	defer req.Body.Close()
	if req.ContentLength == 0 && len(req.TransferEncoding) == 0 && req.Header.Get("Content-Length") == "" {
		body, err = io.ReadAll(reader)
	} else {
		body, err = io.ReadAll(req.Body)
	}
	if err != nil {
		err = fmt.Errorf("reading body: %w", err)
	}
	return
}

// netcatHubRequest is the request as sent to the hub: the same method, path and query,
// over https to the hub, with the caller's headers other than those that describe the
// connection or that are ours to supply, our credentials, and the body's length known
func netcatHubRequest(req *http.Request, body []byte, hub string, credentials *lib.ConfigCreds) (*http.Request, error) {

	// Where it goes
	hubURL := *req.URL
	hubURL.Scheme = "https"
	hubURL.Host = hub
	hubURL.User = nil
	hubReq, err := http.NewRequest(req.Method, hubURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// What it says.  The Host header, if the caller sent one, was already set aside by
	// the parser, and the hub's is sent in its place.
	netcatStripHopByHop(req.Header)
	for _, name := range netcatReplaced {
		req.Header.Del(name)
	}
	hubReq.Header = req.Header
	if hubReq.Header.Get("User-Agent") == "" {
		hubReq.Header.Set("User-Agent", netcatUserAgent)
	}
	credentials.AddHttpAuthHeader(hubReq)

	return hubReq, nil

}

// netcatStripHopByHop removes the hop-by-hop headers, and those the Connection header names
func netcatStripHopByHop(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range netcatHopByHop {
		header.Del(name)
	}
}

// netcatTransport is how the hub is reached: the default transport, or the given one,
// except that it neither asks for compression nor undoes it, so that the response is
// relayed as the hub framed it.  There is no response timeout, since a response may
// stream for as long as the caller likes.
func netcatTransport(base *http.Transport) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport)
	}
	transport := base.Clone()
	transport.DisableCompression = true
	return transport
}

// netcatFail writes a response of our own, with the reason in the JSON form the hub uses
// for an error, and says why on stderr.  The exit code tells the caller the hub wasn't
// asked.
func netcatFail(out io.Writer, errOut io.Writer, status int, reason string) int {
	fmt.Fprintf(errOut, "%s: %s\n", cliName, reason)
	body, _ := note.JSONMarshal(map[string]string{"err": reason})
	rsp := &http.Response{
		StatusCode:    status,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	if err := rsp.Write(out); err != nil {
		fmt.Fprintf(errOut, "%s: writing response: %s\n", cliName, err)
	}
	return exitFail
}
