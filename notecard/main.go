// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
)

// Exit codes
const exitFail = 1

// The open notecard
var card *notecard.Context

// CLI Version - Set by ldflags during build/release
var version = "development"

// Define flag groups
type FlagGroup struct {
	Name        string
	Description string
	Flags       []*flag.Flag
}

// getFlagGroups returns the organized flag groups
func getFlagGroups() []FlagGroup {
	return []FlagGroup{
		{
			Name:        "config",
			Description: "Basic Configuration",
			Flags: []*flag.Flag{
				getFlagByName("product"),
				getFlagByName("sn"),
				getFlagByName("hub"),
				getFlagByName("info"),
			},
		},
		{
			Name:        "device",
			Description: "Device Management",
			Flags: []*flag.Flag{
				getFlagByName("scan"),
				getFlagByName("factory"),
				getFlagByName("format"),
				getFlagByName("setup"),
				getFlagByName("setup-sku"),
				getFlagByName("provision"),
				getFlagByName("sideload"),
			},
		},
		{
			Name:        "comm",
			Description: "Communication & Debug",
			Flags: []*flag.Flag{
				getFlagByName("verbose"),
				getFlagByName("pretty"),
				getFlagByName("req"),
				getFlagByName("input"),
				getFlagByName("output"),
				getFlagByName("fast"),
				getFlagByName("trace"),
			},
		},
		{
			Name:        "hub",
			Description: "Notehub Sync & Status",
			Flags: []*flag.Flag{
				getFlagByName("when-connected"),
				getFlagByName("when-disconnected"),
				getFlagByName("when-disarmed"),
				getFlagByName("when-synced"),
				getFlagByName("sync"),
				getFlagByName("watch"),
			},
		},
		{
			Name:        "tools",
			Description: "Utilities & Tools",
			Flags: []*flag.Flag{
				getFlagByName("play"),
				getFlagByName("playtime"),
				getFlagByName("commtest"),
				getFlagByName("echo"),
				getFlagByName("binpack"),
			},
		},
		{
			Name:        "notefile",
			Description: "Notefile Management",
			Flags: []*flag.Flag{
				getFlagByName("explore"),
				getFlagByName("reserved"),
				getFlagByName("log"),
			},
		},
		{
			Name:        "cli",
			Description: "CLI Configuration",
			Flags: []*flag.Flag{
				getFlagByName("interface"),
				getFlagByName("port"),
				getFlagByName("portconfig"),
			},
		},
		{
			Name:        "other",
			Description: "Other",
			Flags: []*flag.Flag{
				getFlagByName("version"),
			},
		},
	}
}

// Main entry
func main() {
	// Channel to handle OS signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Make sure to close the Notecard connection before the program ends.
	defer func() {
		if card != nil {
			card.Close()
		}
	}()

	// Similarly, close the Notecard connection on SIGINT, SIGTERM, and SIGQUIT.
	go func() {
		sig := <-signalChan
		fmt.Printf("Received signal: %s\n", sig)
		if card != nil {
			card.Close()
		}
		os.Exit(exitFail)
	}()

	// Override the default usage function to use our grouped format
	flag.Usage = func() {
		printGroupedFlags(getFlagGroups())
	}

	// Process actions
	var actionPretty bool
	flag.BoolVar(&actionPretty, "pretty", false, "format JSON output indented")
	var actionRequest string
	flag.StringVar(&actionRequest, "req", "", "perform the specified request (in quotes)")
	var actionWhenConnected bool
	flag.BoolVar(&actionWhenConnected, "when-connected", false, "wait until connected")
	var actionWhenDisconnected bool
	flag.BoolVar(&actionWhenDisconnected, "when-disconnected", false, "wait until disconnected")
	var actionWhenDisarmed bool
	flag.BoolVar(&actionWhenDisarmed, "when-disarmed", false, "wait until ATTN is disarmed")
	var actionVerbose bool
	flag.BoolVar(&actionVerbose, "verbose", false, "display notecard requests and responses")
	var actionWhenSynced bool
	flag.BoolVar(&actionWhenSynced, "when-synced", false, "sync if needed and wait until sync completed")
	var actionReserved bool
	flag.BoolVar(&actionReserved, "reserved", false, "when exploring, include reserved notefiles")
	var actionExplore bool
	flag.BoolVar(&actionExplore, "explore", false, "explore the contents of the device")
	var actionFactory bool
	flag.BoolVar(&actionFactory, "factory", false, "reset notecard to factory defaults")
	var actionFormat bool
	flag.BoolVar(&actionFormat, "format", false, "reset notecard's notefile storage but retain configuration")
	var actionInput string
	flag.StringVar(&actionInput, "input", "", "add the contents of this file as a payload to the request")
	var actionOutput string
	flag.StringVar(&actionOutput, "output", "", "output file")
	var actionLog string
	flag.StringVar(&actionLog, "log", "", "add a text string to the _log.qo notefile")
	var actionTrace bool
	flag.BoolVar(&actionTrace, "trace", false, "watch Notecard's trace output")
	var actionPlayground bool
	flag.BoolVar(&actionPlayground, "play", false, "enter JSON request/response playground")
	var actionPlaytime int
	flag.IntVar(&actionPlaytime, "playtime", 0, "enter number of minutes to play")
	var actionSync bool
	flag.BoolVar(&actionSync, "sync", false, "manually initiate a sync")
	var actionProduct string
	flag.StringVar(&actionProduct, "product", "", "set product UID")
	var actionSN string
	flag.StringVar(&actionSN, "sn", "", "set serial number")
	var actionInfo bool
	flag.BoolVar(&actionInfo, "info", false, "show information about the Notecard")
	var actionHub string
	flag.StringVar(&actionHub, "hub", "", "set notehub domain")
	var actionWatchLevel int
	flag.IntVar(&actionWatchLevel, "watch", -1, "watch ongoing sync status of a given level (0-5)")
	var actionCommtest bool
	flag.BoolVar(&actionCommtest, "commtest", false, "perform repetitive request/response test to validate comms with the Notecard")
	var actionSetup string
	flag.StringVar(&actionSetup, "setup", "", "issue requests sequentially as stored in the specified .json file")
	var actionSetupSKU string
	flag.StringVar(&actionSetupSKU, "setup-sku", "", "configure a notecard for self-setup even after factory restore, with  requests in the specified .json file")
	var actionScan string
	flag.StringVar(&actionScan, "scan", "", "scan a batch of notecards to collect info or to set them up")
	var actionProvision string
	flag.StringVar(&actionProvision, "provision", "", "provision into carrier account using AccountSID:AuthTOKEN")
	var actionDFUPackage string
	flag.StringVar(&actionDFUPackage, "binpack", "", "package multiple .bin's for DFU into a single .bins package")
	var actionFast bool
	flag.BoolVar(&actionFast, "fast", false, "use low timeouts and big buffers when sending to notecard knowing that {io} errors are to be expected")
	var actionSideload string
	flag.StringVar(&actionSideload, "sideload", "", "side-load a .bin or .bins into the notecard's storage")
	var actionEcho int
	flag.IntVar(&actionEcho, "echo", 0, "perform <N> iterations of a communications reliability test to the notecard")
	var actionVersion bool
	flag.BoolVar(&actionVersion, "version", false, "print the current version of the CLI")

	// Parse these flags and also the note tool config flags
	err := lib.FlagParse(true, false)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(exitFail)
	}

	// If no action specified (i.e. just -port x), exit so that we don't touch the wrong port
	if len(os.Args) == 1 {
		printGroupedFlags(getFlagGroups())
		lib.ConfigShow()
		fmt.Printf("\n")
		nInterface, nPort, _ := notecard.Defaults()
		if lib.Config.Interface != "" {
			nInterface = lib.Config.Interface
			nPort = lib.Config.IPort[lib.Config.Interface].Port
		}
		var ports []string
		if nInterface == notecard.NotecardInterfaceSerial {
			ports, _, _, _ = notecard.SerialPorts()
		}
		if nInterface == notecard.NotecardInterfaceI2C {
			ports, _, _, _ = notecard.I2CPorts()
		}
		if len(ports) != 0 {
			fmt.Printf("Ports on '%s':\n", nInterface)
			for _, port := range ports {
				if port == nPort {
					fmt.Printf("   %s ***\n", port)
				} else {
					fmt.Printf("   %s\n", port)
				}
			}
		}
		return
	}

	// Process the main part of the command line as a -req if neither Req nor DFU are specified
	if actionDFUPackage == "" && actionRequest == "" {
		argsLeft := len(flag.Args())
		if argsLeft == 1 {
			actionRequest = flag.Args()[0]
		} else if argsLeft > 0 {
			fmt.Printf("to send a JSON request to the notecard, please place it in quotes")
			os.Exit(exitFail)
		}
	}

	// Both actionDFUPackage and actionRequest potentially use the 'remaining args' outside the flags
	if actionDFUPackage != "" && actionRequest != "" {
		fmt.Printf("-req and -binpack may not be combined into one command")
		os.Exit(exitFail)
	}

	// Open the card, just to make sure errors are reported early
	configVal := lib.Config.IPort[lib.Config.Interface].PortConfig
	if actionPlaytime != 0 {
		configVal = actionPlaytime
		actionPlayground = true
	}
	notecard.InitialDebugMode = actionVerbose
	notecard.InitialTraceMode = actionTrace
	card, err = notecard.Open(lib.Config.Interface, lib.Config.IPort[lib.Config.Interface].Port, configVal)

	// Process non-config commands
	var rsp notecard.Request

	// The timouts in the note-go library are set under the assumption that
	// the device is connected with long header wires that have significant
	// capacitance and resistance, and where there may be arbitrary activity
	// on the Notecard. This switch uses a larger buffer and shorter inter-segment
	// delays under the assumption that perhaps this is being done using USB.
	// NOTE: the comment above was from before USB flow control was implemented
	// in the Notecard, in build 15741 Jan 26 2023.  At that time, this was
	// 1024/30, but is no longer relevant as we can pound the Notecard on the
	// USB port because of hardware flow control.
	if err == nil && actionFast {
		notecard.RequestSegmentMaxLen = 1024
		notecard.RequestSegmentDelayMs = 5
	}

	// Wait until disconnected
	if err == nil && actionWhenDisconnected {
		for {
			rsp, err := card.TransactionRequest(notecard.Request{Req: "hub.status", NotefileID: notecard.SyncLogNotefile, Delete: true})
			if err != nil {
				fmt.Printf("%s\n", err)
				break
			}
			if strings.Contains(rsp.Status, note.ErrTransportDisconnected) {
				break
			}
			fmt.Printf("%s\n", rsp.Status)
			time.Sleep(3 * time.Second)
		}
	}

	// Wait until connected
	if err == nil && actionWhenConnected {
		for {
			delay := true
			rsp, err := card.TransactionRequest(notecard.Request{Req: "note.get", NotefileID: notecard.SyncLogNotefile, Delete: true})
			if err != nil && note.ErrorContains(err, note.ErrNoteNoExist) {
				delay = true
				err = nil
			}
			if err != nil {
				fmt.Printf("%s\n", err)
				break
			}
			if rsp.Connected {
				break
			} else if rsp.Body != nil {
				var body notecard.SyncLogBody
				note.BodyToObject(rsp.Body, &body)
				fmt.Printf("%s\n", body.Text)
			}
			if delay {
				time.Sleep(3 * time.Second)
			}
		}
	}

	// Wait until disarmed
	if err == nil && actionWhenDisarmed {
		for {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "card.attn"})
			if err != nil {
				fmt.Printf("%s\n", err)
			} else if rsp.Set {
				break
			}
			time.Sleep(3 * time.Second)
		}
	}

	// Wait until synced
	if err == nil && actionWhenSynced {
		req := notecard.Request{Req: "hub.sync.status"}
		req.Sync = true // Initiate sync if sync is needed
		rsp, err = card.TransactionRequest(req)
		for err == nil {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.sync.status"})
			if err != nil {
				fmt.Printf("%s\n", err)
				break
			}
			if rsp.Alert {
				fmt.Printf("sync error\n")
				break
			}
			if rsp.Completed > 0 {
				break
			}
			fmt.Printf("%s\n", rsp.Status)
			time.Sleep(3 * time.Second)
		}
	}

	// Turn on Notecard library debug output
	if card != nil {
		card.DebugOutput(actionVerbose, false)
	}

	// Do SKU setup before anything else, particularly because if we are going
	// to do a factory reset it needs to be done after we set up the SKU
	if err == nil && actionSetupSKU != "" && actionScan == "" {
		var requestsString string
		requestsString, err = loadRequestsString(actionSetupSKU)
		if err == nil {
			req := notecard.Request{Req: "card.setup"}
			req.Text = requestsString
			_, err = card.TransactionRequest(req)
		}
		if err == nil && !(actionFactory || actionFormat) {
			_, err = card.TransactionRequest(notecard.Request{Req: "card.restart"})
			if err == nil {
				for i := 0; i < 5; i++ {
					_, err = card.TransactionRequest(notecard.Request{Req: "hub.get"})
					if err == nil {
						break
					}
				}
			}
		}
	}

	// Factory reset & format
	verifyCompletion := false
	if err == nil && actionFormat {
		req := notecard.Request{Req: "card.restore"}
		card.TransactionRequest(req)
		verifyCompletion = true
	}
	if err == nil && actionFactory && (actionScan == "" && actionSetup == "") {
		req := notecard.Request{Req: "card.restore"}
		req.Delete = true
		_, err = card.TransactionRequest(req)
		verifyCompletion = true
	}
	if err == nil && verifyCompletion {
		for i := 0; i < 5; i++ {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.get"})
			if err == nil {
				break
			}
		}
	}

	if err == nil && actionInfo {

		var infoErr error
		if !actionVerbose {
			card.DebugOutput(false, false)
		}

		cardDeviceUID := ""
		cardName := ""
		cardSKU := ""
		cardVersion := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.version"})
		if err == nil {
			cardDeviceUID = rsp.DeviceUID
			cardName = rsp.Name
			cardSKU = rsp.SKU
			cardVersion = rsp.Version
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardICCID := ""
		cardIMSI := ""
		cardIMEI := ""
		cardICCIDX := ""
		cardIMSIX := ""
		cardModem := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.wireless"})
		if err == nil {
			cardModem = rsp.Net.ModemFirmware
			cardIMEI = rsp.Net.Imei
			cardIMSI = rsp.Net.Imsi
			cardICCID = rsp.Net.Iccid
			cardIMSIX = rsp.Net.ImsiExternal
			cardICCIDX = rsp.Net.IccidExternal
		} else if !strings.Contains(err.Error(), "{not-supported}") {
			infoErr = accumulateInfoErr(infoErr, err)
		}

		cardSN := ""
		cardHost := ""
		cardProductUID := ""
		cardSyncMode := ""
		OutboundPeriod := "-"
		InboundPeriod := "-"
		rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.get"})
		if err == nil {
			cardSN = rsp.SN
			cardHost = rsp.Host
			cardProductUID = rsp.ProductUID
			cardSyncMode = rsp.Mode
			if rsp.Minutes != 0 {
				OutboundPeriod = fmt.Sprintf("%d minutes", rsp.Minutes)
			}
			if rsp.Outbound != 0 {
				OutboundPeriod = fmt.Sprintf("%d minutes", rsp.Outbound)
			}
			if rsp.OutboundV != "" {
				OutboundPeriod = rsp.OutboundV
			}
			if rsp.Hours != 0 {
				InboundPeriod = fmt.Sprintf("%d hours", rsp.Hours)
			}
			if rsp.Inbound != 0 {
				InboundPeriod = fmt.Sprintf("%d minutes", rsp.Inbound)
			}
			if rsp.InboundV != "" {
				InboundPeriod = rsp.InboundV
			}
			if cardProductUID == "" {
				cardProductUID = "*** Product UID is not set. Please use notehub.io to create a project and a product UID ***"
			}
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardVoltage := 0.0
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.voltage"})
		if err == nil {
			cardVoltage = rsp.Value
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardTemp := 0.0
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.temp"})
		if err == nil {
			cardTemp = rsp.Value
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardGPSMode := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.location.mode"})
		if err == nil {
			if rsp.Status == "" {
				cardGPSMode = rsp.Mode
			} else {
				cardGPSMode = rsp.Mode + " (" + rsp.Status + ")"
			}
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardTime := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.time"})
		if err == nil && rsp.Time > 0 {
			cardTime = time.Unix(int64(rsp.Time), 0).Format("2006-01-02T15:04:05Z") + " (" +
				time.Unix(int64(rsp.Time), 0).Local().Format("2006-01-02 3:04:05 PM MST") + ")"
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardLocation := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.location"})
		if err == nil {
			if rsp.Latitude != 0 || rsp.Longitude != 0 {
				cardLocation = fmt.Sprintf("%f,%f (%s)", rsp.Latitude, rsp.Longitude, rsp.LocationOLC)
			}
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardBootedTime := ""
		cardStorageUsedPct := 0
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.status"})
		if err == nil {
			if rsp.Time > 0 {
				cardBootedTime = time.Unix(int64(rsp.Time), 0).Format("2006-01-02T15:04:05Z") + " (" +
					time.Unix(int64(rsp.Time), 0).Local().Format("2006-01-02 3:04:05 PM MST") + ")"
			}
			cardStorageUsedPct = int(rsp.Storage)
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardSyncedTime := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.sync.status"})
		if err == nil && rsp.Time > 0 {
			cardSyncedTime = time.Unix(int64(rsp.Time), 0).Format("2006-01-02T15:04:05Z") + " (" +
				time.Unix(int64(rsp.Time), 0).Local().Format("2006-01-02 3:04:05 PM MST") + ")"
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardServiceStatus := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.status"})
		if err == nil {
			cardServiceStatus = rsp.Status
			if rsp.Connected {
				cardServiceStatus += " (connected)"
			}
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardProvisionedTime := ""
		cardUsedBytes := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "card.usage.get"})
		if err == nil {
			if rsp.Time > 0 {
				cardProvisionedTime = time.Unix(int64(rsp.Time), 0).Format("2006-01-02T15:04:05Z") + " (" +
					time.Unix(int64(rsp.Time), 0).Local().Format("2006-01-02 3:04:05 PM MST") + ")"
			}
			cardUsedBytes = fmt.Sprint(int(rsp.BytesSent + rsp.BytesReceived))
		} else if strings.Contains(err.Error(), "{not-supported}") {
			err = nil
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardEnv := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "env.get"})
		if err == nil {
			cardEnvBytes, _ := note.JSONMarshalIndent(rsp.Body, "                          ", "  ")
			cardEnv = string(cardEnvBytes)
			cardEnv = strings.TrimSuffix(cardEnv, "\n")
		}
		infoErr = accumulateInfoErr(infoErr, err)

		cardNotefiles := ""
		rsp, err = card.TransactionRequest(notecard.Request{Req: "file.changes"})
		if err == nil {
			if rsp.FileInfo != nil {
				for notefileID, info := range *rsp.FileInfo {
					if cardNotefiles != "" {
						cardNotefiles += ", "
					}
					if info.Changes > 0 {
						cardNotefiles += fmt.Sprintf("%s (%d)", notefileID, info.Changes)
					} else {
						cardNotefiles += notefileID
					}
				}
			}
		}
		infoErr = accumulateInfoErr(infoErr, err)

		fmt.Printf("\n%s\n", cardName)
		fmt.Printf("              ProductUID: %s\n", cardProductUID)
		fmt.Printf("               DeviceUID: %s\n", cardDeviceUID)
		fmt.Printf("           Serial Number: %s\n", cardSN)
		fmt.Printf("            Notehub Host: %s\n", cardHost)
		fmt.Printf("        Firmware Version: %s\n", cardVersion)
		fmt.Printf("                     SKU: %s\n", cardSKU)
		if cardModem != "" {
			fmt.Printf("                   Modem: %s\n", cardModem)
			fmt.Printf("                   ICCID: %s\n", cardICCID)
			fmt.Printf("                    IMSI: %s\n", cardIMSI)
			fmt.Printf("                    IMEI: %s\n", cardIMEI)
		}
		if cardICCIDX != "" {
			fmt.Printf("          External ICCID: %s\n", cardICCIDX)
			fmt.Printf("           External IMSI: %s\n", cardIMSIX)
		}
		if cardProvisionedTime != "" {
			fmt.Printf("             Provisioned: %s\n", cardProvisionedTime)
		}
		if cardUsedBytes != "" {
			fmt.Printf("       Used Over-the-Air: %s bytes\n", cardUsedBytes)
		}
		fmt.Printf("               Sync Mode: %s\n", cardSyncMode)
		fmt.Printf("    Sync Outbound Period: %s\n", OutboundPeriod)
		fmt.Printf("          Inbound Period: %s\n", InboundPeriod)
		fmt.Printf("          Notehub Status: %s\n", cardServiceStatus)
		fmt.Printf("             Last Synced: %s\n", cardSyncedTime)
		fmt.Printf("                 Voltage: %0.02fV\n", cardVoltage)
		fmt.Printf("             Temperature: %0.02fC\n", cardTemp)
		fmt.Printf("                GPS Mode: %s\n", cardGPSMode)
		fmt.Printf("                Location: %s\n", cardLocation)
		fmt.Printf("            Current Time: %s\n", cardTime)
		fmt.Printf("               Boot Time: %s\n", cardBootedTime)
		fmt.Printf("               Notefiles: %s\n", cardNotefiles)
		fmt.Printf("   Notefile Storage Used: %d%%\n", cardStorageUsedPct)
		fmt.Printf("                     Env: %v\n", cardEnv)

		err = infoErr
	}

	if err == nil && actionProduct != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", ProductUID: actionProduct})
	}

	if err == nil && actionSN != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", SN: actionSN})
	}

	if err == nil && actionHub != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", Host: actionHub})
		lib.ConfigSetHub(actionHub)
	}

	if err == nil && actionSideload != "" && actionScan == "" {
		err = dfuSideload(actionSideload, actionVerbose)
	}

	if err == nil && actionDFUPackage != "" {
		err = dfuPackage(actionVerbose, actionOutput, actionDFUPackage, flag.Args())
		actionRequest = ""
	}

	if err == nil && actionRequest != "" {
		if err == nil {
			var rspJSON []byte
			var req, rsp notecard.Request
			note.JSONUnmarshal([]byte(actionRequest), &req)

			// If we want to read the payload from a file, do so
			if actionInput != "" {
				var contents []byte
				contents, err = ioutil.ReadFile(actionInput)
				if err == nil {
					req.Payload = &contents
				}
			}

			// Perform the transaction and do special handling for binary
			if req.Req == "card.binary.get" {
				expectedMD5 := req.Status
				rsp, err = card.TransactionRequest(req)
				if err == nil {
					var rspBytes []byte
					rspBytes, err = card.ReceiveBytes()
					if err == nil {
						rspBytes = bytes.TrimSuffix(rspBytes, []byte("\n"))
						rspBytes, err = notecard.CobsDecode(rspBytes, byte('\n'))
						if err == nil {
							actualMD5 := fmt.Sprintf("%x", md5.Sum(rspBytes))
							if expectedMD5 != actualMD5 {
								err = fmt.Errorf("actual MD5 %s != supplied 'status' field %s", actualMD5, expectedMD5)
							} else {
								rsp.Payload = &rspBytes
								rsp.Cobs = 0
							}
						}
					}
				}
			} else if req.Req == "card.binary.put" {
				payload := *req.Payload
				actualMD5 := fmt.Sprintf("%x", md5.Sum(payload))
				if req.Status != "" && !strings.EqualFold(req.Status, actualMD5) {
					err = fmt.Errorf("actual MD5 %s != supplied 'status' field %s", actualMD5, req.Status)
				} else {
					req.Status = actualMD5
					payload, err = notecard.CobsEncode(payload, byte('\n'))
					if err == nil {
						req.Payload = nil
						req.Cobs = int32(len(payload))
						rsp, err = card.TransactionRequest(req)
						if err == nil {
							payload = append(payload, byte('\n'))
							err = card.SendBytes(payload)
						}
					}
				}
			} else {
				actionRequest = strings.ReplaceAll(actionRequest, "\\n", "\n")
				rspJSON, err = card.TransactionJSON([]byte(actionRequest))
				if err == nil {
					_ = note.JSONUnmarshal(rspJSON, &rsp)
				}
			}

			// Write the payload to an output file if appropriate
			if err == nil && actionOutput != "" {
				if rsp.Payload != nil {
					err = ioutil.WriteFile(actionOutput, *rsp.Payload, 0644)
					if err != nil {
						rsp.Payload = nil
					}
				}
			}

			// Output the response to the console
			if !actionVerbose {
				if err == nil {
					if actionPretty {
						rspJSON, _ = note.JSONMarshalIndent(rsp, "", "    ")
					} else {
						rspJSON, _ = note.JSONMarshal(rsp)
					}
					fmt.Printf("%s\n", rspJSON)
				}
			}
		}
	}

	if err == nil && actionLog != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.log", Text: actionLog})
	}

	if err == nil && actionSync {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.sync"})
	}

	if err == nil && actionSetup != "" && actionScan == "" {
		var requests []map[string]interface{}
		requests, err = loadRequests(actionSetup)
		if err == nil {
			card.DebugOutput(true, false)
			err = processRequests(actionFactory, requests)
		}
	}

	if err == nil && actionScan != "" {
		err = scan(actionVerbose, actionFactory, actionSetup, actionSetupSKU, actionProvision, actionFactory, actionSideload, actionScan)
	}

	if err == nil && actionCommtest {

		// Turn off debug output
		card.DebugOutput(false, false)

		// Turn off tracing because it can interfere with our rapid transaction I/O
		card.TransactionRequest(notecard.Request{Req: "card.io", Mode: "trace-off"})

		// Go into a high-frequency transaction loop
		transactions := 0
		began := time.Now()
		lastMessage := time.Now()
		for {
			_, err = card.TransactionRequest(notecard.Request{Req: "card.version"})
			if err != nil {
				break
			}
			transactions++
			if time.Since(lastMessage).Seconds() > 2 {
				lastMessage = time.Now()
				fmt.Printf("%d successful transactions (%0.2f/sec)\n", transactions, float64(transactions)/time.Since(began).Seconds())
			}
		}
	}

	if err == nil && actionTrace {
		err = card.Trace()
	}

	if err == nil && actionPlayground {
		os.Exit(NewREPL(card).Start())
	}

	if err == nil && actionEcho != 0 {
		err = echo(actionEcho)
	}

	if err == nil && actionVersion {
		fmt.Printf("Notecard CLI Version: %s\n", version)
	}

	if err == nil && actionExplore {
		err = explore(actionReserved, actionPretty)
	}

	// Process errors
	if err != nil {
		if actionRequest != "" && !actionVerbose {
			jerr := map[string]interface{}{}
			jerr["err"] = err.Error()
			jj, _ := note.JSONMarshal(jerr)
			fmt.Printf("%s\n", string(jj))
		} else {
			fmt.Printf("%s\n", err)
		}
		os.Exit(exitFail)
	}
}

func accumulateInfoErr(infoErr error, newErr error) error {
	if newErr == nil {
		return infoErr
	}
	if infoErr == nil {
		return newErr
	}
	return fmt.Errorf("%s\n%s", infoErr, newErr)
}

// Helper function to print grouped commands
func printGroupedFlags(groups []FlagGroup) {
	fmt.Println("Notecard CLI - Command line tool for interacting with Notecards\n")
	fmt.Println("USAGE: notecard [options]\n")

	// First pass: find the longest flag name + type
	maxLen := 0
	for _, group := range groups {
		for _, f := range group.Flags {
			typeName, _ := flag.UnquoteUsage(f)
			length := len(f.Name)
			if len(typeName) > 0 {
				length += len(typeName) + 3 // +3 for flagText formatting
			}
			if length > maxLen {
				maxLen = length
			}
		}
	}

	// Add padding for the flag prefix "  -" and some extra space
	padding := maxLen + 5

	for _, group := range groups {
		fmt.Printf("%s:\n", group.Description)
		for _, f := range group.Flags {
			typeName, usage := flag.UnquoteUsage(f)
			flagText := f.Name
			if len(typeName) > 0 {
				flagText = fmt.Sprintf("%s (%s)", f.Name, typeName)
			}
			fmt.Printf("  -%*s%s\n", -padding, flagText, usage)
		}
		fmt.Println()
	}

	fmt.Println("For more detailed documentation and examples, visit:")
	fmt.Println("https://dev.blues.io/tools-and-sdks/notecard-cli/\n")
}

// Helper function to get flag by name from the default command line flags
func getFlagByName(name string) *flag.Flag {
	return flag.CommandLine.Lookup(name)
}
