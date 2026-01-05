// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// dfuCmd represents the dfu command
var dfuCmd = &cobra.Command{
	Use:   "dfu",
	Short: "Manage device firmware updates",
	Long:  `Commands for scheduling and managing firmware updates for Notecards and host MCUs.`,
}

// dfuUpdateCmd represents the dfu update command
var dfuUpdateCmd = &cobra.Command{
	Use:   "update [firmware-type] [filename] [scope]",
	Short: "Schedule a firmware update",
	Long: `Schedule a firmware update for devices. Firmware type must be either 'host' or 'notecard'.

The filename should match a firmware file that has been uploaded to your Notehub project.

Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

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
		GetCredentials() // Validates and exits if not authenticated

		firmwareType := args[0]
		filename := args[1]
		scope := args[2]

		// Validate firmware type
		if firmwareType != "host" && firmwareType != "notecard" {
			return fmt.Errorf("firmware type must be 'host' or 'notecard', got '%s'", firmwareType)
		}

		// Resolve scope to device UIDs
		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
		if err != nil {
			return err
		}

		verbose := GetVerbose()

		// Get additional filter flags
		tags, _ := cmd.Flags().GetString("tag")
		serialNumbers, _ := cmd.Flags().GetString("serial")
		location, _ := cmd.Flags().GetString("location")
		notecardFirmware, _ := cmd.Flags().GetString("notecard-firmware")
		hostFirmware, _ := cmd.Flags().GetString("host-firmware")
		productUID, _ := cmd.Flags().GetString("product")
		sku, _ := cmd.Flags().GetString("sku")

		// Build request body
		dfuRequest := notehub.NewDfuActionRequest()
		dfuRequest.SetFilename(filename)

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Build request with SDK
		req := client.ProjectAPI.PerformDfuAction(ctx, appMetadata.App.UID, firmwareType, "update").
			DfuActionRequest(*dfuRequest)

		// Add device UIDs
		if len(scopeDevices) > 0 {
			req = req.DeviceUID(scopeDevices)
		}

		// Add optional filters
		if tags != "" {
			req = req.Tag(strings.Split(tags, ","))
		}
		if serialNumbers != "" {
			req = req.SerialNumber(strings.Split(serialNumbers, ","))
		}
		if location != "" {
			req = req.Location([]string{location})
		}
		if notecardFirmware != "" {
			req = req.NotecardFirmware([]string{notecardFirmware})
		}
		if hostFirmware != "" {
			req = req.HostFirmware([]string{hostFirmware})
		}
		if productUID != "" {
			req = req.ProductUID([]string{productUID})
		}
		if sku != "" {
			req = req.Sku([]string{sku})
		}

		// Execute the DFU update
		_, err = req.Execute()
		if err != nil {
			return fmt.Errorf("failed to schedule firmware update: %w", err)
		}

		fmt.Printf("\nFirmware update scheduled successfully!\n\n")
		fmt.Printf("Firmware Type: %s\n", firmwareType)
		fmt.Printf("Filename: %s\n", filename)
		fmt.Printf("Scope: %s\n", scope)
		fmt.Printf("Target Devices: %d device(s)\n", len(scopeDevices))
		if verbose && len(scopeDevices) > 0 {
			fmt.Printf("Device UIDs: %s\n", strings.Join(scopeDevices, ","))
		}
		if tags != "" {
			fmt.Printf("Additional Tag Filter: %s\n", tags)
		}
		if serialNumbers != "" {
			fmt.Printf("Additional Serial Filter: %s\n", serialNumbers)
		}
		if location != "" {
			fmt.Printf("Additional Location Filter: %s\n", location)
		}
		if sku != "" {
			fmt.Printf("Additional SKU Filter: %s\n", sku)
		}
		fmt.Println()

		return nil
	},
}

// dfuCancelCmd represents the dfu cancel command
var dfuCancelCmd = &cobra.Command{
	Use:   "cancel [firmware-type] [scope]",
	Short: "Cancel pending firmware updates",
	Long: `Cancel pending firmware updates for devices. Firmware type must be either 'host' or 'notecard'.

Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

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
		GetCredentials() // Validates and exits if not authenticated

		firmwareType := args[0]
		scope := args[1]

		// Validate firmware type
		if firmwareType != "host" && firmwareType != "notecard" {
			return fmt.Errorf("firmware type must be 'host' or 'notecard', got '%s'", firmwareType)
		}

		// Resolve scope to device UIDs
		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(scope)
		if err != nil {
			return err
		}

		verbose := GetVerbose()

		// Get additional filter flags
		tags, _ := cmd.Flags().GetString("tag")
		serialNumbers, _ := cmd.Flags().GetString("serial")

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Build cancel request with SDK
		req := client.ProjectAPI.PerformDfuAction(ctx, appMetadata.App.UID, firmwareType, "cancel")

		// Add device UIDs
		if len(scopeDevices) > 0 {
			req = req.DeviceUID(scopeDevices)
		}

		// Add optional filters
		if tags != "" {
			req = req.Tag(strings.Split(tags, ","))
		}
		if serialNumbers != "" {
			req = req.SerialNumber(strings.Split(serialNumbers, ","))
		}

		// Execute the DFU cancel
		_, err = req.Execute()
		if err != nil {
			return fmt.Errorf("failed to cancel firmware update: %w", err)
		}

		fmt.Printf("\nFirmware update cancelled successfully!\n\n")
		fmt.Printf("Firmware Type: %s\n", firmwareType)
		fmt.Printf("Scope: %s\n", scope)
		fmt.Printf("Target Devices: %d device(s)\n", len(scopeDevices))
		if verbose && len(scopeDevices) > 0 {
			fmt.Printf("Device UIDs: %s\n", strings.Join(scopeDevices, ","))
		}
		if tags != "" {
			fmt.Printf("Additional Tag Filter: %s\n", tags)
		}
		if serialNumbers != "" {
			fmt.Printf("Additional Serial Filter: %s\n", serialNumbers)
		}
		fmt.Println()

		return nil
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
		GetCredentials() // Validates and exits if not authenticated

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get filter flags
		firmwareType, _ := cmd.Flags().GetString("type")
		productUID, _ := cmd.Flags().GetString("product")
		version, _ := cmd.Flags().GetString("version")
		target, _ := cmd.Flags().GetString("target")
		filename, _ := cmd.Flags().GetString("filename")
		unpublished, _ := cmd.Flags().GetBool("unpublished")

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

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

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(firmwareList, "", "  ")
			} else {
				output, err = note.JSONMarshal(firmwareList)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(firmwareList) == 0 {
			fmt.Println("No firmware files found.")
			return nil
		}

		// Display firmware in human-readable format
		fmt.Printf("\nAvailable Firmware Files:\n")
		fmt.Printf("=========================\n\n")

		// Group by type
		hostFirmware := []notehub.FirmwareInfo{}
		notecardFirmware := []notehub.FirmwareInfo{}
		otherFirmware := []notehub.FirmwareInfo{}

		for _, fw := range firmwareList {
			if fw.Type != nil && *fw.Type == "host" {
				hostFirmware = append(hostFirmware, fw)
			} else if fw.Type != nil && *fw.Type == "notecard" {
				notecardFirmware = append(notecardFirmware, fw)
			} else {
				otherFirmware = append(otherFirmware, fw)
			}
		}

		// Display host firmware
		if len(hostFirmware) > 0 {
			fmt.Printf("Host Firmware (%d):\n", len(hostFirmware))
			fmt.Printf("------------------\n")
			for _, fw := range hostFirmware {
				if fw.Filename != nil {
					fmt.Printf("  %s", *fw.Filename)
				}
				if fw.Version != nil && *fw.Version != "" {
					fmt.Printf(" (v%s)", *fw.Version)
				}
				if fw.Published != nil && !*fw.Published {
					fmt.Printf(" [unpublished]")
				}
				fmt.Println()
				if fw.Description != nil && *fw.Description != "" {
					fmt.Printf("    Description: %s\n", *fw.Description)
				}
				if fw.Built != nil && *fw.Built != "" {
					fmt.Printf("    Built: %s\n", *fw.Built)
				}
				if fw.Target != nil && *fw.Target != "" {
					fmt.Printf("    Target: %s\n", *fw.Target)
				}
				fmt.Println()
			}
		}

		// Display notecard firmware
		if len(notecardFirmware) > 0 {
			fmt.Printf("Notecard Firmware (%d):\n", len(notecardFirmware))
			fmt.Printf("----------------------\n")
			for _, fw := range notecardFirmware {
				if fw.Filename != nil {
					fmt.Printf("  %s", *fw.Filename)
				}
				if fw.Version != nil && *fw.Version != "" {
					fmt.Printf(" (v%s)", *fw.Version)
				}
				if fw.Published != nil && !*fw.Published {
					fmt.Printf(" [unpublished]")
				}
				fmt.Println()
				if fw.Description != nil && *fw.Description != "" {
					fmt.Printf("    Description: %s\n", *fw.Description)
				}
				if fw.Built != nil && *fw.Built != "" {
					fmt.Printf("    Built: %s\n", *fw.Built)
				}
				if fw.Target != nil && *fw.Target != "" {
					fmt.Printf("    Target: %s\n", *fw.Target)
				}
				fmt.Println()
			}
		}

		// Display other firmware
		if len(otherFirmware) > 0 {
			fmt.Printf("Other Firmware (%d):\n", len(otherFirmware))
			fmt.Printf("-------------------\n")
			for _, fw := range otherFirmware {
				if fw.Filename != nil {
					fmt.Printf("  %s", *fw.Filename)
				}
				if fw.Version != nil && *fw.Version != "" {
					fmt.Printf(" (v%s)", *fw.Version)
				}
				if fw.Type != nil && *fw.Type != "" {
					fmt.Printf(" [%s]", *fw.Type)
				}
				if fw.Published != nil && !*fw.Published {
					fmt.Printf(" [unpublished]")
				}
				fmt.Println()
				if fw.Description != nil && *fw.Description != "" {
					fmt.Printf("    Description: %s\n", *fw.Description)
				}
				if fw.Built != nil && *fw.Built != "" {
					fmt.Printf("    Built: %s\n", *fw.Built)
				}
				if fw.Target != nil && *fw.Target != "" {
					fmt.Printf("    Target: %s\n", *fw.Target)
				}
				fmt.Println()
			}
		}

		fmt.Printf("Total firmware files: %d\n\n", len(firmwareList))

		return nil
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
