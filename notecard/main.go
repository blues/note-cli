// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
)

// Exit codes
const exitFail = 1

// The open Notecard
var card *notecard.Context

// CLI Version - Set by ldflags during build/release
var version = "development"

// JSON schema control variables
var validateJSON bool = false
var jsonSchemaUrl string = "https://github.com/blues/notecard-schema/releases/latest/download/notecard.api.json"

// exitFailAndCloseCard closes the Notecard, if it is open, and exits with failure
func exitFailAndCloseCard() {
	if card != nil {
		card.Close()
	}
	os.Exit(exitFail)
}

// Main entry
func main() {

	// Describe this CLI to the framework
	cli.Define(notecardTool())

	// Close the Notecard before the program ends, and on SIGINT, SIGTERM and SIGQUIT
	defer func() {
		if card != nil {
			card.Close()
		}
	}()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-signalChan
		fmt.Fprintf(os.Stderr, "received signal: %s\n", sig)
		exitFailAndCloseCard()
	}()

	// Check the environment for JSON schema control variables
	_, validateJSON = os.LookupEnv("BLUES") // Opt-in Blues employees to validation
	if url := os.Getenv("NOTE_JSON_SCHEMA_URL"); url != "" {
		jsonSchemaUrl = url // Override the default schema URL
	}

	// Parse the command line, and get the resulting configuration
	mode, args := cli.Start(os.Args[1:])
	config, err := cli.Parse(mode, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(exitFail)
	}

	// Run it.  A request's failure is answered as the Notecard would answer it, as JSON.
	if err = mode.Run(config); err != nil {
		if flagRequest != "" && !flagVerbose {
			errJSON, _ := note.JSONMarshal(map[string]any{"err": err.Error()})
			fmt.Printf("%s\n", errJSON)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		exitFailAndCloseCard()
	}

}

// runDefault is the handler for the one mode: the request, or the actions, on the command
// line
func runDefault(config *lib.ConfigSettings) (err error) {

	// With nothing to do, show where to go next, the saved settings, and the ports
	if len(os.Args) == 1 {
		cli.PrintHelp(cli.ModeNamed(cli.ModeDefault), false)
		cli.PrintSettings(config)
		printPorts(config)
		return nil
	}

	// What needs no Notecard comes first
	if flagVersion {
		fmt.Printf("Notecard CLI Version: %s\n", version)
		return nil
	}
	if flagDFUPackage != "" {
		if flagRequest != "" {
			return fmt.Errorf("--req and --binpack may not be combined into one command")
		}
		return dfuPackage(flagVerbose, flagOutput, flagDFUPackage, flag.Args())
	}

	// The rest of the command line is the request, unless --req gave it
	switch {
	case len(flag.Args()) == 0:
	case len(flag.Args()) == 1 && flagRequest == "":
		flagRequest = flag.Args()[0]
	default:
		return fmt.Errorf("a request must be in quotes, as the last argument, with the options before it")
	}

	// Recording packets is exclusive with the other things that talk to the Notecard
	if flagPcap != "" {
		if flagPcap != "usb" && flagPcap != "aux" {
			return fmt.Errorf("--pcap must be 'usb' or 'aux'")
		}
		if flagRequest != "" || flagTrace || flagPlayground || flagCommtest || flagEcho != 0 {
			return fmt.Errorf("--pcap cannot be combined with other actions")
		}
	}

	// Open the card, just to make sure errors are reported early
	configVal := config.IPort[config.Interface].PortConfig
	if flagPlaytime != 0 {
		configVal = flagPlaytime
		flagPlayground = true
	}
	notecard.InitialDebugMode = flagVerbose
	notecard.InitialTraceMode = flagTrace
	card, err = notecard.Open(config.Interface, config.IPort[config.Interface].Port, configVal)

	// Process non-config commands
	var rsp notecard.Request

	// The timeouts in the note-go library are set under the assumption that
	// the device is connected with long header wires that have significant
	// capacitance and resistance, and where there may be arbitrary activity
	// on the Notecard. This switch uses a larger buffer and shorter inter-segment
	// delays under the assumption that perhaps this is being done using USB.
	// NOTE: the comment above was from before USB flow control was implemented
	// in the Notecard, in build 15741 Jan 26 2023.  At that time, this was
	// 1024/30, but is no longer relevant as we can pound the Notecard on the
	// USB port because of hardware flow control.
	if err == nil && flagFast {
		notecard.RequestSegmentMaxLen = 1024
		notecard.RequestSegmentDelayMs = 5
	}

	// Wait until disconnected
	if err == nil && flagWhenDisconnected {
		for {
			rsp, err := card.TransactionRequest(notecard.Request{Req: "hub.status", NotefileID: notecard.SyncLogNotefile, Delete: true})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				break
			}
			if strings.Contains(rsp.Status, note.StatusTransportDisconnected) {
				break
			}
			fmt.Printf("%s\n", rsp.Status)
			time.Sleep(3 * time.Second)
		}
	}

	// Wait until connected
	if err == nil && flagWhenConnected {
		for {
			delay := true
			rsp, err := card.TransactionRequest(notecard.Request{Req: "note.get", NotefileID: notecard.SyncLogNotefile, Delete: true})
			if err != nil && note.ErrorContains(err, note.ErrNoteNoExist) {
				delay = true
				err = nil
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
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
	if err == nil && flagWhenDisarmed {
		for {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "card.attn"})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			} else if rsp.Set {
				break
			}
			time.Sleep(3 * time.Second)
		}
	}

	// Wait until synced
	if err == nil && flagWhenSynced {
		req := notecard.Request{Req: "hub.sync.status"}
		req.Sync = true // Initiate sync if sync is needed
		rsp, err = card.TransactionRequest(req)
		for err == nil {
			rsp, err = card.TransactionRequest(notecard.Request{Req: "hub.sync.status"})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				break
			}
			if rsp.Alert {
				fmt.Fprintf(os.Stderr, "sync error\n")
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
		card.DebugOutput(flagVerbose, false)
	}

	// Do SKU setup before anything else, particularly because if we are going
	// to do a factory reset it needs to be done after we set up the SKU
	if err == nil && flagSetupSKU != "" && flagScan == "" {
		var requestsString string
		requestsString, err = loadRequestsString(flagSetupSKU)
		if err == nil {
			req := notecard.Request{Req: "card.setup"}
			req.Text = requestsString
			_, err = card.TransactionRequest(req)
		}
		if err == nil && !(flagFactory || flagFormat) {
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
	if err == nil && flagFormat {
		req := notecard.Request{Req: "card.restore"}
		card.TransactionRequest(req)
		verifyCompletion = true
	}
	if err == nil && flagFactory && (flagScan == "" && flagSetup == "") {
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

	if err == nil && flagInfo {

		var infoErr error
		if !flagVerbose {
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

	if err == nil && flagProduct != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", ProductUID: flagProduct})
	}

	if err == nil && flagSN != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", SN: flagSN})
	}

	if err == nil && flagHub != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.set", Host: flagHub})
	}

	if err == nil && flagSideload != "" && flagScan == "" {
		err = dfuSideload(flagSideload, flagNoBin, flagVerbose)
	}

	if err == nil && flagUpload != "" {
		err = uploadFile(flagUpload, flagRoute, flagTarget)
	}

	if err == nil && flagRequest != "" {
		var rspJSON []byte
		var req, rsp notecard.Request
		note.JSONUnmarshal([]byte(flagRequest), &req)

		// Append payload from the specified file
		if flagInput != "" {
			var contents []byte
			contents, err = os.ReadFile(flagInput)
			if err == nil {
				req.Payload = &contents

				// Update the original request with the payload
				var reqBytes []byte
				reqBytes, err = note.JSONMarshal(req)
				if err == nil {
					flagRequest = string(reqBytes)
				}
			}
		}

		// Validate the request against the schema
		if err == nil && validateJSON {
			var reqMap map[string]interface{}
			err = note.JSONUnmarshal([]byte(flagRequest), &reqMap)
			if err == nil {
				validationErr := validateRequest(reqMap, jsonSchemaUrl, flagVerbose)
				if validationErr != nil {
					fmt.Fprintf(os.Stderr, "warning: %s\n", validationErr)
				}
			}
		}

		// Perform the transaction and do special handling for binary
		if err == nil && req.Req == "card.binary.get" {
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
		} else if err == nil && req.Req == "card.binary.put" && (req.Body == nil || len(*req.Body) == 0) {
			if req.Payload == nil {
				err = fmt.Errorf("card.binary.put requires payload data")
			} else {
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
			}
		} else if err == nil {
			// Transact using CLI input to avoid JSON parsing complications
			if !flagDry {
				flagRequest = strings.ReplaceAll(flagRequest, "\\n", "\n")
				rspJSON, err = card.TransactionJSON([]byte(flagRequest))
				if err == nil {
					_ = note.JSONUnmarshal(rspJSON, &rsp)
				}
			} else if flagVerbose {
				fmt.Printf("%s\n", flagRequest)
			}
		}

		// Write the payload to an output file if appropriate
		if err == nil && flagOutput != "" && rsp.Payload != nil {
			err = os.WriteFile(flagOutput, *rsp.Payload, 0644)
			// If we can't write the file, set the payload to nil so
			// we don't try to print it out and cause a JSON error.
			if err != nil {
				rsp.Payload = nil
			}
		}

		// Output the response to the console
		if err == nil && !flagVerbose && !flagDry {
			if flagPretty {
				rspJSON, _ = note.JSONMarshalIndent(rsp, "", "    ")
			} else {
				rspJSON, _ = note.JSONMarshal(rsp)
			}
			fmt.Printf("%s\n", rspJSON)
		}
	}

	if err == nil && flagLog != "" {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.log", Text: flagLog})
	}

	if err == nil && flagSync {
		_, err = card.TransactionRequest(notecard.Request{Req: "hub.sync"})
	}

	if err == nil && flagSetup != "" && flagScan == "" {
		var requests []map[string]interface{}
		requests, err = loadRequests(flagSetup)
		if err == nil {
			card.DebugOutput(true, false)
			err = processRequests(flagFactory, requests)
		}
	}

	if err == nil && flagScan != "" {
		err = scan(flagVerbose, flagFactory, flagSetup, flagSetupSKU, flagProvision, flagFactory, flagSideload, flagNoBin, flagScan)
	}

	if err == nil && flagCommtest {

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

	if err == nil && flagTrace {
		err = card.Trace()
	}

	if err == nil && flagPlayground {
		os.Exit(NewREPL(card).Start())
	}

	if err == nil && flagEcho != 0 {
		err = echo(flagEcho)
	}

	if err == nil && flagRTC != "" {
		err = rtc(flagRTC, flagVerbose)
	}

	if err == nil && flagPcap != "" {
		err = pcapRecord(flagOutput, flagPcap, card)
	}

	if err == nil && flagExplore {
		err = explore(flagReserved, flagPretty)
	}

	return err

}

// printPorts lists the ports of the interface in use, marking the one that is saved
func printPorts(config *lib.ConfigSettings) {
	nInterface, nPort, _ := notecard.Defaults()
	if config.Interface != "" {
		nInterface = config.Interface
		nPort = config.IPort[config.Interface].Port
	}
	var ports []string
	switch nInterface {
	case notecard.NotecardInterfaceSerial:
		ports, _, _, _ = notecard.SerialPorts()
	case notecard.NotecardInterfaceI2C:
		ports, _, _, _ = notecard.I2CPorts()
	}
	if len(ports) == 0 {
		return
	}
	fmt.Printf("\nPorts on '%s':\n", nInterface)
	for _, port := range ports {
		if port != nPort {
			fmt.Printf("   %s\n", port)
		} else if nPortConfig := config.IPort[config.Interface].PortConfig; nPortConfig == 0 {
			fmt.Printf("   %s ***\n", port)
		} else {
			fmt.Printf("   %s (%d) ***\n", port, nPortConfig)
		}
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
