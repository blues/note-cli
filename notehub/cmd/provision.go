// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// provisionCmd represents the provision command
var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision devices to a product",
	Long: `Provision devices to a product.

The --product flag specifies the target product UID. The --project flag (or
config file) specifies which project contains the devices to provision.
Command-line flags override config file values.` + scopeHelpLong + `

Examples:
  # Provision a single device (project from config)
  notehub provision --scope dev:864475046552567 --product com.company:product

  # Provision all devices in a fleet
  notehub provision --scope @production --product com.company:product

  # Provision all devices in project
  notehub provision --scope @ --product com.company:product

  # Provision devices from a file
  notehub provision --scope @devices.txt --product com.company:product

  # Override project from command line
  notehub provision --scope dev:xxxx --product com.company:product --project app:xxxx

  # Provision with serial number
  notehub provision --scope dev:xxxx --product com.company:product --sn SENSOR-001`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAuth(); err != nil {
			return err
		}

		if flagScope == "" {
			return fmt.Errorf("--scope is required")
		}

		product := GetProduct()
		if product == "" {
			return fmt.Errorf("--product is required")
		}

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

func init() {
	rootCmd.AddCommand(provisionCmd)

	addScopeFlag(provisionCmd, "Device scope (required)")
	provisionCmd.Flags().StringVar(&flagSn, "sn", "", "Serial number for provisioning")
	provisionCmd.MarkFlagRequired("scope")
}
