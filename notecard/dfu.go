// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
	"github.com/blues/note-go/notehub"
)

// Side-loads a file to the DFU area of the notecard, to avoid download
func dfuSideload(filename string, verbose bool) (err error) {
	var req notecard.Request

	// Read the file up-front so we can handle this common failure
	// before we go into dfu mode
	var bin []byte
	bin, err = ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	// Sideloading on the Notecard requires that the Notecard's time is set.  This means that
	// in order to sideload, the Notecard might normally need a ProductUID configured and would
	// need to talk to the cloud.  Since this would also mean that the SIM would be provisioned,
	// we clearly could not do this at point of manufacture.  As such, this code uses a feature
	// whereby the Notecard's time can be set if and only if it hasn't yet been set.  We don't
	// trust the time of the local PC, so instead we fetch it from Notehub.
	epochTime, err := notehubTime()
	if err != nil {
		return
	}
	_, err = card.TransactionRequest(notecard.Request{Req: "card.time", Time: epochTime})
	if err != nil {
		return
	}

	// Place the card into dfu mode
	fmt.Printf("placing notecard into DFU mode so that we can send file to its external flash storage\n")
	req = notecard.Request{Req: "hub.set"}
	req.Mode = "dfu"
	_, err = card.TransactionRequest(req)
	if err != nil {
		return
	}

	// Set the current mode to DFU mode
	fmt.Printf("placing notecard into DFU mode so that we can send file to its external flash storage\n")
	_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", Mode: "dfu"})
	if err != nil {
		return
	}

	// Now that we're in DFU mode, make sure we restore the mode on exit
	defer func() {
		fmt.Printf("restoring notecard so that it is no longer in DFU mode\n")
		card.TransactionRequest(notecard.Request{Req: "hub.set", Mode: "dfu-completed"})
	}()

	// Wait until dfu status says that we're in DFU mode
	for {
		fmt.Printf("waiting for notecard to power-up the external flash storage\n")
		_, err = card.TransactionRequest(notecard.Request{Req: "dfu.put"})
		if err != nil && !note.ErrorContains(err, note.ErrDFUNotReady) && !note.ErrorContains(err, note.ErrCardIo) {
			return
		}
		if err == nil {
			break
		}
		time.Sleep(1500 * time.Millisecond)
	}

	// Do the write
	fmt.Printf("sending DFU binary to notecard\n")
	err = loadBin(filename, bin)
	if err != nil {
		return
	}

	// Done
	fmt.Printf("sideload completed\n")
	return
}

// Side-load a binary image
func loadBin(filename string, bin []byte) (err error) {
	var req, rsp notecard.Request
	totalLen := len(bin)

	// Clean up the name to be just the filename portion
	s := strings.Split(filename, "/")
	if len(s) > 1 {
		filename = s[len(s)-1]
	}
	s = strings.Split(filename, "\\")
	if len(s) > 1 {
		filename = s[len(s)-1]
	}

	// Generate the simulated firmware info
	var dbu notehub.HubRequestFile
	dbu.Created = time.Now().Unix()
	dbu.Source = filename
	dbu.MD5 = fmt.Sprintf("%x", md5.Sum(bin))
	dbu.CRC32 = crc32.ChecksumIEEE(bin)
	dbu.Length = totalLen
	dbu.Name = filename
	dbu.FileType = notehub.HubFileTypeUserFirmware
	if dfuIsNotecardFirmware(&bin) {
		dbu.FileType = notehub.HubFileTypeCardFirmware
	}
	var body map[string]interface{}
	body, err = note.ObjectToBody(dbu)
	if err != nil {
		return
	}

	// Issue the first request, which is to initiate the DFU put
	req = notecard.Request{Req: "dfu.put"}
	req.Body = &body
	rsp, err = card.TransactionRequest(req)
	if err != nil {
		return
	}
	chunkLen := int(rsp.Length)

	// Send the chunk to sideload
	offset := 0
	lenRemaining := totalLen
	for lenRemaining > 0 {

		// Determine how much to send
		thisLen := lenRemaining
		if thisLen > chunkLen {
			thisLen = chunkLen
		}

		// Send the chunk
		fmt.Printf("side-loading %d bytes (%d remaining)\n", thisLen, lenRemaining-thisLen)
		req = notecard.Request{Req: "dfu.put"}
		payload := bin[offset : offset+thisLen]
		req.Payload = &payload
		req.Offset = int32(offset)
		req.Length = int32(thisLen)
		rsp, err = card.TransactionRequest(req)
		if err != nil {
			if note.ErrorContains(err, note.ErrCardIo) {
				// Just silently retry {io} errors
				continue
			}
			fmt.Printf("aborting after side-loading error: %s\n", err)
			return
		}

		// Move on to next chunk
		lenRemaining -= thisLen
		offset += thisLen

		// Wait until the migration succeeds
		for rsp.Pending {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "dfu.put"})
			if err != nil {
				if note.ErrorContains(err, note.ErrDFUNotReady) && lenRemaining == 0 {
					err = nil
					break
				}
				fmt.Printf("aborting after error retrieving side-loading status: %s\n", err)
				return
			}
			time.Sleep(750 * time.Millisecond)
		}

	}

	// Done
	return

}
