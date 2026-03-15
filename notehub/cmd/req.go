// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/blues/note-go/note"
)

// Used by req functions
var reqFlagApp string
var reqFlagProduct string
var reqFlagDevice string

// Add an arg to an URL query string
func addQuery(in string, key string, value string) (out string) {
	out = in
	if value != "" {
		if out == "" {
			out += "?"
		} else {
			out += "&"
		}
		out += key
		out += "=\""
		out += value
		out += "\""
	}
	return
}

// Perform a V0 HTTP request
func reqHubV0JSON(verbose bool, hub string, request []byte, requestFile string, filetype string, filetags string, filenotes string, overwrite bool, dropNonJSON bool, outq chan string) (response []byte, err error) {
	fn := ""
	path := strings.Split(requestFile, "/")
	if len(path) > 0 {
		fn = path[len(path)-1]
	}

	if hub == "" {
		hub = GetAPIHub()
	}

	httpurl := fmt.Sprintf("https://%s/req", hub)
	query := addQuery("", "app", reqFlagApp)
	if reqFlagApp == "" {
		query = addQuery("", "product", reqFlagProduct)
	}
	query = addQuery(query, "device", reqFlagDevice)
	query = addQuery(query, "upload", fn)
	if overwrite {
		query = addQuery(query, "overwrite", "true")
	}
	if filetype != "" {
		query = addQuery(query, "type", filetype)
	}
	if filetags != "" {
		query = addQuery(query, "tags", filetags)
	}
	if filenotes != "" {
		query = addQuery(query, "filenotes", url.PathEscape(filenotes))
	}
	httpurl += query

	var fileContents []byte
	var fileLength int
	buffer := bytes.NewBuffer(request)
	if requestFile != "" {
		fileContents, err = os.ReadFile(requestFile)
		if err != nil {
			return
		}
		fileLength = len(fileContents)
		buffer = bytes.NewBuffer(fileContents)
	}

	httpReq, err := http.NewRequest("POST", httpurl, buffer)
	if err != nil {
		return
	}
	httpReq.Header.Set("User-Agent", "notehub-client")
	if requestFile != "" {
		httpReq.Header.Set("Content-Length", fmt.Sprintf("%d", fileLength))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	err = AddAuthenticationHeader(httpReq)
	if err != nil {
		return
	}

	if verbose {
		fmt.Printf("%s\n", string(request))
	}

	httpClient := newRetryHTTPClient()
	httpRsp, err2 := httpClient.Do(httpReq)
	if err2 != nil {
		if isNetworkError(err2) {
			err = fmt.Errorf("unable to connect to %s: %w", hub, err2)
		} else {
			err = err2
		}
		return
	}

	// Note that we must do this with no timeout specified on
	// the httpClient, else monitor mode would time out.
	b := make([]byte, 2048)
	linebuf := []byte{}
	for {
		n, err2 := httpRsp.Body.Read(b)
		if n > 0 {
			// Append to result buffer if no outq is specified
			if outq == nil {
				response = append(response, b[:n]...)
			} else {
				// Enqueue lines for monitoring
				linebuf = append(linebuf, b[:n]...)
				for {
					// Parse out a full line and queue it, saving the leftover
					i := bytes.IndexRune(linebuf, '\n')
					if i == -1 {
						break
					}
					line := linebuf[0 : i+1]
					linebuf = linebuf[i+1:]
					if !dropNonJSON {
						outq <- string(line)
					} else {
						if strings.HasPrefix(string(line), "{") {
							outq <- string(line)
						}
					}

					// Remember the very last line as the response, in case it
					// was an error and we're about to get an io.EOF
					response = line
				}
			}
		}
		if err2 != nil {
			if err2 != io.EOF {
				err = err2
				return
			}
			break
		}
	}

	if verbose {
		fmt.Printf("%s\n", string(response))
	}

	return
}

// Process an HTTPS request
func reqHubV1JSON(verbose bool, hub string, verb string, url string, body []byte) (response []byte, err error) {
	verb = strings.ToUpper(verb)

	httpurl := fmt.Sprintf("https://%s%s", hub, url)
	buffer := &bytes.Buffer{}
	if body != nil {
		buffer = bytes.NewBuffer(body)
	}
	httpReq, err := http.NewRequest(verb, httpurl, buffer)
	if err != nil {
		return
	}
	httpReq.Header.Set("User-Agent", "notehub-client")
	httpReq.Header.Set("Content-Type", "application/json")
	err = AddAuthenticationHeader(httpReq)
	if err != nil {
		return
	}

	if verbose {
		fmt.Printf("%s %s\n", verb, httpurl)
		if len(body) != 0 {
			fmt.Printf("%s\n", string(body))
		}
	}

	httpClient := newRetryHTTPClient()
	httpRsp, err2 := httpClient.Do(httpReq)
	if err2 != nil {
		if isNetworkError(err2) {
			err = fmt.Errorf("unable to connect to %s: %w", hub, err2)
		} else {
			err = err2
		}
		return
	}
	if verbose {
		fmt.Printf("STATUS %d\n", httpRsp.StatusCode)
	}

	response, err = io.ReadAll(httpRsp.Body)
	if err != nil {
		return
	}

	if verbose && len(response) != 0 {
		fmt.Printf("%s\n", string(response))
	}

	// Check for HTTP error status codes
	if httpRsp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("please use -signin to authenticate")
		return
	}

	// Check for other HTTP error status codes (4xx, 5xx)
	if httpRsp.StatusCode >= 400 {
		// Try to parse error message from response body
		if len(response) > 0 {
			var errResp map[string]interface{}
			if unmarshalErr := note.JSONUnmarshal(response, &errResp); unmarshalErr == nil {
				if errMsg, ok := errResp["err"].(string); ok {
					err = fmt.Errorf("HTTP %d: %s", httpRsp.StatusCode, errMsg)
					return
				}
			}
		}
		// Fallback to generic error if we couldn't parse the error message
		err = fmt.Errorf("HTTP %d: %s", httpRsp.StatusCode, http.StatusText(httpRsp.StatusCode))
		return
	}

	return
}
