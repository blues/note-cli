// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package notecard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/blues/note-go/note"
)

// Leaseing service parameters
const leaseServiceUrl = "https://notepod.io:8123"

// Lease transaction
type LeaseTransaction struct {
	Request    string `json:"req,omitempty"`
	Lessor     string `json:"lessor,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Expires    int64  `json:"expires,omitempty"`
	Error      string `json:"err,omitempty"`
	Handle     string `json:"handle,omitempty"`
	NoResponse bool   `json:"no_response,omitempty"`
	ReqJSON    []byte `json:"request_json,omitempty"`
	RspJSON    []byte `json:"response_json,omitempty"`
}

// Request types
const (
	ReqReserve     = "reserve"
	ReqTransaction = "transaction"
)

// Perform an HTTP transaction to the lease service
func leaseService(req LeaseTransaction, promoteError bool) (rsp LeaseTransaction, err error) {

	reqj, err := json.Marshal(req)
	if err != nil {
		return rsp, err
	}

	// Send the transaction
	hreq, err := http.NewRequest("POST", leaseServiceUrl, bytes.NewBuffer(reqj))
	if err != nil {
		return rsp, fmt.Errorf("%s %s", err, note.ErrCardIo)
	}
	hcli := &http.Client{Timeout: time.Second * 90}
	hrsp, err := hcli.Do(hreq)
	if err != nil {
		return rsp, fmt.Errorf("%s %s", err, note.ErrCardIo)
	}
	defer hrsp.Body.Close()

	// Read the response
	var rspjb bytes.Buffer
	_, err = io.Copy(&rspjb, hrsp.Body)
	if err != nil {
		return rsp, fmt.Errorf("%s %s", err, note.ErrCardIo)
	}
	rspj := rspjb.Bytes()

	err = note.JSONUnmarshal(rspj, &rsp)
	if err != nil {
		return rsp, fmt.Errorf("%s %s", err, note.ErrCardIo)
	}

	if promoteError && rsp.Error != "" {
		return rsp, fmt.Errorf("%s", rsp.Error)
	}

	return rsp, nil

}

// Open or reopen the remote card by taking out a lease, or by renewing the lease.
func leaseReopen(context *Context, portConfig int) (err error) {

	// Perform the lease transaction
	req := LeaseTransaction{}
	req.Request = ReqReserve
	req.Lessor = callerID()
	req.Scope = context.leaseScope
	req.Expires = context.leaseExpires
	rsp, err := leaseService(req, true)
	if err != nil {
		return
	}

	// Save the handle to the allocated device
	context.leaseHandle = rsp.Handle

	return
}

// Close a remote notecard
func leaseClose(context *Context) {
}

// Perform a remote transaction
func leaseTransaction(context *Context, portConfig int, noResponse bool, reqJSON []byte) (rspJSON []byte, err error) {

	// Perform the lease transaction
	req := LeaseTransaction{}
	req.Request = ReqTransaction
	req.Handle = context.leaseHandle
	req.ReqJSON = reqJSON
	req.NoResponse = noResponse
	rsp, err := leaseService(req, true)
	if err != nil {
		return
	}

	// Done
	return rsp.RspJSON, nil

}
