// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"math/rand"

	"github.com/blues/note-go/notecard"
)

// Performs N iterations of an echo test
func echo(iterations int) (err error) {
	var req, rsp notecard.Request

	len := 1
	maxLen := 8192
	lenIterations := 0
	lenMaxIterations := 10
	for i := 0; i < iterations; i++ {

		lenIterations++
		if lenIterations > lenMaxIterations {
			lenIterations = 0
			len = len * 2
			if len > maxLen {
				len = 1
			}
		}

		fmt.Printf("%d: %d bytes\n", i, len)

		bin := make([]byte, len)
		rand.Read(bin)
		req = notecard.Request{Req: "echo"}
		req.Payload = &bin
		rsp, err = card.TransactionRequest(req)
		if err != nil {
			return
		}
		if !bytes.Equal(bin, *rsp.Payload) {
			return fmt.Errorf("request or response corrupted")
		}

	}

	return

}
