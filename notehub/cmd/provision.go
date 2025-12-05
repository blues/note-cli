// Copyright 2024 Blues Inc.  All rights reserved.
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
Command-line flags override config file values.

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
		GetCredentials() // Validate credentials

		if flagScope == "" {
			return fmt.Errorf("use --scope to specify device(s) to be provisioned")
		}

		product := GetProduct()
		if product == "" {
			return fmt.Errorf("--product must be specified (the product UID to provision devices to)")
		}

		verbose := GetVerbose()
		appMetadata, scopeDevices, _, err := appGetScope(flagScope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 {
			return fmt.Errorf("no devices to provision")
		}

		err = varsProvisionDevices(appMetadata, scopeDevices, product, flagSn, verbose)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully provisioned %d device(s) to product %s\n", len(scopeDevices), product)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(provisionCmd)

	provisionCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device scope (required)")
	provisionCmd.Flags().StringVar(&flagSn, "sn", "", "Serial number for provisioning")
	provisionCmd.MarkFlagRequired("scope")
}
