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
	"github.com/golang/snappy"
)

// Side-loads a file to the DFU area of the notecard, to avoid download
func dfuSideload(filename string, verbose bool) (err error) {

	// Do a card.binary transaction to see if the notecard is capable of
	// doing binary sideloads, and if so, how large.
	binaryMax := 0
	var rsp notecard.Request
	rsp, err = card.TransactionRequest(notecard.Request{Req: "card.binary"})
	if err == nil {

		// Get the maximum size that the notecard can handle
		binaryMax = int(rsp.Max)

		// Use shorter delays when sending to Notecard, for performance
		notecard.RequestSegmentMaxLen = 1024
		notecard.RequestSegmentDelayMs = 5

	}

	// Read the file up-front so we can handle this common failure
	// before we go into dfu mode
	var bin []byte
	bin, err = ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	// Determine the file type
	filetype := notehub.HubFileTypeUserFirmware
	if dfuIsNotecardFirmware(&bin) {
		filetype = notehub.HubFileTypeCardFirmware

		// Don't allow notecard firmware to be uploaded unless we can do it with binary
		if binaryMax == 0 {
			return fmt.Errorf("notecard is running firmware that is too old to use sideload")
		}

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

	// If not a notecard DFU operation, place the card into dfu mode to access external storage
	if filetype != notehub.HubFileTypeCardFirmware {

		fmt.Printf("placing notecard into DFU mode so that we can send file to its external flash storage\n")

		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", Mode: "dfu"})
		if err != nil {
			return
		}

		// Make sure we restore the mode on exit
		defer func() {
			fmt.Printf("restoring notecard so that it is no longer in DFU mode\n")
			card.TransactionRequest(notecard.Request{Req: "hub.set", Mode: "dfu-completed"})
		}()

		// Wait until dfu status says that we're in DFU mode
		for {
			fmt.Printf("waiting for notecard to power-up the external storage\n")
			_, err = card.TransactionRequest(notecard.Request{Req: "dfu.put"})
			if err != nil && !note.ErrorContains(err, note.ErrDFUNotReady) && !note.ErrorContains(err, note.ErrCardIo) {
				return
			}
			if err == nil {
				break
			}
			time.Sleep(1500 * time.Millisecond)
		}

	}

	// Do the write
	fmt.Printf("sending DFU binary to notecard\n")
	err = loadBin(filetype, filename, bin, binaryMax)
	if err != nil {
		return
	}

	// Done
	fmt.Printf("sideload completed\n")
	return

}

// Side-load a binary image
func loadBin(filetype string, filename string, bin []byte, binaryMax int) (err error) {
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
	dbu.FileType = filetype
	var body map[string]interface{}
	body, err = note.ObjectToBody(dbu)
	if err != nil {
		return
	}

	// Issue the first request, which is to initiate the DFU put
	chunkLen := 0
	compressionMode := ""
	for {
		req = notecard.Request{Req: "dfu.put"}
		req.Body = &body
		rsp, err = card.TransactionRequest(req)
		if err != nil {
			return
		}

		// By default, use the chunk length being supplied to us by the notecard
		compressionMode = rsp.Mode
		chunkLen = int(rsp.Length)

		// If we support binary, use the binary maximum for performance & reliability.
		// Note that we are guaranteed that if we support large binaries that the
		// notecard will tell us not to use compression.
		if binaryMax > 0 {
			chunkLen = binaryMax
		}

		// Occasionally because of comms being out-of-sync (because of killing
		// the command line utility) we get a response that doesn't have the appropriate
		// fields because we are out of sync.  This is defensive
		// coding that ensures that we don't proceed until we get in sync.
		if chunkLen > 0 {
			break
		}
		time.Sleep(750)
	}

	// Send the chunk to sideload
	offset := 0
	lenRemaining := totalLen
	beganSecs := time.Now().UTC().Unix()
	for lenRemaining > 0 {

		// Determine how much to send
		thisLen := lenRemaining
		if thisLen > chunkLen {
			thisLen = chunkLen
		}

		// Send the chunk
		fmt.Printf("side-loading %d bytes (%.0f%% %d remaining)\n", thisLen, float64(lenRemaining*100)/float64(totalLen), lenRemaining)
		req = notecard.Request{Req: "dfu.put"}
		req.Offset = int32(offset)
		req.Length = int32(thisLen)
		payload := bin[offset : offset+thisLen]
		if compressionMode == "snappy" {
			compressedPayload := snappy.Encode(nil, payload)
			req.Payload = &compressedPayload
		} else {
			req.Payload = &payload
		}
		req.Status = fmt.Sprintf("%x", md5.Sum(*req.Payload))

		// If we're doing binary, do the transaction
		if binaryMax > 0 {

			// Encode COBS
			var payloadEncoded []byte
			payloadEncoded, err = notecard.CobsEncode(payload, byte('\n'))
			if err != nil {
				return
			}

			// Send the COBS data to the notecard
			req2 := notecard.Request{Req: "card.binary.put"}
			req2.Cobs = int32(len(payloadEncoded))
			rsp, err = card.TransactionRequest(req2)
			if err != nil {
				return
			}
			payloadEncoded = append(payloadEncoded, byte('\n'))
			err = card.SendBytes(payloadEncoded)
			if err != nil {
				return
			}

			// Verify that the binary made it to the notecard
			var rsp2 notecard.Request
			rsp2, err = card.TransactionRequest(notecard.Request{Req: "card.binary"})
			if err != nil {
				return
			}
			if int(rsp2.Length) != len(payload) {
				return fmt.Errorf("notecard payload is insufficient (%d sent, %d received)", len(payload), rsp2.Length)
			}

			// Now that it's been received successfully, remove the payload and
			// tell the notecard to fetch the payload from the large binary area.
			req.Payload = nil
			req.Binary = true

		}

		// Perform the request
		rsp, err = card.TransactionRequest(req)
		if err != nil {
			if note.ErrorContains(err, note.ErrCardIo) {
				// Just silently retry {io} errors
				fmt.Printf("retrying after error: %s\n", err)
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
	elapsedSecs := (time.Now().UTC().Unix() - beganSecs) + 1
	fmt.Printf("%d seconds (%.0f Bps)\n", elapsedSecs, float64(totalLen)/float64(elapsedSecs))

	// Done
	return

}
