// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/blues/note-go/note"
	"github.com/spf13/cobra"
)

var (
	flagScope string
	flagSn    string
)

// varsCmd represents the vars command
var varsCmd = &cobra.Command{
	Use:   "vars",
	Short: "Manage environment variables",
	Long:  `Commands for getting and setting environment variables for devices and fleets.`,
}

// varsGetCmd represents the vars get command
var varsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get environment variables",
	Long: `Get environment variables for devices or fleets.

Scope can be:
  - A device UID (dev:xxxx)
  - A fleet UID (fleet:xxxx)
  - A fleet name or pattern (e.g., "production" or "prod*")
  - @fleetname to get all devices in a fleet
  - @ to get all devices in the project
  - A comma-separated list of any of the above
  - @filename to read scope from a file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		if flagScope == "" {
			return fmt.Errorf("use --scope to specify device(s) or fleet(s)")
		}

		verbose := GetVerbose()
		appMetadata, scopeDevices, scopeFleets, err := appGetScope(flagScope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 && len(scopeFleets) == 0 {
			return fmt.Errorf("no devices or fleets found within the specified scope")
		}

		if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
			return fmt.Errorf("scope may include devices or fleets but not both")
		}

		var vars map[string]Vars
		if len(scopeDevices) != 0 {
			vars, err = varsGetFromDevices(appMetadata, scopeDevices, verbose)
		} else if len(scopeFleets) != 0 {
			vars, err = varsGetFromFleets(appMetadata, scopeFleets, verbose)
		}
		if err != nil {
			return err
		}

		var varsJSON []byte
		if GetPretty() {
			varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
		} else {
			varsJSON, err = note.JSONMarshal(vars)
		}
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", varsJSON)
		return nil
	},
}

// varsSetCmd represents the vars set command
var varsSetCmd = &cobra.Command{
	Use:   "set [json]",
	Short: "Set environment variables",
	Long: `Set environment variables for devices or fleets.

The JSON argument can be a JSON object or @filename to read from a file.

Example:
  notehub vars set --scope dev:xxxx '{"VAR1":"value1","VAR2":"value2"}'
  notehub vars set --scope @production @vars.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		if flagScope == "" {
			return fmt.Errorf("use --scope to specify device(s) or fleet(s)")
		}

		verbose := GetVerbose()
		appMetadata, scopeDevices, scopeFleets, err := appGetScope(flagScope, verbose)
		if err != nil {
			return err
		}

		if len(scopeDevices) == 0 && len(scopeFleets) == 0 {
			return fmt.Errorf("no devices or fleets found within the specified scope")
		}

		if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
			return fmt.Errorf("scope may include devices or fleets but not both")
		}

		// Parse template
		template := Vars{}
		varsArg := args[0]
		if strings.HasPrefix(varsArg, "@") {
			var templateJSON []byte
			templateJSON, err = os.ReadFile(strings.TrimPrefix(varsArg, "@"))
			if err != nil {
				return err
			}
			err = note.JSONUnmarshal(templateJSON, &template)
		} else {
			err = note.JSONUnmarshal([]byte(varsArg), &template)
		}
		if err != nil {
			return err
		}

		var vars map[string]Vars
		if len(scopeDevices) != 0 {
			vars, err = varsSetFromDevices(appMetadata, scopeDevices, template, verbose)
		} else if len(scopeFleets) != 0 {
			vars, err = varsSetFromFleets(appMetadata, scopeFleets, template, verbose)
		}
		if err != nil {
			return err
		}

		var varsJSON []byte
		if GetPretty() {
			varsJSON, err = note.JSONMarshalIndent(vars, "", "    ")
		} else {
			varsJSON, err = note.JSONMarshal(vars)
		}
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", varsJSON)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(varsCmd)
	varsCmd.AddCommand(varsGetCmd)
	varsCmd.AddCommand(varsSetCmd)

	// Flags for vars commands
	varsGetCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device/fleet scope (required)")
	varsSetCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device/fleet scope (required)")
}
