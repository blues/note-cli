// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/blues/note-go/note"
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

		// Define device types
		type Device struct {
			UID            string    `json:"uid"`
			SerialNumber   string    `json:"serial_number"`
			ProductUID     string    `json:"product_uid,omitempty"`
			FleetUIDs      []string  `json:"fleet_uids,omitempty"`
			LastActivity   time.Time `json:"last_activity,omitempty"`
			ContactedAt    time.Time `json:"contacted,omitempty"`
			Provisioned    time.Time `json:"provisioned,omitempty"`
			LocationName   string    `json:"tower_location_name,omitempty"`
			LocationWhen   time.Time `json:"tower_when,omitempty"`
			DeviceType     string    `json:"sku,omitempty"`
			NotecardVersion string   `json:"notecard_firmware_version,omitempty"`
			HostVersion    string    `json:"host_firmware_version,omitempty"`
		}

		type DevicesResponse struct {
			Devices    []Device `json:"devices"`
			HasMore    bool     `json:"has_more"`
			TotalCount int      `json:"total_count,omitempty"`
		}

		// Get devices using V1 API: GET /v1/projects/{projectUID}/devices
		devicesRsp := DevicesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices", projectUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &devicesRsp)
		if err != nil {
			return fmt.Errorf("failed to list devices: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(devicesRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(devicesRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(devicesRsp.Devices) == 0 {
			fmt.Println("No devices found in this project.")
			return nil
		}

		// Display devices in human-readable format
		fmt.Printf("\nDevices in Project:\n")
		fmt.Printf("===================\n\n")

		for _, device := range devicesRsp.Devices {
			fmt.Printf("Device: %s\n", device.UID)
			if device.SerialNumber != "" {
				fmt.Printf("  Serial Number: %s\n", device.SerialNumber)
			}
			if device.ProductUID != "" {
				fmt.Printf("  Product: %s\n", device.ProductUID)
			}
			if device.DeviceType != "" {
				fmt.Printf("  Type: %s\n", device.DeviceType)
			}
			if device.NotecardVersion != "" {
				fmt.Printf("  Notecard Firmware: %s\n", device.NotecardVersion)
			}
			if device.HostVersion != "" {
				fmt.Printf("  Host Firmware: %s\n", device.HostVersion)
			}
			if !device.LastActivity.IsZero() {
				fmt.Printf("  Last Activity: %s\n", device.LastActivity.Format("2006-01-02 15:04:05 MST"))
			}
			if !device.ContactedAt.IsZero() {
				fmt.Printf("  Last Contact: %s\n", device.ContactedAt.Format("2006-01-02 15:04:05 MST"))
			}
			if !device.Provisioned.IsZero() {
				fmt.Printf("  Provisioned: %s\n", device.Provisioned.Format("2006-01-02 15:04:05 MST"))
			}
			if device.LocationName != "" {
				fmt.Printf("  Location: %s\n", device.LocationName)
			}
			if len(device.FleetUIDs) > 0 {
				fmt.Printf("  Fleets: %d\n", len(device.FleetUIDs))
			}
			fmt.Println()
		}

		fmt.Printf("Total devices: %d\n", len(devicesRsp.Devices))
		if devicesRsp.HasMore {
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

		verbose := GetVerbose()
		appMetadata, scopeDevices, _, err := appGetScope(scope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 {
			return fmt.Errorf("no devices to enable")
		}

		// Enable each device
		for _, deviceUID := range scopeDevices {
			url := fmt.Sprintf("/v1/projects/%s/devices/%s/enable", appMetadata.App.UID, deviceUID)
			err := reqHubV1(verbose, GetAPIHub(), "POST", url, nil, nil)
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

		verbose := GetVerbose()
		appMetadata, scopeDevices, _, err := appGetScope(scope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 {
			return fmt.Errorf("no devices to disable")
		}

		// Disable each device
		for _, deviceUID := range scopeDevices {
			url := fmt.Sprintf("/v1/projects/%s/devices/%s/disable", appMetadata.App.UID, deviceUID)
			err := reqHubV1(verbose, GetAPIHub(), "POST", url, nil, nil)
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

		verbose := GetVerbose()
		appMetadata, scopeDevices, _, err := appGetScope(scope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 {
			return fmt.Errorf("no devices to move")
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

		// Define request body types
		type FleetRequest struct {
			FleetUIDs []string `json:"fleet_uids"`
		}

		type FleetResponse struct {
			Fleets []struct {
				UID   string `json:"uid"`
				Label string `json:"label"`
			} `json:"fleets"`
		}

		// Move each device to the target fleet
		for _, deviceUID := range scopeDevices {
			url := fmt.Sprintf("/v1/projects/%s/devices/%s/fleets", appMetadata.App.UID, deviceUID)

			// First, get the device's current fleets
			var currentFleets FleetResponse
			err := reqHubV1(verbose, GetAPIHub(), "GET", url, nil, &currentFleets)
			if err != nil {
				return fmt.Errorf("failed to get current fleets for device %s: %w", deviceUID, err)
			}

			// Remove device from all current fleets if it has any
			if len(currentFleets.Fleets) > 0 {
				currentFleetUIDs := make([]string, len(currentFleets.Fleets))
				for i, fleet := range currentFleets.Fleets {
					currentFleetUIDs[i] = fleet.UID
				}
				removeBody := FleetRequest{FleetUIDs: currentFleetUIDs}
				removeJSON, err := note.JSONMarshal(removeBody)
				if err != nil {
					return fmt.Errorf("failed to marshal remove request: %w", err)
				}
				err = reqHubV1(verbose, GetAPIHub(), "DELETE", url, removeJSON, nil)
				if err != nil {
					return fmt.Errorf("failed to remove device %s from current fleets: %w", deviceUID, err)
				}
				if verbose {
					fmt.Printf("Device %s removed from %d fleet(s)\n", deviceUID, len(currentFleetUIDs))
				}
			}

			// Add device to the target fleet
			addBody := FleetRequest{FleetUIDs: []string{targetFleetUID}}
			addJSON, err := note.JSONMarshal(addBody)
			if err != nil {
				return fmt.Errorf("failed to marshal add request: %w", err)
			}
			err = reqHubV1(verbose, GetAPIHub(), "PUT", url, addJSON, nil)
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

		// Define health log types
		type HealthLogEntry struct {
			When  time.Time `json:"when"`
			Alert bool      `json:"alert"`
			Text  string    `json:"text"`
		}

		type HealthLogResponse struct {
			HealthLog []HealthLogEntry `json:"health_log"`
		}

		// Get device health log using V1 API: GET /v1/projects/{projectUID}/devices/{deviceUID}/health-log
		healthLogRsp := HealthLogResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/health-log", projectUID, deviceUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &healthLogRsp)
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

		// Define session types
		type Tower struct {
			Time   int64   `json:"time,omitempty"`
			Name   string  `json:"n,omitempty"`
			Country string `json:"c,omitempty"`
			Lat    float64 `json:"lat,omitempty"`
			Lon    float64 `json:"lon,omitempty"`
			Zone   string  `json:"zone,omitempty"`
		}

		type Period struct {
			Since      int64 `json:"since,omitempty"`
			Duration   int64 `json:"duration,omitempty"`
			BytesRcvd  int64 `json:"bytes_rcvd,omitempty"`
			BytesSent  int64 `json:"bytes_sent,omitempty"`
			SessionsTLS int64 `json:"sessions_tls,omitempty"`
			NotesSent  int64 `json:"notes_sent,omitempty"`
		}

		type Session struct {
			SessionUID       string    `json:"session"`
			Device           string    `json:"device,omitempty"`
			Product          string    `json:"product,omitempty"`
			Fleets           []string  `json:"fleets,omitempty"`
			When             int64     `json:"when,omitempty"`
			SessionBegan     int64     `json:"session_began,omitempty"`
			SessionEnded     int64     `json:"session_ended,omitempty"`
			WhyOpened        string    `json:"why_session_opened,omitempty"`
			WhyClosed        string    `json:"why_session_closed,omitempty"`
			Cell             string    `json:"cell,omitempty"`
			RSSI             int       `json:"rssi,omitempty"`
			SINR             int       `json:"sinr,omitempty"`
			RSRP             int       `json:"rsrp,omitempty"`
			RSRQ             int       `json:"rsrq,omitempty"`
			Bars             int       `json:"bars,omitempty"`
			RAT              string    `json:"rat,omitempty"`
			Bearer           string    `json:"bearer,omitempty"`
			IP               string    `json:"ip,omitempty"`
			ICCID            string    `json:"iccid,omitempty"`
			APN              string    `json:"apn,omitempty"`
			Tower            *Tower    `json:"tower,omitempty"`
			Voltage          float64   `json:"voltage,omitempty"`
			Temp             float64   `json:"temp,omitempty"`
			Continuous       bool      `json:"continuous,omitempty"`
			TLS              bool      `json:"tls,omitempty"`
			Events           int       `json:"events,omitempty"`
			Moved            int64     `json:"moved,omitempty"`
			Orientation      string    `json:"orientation,omitempty"`
			Period           *Period   `json:"period,omitempty"`
		}

		type SessionsResponse struct {
			Sessions []Session `json:"sessions"`
			HasMore  bool      `json:"has_more"`
		}

		// Get device sessions using V1 API: GET /v1/projects/{projectUID}/devices/{deviceUID}/sessions
		sessionsRsp := SessionsResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/sessions", projectUID, deviceUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &sessionsRsp)
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
			fmt.Printf("Session: %s\n", session.SessionUID)
			if session.When > 0 {
				sessionTime := time.Unix(session.When, 0)
				fmt.Printf("  Time: %s\n", sessionTime.Format("2006-01-02 15:04:05 MST"))
			}

			// Session status
			if session.WhyOpened != "" {
				fmt.Printf("  Opened: %s\n", session.WhyOpened)
			}
			if session.WhyClosed != "" {
				fmt.Printf("  Closed: %s\n", session.WhyClosed)
			}

			// Network information
			if session.RAT != "" || session.Bearer != "" {
				fmt.Printf("  Network: %s", session.RAT)
				if session.Bearer != "" {
					fmt.Printf(" (%s)", session.Bearer)
				}
				fmt.Println()
			}

			// Signal quality
			if session.Bars > 0 {
				fmt.Printf("  Signal: %d bars", session.Bars)
				if session.RSSI != 0 {
					fmt.Printf(" (RSSI: %d)", session.RSSI)
				}
				fmt.Println()
			}

			// Location
			if session.Tower != nil && session.Tower.Name != "" {
				fmt.Printf("  Location: %s", session.Tower.Name)
				if session.Tower.Country != "" {
					fmt.Printf(", %s", session.Tower.Country)
				}
				fmt.Println()
			}

			// Device status
			if session.Voltage > 0 {
				fmt.Printf("  Voltage: %.3fV", session.Voltage)
				if session.Temp > 0 {
					fmt.Printf(", Temp: %.1f°C", session.Temp)
				}
				fmt.Println()
			}

			// Session stats
			if session.Events > 0 {
				fmt.Printf("  Events: %d", session.Events)
				if session.TLS {
					fmt.Printf(" (TLS)")
				}
				fmt.Println()
			}

			// Data transfer
			if session.Period != nil {
				if session.Period.BytesSent > 0 || session.Period.BytesRcvd > 0 {
					fmt.Printf("  Data: sent %d bytes, received %d bytes", session.Period.BytesSent, session.Period.BytesRcvd)
					if session.Period.Duration > 0 {
						fmt.Printf(" (duration: %ds)", session.Period.Duration)
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
