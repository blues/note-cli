// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"

	"github.com/blues/note-go/notecard"
)

// hostTimeValue expresses a host timestamp as <epoch-seconds>.<six-digits-of-microseconds>, the
// form the "card.time.calibrate" request expects in its "value" field. We build it from the host's
// microsecond clock so the Notecard sees the most precise instant we can report.
func hostTimeValue(t time.Time) float64 {
	return float64(t.UnixMicro()) / 1000000.0
}

// rtc measures the drift of the Notecard's real-time clock relative to the host computer's clock.
// Rather than computing the drift on the host, it feeds the host's current time to the Notecard's
// "card.time.calibrate" calibration request once per second and lets the Notecard do the math, reporting back the
// estimated daily drift and its confidence in that estimate.
//
// The mode selects the behavior:
//   - "test": run the measurement loop until the Notecard reports 100% confidence, printing each
//     second's estimate, without altering the Notecard's stored calibration.
//   - "calibrate": same as "test", but reset the stored calibration on the first request and commit
//     the freshly measured calibration once 100% confidence is reached.
//   - "reset": clear the Notecard's stored calibration and exit immediately, without measuring.
//   - "calibration": print the Notecard's current stored calibration (returned in "daily") and exit.
//
// The measuring modes assume the host has an accurate, high-resolution clock to calibrate against.
func rtc(mode string, verbose bool) (err error) {

	// Quiet the debug output unless the user asked for verbosity, so that our once-per-second
	// lines aren't interleaved with transaction tracing.
	if !verbose {
		card.DebugOutput(false, false)
	}

	// The mode selects what we do. "reset" and "calibration" are one-shot requests that exit
	// immediately; "test" and "calibrate" run the multi-second measurement loop below.
	var calibrating bool
	switch mode {

	case "reset":
		// Clear the Notecard's stored calibration with "reset":true and exit immediately, without
		// running any measurement.
		_, err = card.TransactionRequest(notecard.Request{Req: "card.time.calibrate", Reset: true})
		return

	case "calibration":
		// Read the Notecard's current stored calibration. A bare "card.time.calibrate" with no
		// arguments returns the calibration in "daily" as seconds of drift per day.
		var rsp notecard.Request
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.time.calibrate"})
		if err != nil {
			return
		}
		fmt.Printf("rtc: daily:%.6f\n", rsp.Daily)
		return

	case "test":
		calibrating = false
	case "calibrate":
		calibrating = true
	default:
		return fmt.Errorf("rtc: mode must be \"test\", \"calibrate\", \"reset\", or \"calibration\", not %q", mode)
	}

	// Open the calibration run with "start":true, handing the Notecard the host's current time as
	// the origin from which it will track drift. Every "card.time.calibrate" request always carries
	// "value"; only this first one carries "start". When calibrating, we also reset the Notecard's
	// stored calibration here so the run starts from a clean slate.
	_, err = card.TransactionRequest(notecard.Request{Req: "card.time.calibrate", Value: hostTimeValue(time.Now()), Start: true, Reset: calibrating})
	if err != nil {
		return
	}

	// Emit one measurement per second, on a one-second cadence. Each tick sends the host's current
	// time and prints the drift estimate the Notecard returns. We run until the Notecard reports
	// 100% confidence three times, then we're done.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	hundreds := 0
	for i := 1; ; i++ {
		<-ticker.C

		var rsp notecard.Request
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.time.calibrate", Value: hostTimeValue(time.Now())})
		if err != nil {
			return
		}

		// "daily" is the Notecard's estimated clock drift in seconds per day; "calibration" is the
		// Notecard's percentage confidence in that estimate, which climbs as the run lengthens. When
		// the Notecard returns a "status" we append it in parens so the operator sees it each second.
		msg := fmt.Sprintf("%d rtc: daily:%.6f calibration:%.1f%%", i, rsp.Daily, rsp.Calibration)
		if rsp.Status != "" {
			msg += fmt.Sprintf(" (%s)", rsp.Status)
		}
		fmt.Println(msg)

		if rsp.Calibration < 100 {
			continue
		}

		// After a min number of 100% readings we're confident in the measurement and we're done. When
		// calibrating, commit the freshly measured calibration to the Notecard with "set":true
		// before exiting.
		hundreds++
		if hundreds >= 10 {
			if calibrating {
				_, err = card.TransactionRequest(notecard.Request{Req: "card.time.calibrate", Value: hostTimeValue(time.Now()), Set: true})
			}
			return
		}
	}
}
