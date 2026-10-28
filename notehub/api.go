// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub api' sends one request to the hub's API, authorized as you, and writes the
// response to stdout, the way 'gh api' does for GitHub: a method and a path, parameters
// that go in the query string of a GET and in a JSON body otherwise, or a body from a
// file or stdin.  It is for people and agents who know the API, or who read its OpenAPI
// spec, which --spec shows.  A response is written as it arrives, however long it streams.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
)

// Switches used only in this mode
var (
	flagApiInput    string
	flagApiFields   cli.StringList
	flagApiRaw      cli.StringList
	flagApiHeaders  cli.StringList
	flagApiInclude  bool
	flagApiPaginate bool
	flagApiSpec     bool
)

// The modes in which the request switches may be used
var inApi = []string{modeApi}

// apiSwitches returns the switches used only in this mode
func apiSwitches() []*cli.Switch {
	return []*cli.Switch{
		{Name: "input", Target: &flagApiInput, Group: "request", Modes: inApi,
			Usage: "send a `file` as the body, or stdin with -"},
		{Name: "field", Target: &flagApiFields, Group: "request", Modes: inApi,
			Usage: "add a `key=value` parameter, typed: true, false, null and numbers as such, @file for a file's contents, @- for stdin"},
		{Name: "raw-field", Target: &flagApiRaw, Group: "request", Modes: inApi,
			Usage: "add a `key=value` parameter as a string, whatever it looks like"},
		{Name: "header", Target: &flagApiHeaders, Group: "request", Modes: inApi,
			Usage: "add a `key:value` header"},
		{Name: "include", Target: &flagApiInclude, Group: "request", Modes: inApi,
			Usage: "write the status line and headers before the body"},
		{Name: "paginate", Target: &flagApiPaginate, Group: "request", Modes: inApi,
			Usage: "follow a GET's modification cursor until has_more is false; starts at cursor=0 unless given"},
		{Name: "spec", Target: &flagApiSpec, Group: "request", Modes: inApi,
			Usage: "show the hub's OpenAPI spec, from https://<hub>/openapi.yaml, instead of sending a request"},
	}
}

// apiExamples returns the examples shown in this mode's help
func apiExamples() []cli.Example {
	return []cli.Example{
		{What: "List your projects", Command: cliName + " api /v1/projects"},
		{What: "List a project's devices, with the project written into the path", Command: cliName + " api /v1/projects/app:123/devices"},
		{What: "The same, with --project filling in {project}", Command: cliName + " api /v1/projects/{project}/devices --project app:123"},
		{What: "Read all device changes using modification cursors", Command: cliName + " api GET '/v1/projects/{project}/devices' --paginate --project <projectUID>"},
		{What: "Get a device's environment variables, --project and --device filling in the placeholders", Command: cliName + " api /v1/projects/{project}/devices/{device}/environment_variables --project <projectUID> --device <deviceUID>"},
		{What: "Set them, with the body from a file", Command: cliName + " api PUT /v1/projects/{project}/devices/{device}/environment_variables --input vars.json --project <projectUID> --device <deviceUID>"},
		{What: "Send a body from stdin", Command: "cat fleet.json | " + cliName + " api POST /v1/projects/{project}/fleets --input - --project <projectUID>"},
		{What: "Add parameters: a JSON body, and a POST unless the method is given", Command: cliName + " api /v1/projects/{project}/fleets --field label=Field --project <projectUID>"},
		{What: "Parameters in the query string of a GET, which the method must then name", Command: cliName + " api GET /v1/projects/{project}/events --field pageSize=5 --project <projectUID>"},
		{What: "See the status line and headers too", Command: cliName + " api /v1/projects --include"},
		{What: "Add a header", Command: cliName + ` api /v1/projects --header "Accept: application/json"`},
		{What: "Show the hub's OpenAPI spec, which describes every endpoint", Command: cliName + " api --spec"},
	}
}

// apiUserAgent is sent to the hub when the caller named none
const apiUserAgent = "notehub-client"

// apiMethods are the HTTP methods a request may name
var apiMethods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true, http.MethodPatch: true,
	http.MethodDelete: true, http.MethodHead: true, http.MethodOptions: true,
}

// apiBaseURL is where the hub's API is, which a test may point elsewhere
var apiBaseURL = func() string { return "https://" + lib.ConfigAPIHub() }

// apiSpecURLs are where the hub's OpenAPI spec may be found, which a test may point
// elsewhere: the API's host, and the hub's own
var apiSpecURLs = func() []string {
	return []string{"https://" + lib.ConfigAPIHub() + "/openapi.yaml", "https://" + lib.ConfigNotecardHub() + "/openapi.yaml"}
}

// apiRequest is a request as the command line describes it
type apiRequest struct {
	Method string
	URL    *url.URL // the path and query; the scheme and host are the hub's
	Header http.Header
	Body   []byte // nil when there is none
}

// apiResponse is what the hub answered
type apiResponse struct {
	Proto      string
	Status     string
	StatusCode int
	Header     http.Header
	Body       []byte
}

// runApi is the handler for 'notehub api'.  Switches may come before or after the request,
// and with no request at all, the help is shown.
func runApi(config *lib.ConfigSettings) error {
	args, err := cli.ParseCommandSwitches(cli.ModeNamed(modeApi), flag.Args())
	if err != nil {
		return err
	}
	if len(args) == 0 && !flagApiSpec {
		cli.PrintHelp(cli.ModeNamed(modeApi), false)
		return nil
	}
	return apiCommand(args, os.Stdin, os.Stdout, &http.Client{})
}

// apiCommand sends the request the command line describes and writes the response, or
// with --spec shows the hub's OpenAPI spec.  A response with an error status is written
// too, and is also an error, so the exit code says so.
func apiCommand(args []string, in io.Reader, out io.Writer, client *http.Client) error {

	if flagApiSpec {
		if len(args) != 0 {
			return errors.New("--spec shows the spec, and takes no request")
		}
		return apiSpec(out, client)
	}

	request, err := apiRequestFrom(args, in)
	if err != nil {
		return err
	}

	// One request, unless every page is wanted.  An answer to indent, or an error to
	// explain, is read whole; any other is written as it arrives, however long it streams.
	if !flagApiPaginate {
		httpRsp, err := apiOpen(client, request)
		if err != nil {
			return err
		}
		if flagPretty || httpRsp.StatusCode >= 400 {
			rsp, readErr := apiReadResponse(httpRsp)
			if readErr != nil {
				return readErr
			}
			apiWrite(out, rsp)
			return apiFailure(rsp)
		}
		return apiStream(out, httpRsp)
	}
	if request.Method != http.MethodGet {
		return errors.New("--paginate follows the pages of a GET")
	}
	query := request.URL.Query()
	if query.Has("pageNum") && query.Get("pageNum") != "1" {
		return errors.New("--paginate uses modification cursors; omit pageNum and use cursor to resume")
	}
	query.Del("pageNum")
	cursor := query.Get("cursor")
	if cursor == "" {
		cursor = "0"
	}
	seen := map[string]bool{cursor: true}
	for {
		query.Set("cursor", cursor)
		request.URL.RawQuery = query.Encode()
		rsp, err := apiSend(client, request)
		if err != nil {
			return err
		}
		apiWrite(out, rsp)
		if err := apiFailure(rsp); err != nil {
			return err
		}
		next, more, err := apiCursorPage(rsp)
		if err != nil {
			return fmt.Errorf("--paginate: %w", err)
		}
		if !more {
			return nil
		}
		if seen[next] {
			return errors.New("--paginate: the hub repeated a modification cursor while has_more is true")
		}
		seen[next] = true
		cursor = next
	}

}

// apiCursorPage reads the modification-feed continuation. Headers also support
// endpoints with array or streamed bodies; object responses may carry the same
// values in JSON. Keep cursors as strings so their full precision is preserved.
func apiCursorPage(rsp *apiResponse) (string, bool, error) {
	var page struct {
		NextCursor *string `json:"next_cursor"`
		HasMore    *bool   `json:"has_more"`
	}
	next := rsp.Header.Get("X-Next-Cursor")
	moreHeader := rsp.Header.Get("X-Has-More")
	if next == "" || moreHeader == "" {
		if err := json.Unmarshal(rsp.Body, &page); err != nil {
			return "", false, errors.New("expected modification cursor metadata in X-Next-Cursor/X-Has-More headers or next_cursor/has_more JSON fields")
		}
	}
	if next == "" && page.NextCursor != nil {
		next = *page.NextCursor
	}
	if strings.TrimSpace(next) == "" {
		return "", false, errors.New("missing next_cursor; use --paginate on an endpoint that supports modification cursors")
	}
	if moreHeader != "" {
		switch moreHeader {
		case "true":
			return next, true, nil
		case "false":
			return next, false, nil
		default:
			return "", false, fmt.Errorf("invalid X-Has-More header: %q", moreHeader)
		}
	}
	if page.HasMore == nil {
		return "", false, errors.New("missing has_more in the modification cursor response")
	}
	return next, *page.HasMore, nil
}

// apiRequestFrom builds the request from the command line: the method and path, the
// placeholders filled, the parameters in the query string or the body, and the headers
func apiRequestFrom(args []string, in io.Reader) (*apiRequest, error) {

	method, path, err := apiRequestLine(args)
	if err != nil {
		return nil, err
	}
	if path, err = apiFillPlaceholders(path); err != nil {
		return nil, err
	}
	target, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("'%s' can't be read as a path: %s", path, err)
	}

	// The body: a file or stdin, or else the parameters when the method takes a body
	params, err := apiParams(in)
	if err != nil {
		return nil, err
	}
	var body []byte
	if flagApiInput != "" {
		if len(params) != 0 {
			return nil, errors.New("--input is the body, so --field and --raw-field can't be sent with it")
		}
		if body, err = apiInput(flagApiInput, in); err != nil {
			return nil, err
		}
	}
	if method == "" {
		method = http.MethodGet
		if body != nil || len(params) != 0 {
			method = http.MethodPost
		}
	}
	if len(params) != 0 {
		if method == http.MethodGet || method == http.MethodHead {
			query := target.Query()
			for key, value := range params {
				query.Set(key, apiQueryValue(value))
			}
			target.RawQuery = query.Encode()
		} else if body, err = note.JSONMarshal(params); err != nil {
			return nil, err
		}
	}

	header, err := apiHeaders()
	if err != nil {
		return nil, err
	}
	return &apiRequest{Method: method, URL: target, Header: header, Body: body}, nil

}

// apiRequestLine reads the method, if one is given, and the path
func apiRequestLine(args []string) (method string, path string, err error) {
	switch len(args) {
	case 1:
		path = args[0]
	case 2:
		method, path = strings.ToUpper(args[0]), args[1]
		if !apiMethods[method] {
			return "", "", fmt.Errorf("'%s' is not an HTTP method - a request is [METHOD] <path>, as in: %s api GET /v1/projects", args[0], cliName)
		}
	default:
		return "", "", fmt.Errorf("usage: %s api [options] [METHOD] <path>", cliName)
	}
	if strings.Contains(path, "://") {
		return "", "", fmt.Errorf("give the path, such as /v1/projects, not a URL: the request goes to the active hub, or to the one given by --hub")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return
}

// apiPlaceholderRE matches a placeholder in a path, such as {project}
var apiPlaceholderRE = regexp.MustCompile(`\{([A-Za-z]+)\}`)

// apiFillPlaceholders fills {project}, {product} and {device} in a path, and the spellings
// the OpenAPI spec uses, from --project, --product and --device.  A project may be named
// by either of its UIDs, so {project} takes --product when --project wasn't given.
func apiFillPlaceholders(path string) (string, error) {
	project := flagApp
	if project == "" {
		project = flagProduct
	}
	values := map[string]string{
		"project": project, "projectUID": project, "projectOrProductUID": project,
		"product": flagProduct, "productUID": flagProduct,
		"device": flagDevice, "deviceUID": flagDevice,
	}
	switches := map[string]string{
		"project": "--project or --product", "projectUID": "--project", "projectOrProductUID": "--project or --product",
		"product": "--product", "productUID": "--product",
		"device": "--device", "deviceUID": "--device",
	}
	var err error
	filled := apiPlaceholderRE.ReplaceAllStringFunc(path, func(placeholder string) string {
		name := placeholder[1 : len(placeholder)-1]
		value, known := values[name]
		switch {
		case !known:
			err = fmt.Errorf("%s isn't a placeholder - they are {project}, {product} and {device}, filled from --project, --product and --device", placeholder)
		case value == "":
			err = fmt.Errorf("%s needs %s", placeholder, switches[name])
		}
		return value
	})
	return filled, err
}

// apiParams returns the parameters given with --field, typed, and --raw-field, as strings
func apiParams(in io.Reader) (map[string]any, error) {
	params := map[string]any{}
	for _, raw := range flagApiRaw {
		key, value, found := strings.Cut(raw, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("'%s' is not a key=value parameter", raw)
		}
		params[key] = value
	}
	for _, field := range flagApiFields {
		key, value, found := strings.Cut(field, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("'%s' is not a key=value parameter", field)
		}
		typed, err := apiTypedValue(value, in)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", key, err)
		}
		params[key] = typed
	}
	return params, nil
}

// apiTypedValue reads a --field value as gh does: true, false and null as such, a number as
// a number, @file as the file's contents, @- as stdin, and anything else as a string
func apiTypedValue(value string, in io.Reader) (any, error) {
	switch {
	case value == "@-":
		contents, err := io.ReadAll(in)
		return string(contents), err
	case strings.HasPrefix(value, "@"):
		contents, err := os.ReadFile(value[1:])
		return string(contents), err
	case value == "true":
		return true, nil
	case value == "false":
		return false, nil
	case value == "null":
		return nil, nil
	case value != "" && (value[0] == '-' || (value[0] >= '0' && value[0] <= '9')) && json.Valid([]byte(value)):
		return json.Number(value), nil
	}
	return value, nil
}

// apiQueryValue is a parameter as it goes in a query string
func apiQueryValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return string(v)
	}
	return fmt.Sprint(value)
}

// apiInput reads the body from the file --input names, or from stdin for -
func apiInput(name string, in io.Reader) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(in)
	}
	return os.ReadFile(name)
}

// apiHeaders returns the headers given with --header
func apiHeaders() (http.Header, error) {
	header := http.Header{}
	for _, given := range flagApiHeaders {
		key, value, found := strings.Cut(given, ":")
		if !found || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("'%s' is not a key:value header", given)
		}
		header.Add(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	return header, nil
}

// apiSend sends a request to the hub as ourselves and reads the whole response
func apiSend(client *http.Client, request *apiRequest) (*apiResponse, error) {
	httpRsp, err := apiOpen(client, request)
	if err != nil {
		return nil, err
	}
	return apiReadResponse(httpRsp)
}

// apiOpen sends a request to the hub as ourselves and returns the response with its body
// still to be read, and closed, by the caller
func apiOpen(client *http.Client, request *apiRequest) (*http.Response, error) {

	var body io.Reader
	if request.Body != nil {
		body = bytes.NewReader(request.Body)
	}
	httpReq, err := http.NewRequest(request.Method, apiBaseURL()+request.URL.String(), body)
	if err != nil {
		return nil, err
	}
	httpReq.Header = request.Header.Clone()
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", apiUserAgent)
	}
	if request.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	if flagVerbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", httpReq.Method, httpReq.URL)
		for _, key := range apiSortedKeys(httpReq.Header) {
			if key != "Authorization" {
				fmt.Fprintf(os.Stderr, "> %s: %s\n", key, strings.Join(httpReq.Header[key], ", "))
			}
		}
		if request.Body != nil {
			fmt.Fprintf(os.Stderr, "> %s\n", request.Body)
		}
	}

	httpRsp, err := authenticatedDo(client, httpReq)
	if err != nil {
		return nil, err
	}
	if flagVerbose {
		fmt.Fprintf(os.Stderr, "< %s %s\n", httpRsp.Proto, httpRsp.Status)
	}
	return httpRsp, nil

}

// apiReadResponse reads the whole of a response, and closes it
func apiReadResponse(httpRsp *http.Response) (*apiResponse, error) {
	defer httpRsp.Body.Close()
	contents, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the response: %w", err)
	}
	return &apiResponse{Proto: httpRsp.Proto, Status: httpRsp.Status, StatusCode: httpRsp.StatusCode, Header: httpRsp.Header, Body: contents}, nil
}

// apiWriteHead writes a response's status line and headers, for --include
func apiWriteHead(out io.Writer, proto string, status string, header http.Header) {
	fmt.Fprintf(out, "%s %s\n", proto, status)
	for _, key := range apiSortedKeys(header) {
		fmt.Fprintf(out, "%s: %s\n", key, strings.Join(header[key], ", "))
	}
	fmt.Fprintln(out)
}

// apiStream writes a response as it arrives: with --include its status line and headers
// first, then the body, for as long as the hub keeps sending, ending in a newline
func apiStream(out io.Writer, httpRsp *http.Response) error {
	defer httpRsp.Body.Close()
	if flagApiInclude {
		apiWriteHead(out, httpRsp.Proto, httpRsp.Status, httpRsp.Header)
	}
	last := byte('\n')
	buffer := make([]byte, 32*1024)
	for {
		n, err := httpRsp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			last = buffer[n-1]
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading the response: %w", err)
		}
	}
	if last != '\n' {
		fmt.Fprintln(out)
	}
	return nil
}

// apiWrite writes a response read whole: with --include its status line and headers
// first, and the body as it came, or indented with --pretty when it is JSON, ending in a
// newline
func apiWrite(out io.Writer, rsp *apiResponse) {
	if flagApiInclude {
		apiWriteHead(out, rsp.Proto, rsp.Status, rsp.Header)
	}
	body := rsp.Body
	if flagPretty {
		var decoded any
		if json.Unmarshal(body, &decoded) == nil {
			if indented, err := note.JSONMarshalIndent(decoded, "", "    "); err == nil {
				body = indented
			}
		}
	}
	out.Write(body)
	if len(body) != 0 && body[len(body)-1] != '\n' {
		fmt.Fprintln(out)
	}
}

// apiFailure is the error for a response whose status says the request failed, with the
// reason the hub gave in its JSON body, if any
func apiFailure(rsp *apiResponse) error {
	if rsp.StatusCode < 400 {
		return nil
	}
	reason := ""
	var body map[string]any
	if json.Unmarshal(rsp.Body, &body) == nil {
		for _, key := range []string{"err", "message", "error"} {
			if text, isString := body[key].(string); isString && text != "" {
				reason = ": " + text
				break
			}
		}
	}
	return fmt.Errorf("HTTP %s%s", rsp.Status, reason)
}

// apiSortedKeys returns a header's keys in order, for a stable display
func apiSortedKeys(header http.Header) []string {
	keys := []string{}
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// apiSpec shows the hub's OpenAPI spec, which is public, from wherever the hub serves it
func apiSpec(out io.Writer, client *http.Client) error {
	tried := apiSpecURLs()
	for _, specURL := range tried {
		rsp, err := client.Get(specURL)
		if err != nil {
			continue
		}
		contents, err := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		if err != nil || rsp.StatusCode != http.StatusOK || strings.HasPrefix(rsp.Header.Get("Content-Type"), "text/html") {
			continue
		}
		out.Write(contents)
		if len(contents) != 0 && contents[len(contents)-1] != '\n' {
			fmt.Fprintln(out)
		}
		return nil
	}
	return fmt.Errorf("no OpenAPI spec was found at %s", strings.Join(tried, " or "))
}
