// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// provisionEntry represents a single device to be provisioned from a bulk file.
type provisionEntry struct {
	DeviceUID    string   `json:"device_uid"`
	SerialNumber string   `json:"serial_number,omitempty"`
	FleetUIDs    []string `json:"fleet_uids,omitempty"`
}

// provisionCmd represents the provision command
var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision devices to a product",
	Long: `Provision devices to a product.

The --product flag specifies the target product UID. Devices can be specified
using --scope (for existing devices) or --file (for bulk provisioning from
a CSV or JSON file).

When using --file, each row/entry can specify a device UID, an optional serial
number, and optional fleet UIDs. The file format is detected from the extension.

CSV format (header row required):
  device_uid,serial_number,fleet_uids
  dev:864475046552567,SENSOR-001,"fleet:xxx,fleet:yyy"
  dev:864475046552568,SENSOR-002,

JSON format (array of objects):
  [
    {"device_uid": "dev:864475046552567", "serial_number": "SENSOR-001", "fleet_uids": ["fleet:xxx"]},
    {"device_uid": "dev:864475046552568", "serial_number": "SENSOR-002"}
  ]` + scopeHelpLong + `

Examples:
  # Provision a single device
  notehub provision --scope dev:864475046552567 --product com.company:product

  # Provision with serial number
  notehub provision --scope dev:xxxx --product com.company:product --sn SENSOR-001

  # Bulk provision from CSV
  notehub provision --file devices.csv --product com.company:product

  # Bulk provision from JSON
  notehub provision --file devices.json --product com.company:product

  # Provision all devices in a fleet
  notehub provision --scope @production --product com.company:product`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAuth(); err != nil {
			return err
		}

		product := GetProduct()
		if product == "" {
			return fmt.Errorf("--product is required")
		}

		filePath, _ := cmd.Flags().GetString("file")

		// --file and --scope are mutually exclusive
		if filePath != "" && flagScope != "" {
			return fmt.Errorf("use either --file or --scope, not both")
		}
		if filePath == "" && flagScope == "" {
			return fmt.Errorf("--scope or --file is required")
		}

		// Bulk provisioning from file
		if filePath != "" {
			return provisionFromFile(cmd, filePath, product)
		}

		// Scope-based provisioning (existing behavior)
		appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(flagScope)
		if err != nil {
			return err
		}

		verbose := GetVerbose()
		err = varsProvisionDevices(appMetadata, scopeDevices, product, flagSn, verbose)
		if err != nil {
			return err
		}

		return printActionResult(cmd, map[string]any{
			"action":      "provision",
			"devices":     scopeDevices,
			"count":       len(scopeDevices),
			"product_uid": product,
		}, fmt.Sprintf("Provisioned %d device(s) to product %s", len(scopeDevices), product))
	},
}

// provisionFromFile reads a CSV or JSON file and provisions each device.
func provisionFromFile(cmd *cobra.Command, filePath, productUID string) error {
	entries, err := parseProvisionFile(filePath)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("no devices found in %s", filePath)
	}

	client, ctx, projectUID, err := initCommand()
	if err != nil {
		return err
	}

	verbose := GetVerbose()
	var succeeded, failed int

	for _, entry := range entries {
		provReq := notehub.NewProvisionDeviceRequest(productUID)
		if entry.SerialNumber != "" {
			provReq.SetDeviceSn(entry.SerialNumber)
		}
		if len(entry.FleetUIDs) > 0 {
			provReq.SetFleetUids(entry.FleetUIDs)
		}

		_, _, err := client.DeviceAPI.ProvisionDevice(ctx, projectUID, entry.DeviceUID).
			ProvisionDeviceRequest(*provReq).
			Execute()
		if err != nil {
			failed++
			cmd.PrintErrln(fmt.Sprintf("Failed to provision %s: %s", entry.DeviceUID, err))
			continue
		}
		succeeded++
		if verbose {
			sn := ""
			if entry.SerialNumber != "" {
				sn = fmt.Sprintf(" (SN: %s)", entry.SerialNumber)
			}
			cmd.Printf("Provisioned %s%s\n", entry.DeviceUID, sn)
		}
	}

	result := map[string]any{
		"action":      "provision",
		"product_uid": productUID,
		"total":       len(entries),
		"succeeded":   succeeded,
		"failed":      failed,
		"file":        filePath,
	}

	msg := fmt.Sprintf("Provisioned %d/%d device(s) to product %s", succeeded, len(entries), productUID)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}

	return printActionResult(cmd, result, msg)
}

// parseProvisionFile reads a CSV or JSON file and returns provision entries.
func parseProvisionFile(filePath string) ([]provisionEntry, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return parseProvisionCSV(filePath)
	case ".json":
		return parseProvisionJSON(filePath)
	default:
		return nil, fmt.Errorf("unsupported file format '%s' (use .csv or .json)", ext)
	}
}

// parseProvisionCSV reads a CSV file with columns: device_uid, serial_number, fleet_uids
func parseProvisionCSV(filePath string) ([]provisionEntry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have a header row and at least one data row")
	}

	// Parse header to find column indices
	header := records[0]
	colIndex := map[string]int{}
	for i, col := range header {
		colIndex[strings.TrimSpace(strings.ToLower(col))] = i
	}

	uidCol, ok := colIndex["device_uid"]
	if !ok {
		return nil, fmt.Errorf("CSV must have a 'device_uid' column")
	}

	snCol := -1
	if idx, ok := colIndex["serial_number"]; ok {
		snCol = idx
	}
	fleetCol := -1
	if idx, ok := colIndex["fleet_uids"]; ok {
		fleetCol = idx
	}

	var entries []provisionEntry
	for i, row := range records[1:] {
		if len(row) <= uidCol {
			return nil, fmt.Errorf("row %d: missing device_uid column", i+2)
		}

		deviceUID := strings.TrimSpace(row[uidCol])
		if deviceUID == "" {
			continue // skip empty rows
		}

		entry := provisionEntry{DeviceUID: deviceUID}

		if snCol >= 0 && snCol < len(row) {
			entry.SerialNumber = strings.TrimSpace(row[snCol])
		}

		if fleetCol >= 0 && fleetCol < len(row) {
			fleetStr := strings.TrimSpace(row[fleetCol])
			if fleetStr != "" {
				for _, f := range strings.Split(fleetStr, ",") {
					f = strings.TrimSpace(f)
					if f != "" {
						entry.FleetUIDs = append(entry.FleetUIDs, f)
					}
				}
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parseProvisionJSON reads a JSON file containing an array of provision entries.
func parseProvisionJSON(filePath string) ([]provisionEntry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var entries []provisionEntry
	if err := note.JSONUnmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate all entries have device_uid
	for i, e := range entries {
		if e.DeviceUID == "" {
			return nil, fmt.Errorf("entry %d: missing device_uid", i+1)
		}
	}

	return entries, nil
}

func init() {
	rootCmd.AddCommand(provisionCmd)

	addScopeFlag(provisionCmd, "Device scope")
	provisionCmd.Flags().StringVar(&flagSn, "sn", "", "Serial number for provisioning (used with --scope)")
	provisionCmd.Flags().String("file", "", "CSV or JSON file for bulk provisioning")
}
