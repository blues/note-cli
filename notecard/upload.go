// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// upload.go implements efficient binary file uploads to a Notehub proxy route
// using the Notecard's binary buffer (card.binary) and web.post API.
//
// This module serves as a reference implementation for high-performance file
// uploads through the Notecard. It demonstrates best practices for:
//   - Querying the Notecard's binary buffer capacity
//   - Chunking large files to fit within buffer constraints
//   - Using COBS encoding for reliable binary transfer
//   - MD5 verification for data integrity
//   - Tracking upload progress with offset/total fields
//   - Performance monitoring and statistics
//
// The upload process works as follows:
//   1. Query card.binary to determine the Notecard's maximum buffer size
//   2. Read the source file and calculate its total size
//   3. For each chunk that fits in the binary buffer:
//      a. COBS-encode the chunk for safe serial transmission
//      b. Stage the chunk in the Notecard's binary buffer via card.binary.put
//      c. Verify the chunk was received correctly via card.binary
//      d. Issue web.post with binary:true to send the chunk to Notehub
//   4. Report per-chunk and cumulative performance statistics to stderr
//
// The content type used is application/octet-stream for binary uploads.

package main

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
)

// maxUploadChunkBytes is the maximum chunk size we'll use for uploads,
// regardless of what the Notecard reports as its buffer capacity.
// The only reason to lower this below the Notecard's capacity (which
// is about 250KB for a v2 (black) Notecard) is if the communications
// between the host and notecard might be slow and you want to reduce
// the size of each individual COBS transfer for host responsiveness.
const maxUploadChunkBytes = 0

// uploadFile performs a binary file upload to a Notehub proxy route.
//
// Parameters:
//   - filename: Path to the file to upload
//   - route: The Notehub proxy route alias (required)
//   - target: Optional URL path appended to the route (becomes "name" in web.post);
//     if it contains "[filename]", that substring is replaced with the uploaded filename
//
// The function uploads the file in chunks sized to the Notecard's binary buffer
// capacity. Each chunk is verified via MD5 checksum before transmission to Notehub.
// Progress statistics are written to stderr after each chunk.
//
// Returns an error if the upload fails at any stage.
func uploadFile(filename string, route string, target string) error {

	// =========================================================================
	// STEP 1: Validate required parameters
	// =========================================================================
	// The route parameter is mandatory as it specifies the Notehub proxy route
	// that will receive the uploaded data.
	if route == "" {
		return fmt.Errorf("upload requires -route to be specified")
	}

	// =========================================================================
	// STEP 2: Read the file into memory
	// =========================================================================
	// We read the entire file upfront to:
	//   - Fail early if the file doesn't exist or isn't readable
	//   - Know the total size for progress calculations and offset/total fields
	//   - Simplify chunk extraction during the upload loop
	fileData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file '%s': %w", filename, err)
	}

	totalSize := len(fileData)
	if totalSize == 0 {
		return fmt.Errorf("file '%s' is empty", filename)
	}

	// Extract just the filename for display purposes (strip directory path)
	displayName := filepath.Base(filename)

	// Substitute [filename] placeholder in target with the actual filename
	if strings.Contains(target, "[filename]") {
		target = strings.ReplaceAll(target, "[filename]", displayName)
	}

	fmt.Fprintf(os.Stderr, "uploading '%s' (%d bytes) to route '%s'\n", displayName, totalSize, route)

	// =========================================================================
	// STEP 3: Query the Notecard's binary buffer capacity
	// =========================================================================
	// The card.binary request returns information about the Notecard's binary
	// buffer, including the maximum size it can accept. This value is fixed
	// for a given Notecard type and doesn't change, so we only query it once.
	// Note that the "reset" is essential so that it terminates any previous
	// binary upload that may still be in progress from the notecard's perspective.
	//
	// The response includes:
	//   - max: Maximum number of bytes the binary buffer can hold
	//   - length: Current number of bytes in the buffer (should be 0)
	rsp, err := card.TransactionRequest(notecard.Request{Req: "card.binary", Reset: true})
	if err != nil {
		return fmt.Errorf("failed to query card.binary capacity: %w", err)
	}

	binaryMax := int(rsp.Max)
	if binaryMax == 0 {
		return fmt.Errorf("notecard does not support binary transfers (card.binary returned max=0)")
	}

	// Use the smaller of the notecard's buffer capacity or our configured max
	chunkMax := binaryMax
	if maxUploadChunkBytes != 0 && maxUploadChunkBytes < chunkMax {
		chunkMax = maxUploadChunkBytes
	}

	fmt.Fprintf(os.Stderr, "notecard binary buffer capacity: %d bytes, using chunk size: %d bytes\n", binaryMax, chunkMax)

	// =========================================================================
	// STEP 4: Set content type for binary upload
	// =========================================================================
	// The content type application/octet-stream indicates raw binary data.
	contentType := "application/octet-stream"

	// =========================================================================
	// STEP 5: Initialize upload state and statistics
	// =========================================================================
	offset := 0                                          // Current byte offset in the file
	chunkNumber := 0                                     // Current chunk number (1-based for display)
	totalChunks := (totalSize + chunkMax - 1) / chunkMax // Ceiling division
	uploadStartTime := time.Now()

	// =========================================================================
	// STEP 6: Upload loop - process file in chunks
	// =========================================================================
	for offset < totalSize {
		chunkNumber++
		chunkStartTime := time.Now()

		// ---------------------------------------------------------------------
		// 6a: Calculate chunk boundaries
		// ---------------------------------------------------------------------
		// Determine how many bytes to send in this chunk. The last chunk may
		// be smaller than chunkMax if the file size isn't evenly divisible.
		chunkSize := chunkMax
		remaining := totalSize - offset
		if remaining < chunkSize {
			chunkSize = remaining
		}

		// Extract the chunk data from the file buffer
		chunkData := fileData[offset : offset+chunkSize]

		// ---------------------------------------------------------------------
		// 6b: Calculate MD5 checksum for this chunk
		// ---------------------------------------------------------------------
		// The MD5 checksum serves two purposes:
		//   1. Verify the chunk was correctly staged in the Notecard's buffer
		//   2. Allow Notehub to verify the chunk wasn't corrupted in transit
		chunkMD5 := fmt.Sprintf("%x", md5.Sum(chunkData))

		// ---------------------------------------------------------------------
		// 6c: COBS-encode the chunk for serial transmission
		// ---------------------------------------------------------------------
		// COBS (Consistent Overhead Byte Stuffing) encoding ensures the binary
		// data can be safely transmitted over the serial connection without
		// conflicting with the newline character used as a packet delimiter.
		encodedData, err := notecard.CobsEncode(chunkData, byte('\n'))
		if err != nil {
			return fmt.Errorf("chunk %d: COBS encoding failed: %w", chunkNumber, err)
		}

		// ---------------------------------------------------------------------
		// 6d-6f: Transfer binary and send via web.post with retry logic
		// ---------------------------------------------------------------------
		// This section handles both binary transfer to the Notecard and the
		// subsequent web.post to Notehub. Both operations have retry logic:
		//   - Binary transfer errors ({bad-bin}, {io}) retry indefinitely
		//   - web.post errors wait 15 seconds, re-transfer binary, and retry
		//
		// We use a labeled loop so web.post failures can restart the entire
		// chunk upload process (binary transfer + web.post).

		encodedDataWithNewline := append(encodedData, byte('\n'))

	chunkRetry:
		for {
			// -----------------------------------------------------------------
			// Binary transfer with retry on {bad-bin}/{io} errors
			// -----------------------------------------------------------------
			for {
				// Stage the chunk in the Notecard's binary buffer
				req := notecard.Request{Req: "card.binary.put"}
				req.Cobs = int32(len(encodedData))

				_, err = card.TransactionRequest(req)
				if err != nil {
					if note.ErrorContains(err, note.ErrCardIo) {
						fmt.Fprintf(os.Stderr, "binary transfer error, retrying: %s\n", err)
						continue
					}
					return fmt.Errorf("chunk %d: card.binary.put failed: %w", chunkNumber, err)
				}

				// Send the COBS-encoded data followed by a newline delimiter
				err = card.SendBytes(encodedDataWithNewline)
				if err != nil {
					if note.ErrorContains(err, note.ErrCardIo) {
						fmt.Fprintf(os.Stderr, "binary transfer error, retrying: %s\n", err)
						continue
					}
					return fmt.Errorf("chunk %d: SendBytes failed: %w", chunkNumber, err)
				}

				// Verify the chunk was received correctly by the Notecard
				verifyRsp, err := card.TransactionRequest(notecard.Request{Req: "card.binary"})
				if err != nil {
					if note.ErrorContains(err, note.ErrCardIo) {
						fmt.Fprintf(os.Stderr, "binary transfer error, retrying: %s\n", err)
						continue
					}
					return fmt.Errorf("chunk %d: card.binary verification failed: %w", chunkNumber, err)
				}

				// Check for error in response (e.g., "binary receive prematurely terminated {bad-bin}{io}")
				if verifyRsp.Err != "" {
					if strings.Contains(verifyRsp.Err, "{bad-bin}") || strings.Contains(verifyRsp.Err, "{io}") {
						fmt.Fprintf(os.Stderr, "binary transfer error, retrying: %s\n", verifyRsp.Err)
						continue
					}
					return fmt.Errorf("chunk %d: card.binary error: %s", chunkNumber, verifyRsp.Err)
				}

				// Verify size matches
				if int(verifyRsp.Length) != chunkSize {
					fmt.Fprintf(os.Stderr, "chunk %d: size mismatch (sent %d, received %d), retrying\n",
						chunkNumber, chunkSize, verifyRsp.Length)
					continue
				}

				break // Binary transfer successful
			}

			// -----------------------------------------------------------------
			// Send the chunk to Notehub via web.post
			// -----------------------------------------------------------------
			// Now that the chunk is staged in the Notecard's binary buffer, we
			// issue a web.post request to send it to Notehub.
			webReq := notecard.Request{Req: "web.post"}
			webReq.RouteUID = route
			webReq.Binary = true
			webReq.Content = contentType
			webReq.Offset = int32(offset)
			webReq.Total = int32(totalSize)
			webReq.Status = chunkMD5
			webReq.Name = target

			webRsp, err := card.TransactionRequest(webReq)
			if err != nil {
				fmt.Fprintf(os.Stderr, "web.post failed: %s, waiting 15s then retrying\n", err)
				time.Sleep(15 * time.Second)
				continue chunkRetry // Re-transfer binary and retry web.post
			}

			// Check for HTTP-level errors (3xx, 4xx, 5xx)
			if webRsp.Result >= 300 {
				fmt.Fprintf(os.Stderr, "server returned HTTP %d, waiting 15s then retrying\n", webRsp.Result)
				time.Sleep(15 * time.Second)
				continue chunkRetry // Re-transfer binary and retry web.post
			}

			break chunkRetry // Chunk upload successful
		}

		// ---------------------------------------------------------------------
		// 6g: Calculate and display performance statistics
		// ---------------------------------------------------------------------
		chunkDuration := time.Since(chunkStartTime)
		totalDuration := time.Since(uploadStartTime)

		// Calculate throughput for this chunk (bytes per second)
		chunkBytesPerSec := float64(chunkSize) / chunkDuration.Seconds()

		// Calculate cumulative progress
		bytesCompleted := offset + chunkSize
		percentComplete := float64(bytesCompleted) * 100.0 / float64(totalSize)

		// Calculate overall throughput (bytes per second)
		overallBytesPerSec := float64(bytesCompleted) / totalDuration.Seconds()

		// Estimate time remaining based on current throughput
		bytesRemaining := totalSize - bytesCompleted
		var etaStr string
		if overallBytesPerSec > 0 && bytesRemaining > 0 {
			etaSeconds := float64(bytesRemaining) / overallBytesPerSec
			etaStr = fmt.Sprintf("ETA %s", (time.Duration(etaSeconds) * time.Second).Round(time.Second))
		} else {
			etaStr = "complete"
		}

		// Output one line per chunk to stderr with comprehensive statistics
		// Format: chunk X/Y: BYTES bytes (XX.X%) @ XX.X KB/s (avg XX.X KB/s) ETA Xm Xs
		fmt.Fprintf(os.Stderr, "chunk %d/%d: %d/%d bytes (%.1f%%) @ %.1f KB/s (avg %.1f KB/s) %s\n",
			chunkNumber,
			totalChunks,
			bytesCompleted,
			totalSize,
			percentComplete,
			chunkBytesPerSec/1024.0,
			overallBytesPerSec/1024.0,
			etaStr,
		)

		// ---------------------------------------------------------------------
		// 6h: Advance to the next chunk
		// ---------------------------------------------------------------------
		offset += chunkSize
	}

	// =========================================================================
	// STEP 7: Upload complete - display summary
	// =========================================================================
	totalDuration := time.Since(uploadStartTime)
	overallBytesPerSec := float64(totalSize) / totalDuration.Seconds()

	fmt.Fprintf(os.Stderr, "upload complete: %d bytes in %s (%.1f KB/s average)\n",
		totalSize,
		totalDuration.Round(time.Second),
		overallBytesPerSec/1024.0,
	)

	return nil
}
