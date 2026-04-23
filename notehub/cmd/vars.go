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

Scope:
  project            Project-level environment variables
  dev:xxxx           Single device
  fleet:xxxx         Single fleet (by UID)
  production         Single fleet (by name)
  @fleet-name        All devices in a fleet
  @                  All devices in the project
  @devices.txt       Devices from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)

If --scope is omitted, defaults to the active project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAuth(); err != nil {
			return err
		}

		if flagScope == "" {
			flagScope = "project"
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
  notehub vars set --scope @production @vars.json

If --scope is omitted, defaults to the active project.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAuth(); err != nil {
			return err
		}

		if flagScope == "" {
			flagScope = "project"
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

// varsDeleteCmd represents the vars delete command
var varsDeleteCmd = &cobra.Command{
	Use:   "delete [key...]",
	Short: "Delete environment variables",
	Long: `Delete one or more environment variables from devices, fleets, or the project.

Scope:
  project            Project-level environment variables
  dev:xxxx           Single device
  fleet:xxxx         Single fleet (by UID)
  production         Single fleet (by name)

Examples:
  notehub vars delete --scope dev:xxxx VAR1 VAR2
  notehub vars delete --scope project VAR1
  notehub vars delete --scope @production VAR1

If --scope is omitted, defaults to the active project.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateAuth(); err != nil {
			return err
		}

		if flagScope == "" {
			flagScope = "project"
		}

		keys := args

		if flagScope == "project" {
			client, ctx, projectUID, err := initCommand()
			if err != nil {
				return err
			}

			for _, key := range keys {
				_, _, err := client.ProjectAPI.DeleteProjectEnvironmentVariable(ctx, projectUID, key).Execute()
				if err != nil {
					return fmt.Errorf("failed to delete project variable '%s': %w", key, err)
				}
			}

			return printActionResult(cmd, map[string]any{
				"action": "delete",
				"scope":  "project",
				"keys":   keys,
			}, fmt.Sprintf("Deleted %d variable(s) from project", len(keys)))
		}

		appMetadata, scopeDevices, scopeFleets, err := ResolveScopeWithValidation(flagScope)
		if err != nil {
			return err
		}

		if len(scopeDevices) != 0 && len(scopeFleets) != 0 {
			return fmt.Errorf("scope may include devices or fleets but not both")
		}

		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		if len(scopeDevices) != 0 {
			for _, deviceUID := range scopeDevices {
				for _, key := range keys {
					_, _, err := client.DeviceAPI.DeleteDeviceEnvironmentVariable(ctx, appMetadata.App.UID, deviceUID, key).Execute()
					if err != nil {
						return fmt.Errorf("failed to delete variable '%s' from device %s: %w", key, deviceUID, err)
					}
				}
			}
		} else if len(scopeFleets) != 0 {
			for _, fleetUID := range scopeFleets {
				for _, key := range keys {
					_, _, err := client.ProjectAPI.DeleteFleetEnvironmentVariable(ctx, appMetadata.App.UID, fleetUID, key).Execute()
					if err != nil {
						return fmt.Errorf("failed to delete variable '%s' from fleet %s: %w", key, fleetUID, err)
					}
				}
			}
		}

		return printActionResult(cmd, map[string]any{
			"action": "delete",
			"scope":  flagScope,
			"keys":   keys,
		}, fmt.Sprintf("Deleted %d variable(s) from %s", len(keys), flagScope))
	},
}

func init() {
	rootCmd.AddCommand(varsCmd)
	varsCmd.AddCommand(varsGetCmd)
	varsCmd.AddCommand(varsSetCmd)
	varsCmd.AddCommand(varsDeleteCmd)

	// Flags for vars commands
	addScopeFlag(varsGetCmd, "Device/fleet/project scope (defaults to project)")
	addScopeFlag(varsSetCmd, "Device/fleet/project scope (defaults to project)")
	addScopeFlag(varsDeleteCmd, "Device/fleet/project scope (defaults to project)")
}
