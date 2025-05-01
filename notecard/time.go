// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Get the unix epoch time from the notehub
func notehubTime() (epochTime int64, err error) {

	// Do a ping transaction to the Notehub to get the time
	var webreq *http.Request
	var webrsp *http.Response
	webreq, err = http.NewRequest("GET", "https://api.notefile.net/ping", nil)
	if err != nil {
		return
	}
	httpclient := &http.Client{
		Timeout: time.Second * time.Duration(15),
	}
	webrsp, err = httpclient.Do(webreq)
	if err != nil {
		return
	}

	// Unmarshal the ping response
	var webrspJSON []byte
	webrspJSON, err = os.ReadAll(webrsp.Body)
	webrsp.Body.Close()
	if err != nil {
		return
	}
	var pingrsp map[string]interface{}
	err = json.Unmarshal(webrspJSON, &pingrsp)
	if err != nil {
		return
	}

	// Extract the body, and the time field as an ISO string
	body, exists := pingrsp["body"].(map[string]interface{})
	if !exists {
		err = fmt.Errorf("badly formatted ping reply: missing body field")
		return
	}
	timestr, exists := body["time"].(string)
	if !exists {
		err = fmt.Errorf("badly formatted ping reply: missing time field")
		return
	}

	// Parse the ISO time string
	var t time.Time
	t, err = time.Parse("2006-01-02T15:04:05Z", timestr)
	if err != nil {
		err = fmt.Errorf("badly formatted time in ping reply: %s", err)
		return
	}

	// Return a unix Epoch time
	epochTime = t.Unix()
	return

}
