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

// dfuCmd represents the dfu command
var dfuCmd = &cobra.Command{
	Use:   "dfu",
	Short: "Manage device firmware updates",
	Long:  `Commands for scheduling and managing firmware updates for Notecards and host MCUs.`,
}

// dfuAction is the shared implementation for DFU update and cancel commands.
func dfuAction(cmd *cobra.Command, firmwareType, action, scope, filename string) error {
	// Validate firmware type
	if firmwareType != "host" && firmwareType != "notecard" {
		return fmt.Errorf("firmware type must be 'host' or 'notecard', got '%s'", firmwareType)
	}

	// Resolve scope to device UIDs
	appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
	if err != nil {
		return err
	}

	// Get filter flags (shared by both update and cancel)
	tags, _ := cmd.Flags().GetString("tag")
	serialNumbers, _ := cmd.Flags().GetString("serial")

	// Get SDK client
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return err
	}

	// Build request with SDK
	req := client.ProjectAPI.PerformDfuAction(ctx, appMetadata.App.UID, firmwareType, action)

	// Set filename for update action
	if filename != "" {
		dfuRequest := notehub.NewDfuActionRequest()
		dfuRequest.SetFilename(filename)
		req = req.DfuActionRequest(*dfuRequest)
	}

	// Add device UIDs
	if len(scopeDevices) > 0 {
		req = req.DeviceUID(scopeDevices)
	}

	// Add shared filters
	if tags != "" {
		req = req.Tag(strings.Split(tags, ","))
	}
	if serialNumbers != "" {
		req = req.SerialNumber(strings.Split(serialNumbers, ","))
	}

	// Add update-only filters
	if action == "update" {
		if location, _ := cmd.Flags().GetString("location"); location != "" {
			req = req.Location([]string{location})
		}
		if notecardFirmware, _ := cmd.Flags().GetString("notecard-firmware"); notecardFirmware != "" {
			req = req.NotecardFirmware([]string{notecardFirmware})
		}
		if hostFirmware, _ := cmd.Flags().GetString("host-firmware"); hostFirmware != "" {
			req = req.HostFirmware([]string{hostFirmware})
		}
		if productUID, _ := cmd.Flags().GetString("product"); productUID != "" {
			req = req.ProductUID([]string{productUID})
		}
		if sku, _ := cmd.Flags().GetString("sku"); sku != "" {
			req = req.Sku([]string{sku})
		}
	}

	// Execute the DFU action
	_, err = req.Execute()
	if err != nil {
		return fmt.Errorf("failed to %s firmware update: %w", action, err)
	}

	// Build result
	result := map[string]any{
		"action":        action,
		"firmware_type": firmwareType,
		"scope":         scope,
		"devices":       scopeDevices,
		"device_count":  len(scopeDevices),
	}
	if filename != "" {
		result["filename"] = filename
	}
	if tags != "" {
		result["tag_filter"] = tags
	}
	if serialNumbers != "" {
		result["serial_filter"] = serialNumbers
	}

	var successMsg string
	if action == "update" {
		successMsg = fmt.Sprintf("Firmware update scheduled for %d device(s)\nFirmware Type: %s\nFilename: %s\nScope: %s", len(scopeDevices), firmwareType, filename, scope)
	} else {
		successMsg = fmt.Sprintf("Firmware update cancelled for %d device(s)\nFirmware Type: %s\nScope: %s", len(scopeDevices), firmwareType, scope)
	}

	return printActionResult(cmd, result, successMsg)
}

// dfuUpdateCmd represents the dfu update command
var dfuUpdateCmd = &cobra.Command{
	Use:   "update [firmware-type] [filename] [scope]",
	Short: "Schedule a firmware update",
	Long: `Schedule a firmware update for devices. Firmware type must be either 'host' or 'notecard'.

The filename should match a firmware file that has been uploaded to your Notehub project.` + scopeHelpLong + `

Additional filters can be used to narrow down the scope:
  --location          Filter by location
  --notecard-firmware Filter by Notecard firmware version
  --host-firmware     Filter by host firmware version
  --product           Filter by product UID
  --sku               Filter by SKU
  --tag               Filter by device tags (comma-separated)
  --serial            Filter by serial numbers (comma-separated)

Examples:
  # Schedule notecard firmware update for a specific device
  notehub dfu update notecard notecard-6.2.1.bin dev:864475046552567

  # Schedule host firmware update for all devices in a fleet
  notehub dfu update host app-v1.2.3.bin @production

  # Schedule update for multiple devices
  notehub dfu update notecard notecard-6.2.1.bin dev:aaa,dev:bbb,dev:ccc

  # Schedule update for all devices in project
  notehub dfu update notecard notecard-6.2.1.bin @

  # Schedule update for devices from a file
  notehub dfu update host app-v1.2.3.bin @devices.txt

  # Schedule update with additional filters
  notehub dfu update notecard notecard-6.2.1.bin @production --sku NOTE-WBEX`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return dfuAction(cmd, args[0], "update", args[2], args[1])
	},
}

// dfuCancelCmd represents the dfu cancel command
var dfuCancelCmd = &cobra.Command{
	Use:   "cancel [firmware-type] [scope]",
	Short: "Cancel pending firmware updates",
	Long: `Cancel pending firmware updates for devices. Firmware type must be either 'host' or 'notecard'.` + scopeHelpLong + `

Additional filters can be used to narrow down the scope:
  --tag               Filter by device tags (comma-separated)
  --serial            Filter by serial numbers (comma-separated)

Examples:
  # Cancel notecard firmware update for a specific device
  notehub dfu cancel notecard dev:864475046552567

  # Cancel host firmware updates for all devices in a fleet
  notehub dfu cancel host @production

  # Cancel updates for multiple devices
  notehub dfu cancel notecard dev:aaa,dev:bbb,dev:ccc

  # Cancel updates for all devices in project
  notehub dfu cancel notecard @

  # Cancel updates for devices from a file
  notehub dfu cancel host @devices.txt`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return dfuAction(cmd, args[0], "cancel", args[1], "")
	},
}

// dfuListCmd represents the dfu list command
var dfuListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available firmware files",
	Long: `List all firmware files available in the current project.

You can filter by firmware type (host or notecard) and other criteria.

Examples:
  # List all firmware files
  notehub dfu list

  # List only host firmware
  notehub dfu list --type host

  # List only notecard firmware
  notehub dfu list --type notecard

  # List with JSON output
  notehub dfu list --pretty`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		// Get filter flags
		firmwareType, _ := cmd.Flags().GetString("type")
		productUID, _ := cmd.Flags().GetString("product")
		version, _ := cmd.Flags().GetString("version")
		target, _ := cmd.Flags().GetString("target")
		filename, _ := cmd.Flags().GetString("filename")
		unpublished, _ := cmd.Flags().GetBool("unpublished")

		// Build request with SDK
		req := client.ProjectAPI.GetFirmwareInfo(ctx, projectUID)

		// Add query parameters
		if firmwareType != "" {
			req = req.FirmwareType(firmwareType)
		}
		if productUID != "" {
			req = req.Product(productUID)
		}
		if version != "" {
			req = req.Version(version)
		}
		if target != "" {
			req = req.Target(target)
		}
		if filename != "" {
			req = req.Filename(filename)
		}
		if unpublished {
			req = req.Unpublished(unpublished)
		}

		// Get firmware list using SDK
		firmwareList, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to list firmware: %w", err)
		}

		return printListResult(cmd, firmwareList, "No firmware files found.", func() bool {
			return len(firmwareList) == 0
		})
	},
}

func init() {
	rootCmd.AddCommand(dfuCmd)
	dfuCmd.AddCommand(dfuListCmd)
	dfuCmd.AddCommand(dfuUpdateCmd)
	dfuCmd.AddCommand(dfuCancelCmd)

	// Add flags for dfu list
	dfuListCmd.Flags().String("type", "", "Filter by firmware type (host or notecard)")
	dfuListCmd.Flags().String("product", "", "Filter by product UID")
	dfuListCmd.Flags().String("version", "", "Filter by version")
	dfuListCmd.Flags().String("target", "", "Filter by target device")
	dfuListCmd.Flags().String("filename", "", "Filter by filename")
	dfuListCmd.Flags().Bool("unpublished", false, "Include unpublished firmware")
	dfuListCmd.Flags().MarkHidden("unpublished")

	// Add flags for dfu update (additional filters beyond scope)
	dfuUpdateCmd.Flags().String("tag", "", "Additional filter by device tags (comma-separated)")
	dfuUpdateCmd.Flags().String("serial", "", "Additional filter by serial numbers (comma-separated)")
	dfuUpdateCmd.Flags().String("location", "", "Additional filter by location")
	dfuUpdateCmd.Flags().String("notecard-firmware", "", "Additional filter by Notecard firmware version")
	dfuUpdateCmd.Flags().String("host-firmware", "", "Additional filter by host firmware version")
	dfuUpdateCmd.Flags().String("product", "", "Additional filter by product UID")
	dfuUpdateCmd.Flags().String("sku", "", "Additional filter by SKU")

	// Add flags for dfu cancel (additional filters beyond scope)
	dfuCancelCmd.Flags().String("tag", "", "Additional filter by device tags (comma-separated)")
	dfuCancelCmd.Flags().String("serial", "", "Additional filter by serial numbers (comma-separated)")
}
