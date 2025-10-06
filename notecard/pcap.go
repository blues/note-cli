// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/notecard"
	"go.bug.st/serial"
)

// pcapGlobalHeader represents the PCAP global header structure
var pcapGlobalHeader = []byte{
	0xd4, 0xc3, 0xb2, 0xa1, // magic number
	0x02, 0x00, 0x04, 0x00, // version major, minor
	0x00, 0x00, 0x00, 0x00, // thiszone
	0x00, 0x00, 0x00, 0x00, // sigfigs
	0xff, 0xff, 0x00, 0x00, // snaplen
	0x65, 0x00, 0x00, 0x00, // network (0x65 = 101 = raw IP)
}

// pcapRecord handles PCAP recording from a serial port
func pcapRecord(outputFile string, pcapType string, card *notecard.Context) error {
	// Determine the output file path
	if outputFile == "" {
		outputFile = "notecard.pcap"
	}

	// Make sure the output directory exists
	outputDir := filepath.Dir(outputFile)
	if outputDir != "." {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Remove existing file if it exists
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Removing existing PCAP file: %s\n", outputFile)
		os.Remove(outputFile)
	}

	// Create the PCAP file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create PCAP file: %w", err)
	}
	defer file.Close()

	// Write the PCAP global header
	fmt.Printf("Writing PCAP global header to %s...\n", outputFile)
	_, err = file.Write(pcapGlobalHeader)
	if err != nil {
		return fmt.Errorf("failed to write PCAP header: %w", err)
	}

	// Configure the Notecard for PCAP mode
	fmt.Printf("Configuring Notecard for PCAP mode...\n")

	// Use the explicit PCAP mode provided by the user
	pcapMode := "pcap-" + pcapType
	config, err := lib.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}
	iport := config.IPort[config.Interface]

	fmt.Printf("Using PCAP mode: %s (port: %s)\n", pcapMode, iport.Port)

	// Enable PCAP mode
	_, err = card.TransactionRequest(notecard.Request{
		Req:  "card.aux.serial",
		Mode: pcapMode,
	})
	if err != nil {
		return fmt.Errorf("failed to enable PCAP mode: %w", err)
	}

	// Close the card connection so we can open the raw serial port
	card.Close()

	// Small delay to let the Notecard reconfigure
	time.Sleep(500 * time.Millisecond)

	// Open the raw serial port for PCAP data streaming
	fmt.Printf("Opening serial port for PCAP streaming: %s\n", iport.Port)

	// Configure serial port settings
	baudRate := 115200 // Default baud rate for PCAP
	if iport.PortConfig > 0 {
		baudRate = iport.PortConfig
	}

	mode := &serial.Mode{
		BaudRate: baudRate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(iport.Port, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}
	defer port.Close()

	// Set up signal handling for graceful shutdown of the raw serial port
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Channel to signal the main loop to stop
	stopChan := make(chan bool, 1)

	go func() {
		<-signalChan
		port.Close()
		stopChan <- true
	}()

	fmt.Printf("PCAP streaming started. Press Ctrl+C to stop.\n")
	fmt.Printf("Data will be written to: %s\n", outputFile)
	fmt.Printf("Open the file in Wireshark to view captured packets.\n")

	// Stream data from serial port to PCAP file
	buffer := make([]byte, 4096)
	totalBytesWritten := 0

	for {
		select {
		case <-stopChan:
			// Signal received, exit gracefully
			fmt.Printf("PCAP capture completed. Total bytes written: %d\n", totalBytesWritten)
			return nil
		default:
			// Continue with normal operation
			n, err := port.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}
				// Handle timeout as normal (serial port read timeout)
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				// For any other error (including port closed), just break out
				break
			}

			if n > 0 {
				_, err = file.Write(buffer[:n])
				if err != nil {
					return fmt.Errorf("failed to write to PCAP file: %w", err)
				}

				totalBytesWritten += n

				// Flush the file to ensure data is written
				file.Sync()

				// Print progress occasionally
				if totalBytesWritten%10240 == 0 { // Every 10KB
					fmt.Printf("Captured %d bytes...\n", totalBytesWritten)
				}
			}
		}
	}
}
