// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"

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
	Long: `List devices in the current project or a specified project.

By default, returns up to 50 devices. Use --limit to change the number, or --all to fetch every device.

Examples:
  # List first 50 devices (default)
  notehub device list

  # List first 100 devices
  notehub device list --limit 100

  # List all devices
  notehub device list --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		pageSize, maxResults := getPaginationConfig(cmd)

		var allDevices []notehub.Device
		pageNum := int32(1)
		for {
			devicesResp, _, err := client.DeviceAPI.GetDevices(ctx, projectUID).
				PageSize(pageSize).
				PageNum(pageNum).
				Execute()
			if err != nil {
				return fmt.Errorf("failed to list devices: %w", err)
			}

			allDevices = append(allDevices, devicesResp.Devices...)

			if !devicesResp.HasMore {
				break
			}
			if maxResults > 0 && len(allDevices) >= maxResults {
				allDevices = allDevices[:maxResults]
				break
			}
			pageNum++
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, allDevices)
		}

		if len(allDevices) == 0 {
			cmd.Println("No devices found in this project.")
			return nil
		}
		return printHuman(cmd, allDevices)
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
				cmd.Printf("Device %s enabled\n", deviceUID)
			}
		}

		cmd.Printf("Successfully enabled %d device(s)\n", len(scopeDevices))
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
				cmd.Printf("Device %s disabled\n", deviceUID)
			}
		}

		cmd.Printf("Successfully disabled %d device(s)\n", len(scopeDevices))
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
					cmd.Printf("Device %s removed from %d fleet(s)\n", deviceUID, len(currentFleetUIDs))
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
				cmd.Printf("Device %s moved to fleet %s\n", deviceUID, targetFleetUID)
			}
		}

		cmd.Printf("Successfully moved %d device(s) to fleet %s\n", len(scopeDevices), targetFleetUID)
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
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		healthLogRsp, _, err := client.DeviceAPI.GetDeviceHealthLog(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device health log: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, healthLogRsp)
		}

		if len(healthLogRsp.HealthLog) == 0 {
			cmd.Println("No health log entries found for this device.")
			return nil
		}
		return printHuman(cmd, healthLogRsp)
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

  # Get more sessions
  notehub device session dev:864475046552567 --limit 100

  # Get all sessions
  notehub device session dev:864475046552567 --all`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		pageSize, maxResults := getPaginationConfig(cmd)

		var allSessions []notehub.DeviceSession
		pageNum := int32(1)
		for {
			sessionsRsp, _, err := client.DeviceAPI.GetDeviceSessions(ctx, projectUID, deviceUID).
				PageSize(pageSize).
				PageNum(pageNum).
				Execute()
			if err != nil {
				return fmt.Errorf("failed to get device sessions: %w", err)
			}

			allSessions = append(allSessions, sessionsRsp.Sessions...)

			if !sessionsRsp.HasMore {
				break
			}
			if maxResults > 0 && len(allSessions) >= maxResults {
				allSessions = allSessions[:maxResults]
				break
			}
			pageNum++
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, allSessions)
		}

		if len(allSessions) == 0 {
			cmd.Println("No sessions found for this device.")
			return nil
		}
		return printHuman(cmd, allSessions)
	},
}

// deviceGetCmd represents the device get command
var deviceGetCmd = &cobra.Command{
	Use:   "get [device-uid]",
	Short: "Get device details",
	Long: `Get details about a specific device in the current project.

Examples:
  # Get device details
  notehub device get dev:864475046552567

  # Get device details with JSON output
  notehub device get dev:864475046552567 --json

  # Get device details with pretty JSON
  notehub device get dev:864475046552567 --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		device, _, err := client.DeviceAPI.GetDevice(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device: %w", err)
		}

		return printResult(cmd, device)
	},
}

// deviceDeleteCmd represents the device delete command
var deviceDeleteCmd = &cobra.Command{
	Use:   "delete [device-uid]",
	Short: "Delete a device",
	Long: `Delete a device from the current project.

Examples:
  # Delete a device
  notehub device delete dev:864475046552567`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		_, err = client.DeviceAPI.DeleteDevice(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete device: %w", err)
		}

		cmd.Printf("\nDevice '%s' deleted successfully.\n\n", deviceUID)
		return nil
	},
}

// deviceSignalCmd represents the device signal command
var deviceSignalCmd = &cobra.Command{
	Use:   "signal [device-uid]",
	Short: "Send a signal to a device",
	Long: `Send a signal to a device to check if it is currently connected.

Examples:
  # Signal a device
  notehub device signal dev:864475046552567

  # Signal a device with JSON output
  notehub device signal dev:864475046552567 --json

  # Signal a device with pretty JSON
  notehub device signal dev:864475046552567 --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		signalResp, _, err := client.DeviceAPI.SignalDevice(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to signal device: %w", err)
		}

		return printResult(cmd, signalResp)
	},
}

// deviceEventsCmd represents the device events command
var deviceEventsCmd = &cobra.Command{
	Use:   "events [device-uid]",
	Short: "Get latest events for a device",
	Long: `Get the latest events for a specific device.

Examples:
  # Get latest events for a device
  notehub device events dev:864475046552567

  # Get latest events with JSON output
  notehub device events dev:864475046552567 --json

  # Get latest events with pretty JSON
  notehub device events dev:864475046552567 --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		eventsRsp, _, err := client.DeviceAPI.GetDeviceLatestEvents(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device latest events: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, eventsRsp)
		}

		if len(eventsRsp.LatestEvents) == 0 {
			cmd.Println("No events found for this device.")
			return nil
		}
		return printHuman(cmd, eventsRsp)
	},
}

// devicePlansCmd represents the device plans command
var devicePlansCmd = &cobra.Command{
	Use:   "plans [device-uid]",
	Short: "Get data plans for a device",
	Long: `Get the data plans associated with a device, including primary SIM, external SIM, and satellite connections.

Examples:
  # Get data plans for a device
  notehub device plans dev:864475046552567

  # Get data plans with JSON output
  notehub device plans dev:864475046552567 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		plansRsp, _, err := client.DeviceAPI.GetDevicePlans(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device plans: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, plansRsp)
		}

		if len(plansRsp.CellularPlans) == 0 {
			cmd.Println("No data plans found for this device.")
			return nil
		}
		return printHuman(cmd, plansRsp)
	},
}

// deviceKeysCmd represents the device keys command
var deviceKeysCmd = &cobra.Command{
	Use:   "keys [device-uid]",
	Short: "Get public key for a device",
	Long: `Get the public key for a specific device, or list all device public keys in the project.

Examples:
  # Get public key for a specific device
  notehub device keys dev:864475046552567

  # List all device public keys in the project
  notehub device keys --all

  # List all with JSON output
  notehub device keys --all --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		listAll, _ := cmd.Flags().GetBool("all")

		if listAll {
			limit, _ := cmd.Flags().GetInt("limit")
			pageSize := int32(limit)
			maxResults := limit

			var allKeys []notehub.GetDevicePublicKeys200ResponseDevicePublicKeysInner
			pageNum := int32(1)
			for {
				keysRsp, _, err := client.DeviceAPI.GetDevicePublicKeys(ctx, projectUID).
					PageSize(pageSize).
					PageNum(pageNum).
					Execute()
				if err != nil {
					return fmt.Errorf("failed to list device public keys: %w", err)
				}

				allKeys = append(allKeys, keysRsp.DevicePublicKeys...)

				if !keysRsp.HasMore {
					break
				}
				if maxResults > 0 && len(allKeys) >= maxResults {
					allKeys = allKeys[:maxResults]
					break
				}
				pageNum++
			}

			if wantJSON() {
				return printJSON(cmd, allKeys)
			}

			if len(allKeys) == 0 {
				cmd.Println("No device public keys found.")
				return nil
			}
			return printHuman(cmd, allKeys)
		}

		// Single device public key
		if len(args) == 0 {
			return fmt.Errorf("device UID required, or use --all to list all device public keys")
		}

		deviceUID := args[0]
		keyRsp, _, err := client.DeviceAPI.GetDevicePublicKey(ctx, projectUID, deviceUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get device public key: %w", err)
		}

		return printResult(cmd, keyRsp)
	},
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceGetCmd)
	deviceCmd.AddCommand(deviceDeleteCmd)
	deviceCmd.AddCommand(deviceSignalCmd)
	deviceCmd.AddCommand(deviceEnableCmd)
	deviceCmd.AddCommand(deviceDisableCmd)
	deviceCmd.AddCommand(deviceMoveCmd)
	deviceCmd.AddCommand(deviceHealthCmd)
	deviceCmd.AddCommand(deviceSessionCmd)
	deviceCmd.AddCommand(deviceEventsCmd)
	deviceCmd.AddCommand(devicePlansCmd)
	deviceCmd.AddCommand(deviceKeysCmd)

	deviceKeysCmd.Flags().Bool("all", false, "List public keys for all devices in the project")
	deviceKeysCmd.Flags().Int("limit", 50, "Maximum number of keys to return (used with --all)")

	addPaginationFlags(deviceListCmd, 50)
	addPaginationFlags(deviceSessionCmd, 50)
}
