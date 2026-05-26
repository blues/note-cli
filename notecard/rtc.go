// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"

	"github.com/blues/note-go/notecard"
)

// rtcProbes is the number of rapid card.time reads taken per measurement. We keep only the one
// with the smallest host round-trip time, because that sample has the tightest alignment between
// the Notecard's reported clock and the host clock. This is the standard NTP-style minimum
// round-trip filter, and it removes most of the per-transaction USB/serial latency jitter.
const rtcProbes = 8

// rtcSample reads the Notecard's high-resolution clock using {"req":"card.time","now":true}, which
// returns the time in the "value" field as <epoch-seconds>.<six-digits-of-microseconds>. It probes
// several times in quick succession and returns the reading with the lowest round-trip latency,
// expressed in microseconds. cardUs is the Notecard's clock; hostUs is the host clock at the
// midpoint of that round trip, which is our best estimate of the instant the Notecard sampled.
func rtcSample() (cardUs int64, hostUs int64, err error) {
	bestRTTUs := int64(-1)
	for p := 0; p < rtcProbes; p++ {
		before := time.Now()
		var rsp notecard.Request
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.time", Now: true})
		after := time.Now()
		if err != nil {
			return
		}
		rttUs := after.Sub(before).Microseconds()
		if bestRTTUs < 0 || rttUs < bestRTTUs {
			bestRTTUs = rttUs
			cardUs = int64(rsp.Value * 1000000)
			hostUs = before.Add(after.Sub(before) / 2).UnixMicro()
		}
	}
	return
}

// rtc measures the drift of the Notecard's real-time clock relative to the host computer's clock,
// emitting one line per second for the specified number of seconds. This assumes the host has an
// accurate, high-resolution clock to measure against.
func rtc(seconds int, verbose bool) (err error) {

	// Quiet the debug output unless the user asked for verbosity, so that our once-per-second
	// lines aren't interleaved with transaction tracing.
	if !verbose {
		card.DebugOutput(false, false)
	}

	// Establish the origin against which all subsequent samples are measured: the host clock and
	// the Notecard's clock at the same instant, both in microseconds. We measure each sample's
	// elapsed time relative to this origin so the regression below works with small numbers.
	originCardUs, originHostUs, err := rtcSample()
	if err != nil {
		return
	}

	// Rather than deriving the drift from just the origin and the current sample (which exposes
	// the full per-sample measurement noise of those two readings), we fit a least-squares line
	// through every sample collected so far. We regress the drift itself (microseconds of drift
	// since start) against the host-elapsed time; the slope of that line is the drift rate. Using
	// all N samples averages out the per-sample jitter, so the reported figures stabilize as the
	// run lengthens. We accumulate the regression sums incrementally, seeding them with the origin
	// point (host-elapsed 0, drift 0).
	var n, sumX, sumD, sumXX, sumXD float64
	n = 1 // the origin point (x=0, drift=0) contributes nothing to the sums but counts toward n

	// Emit one measurement per second, on a one-second cadence.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for i := 1; i <= seconds; i++ {
		<-ticker.C

		var cardUs, hostUs int64
		cardUs, hostUs, err = rtcSample()
		if err != nil {
			return
		}

		// Microseconds elapsed on the host, and microseconds of drift, both since the origin.
		// Drift is how much further the Notecard's clock has advanced than the host's clock; a
		// positive value means the Notecard's RTC is running fast.
		x := float64(hostUs - originHostUs)
		drift := float64((cardUs - originCardUs) - (hostUs - originHostUs))

		// Accumulate this sample into the running least-squares fit of drift against host-elapsed.
		n++
		sumX += x
		sumD += drift
		sumXX += x * x
		sumXD += x * drift

		// Least-squares slope of drift vs host-elapsed: microseconds of drift accrued per
		// microsecond of host time. Scale that rate to the requested units.
		avgDriftMsPerSecond := 0.0
		driftSecsPerDay := 0.0
		denom := n*sumXX - sumX*sumX
		if denom != 0 {
			driftRate := (n*sumXD - sumX*sumD) / denom
			avgDriftMsPerSecond = driftRate * 1000
			driftSecsPerDay = driftRate * 86400
		}

		fmt.Printf("%d rtc: avgDriftMsPerSecond:%.3f driftSecsPerDay:%.3f\n", i, avgDriftMsPerSecond, driftSecsPerDay)
	}

	return
}
