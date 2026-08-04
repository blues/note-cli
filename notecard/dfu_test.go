// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/blues/note-go/notecard"
)

func mockCard(t *testing.T, transaction func(notecard.Request, bool, []byte) ([]byte, error)) {
	t.Helper()
	previousCard := card
	card = &notecard.Context{
		TransactionFn: func(_ *notecard.Context, _ int, noResponse bool, requestJSON []byte, _ bool) ([]byte, error) {
			if noResponse {
				return transaction(notecard.Request{}, true, requestJSON)
			}

			var request notecard.Request
			if err := json.Unmarshal(requestJSON, &request); err != nil {
				t.Fatalf("decode request %q: %v", requestJSON, err)
			}
			return transaction(request, false, nil)
		},
	}
	t.Cleanup(func() {
		card = previousCard
	})
}

func noDfuBinaryRecoveryDelay(t *testing.T) {
	t.Helper()
	previousDelay := dfuBinaryRecoveryDelay
	dfuBinaryRecoveryDelay = 0
	t.Cleanup(func() {
		dfuBinaryRecoveryDelay = previousDelay
	})
}

func TestDfuBinaryCapacityResetsStaleReceive(t *testing.T) {
	previousSegmentMaxLen := notecard.RequestSegmentMaxLen
	previousSegmentDelayMs := notecard.RequestSegmentDelayMs
	t.Cleanup(func() {
		notecard.RequestSegmentMaxLen = previousSegmentMaxLen
		notecard.RequestSegmentDelayMs = previousSegmentDelayMs
	})

	var receivedRequest notecard.Request
	mockCard(t, func(request notecard.Request, noResponse bool, _ []byte) ([]byte, error) {
		if noResponse {
			t.Fatal("card.binary capacity request must expect a response")
		}
		receivedRequest = request
		return []byte(`{"max":16384}`), nil
	})

	capacity, err := dfuBinaryCapacity(false)
	if err != nil {
		t.Fatalf("dfuBinaryCapacity returned an error: %v", err)
	}
	if receivedRequest.Req != "card.binary" || !receivedRequest.Reset {
		t.Fatalf("capacity request = %#v, want card.binary reset:true", receivedRequest)
	}
	if capacity != 16384 {
		t.Fatalf("capacity = %d, want 16384", capacity)
	}
}

func TestLoadBinShrinksBinaryChunksAfterFailedTransfer(t *testing.T) {
	noDfuBinaryRecoveryDelay(t)

	var binaryPutSizes []int
	var binaryResets int
	var binarySends int
	var lastBinaryPayloadLen int
	var sentDFUPuts []notecard.Request

	mockCard(t, func(request notecard.Request, noResponse bool, raw []byte) ([]byte, error) {
		if noResponse {
			binarySends++
			if len(raw) == 0 || raw[len(raw)-1] != '\n' {
				t.Fatalf("binary payload must end in a newline")
			}
			decoded, err := notecard.CobsDecode(raw[:len(raw)-1], byte('\n'))
			if err != nil {
				t.Fatalf("decode binary payload: %v", err)
			}
			lastBinaryPayloadLen = len(decoded)
			return []byte("{}"), nil
		}

		switch request.Req {
		case "dfu.put":
			if request.Body != nil {
				return []byte(`{"length":8}`), nil
			}
			sentDFUPuts = append(sentDFUPuts, request)
			return []byte(`{"pending":false}`), nil
		case "card.binary.put":
			binaryPutSizes = append(binaryPutSizes, int(request.Cobs))
			return []byte(`{}`), nil
		case "card.binary":
			if request.Reset {
				binaryResets++
				return []byte(`{}`), nil
			}
			switch binarySends {
			case 1, 2, 3:
				return []byte(`{"err":"binary receive prematurely terminated {bad-bin}"}`), nil
			default:
				return []byte(fmt.Sprintf(`{"length":%d}`, lastBinaryPayloadLen)), nil
			}
		default:
			t.Fatalf("unexpected request: %#v", request)
			return nil, nil
		}
	})

	firmware := make([]byte, 16390)
	for i := range firmware {
		firmware[i] = byte(i)
	}
	if err := loadBin("host", "firmware.bin", firmware, 16384); err != nil {
		t.Fatalf("loadBin returned an error: %v", err)
	}

	if binaryResets != 3 {
		t.Fatalf("card.binary reset calls = %d, want 3", binaryResets)
	}
	if len(binaryPutSizes) != 6 {
		t.Fatalf("binary put calls = %d, want 6 (three failed 16 KiB attempts and three 8 KiB attempts)", len(binaryPutSizes))
	}
	if binaryPutSizes[0] != binaryPutSizes[1] || binaryPutSizes[1] != binaryPutSizes[2] || binaryPutSizes[2] <= binaryPutSizes[3] || binaryPutSizes[3] != binaryPutSizes[4] || binaryPutSizes[5] >= binaryPutSizes[4] {
		t.Fatalf("binary COBS transfer sizes = %v, want three 16 KiB attempts followed by 8 KiB chunks", binaryPutSizes)
	}
	if len(sentDFUPuts) != 3 {
		t.Fatalf("dfu.put calls = %d, want 3", len(sentDFUPuts))
	}
	for i, request := range sentDFUPuts {
		if !request.Binary || request.Payload != nil {
			t.Fatalf("dfu.put %d did not use the staged binary payload: %#v", i, request)
		}
	}
}

func TestLoadBinFallsBackToInlineAfterSmallestBinaryChunkFails(t *testing.T) {
	noDfuBinaryRecoveryDelay(t)

	var binaryResets int
	var inlineDFUPuts []notecard.Request

	mockCard(t, func(request notecard.Request, noResponse bool, _ []byte) ([]byte, error) {
		if noResponse {
			return []byte(`{}`), nil
		}

		switch request.Req {
		case "dfu.put":
			if request.Body != nil {
				return []byte(`{"length":2}`), nil
			}
			inlineDFUPuts = append(inlineDFUPuts, request)
			return []byte(`{"pending":false}`), nil
		case "card.binary.put":
			return []byte(`{}`), nil
		case "card.binary":
			if request.Reset {
				binaryResets++
				return []byte(`{}`), nil
			}
			return []byte(`{"err":"binary receive prematurely terminated {bad-bin}"}`), nil
		default:
			t.Fatalf("unexpected request: %#v", request)
			return nil, nil
		}
	})

	if err := loadBin("host", "firmware.bin", []byte("0123"), 2); err != nil {
		t.Fatalf("loadBin returned an error: %v", err)
	}

	if binaryResets != 3 {
		t.Fatalf("card.binary reset calls = %d, want 3", binaryResets)
	}
	if len(inlineDFUPuts) != 2 {
		t.Fatalf("inline dfu.put calls = %d, want 2", len(inlineDFUPuts))
	}
	for i, request := range inlineDFUPuts {
		if request.Binary || request.Payload == nil || len(*request.Payload) != 2 {
			t.Fatalf("dfu.put %d did not use a two-byte inline payload: %#v", i, request)
		}
	}
}
