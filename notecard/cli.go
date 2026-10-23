// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// This CLI as the shared command line framework sees it: its name, its one mode, and its
// switches and help groups.  To add a switch, add its variable and a row below.

package main

import "github.com/blues/note-cli/lib/cli"

// The name of this CLI as typed on the command line
const cliName = "notecard"

// The variables into which the switches are parsed
var (
	flagProduct          string
	flagSN               string
	flagHub              string
	flagInfo             bool
	flagFactory          bool
	flagFormat           bool
	flagSetup            string
	flagSetupSKU         string
	flagScan             string
	flagProvision        string
	flagSideload         string
	flagNoBin            bool
	flagRequest          string
	flagPretty           bool
	flagVerbose          bool
	flagDry              bool
	flagInput            string
	flagOutput           string
	flagFast             bool
	flagTrace            bool
	flagSync             bool
	flagWhenConnected    bool
	flagWhenDisconnected bool
	flagWhenDisarmed     bool
	flagWhenSynced       bool
	flagExplore          bool
	flagReserved         bool
	flagLog              string
	flagUpload           string
	flagRoute            string
	flagTarget           string
	flagPlayground       bool
	flagPlaytime         int
	flagCommtest         bool
	flagEcho             int
	flagRTC              string
	flagDFUPackage       string
	flagPcap             string
	flagVersion          bool
)

// notecardTool describes this CLI to the framework.  Main defines it before anything
// else, and so do the tests.
func notecardTool() *cli.Tool {
	return &cli.Tool{
		Name:  cliName,
		Title: "the command line for a Notecard",
		Modes: []*cli.Mode{{
			Name:     cli.ModeDefault,
			Summary:  "send requests to the Notecard, set it up, and watch it work",
			Args:     `['{"req":"..."}']`,
			Examples: notecardExamples(),
			Detail: "A request is in the Notecard's own JSON form, such as {\"req\":\"card.version\"},\n" +
				"given as the last argument or with --req, with the options before it; a\n" +
				"standalone -- ends them.  The Notecard's answer is written as JSON, indented\n" +
				"with --pretty, and a payload in it is written to the file named by --output.\n\n" +
				"Given alone, --interface, --port and --portconfig are saved for the commands\n" +
				"that follow; with anything else on the command line, they apply to that\n" +
				"command only.  NOTE_INTERFACE, NOTE_PORT and NOTE_PORT_CONFIG in the\n" +
				"environment override the saved ones.",
			Run: runDefault,
		}},
		Switches:       notecardSwitches(),
		Groups:         notecardGroups(),
		NotecardConfig: true,
	}
}

// notecardExamples shows how the CLI is used
func notecardExamples() []cli.Example {
	return []cli.Example{
		{What: "Send a request to the Notecard, in its own JSON form", Command: cliName + ` '{"req":"card.version"}'`},
		{What: "Show what the Notecard knows about itself", Command: cliName + " --info"},
		{What: "Save the port the Notecard is on, for the commands that follow", Command: cliName + " --interface serial --port /dev/ttyUSB0"},
		{What: "Set the productUID, and sync with Notehub", Command: cliName + " --product com.example:sensor --sync"},
		{What: "List the Notecard's notefiles, and the notes in each", Command: cliName + " --explore"},
		{What: "Watch the Notecard's trace output", Command: cliName + " --trace"},
		{What: "Load firmware into the Notecard from a file", Command: cliName + " --sideload notecard.bin"},
		{What: "Upload a file to Notehub through a proxy route", Command: cliName + " --upload ./data.bin --route MyRoute"},
		{What: "Record the packets on the USB port to a capture file", Command: cliName + " --pcap usb --output capture.pcap"},
		{What: "Set up Notecards one after another, taking an inventory", Command: cliName + " --scan inventory.json --setup setup.json"},
	}
}

// notecardGroups returns the help groups in display order; empty groups aren't shown
func notecardGroups() []cli.Group {
	return []cli.Group{
		{Name: cli.GroupGeneral, Description: "General Options"},
		{Name: "config", Description: "Notecard Configuration"},
		{Name: "device", Description: "Device Management"},
		{Name: "request", Description: "Requests & Debugging"},
		{Name: "hub", Description: "Notehub Sync & Status"},
		{Name: "notefile", Description: "Notefiles"},
		{Name: "upload", Description: "File Upload"},
		{Name: "tools", Description: "Utilities & Tools"},
		{Name: "other", Description: "Other Options"},
	}
}

// The mode lists used by the switches below: there is one mode, and --help is in every one
var (
	inDefault = []string{cli.ModeDefault}
	inAnyMode = []string{cli.AnyMode}
)

// notecardSwitches returns every switch
func notecardSwitches() []*cli.Switch {
	return []*cli.Switch{

		// General Options, the only ones shown when there is nothing to do.  --help is the
		// framework's, and lib registers these three for every CLI that talks to a Notecard.
		{Name: "interface", External: true, ValueType: "interface", Group: cli.GroupGeneral, Modes: inAnyMode,
			Usage: "the interface the Notecard is attached by: serial or i2c"},
		{Name: "port", External: true, ValueType: "port", Group: cli.GroupGeneral, Modes: inAnyMode,
			Usage: "the serial port or I2C bus the Notecard is on, as in /dev/ttyUSB0"},
		{Name: "portconfig", External: true, ValueType: "speed", Group: cli.GroupGeneral, Modes: inAnyMode,
			Usage: "the serial speed, or the I2C address, to use on the port"},

		// Notecard Configuration
		{Name: "product", Target: &flagProduct, Group: "config", Modes: inDefault,
			Usage: "set the Notecard's `productUID`, as hub.set does"},
		{Name: "sn", Target: &flagSN, Group: "config", Modes: inDefault,
			Usage: "set the Notecard's `serial` number, as hub.set does"},
		{Name: "hub", Target: &flagHub, Group: "config", Modes: inDefault,
			Usage: "set the Notehub `domain` the Notecard syncs with, as hub.set host does"},
		{Name: "info", Target: &flagInfo, Group: "config", Modes: inDefault,
			Usage: "show what the Notecard knows about itself: identity, modem, sync, power, location, time and notefiles"},

		// Device Management
		{Name: "factory", Target: &flagFactory, Group: "device", Modes: inDefault,
			Usage: "reset the Notecard to its factory defaults"},
		{Name: "format", Target: &flagFormat, Group: "device", Modes: inDefault,
			Usage: "reset the Notecard's notefile storage, keeping its configuration"},
		{Name: "setup", Target: &flagSetup, Group: "device", Modes: inDefault,
			Usage: "send the requests in a JSON `file` to the Notecard, one after another, as a setup script"},
		{Name: "setup-sku", Target: &flagSetupSKU, Group: "device", Modes: inDefault,
			Usage: "store the requests in a JSON `file` in the Notecard, to run again after every factory reset"},
		{Name: "scan", Target: &flagScan, Group: "device", Modes: inDefault,
			Usage: "set up Notecards one after another, as each is inserted, appending what each is to an inventory `file`"},
		{Name: "provision", Target: &flagProvision, Group: "device", Modes: inDefault,
			Usage: "when scanning, provision each Notecard's SIM with its carrier; for Twilio, `creds` is AccountSID:AuthToken"},
		{Name: "sideload", Target: &flagSideload, Group: "device", Modes: inDefault,
			Usage: "load a .bin or .bins `file` of firmware into the Notecard, as a download from Notehub would"},
		{Name: "nobin", Target: &flagNoBin, Group: "device", Modes: inDefault,
			Usage: "when side-loading, send inline with dfu.put rather than through card.binary"},

		// Requests & Debugging
		{Name: "req", Target: &flagRequest, Group: "request", Modes: inDefault,
			Usage: "a `request` in the Notecard's JSON form; the same as a bare request as the last argument"},
		{Name: "pretty", Target: &flagPretty, Group: "request", Modes: inDefault,
			Usage: "pretty print JSON output"},
		{Name: "verbose", Target: &flagVerbose, Group: "request", Modes: inDefault,
			Usage: "display requests and responses as they go by"},
		{Name: "dry", Target: &flagDry, Group: "request", Modes: inDefault,
			Usage: "don't send the request; with --verbose, show what would be sent"},
		{Name: "input", Target: &flagInput, Group: "request", Modes: inDefault,
			Usage: "a `file` whose contents are sent as the request's payload"},
		{Name: "output", Target: &flagOutput, Group: "request", Modes: inDefault,
			Usage: "the `file` that receives a response's payload, a .bins package from --binpack, or a capture from --pcap"},
		{Name: "fast", Target: &flagFast, Group: "request", Modes: inDefault,
			Usage: "send with short delays and large segments, as a USB connection allows"},
		{Name: "trace", Target: &flagTrace, Group: "request", Modes: inDefault,
			Usage: "watch the Notecard's trace output"},

		// Notehub Sync & Status
		{Name: "sync", Target: &flagSync, Group: "hub", Modes: inDefault,
			Usage: "ask the Notecard to sync with Notehub now"},
		{Name: "when-connected", Target: &flagWhenConnected, Group: "hub", Modes: inDefault,
			Usage: "wait until the Notecard is connected to Notehub"},
		{Name: "when-disconnected", Target: &flagWhenDisconnected, Group: "hub", Modes: inDefault,
			Usage: "wait until the Notecard is disconnected from Notehub"},
		{Name: "when-disarmed", Target: &flagWhenDisarmed, Group: "hub", Modes: inDefault,
			Usage: "wait until the Notecard's ATTN pin is disarmed"},
		{Name: "when-synced", Target: &flagWhenSynced, Group: "hub", Modes: inDefault,
			Usage: "sync if needed, and wait until the sync completes"},

		// Notefiles
		{Name: "explore", Target: &flagExplore, Group: "notefile", Modes: inDefault,
			Usage: "list the Notecard's notefiles, and the notes in each"},
		{Name: "reserved", Target: &flagReserved, Group: "notefile", Modes: inDefault,
			Usage: "when exploring, include the reserved notefiles too"},
		{Name: "log", Target: &flagLog, Group: "notefile", Modes: inDefault,
			Usage: "add a line of `text` to the _log.qo notefile"},

		// File Upload
		{Name: "upload", Target: &flagUpload, Group: "upload", Modes: inDefault,
			Usage: "upload a `file` to Notehub through a proxy route, in binary chunks through the Notecard"},
		{Name: "route", Target: &flagRoute, Group: "upload", Modes: inDefault,
			Usage: "the Notehub proxy route's `alias`, which --upload requires"},
		{Name: "target", Target: &flagTarget, Group: "upload", Modes: inDefault,
			Usage: "a URL `path` appended to the route, as the name in web.post; [filename] stands for the uploaded file's name"},

		// Utilities & Tools
		{Name: "play", Target: &flagPlayground, Group: "tools", Modes: inDefault,
			Usage: "enter the request/response playground, an interactive session with the Notecard"},
		{Name: "playtime", Target: &flagPlaytime, Group: "tools", Modes: inDefault,
			Usage: "with --interface lease, the `minutes` to lease a remote Notecard for; implies --play"},
		{Name: "commtest", Target: &flagCommtest, Group: "tools", Modes: inDefault,
			Usage: "send requests back to back, forever, reporting the rate, to test the connection"},
		{Name: "echo", Target: &flagEcho, Group: "tools", Modes: inDefault,
			Usage: "send `N` echo requests of growing size, to test the reliability of the connection"},
		{Name: "rtc", Target: &flagRTC, Group: "tools", Modes: inDefault,
			Usage: "measure the Notecard's RTC drift against the host clock: `mode` is test to measure, calibrate to measure and store the calibration, reset to clear it, or calibration to show it"},
		{Name: "binpack", Target: &flagDFUPackage, Group: "tools", Modes: inDefault,
			Usage: "package the addr:file.bin pairs that follow into one .bins file for the host `mcu` named, written to --output, for DFU"},
		{Name: "pcap", Target: &flagPcap, Group: "tools", Modes: inDefault,
			Usage: "record the packets on the Notecard's `port`, usb or aux, to the PCAP file named by --output"},

		// Other Options
		{Name: "version", Target: &flagVersion, Group: "other", Modes: inDefault,
			Usage: "print the version of this CLI"},
	}
}
