// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
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
	Long: `Get environment variables for devices, fleets, or the project.

Scope can be:
  - "project" to get project-level environment variables
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
			return fmt.Errorf("use --scope to specify device(s), fleet(s), or project")
		}

		if flagScope == "project" {
			client, ctx, projectUID, err := initCommand()
			if err != nil {
				return err
			}

			varsResp, _, err := client.ProjectAPI.GetProjectEnvironmentVariables(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to get project environment variables: %w", err)
			}

			vars := map[string]Vars{projectUID: varsResp.EnvironmentVariables}
			return printJSON(cmd, vars)
		}

		appMetadata, scopeDevices, scopeFleets, err := ResolveScopeWithValidation(flagScope)
		if err != nil {
			return err
		}

		if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
			return fmt.Errorf("scope may include devices or fleets but not both")
		}

		verbose := GetVerbose()
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

		cmd.Printf("%s\n", varsJSON)
		return nil
	},
}

// varsSetCmd represents the vars set command
var varsSetCmd = &cobra.Command{
	Use:   "set [json]",
	Short: "Set environment variables",
	Long: `Set environment variables for devices, fleets, or the project.

The JSON argument can be a JSON object or @filename to read from a file.

Example:
  notehub vars set --scope dev:xxxx '{"VAR1":"value1","VAR2":"value2"}'
  notehub vars set --scope project '{"VAR1":"value1","VAR2":"value2"}'
  notehub vars set --scope @production @vars.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		if flagScope == "" {
			return fmt.Errorf("use --scope to specify device(s), fleet(s), or project")
		}

		if flagScope == "project" {
			client, ctx, projectUID, err := initCommand()
			if err != nil {
				return err
			}

			// Parse template
			template := Vars{}
			varsArg := args[0]
			if strings.HasPrefix(varsArg, "@") {
				templateJSON, err := os.ReadFile(strings.TrimPrefix(varsArg, "@"))
				if err != nil {
					return err
				}
				if err := note.JSONUnmarshal(templateJSON, &template); err != nil {
					return err
				}
			} else {
				if err := note.JSONUnmarshal([]byte(varsArg), &template); err != nil {
					return err
				}
			}

			envVars := notehub.NewEnvironmentVariables(template)
			varsResp, _, err := client.ProjectAPI.SetProjectEnvironmentVariables(ctx, projectUID).
				EnvironmentVariables(*envVars).
				Execute()
			if err != nil {
				return fmt.Errorf("failed to set project environment variables: %w", err)
			}

			vars := map[string]Vars{projectUID: varsResp.EnvironmentVariables}
			return printJSON(cmd, vars)
		}

		appMetadata, scopeDevices, scopeFleets, err := ResolveScopeWithValidation(flagScope)
		if err != nil {
			return err
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

		verbose := GetVerbose()
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

		cmd.Printf("%s\n", varsJSON)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(varsCmd)
	varsCmd.AddCommand(varsGetCmd)
	varsCmd.AddCommand(varsSetCmd)

	// Flags for vars commands
	varsGetCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device/fleet/project scope (required)")
	varsSetCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device/fleet/project scope (required)")
}
