// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
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

func noDfuBinaryRetryDelay(t *testing.T) {
	t.Helper()
	previousDelay := dfuBinaryRetryDelay
	dfuBinaryRetryDelay = 0
	t.Cleanup(func() {
		dfuBinaryRetryDelay = previousDelay
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

func TestLoadBinRetriesBadBinValidation(t *testing.T) {
	noDfuBinaryRetryDelay(t)

	var binaryPuts int
	var binarySends int
	var dfuPuts []notecard.Request

	mockCard(t, func(request notecard.Request, noResponse bool, _ []byte) ([]byte, error) {
		if noResponse {
			binarySends++
			return []byte(`{}`), nil
		}

		switch request.Req {
		case "dfu.put":
			if request.Body != nil {
				return []byte(`{"length":4}`), nil
			}
			dfuPuts = append(dfuPuts, request)
			return []byte(`{"pending":false}`), nil
		case "card.binary.put":
			binaryPuts++
			return []byte(`{}`), nil
		case "card.binary":
			if binarySends == 1 {
				return []byte(`{"err":"binary receive prematurely terminated {bad-bin}"}`), nil
			}
			return []byte(`{"length":4}`), nil
		default:
			t.Fatalf("unexpected request: %#v", request)
			return nil, nil
		}
	})

	if err := loadBin("host", "firmware.bin", []byte("0123"), 4); err != nil {
		t.Fatalf("loadBin returned an error: %v", err)
	}

	if binaryPuts != 2 || binarySends != 2 {
		t.Fatalf("binary transfer attempts = %d puts, %d sends; want 2", binaryPuts, binarySends)
	}
	if len(dfuPuts) != 1 || !dfuPuts[0].Binary || dfuPuts[0].Payload != nil {
		t.Fatalf("dfu.put did not use the validated binary payload: %#v", dfuPuts)
	}
}

func TestLoadBinStopsAfterBadBinRetryLimit(t *testing.T) {
	noDfuBinaryRetryDelay(t)

	var binaryPuts int
	var binarySends int
	var dfuPuts int

	mockCard(t, func(request notecard.Request, noResponse bool, _ []byte) ([]byte, error) {
		if noResponse {
			binarySends++
			return []byte(`{}`), nil
		}

		switch request.Req {
		case "dfu.put":
			if request.Body != nil {
				return []byte(`{"length":4}`), nil
			}
			dfuPuts++
			return []byte(`{"pending":false}`), nil
		case "card.binary.put":
			binaryPuts++
			return []byte(`{}`), nil
		case "card.binary":
			return []byte(`{"err":"binary receive prematurely terminated {bad-bin}"}`), nil
		default:
			t.Fatalf("unexpected request: %#v", request)
			return nil, nil
		}
	})

	if err := loadBin("host", "firmware.bin", []byte("0123"), 4); err == nil {
		t.Fatal("loadBin succeeded after exhausting {bad-bin} retries")
	}
	if binaryPuts != dfuBinaryRetries || binarySends != dfuBinaryRetries {
		t.Fatalf("binary transfer attempts = %d puts, %d sends; want %d", binaryPuts, binarySends, dfuBinaryRetries)
	}
	if dfuPuts != 0 {
		t.Fatalf("dfu.put calls = %d, want 0 after failed binary validation", dfuPuts)
	}
}

func TestLoadBinDoesNotRetryNonBadBinFailure(t *testing.T) {
	noDfuBinaryRetryDelay(t)

	var binaryPuts int
	mockCard(t, func(request notecard.Request, noResponse bool, _ []byte) ([]byte, error) {
		if noResponse {
			return []byte(`{}`), nil
		}

		switch request.Req {
		case "dfu.put":
			if request.Body != nil {
				return []byte(`{"length":4}`), nil
			}
			return []byte(`{"pending":false}`), nil
		case "card.binary.put":
			binaryPuts++
			return []byte(`{}`), nil
		case "card.binary":
			return []byte(`{"err":"unrelated binary failure"}`), nil
		default:
			t.Fatalf("unexpected request: %#v", request)
			return nil, nil
		}
	})

	if err := loadBin("host", "firmware.bin", []byte("0123"), 4); err == nil {
		t.Fatal("loadBin succeeded after a non-{bad-bin} validation error")
	}
	if binaryPuts != 1 {
		t.Fatalf("binary transfer attempts = %d, want 1 for a non-{bad-bin} error", binaryPuts)
	}
}
