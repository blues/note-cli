// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/base64"
	"fmt"
	"time"
)

// Records
const wtModemPowerOn = 0        // nil: modem powered on
const wtModemPowerOff = 1       // uint32: modem powered off, bytes transferred while on
const wtSimIccid = 2            // string: first six digits of secondary ICCID
const wtSimApn = 3              // string: secondary APN
const wtBearer = 4              // string: explicit bearer ID we are looking for
const wtScanBearer = 5          // string: bearer ID we have seen
const wtScanBand = 6            // byte: band number we are using
const wtScanCellID = 7          // string: cell ID we are seeing (mcc,mnc,lac,cid)
const wtScanSignal = 8          // string: signal we are seeing (rssi,rsrp,sinr,rsrq)
const wtScan = 9                // nil: scan heartbeat
const wtConnectedIp = 10        // nil: connected with IP
const wtConnectedTls = 11       // nil: connected via TLS
const wtConnectedDiscovery = 12 // nil: connected to discovery service
const wtConnectedHandler = 13   // nil: connected to request handler
const wtSyncBoxPull = 14        // byte: number of notebox pulls (pinned at 255)
const wtSyncBoxPush = 15        // byte: number of notebox pushes (pinned at 255)
const wtSyncFilePull = 16       // byte: number of notefile pulls (pinned at 255)
const wtSyncFilePush = 17       // byte: number of notefile pushes (pinned at 255)
const wtSyncNotePull = 18       // byte: number of note pulls (pinned at 255)
const wtSyncNotePush = 19       // byte: number of note pushes (pinned at 255)
const wtAbort = 20              // string: abort reason

// Dump a single trace buffer
func wtraceDump(encoded string) error {

	// Decode base64
	buf, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	bufoff := uint32(0)
	buflen := uint32(len(buf))

	// Extract the time and emit it
	baseTime := wtExtract32(buf, &bufoff, &buflen)
	if !timeIsValidUnix(baseTime) {
		fmt.Printf("%d seconds since boot\n", baseTime)
	} else {
		fmt.Printf("%s\n", time.Unix(int64(baseTime), 0).Format("2006-01-02T15:04:05Z"))
	}

	// Loop over all records
	for buflen > 0 {

		// Process time offset from base
		secsOffset := uint32(0)
		cmd := wtExtract8(buf, &bufoff, &buflen)
		if (cmd & 0x80) != 0 {
			secsOffset = uint32(wtExtract8(buf, &bufoff, &buflen))
			cmd = cmd & 0x7f
		} else {
			secsOffset = wtExtract32(buf, &bufoff, &buflen)
			if timeIsValidUnix(secsOffset) {
				secsOffset -= baseTime
			}
		}
		fmt.Printf("+ %3ds ", secsOffset)

		// Process each record type
		switch cmd {

		case wtModemPowerOn:
			fmt.Printf("modem power turned on\n")

		case wtModemPowerOff:
			fmt.Printf("modem power turned off (%d bytes used over-the-air)\n", wtExtract32(buf, &bufoff, &buflen))

		case wtSimIccid:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("secondary SIM has ICCID prefix %s\n", s)

		case wtSimApn:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("secondary SIM is using APN %s\n", s)

		case wtBearer:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("looking for signal on %s\n", s)

		case wtScanBearer:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("found signal on %s\n", s)

		case wtScanBand:
			fmt.Printf("found signal on BAND %d\n", wtExtract8(buf, &bufoff, &buflen))

		case wtScanCellID:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("found signal on CELL %s\n", s)

		case wtScanSignal:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("found signal with strength %s\n", s)

		case wtScan:
			fmt.Printf("scanning for signal\n")

		case wtConnectedIp:
			fmt.Printf("connected to IP network\n")

		case wtConnectedTls:
			fmt.Printf("connected using TLS security\n")

		case wtConnectedDiscovery:
			fmt.Printf("connected to load balancer\n")

		case wtConnectedHandler:
			fmt.Printf("connected to request handler\n")

		case wtSyncBoxPull:
			fmt.Printf("sync downloaded remote list of notefiles %d time(s)\n", wtExtract8(buf, &bufoff, &buflen))

		case wtSyncBoxPush:
			fmt.Printf("sync uploaded local list of notefiles %d time(s)\n", wtExtract8(buf, &bufoff, &buflen))

		case wtSyncFilePull:
			fmt.Printf("sync downloaded %d notefiles\n", wtExtract8(buf, &bufoff, &buflen))

		case wtSyncFilePush:
			fmt.Printf("sync uploaded %d notefiles\n", wtExtract8(buf, &bufoff, &buflen))

		case wtSyncNotePull:
			fmt.Printf("sync downloaded %d notes\n", wtExtract8(buf, &bufoff, &buflen))

		case wtSyncNotePush:
			fmt.Printf("sync uploaded %d notes\n", wtExtract8(buf, &bufoff, &buflen))

		case wtAbort:
			s := wtExtractString(buf, &bufoff, &buflen)
			fmt.Printf("connection aborted: %s\n", s)

		default:
			buflen = 0

		}

	}

	// Done
	return nil
}

func timeIsValidUnix(epochTime uint32) bool {
	return epochTime > 1668209653
}

func wtExtract8(buf []byte, poff *uint32, pleft *uint32) (data byte) {
	if *pleft == 0 {
		return 0
	}
	data = buf[*poff]
	*poff = *poff + 1
	*pleft = *pleft - 1
	return
}

func wtExtract32(buf []byte, poff *uint32, pleft *uint32) (data uint32) {
	data = uint32(wtExtract8(buf, poff, pleft))
	data |= uint32(wtExtract8(buf, poff, pleft)) << 8
	data |= uint32(wtExtract8(buf, poff, pleft)) << 16
	data |= uint32(wtExtract8(buf, poff, pleft)) << 24
	return
}

func wtExtractString(buf []byte, poff *uint32, pleft *uint32) (data string) {
	len := wtExtract8(buf, poff, pleft)
	if *pleft == 0 || len == 0 {
		return
	}
	for i := uint32(0); i < uint32(len); i++ {
		data += fmt.Sprintf("%c", buf[*poff+i])
	}
	*poff = *poff + uint32(len)
	*pleft -= uint32(len)
	return
}
