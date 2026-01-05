// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// deviceCmd represents the device command
var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage Notehub devices",
	Long:  `Commands for listing and managing devices in Notehub projects.`,
}

// deviceListCmd represents the device list command
var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	Long:  `List all devices in the current project or a specified project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get devices using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		devicesResp, _, err := client.DeviceAPI.GetDevices(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list devices: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(devicesResp, "", "  ")
			} else {
				output, err = note.JSONMarshal(devicesResp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(devicesResp.Devices) == 0 {
			fmt.Println("No devices found in this project.")
			return nil
		}

		// Display devices in human-readable format
		fmt.Printf("\nDevices in Project:\n")
		fmt.Printf("===================\n\n")

		for _, device := range devicesResp.Devices {
			fmt.Printf("Device: %s\n", device.Uid)
			if device.SerialNumber != nil && *device.SerialNumber != "" {
				fmt.Printf("  Serial Number: %s\n", *device.SerialNumber)
			}
			if device.ProductUid != "" {
				fmt.Printf("  Product: %s\n", device.ProductUid)
			}
			if device.Sku != nil && *device.Sku != "" {
				fmt.Printf("  Type: %s\n", *device.Sku)
			}
			if device.FirmwareNotecard != nil && *device.FirmwareNotecard != "" {
				fmt.Printf("  Notecard Firmware: %s\n", *device.FirmwareNotecard)
			}
			if device.FirmwareHost != nil && *device.FirmwareHost != "" {
				fmt.Printf("  Host Firmware: %s\n", *device.FirmwareHost)
			}
			if device.LastActivity.IsSet() {
				if lastActivity := device.LastActivity.Get(); lastActivity != nil && !lastActivity.IsZero() {
					fmt.Printf("  Last Activity: %s\n", lastActivity.Format("2006-01-02 15:04:05 MST"))
				}
			}
			if !device.Provisioned.IsZero() {
				fmt.Printf("  Provisioned: %s\n", device.Provisioned.Format("2006-01-02 15:04:05 MST"))
			}
			if device.FleetUids != nil && len(device.FleetUids) > 0 {
				fmt.Printf("  Fleets: %d\n", len(device.FleetUids))
			}
			fmt.Println()
		}

		fmt.Printf("Total devices: %d\n", len(devicesResp.Devices))
		if devicesResp.HasMore {
			fmt.Println("(showing first page of results)")
		}
		fmt.Println()

		return nil
	},
}

// deviceEnableCmd represents the device enable command
var deviceEnableCmd = &cobra.Command{
	Use:   "enable [scope]",
	Short: "Enable one or more devices",
	Long: `Enable one or more devices in a Notehub project, allowing them to communicate with Notehub.

Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

Examples:
  # Enable a single device
  notehub device enable dev:864475046552567

  # Enable all devices in a fleet
  notehub device enable @production

  # Enable all devices in project
  notehub device enable @

  # Enable devices from a file
  notehub device enable @devices.txt`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		scope := args[0]

		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
		if err != nil {
			return err
		}

		// Enable each device using SDK
		verbose := GetVerbose()
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		for _, deviceUID := range scopeDevices {
			_, err := client.DeviceAPI.EnableDevice(ctx, appMetadata.App.UID, deviceUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to enable device %s: %w", deviceUID, err)
			}
			if verbose {
				fmt.Printf("Device %s enabled\n", deviceUID)
			}
		}

		fmt.Printf("Successfully enabled %d device(s)\n", len(scopeDevices))
		return nil
	},
}

// deviceDisableCmd represents the device disable command
var deviceDisableCmd = &cobra.Command{
	Use:   "disable [scope]",
	Short: "Disable one or more devices",
	Long: `Disable one or more devices in a Notehub project, preventing them from communicating with Notehub.

Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

Examples:
  # Disable a single device
  notehub device disable dev:864475046552567

  # Disable all devices in a fleet
  notehub device disable @production

  # Disable all devices in project
  notehub device disable @

  # Disable devices from a file
  notehub device disable @devices.txt`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		scope := args[0]

		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
		if err != nil {
			return err
		}

		// Disable each device using SDK
		verbose := GetVerbose()
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		for _, deviceUID := range scopeDevices {
			_, err := client.DeviceAPI.DisableDevice(ctx, appMetadata.App.UID, deviceUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to disable device %s: %w", deviceUID, err)
			}
			if verbose {
				fmt.Printf("Device %s disabled\n", deviceUID)
			}
		}

		fmt.Printf("Successfully disabled %d device(s)\n", len(scopeDevices))
		return nil
	},
}

// deviceMoveCmd represents the device move command
var deviceMoveCmd = &cobra.Command{
	Use:   "move [scope] [fleet-uid-or-name]",
	Short: "Move devices to a fleet",
	Long: `Move one or more devices to a fleet. If a device is not in any fleet, it will be assigned.
If a device is already in a fleet, it will be moved to the new fleet.

Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

Examples:
  # Move a single device to a fleet
  notehub device move dev:864475046552567 production

  # Move a device to a fleet by UID
  notehub device move dev:864475046552567 fleet:xxxx

  # Move all devices from one fleet to another
  notehub device move @old-fleet new-fleet

  # Move devices from a file to a fleet
  notehub device move @devices.txt production`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		scope := args[0]
		targetFleetIdentifier := args[1]

		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
		if err != nil {
			return err
		}

		// Find the target fleet by UID or name
		var targetFleetUID string
		if strings.HasPrefix(targetFleetIdentifier, "fleet:") {
			targetFleetUID = targetFleetIdentifier
		} else {
			// Search for fleet by name
			found := false
			for _, fleet := range appMetadata.Fleets {
				if fleet.Name == targetFleetIdentifier {
					targetFleetUID = fleet.UID
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("fleet '%s' not found in project", targetFleetIdentifier)
			}
		}

		verbose := GetVerbose()
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Move each device to the target fleet using SDK
		for _, deviceUID := range scopeDevices {
			// First, get the device's current fleets
			currentFleets, _, err := client.ProjectAPI.GetDeviceFleets(ctx, appMetadata.App.UID, deviceUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to get current fleets for device %s: %w", deviceUID, err)
			}

			// Remove device from all current fleets if it has any
			if currentFleets.Fleets != nil && len(currentFleets.Fleets) > 0 {
				currentFleetUIDs := make([]string, len(currentFleets.Fleets))
				for i, fleet := range currentFleets.Fleets {
					currentFleetUIDs[i] = fleet.Uid
				}

				deleteReq := notehub.NewDeleteDeviceFromFleetsRequest(currentFleetUIDs)
				_, _, err = client.ProjectAPI.DeleteDeviceFromFleets(ctx, appMetadata.App.UID, deviceUID).
					DeleteDeviceFromFleetsRequest(*deleteReq).
					Execute()
				if err != nil {
					return fmt.Errorf("failed to remove device %s from current fleets: %w", deviceUID, err)
				}
				if verbose {
					fmt.Printf("Device %s removed from %d fleet(s)\n", deviceUID, len(currentFleetUIDs))
				}
			}

			// Add device to the target fleet
			addReq := notehub.NewAddDeviceToFleetsRequest([]string{targetFleetUID})
			_, _, err = client.ProjectAPI.AddDeviceToFleets(ctx, appMetadata.App.UID, deviceUID).
				AddDeviceToFleetsRequest(*addReq).
				Execute()
			if err != nil {
				return fmt.Errorf("failed to move device %s to fleet: %w", deviceUID, err)
			}
			if verbose {
				fmt.Printf("Device %s moved to fleet %s\n", deviceUID, targetFleetUID)
			}
		}

		fmt.Printf("Successfully moved %d device(s) to fleet %s\n", len(scopeDevices), targetFleetUID)
		return nil
	},
}

// deviceHealthCmd represents the device health command
var deviceHealthCmd = &cobra.Command{
	Use:   "health [device-uid]",
	Short: "Get device health log",
	Long: `Get the health log for a specific device, showing boot events, DFU completions, and other health-related information.

Examples:
  # Get health log for a device
  notehub device health dev:864475046552567

  # Get health log with JSON output
  notehub device health dev:864475046552567 --json

  # Get health log with pretty JSON
  notehub device health dev:864475046552567 --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		deviceUID := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get device health log using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		healthLogRsp, _, err := client.DeviceAPI.GetDeviceHealthLog(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device health log: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(healthLogRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(healthLogRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(healthLogRsp.HealthLog) == 0 {
			fmt.Println("No health log entries found for this device.")
			return nil
		}

		// Display health log in human-readable format
		fmt.Printf("\nHealth Log for Device: %s\n", deviceUID)
		fmt.Printf("================================\n\n")

		for _, entry := range healthLogRsp.HealthLog {
			alertMarker := " "
			if entry.Alert {
				alertMarker = "!"
			}
			fmt.Printf("[%s] %s %s\n", entry.When.Format("2006-01-02 15:04:05 MST"), alertMarker, entry.Text)
		}

		fmt.Printf("\nTotal entries: %d\n", len(healthLogRsp.HealthLog))
		fmt.Println()

		return nil
	},
}

// deviceSessionCmd represents the device session command
var deviceSessionCmd = &cobra.Command{
	Use:   "session [device-uid]",
	Short: "Get device session log",
	Long: `Get the session log for a specific device, showing connection history, network information, and session statistics.

Examples:
  # Get session log for a device
  notehub device session dev:864475046552567

  # Get session log with JSON output
  notehub device session dev:864475046552567 --json

  # Get session log with pretty JSON
  notehub device session dev:864475046552567 --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		deviceUID := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get device sessions using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		sessionsRsp, _, err := client.DeviceAPI.GetDeviceSessions(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device sessions: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(sessionsRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(sessionsRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(sessionsRsp.Sessions) == 0 {
			fmt.Println("No sessions found for this device.")
			return nil
		}

		// Display sessions in human-readable format
		fmt.Printf("\nSession Log for Device: %s\n", deviceUID)
		fmt.Printf("=================================\n\n")

		for i, session := range sessionsRsp.Sessions {
			if i > 0 {
				fmt.Println("---")
			}

			// Session ID and timing
			if session.Session != nil {
				fmt.Printf("Session: %s\n", *session.Session)
			}
			if session.When != nil && *session.When > 0 {
				sessionTime := time.Unix(*session.When, 0)
				fmt.Printf("  Time: %s\n", sessionTime.Format("2006-01-02 15:04:05 MST"))
			}

			// Session status
			if session.WhySessionOpened != nil && *session.WhySessionOpened != "" {
				fmt.Printf("  Opened: %s\n", *session.WhySessionOpened)
			}
			if session.WhySessionClosed != nil && *session.WhySessionClosed != "" {
				fmt.Printf("  Closed: %s\n", *session.WhySessionClosed)
			}

			// Network information
			if (session.Rat != nil && *session.Rat != "") || (session.Bearer != nil && *session.Bearer != "") {
				if session.Rat != nil {
					fmt.Printf("  Network: %s", *session.Rat)
				}
				if session.Bearer != nil && *session.Bearer != "" {
					fmt.Printf(" (%s)", *session.Bearer)
				}
				fmt.Println()
			}

			// Signal quality
			if session.Bars != nil && *session.Bars > 0 {
				fmt.Printf("  Signal: %d bars", *session.Bars)
				if session.Rssi != nil && *session.Rssi != 0 {
					fmt.Printf(" (RSSI: %d)", *session.Rssi)
				}
				fmt.Println()
			}

			// Location
			if session.Tower != nil && session.Tower.N != nil && *session.Tower.N != "" {
				fmt.Printf("  Location: %s", *session.Tower.N)
				if session.Tower.C != nil && *session.Tower.C != "" {
					fmt.Printf(", %s", *session.Tower.C)
				}
				fmt.Println()
			}

			// Device status
			if session.Voltage != nil && *session.Voltage > 0 {
				fmt.Printf("  Voltage: %.3fV", *session.Voltage)
				if session.Temp != nil && *session.Temp > 0 {
					fmt.Printf(", Temp: %.1f°C", *session.Temp)
				}
				fmt.Println()
			}

			// Session stats
			if session.Events != nil && *session.Events > 0 {
				fmt.Printf("  Events: %d", *session.Events)
				if session.Tls != nil && *session.Tls {
					fmt.Printf(" (TLS)")
				}
				fmt.Println()
			}

			// Data transfer
			if session.Period != nil {
				if (session.Period.BytesSent != nil && *session.Period.BytesSent > 0) ||
					(session.Period.BytesRcvd != nil && *session.Period.BytesRcvd > 0) {
					var sent, rcvd int64
					if session.Period.BytesSent != nil {
						sent = *session.Period.BytesSent
					}
					if session.Period.BytesRcvd != nil {
						rcvd = *session.Period.BytesRcvd
					}
					fmt.Printf("  Data: sent %d bytes, received %d bytes", sent, rcvd)
					if session.Period.Duration != nil && *session.Period.Duration > 0 {
						fmt.Printf(" (duration: %ds)", *session.Period.Duration)
					}
					fmt.Println()
				}
			}
		}

		fmt.Printf("\nTotal sessions: %d", len(sessionsRsp.Sessions))
		if sessionsRsp.HasMore {
			fmt.Printf(" (showing first page)")
		}
		fmt.Println()
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceEnableCmd)
	deviceCmd.AddCommand(deviceDisableCmd)
	deviceCmd.AddCommand(deviceMoveCmd)
	deviceCmd.AddCommand(deviceHealthCmd)
	deviceCmd.AddCommand(deviceSessionCmd)
}
