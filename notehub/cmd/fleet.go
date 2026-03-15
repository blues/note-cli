// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// fleetCmd represents the fleet command
var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Manage Notehub fleets",
	Long:  `Commands for listing and managing fleets in Notehub projects.`,
}

// fleetListCmd represents the fleet list command
var fleetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all fleets",
	Long:  `List all fleets in the current project or a specified project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list fleets: %w", err)
		}

		return printListResult(cmd, fleetsRsp, "No fleets found in this project.", func() bool {
			return len(fleetsRsp.Fleets) == 0
		})
	},
}

// fleetGetCmd represents the fleet get command
var fleetGetCmd = &cobra.Command{
	Use:   "get [fleet-uid-or-name]",
	Short: "Get details about a specific fleet",
	Long:  `Get detailed information about a specific fleet by UID or name. If no argument is provided, uses the active fleet (set with 'fleet set'). If no active fleet is configured, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var fleetIdentifier string
		if len(args) > 0 {
			fleetIdentifier = args[0]
		} else if def := GetFleet(); def != "" {
			fleetIdentifier = def
		} else {
			fleetIdentifier, err = pickFleet(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		selectedFleet, err := resolveFleet(client, ctx, projectUID, fleetIdentifier)
		if err != nil {
			return err
		}

		return printResult(cmd, selectedFleet)
	},
}

// fleetCreateCmd represents the fleet create command
var fleetCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new fleet",
	Long:  `Create a new fleet in the current project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		fleetName := args[0]

		// Get optional flags
		smartRule, _ := cmd.Flags().GetString("smart-rule")
		connectivityAssurance, _ := cmd.Flags().GetBool("connectivity-assurance")

		// Build create request using SDK
		createReq := notehub.NewCreateFleetRequest()
		createReq.SetLabel(fleetName)

		if smartRule != "" {
			createReq.SetSmartRule(smartRule)
		}

		if cmd.Flags().Changed("connectivity-assurance") {
			ca := notehub.NewFleetConnectivityAssurance()
			ca.Enabled.Set(&connectivityAssurance)
			createReq.SetConnectivityAssurance(*ca)
		}

		// Create fleet using SDK
		createdFleet, _, err := client.ProjectAPI.CreateFleet(ctx, projectUID).
			CreateFleetRequest(*createReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create fleet: %w", err)
		}

		return printMutationResult(cmd, createdFleet, "Fleet created")
	},
}

// fleetDeleteCmd represents the fleet delete command
var fleetDeleteCmd = &cobra.Command{
	Use:   "delete [fleet-uid-or-name]",
	Short: "Delete a fleet",
	Long:  `Delete a fleet from the current project. If no argument is provided, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var fleetIdentifier string
		if len(args) > 0 {
			fleetIdentifier = args[0]
		} else {
			fleetIdentifier, err = pickFleet(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		selectedFleet, err := resolveFleet(client, ctx, projectUID, fleetIdentifier)
		if err != nil {
			return err
		}

		if err := confirmAction(cmd, fmt.Sprintf("Delete fleet '%s'?", selectedFleet.Label)); err != nil {
			return nil
		}

		_, err = client.ProjectAPI.DeleteFleet(ctx, projectUID, selectedFleet.Uid).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete fleet: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":    "delete",
			"fleet_uid": selectedFleet.Uid,
			"fleet_name": selectedFleet.Label,
		}, fmt.Sprintf("Fleet '%s' deleted", selectedFleet.Label))
	},
}

// fleetUpdateCmd represents the fleet update command
var fleetUpdateCmd = &cobra.Command{
	Use:   "update [fleet-uid-or-name]",
	Short: "Update a fleet",
	Long:  `Update a fleet's properties such as name, smart rule, connectivity assurance, or watchdog timer. If no argument is provided, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var fleetIdentifier string
		if len(args) > 0 {
			fleetIdentifier = args[0]
		} else {
			fleetIdentifier, err = pickFleet(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		// Get optional flags
		newName, _ := cmd.Flags().GetString("name")
		smartRule, _ := cmd.Flags().GetString("smart-rule")
		connectivityAssurance, _ := cmd.Flags().GetBool("connectivity-assurance")
		watchdogMins, _ := cmd.Flags().GetInt("watchdog-mins")

		// Check if any update flags were provided
		if !cmd.Flags().Changed("name") &&
			!cmd.Flags().Changed("smart-rule") &&
			!cmd.Flags().Changed("connectivity-assurance") &&
			!cmd.Flags().Changed("watchdog-mins") {
			return fmt.Errorf("at least one update flag is required: --name, --smart-rule, --connectivity-assurance, or --watchdog-mins")
		}

		selectedFleet, err := resolveFleet(client, ctx, projectUID, fleetIdentifier)
		if err != nil {
			return err
		}

		// Build update request using SDK
		updateReq := notehub.NewUpdateFleetRequest()

		if cmd.Flags().Changed("name") {
			updateReq.SetLabel(newName)
		}

		if cmd.Flags().Changed("smart-rule") {
			updateReq.SetSmartRule(smartRule)
		}

		if cmd.Flags().Changed("connectivity-assurance") {
			ca := notehub.NewFleetConnectivityAssurance()
			ca.Enabled.Set(&connectivityAssurance)
			updateReq.SetConnectivityAssurance(*ca)
		}

		if cmd.Flags().Changed("watchdog-mins") {
			updateReq.SetWatchdogMins(int64(watchdogMins))
		}

		// Update fleet using SDK
		updatedFleet, _, err := client.ProjectAPI.UpdateFleet(ctx, projectUID, selectedFleet.Uid).
			UpdateFleetRequest(*updateReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to update fleet: %w", err)
		}

		return printMutationResult(cmd, updatedFleet, "Fleet updated")
	},
}

// fleetSetCmd represents the fleet set command
var fleetSetCmd = &cobra.Command{
	Use:   "set [fleet-uid-or-name]",
	Short: "Set the active fleet",
	Long: `Set the active fleet in the configuration. You can specify either the fleet name or UID.
If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var selectedFleet *notehub.Fleet
		if len(args) > 0 {
			selectedFleet, err = resolveFleet(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else {
			fleetUID, err := pickFleet(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
			selectedFleet, err = resolveFleet(client, ctx, projectUID, fleetUID)
			if err != nil {
				return err
			}
		}

		return setDefault(cmd, "fleet", selectedFleet.Uid, selectedFleet.Label)
	},
}

// fleetClearCmd represents the fleet clear command
var fleetClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active fleet",
	Long:  `Clear the active fleet from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearDefault(cmd, "fleet", "notehub fleet set <name-or-uid>")
	},
}

func init() {
	rootCmd.AddCommand(fleetCmd)
	fleetCmd.AddCommand(fleetListCmd)
	fleetCmd.AddCommand(fleetGetCmd)
	fleetCmd.AddCommand(fleetCreateCmd)
	fleetCmd.AddCommand(fleetDeleteCmd)
	fleetCmd.AddCommand(fleetUpdateCmd)
	fleetCmd.AddCommand(fleetSetCmd)
	fleetCmd.AddCommand(fleetClearCmd)

	// Add flags for fleet create
	fleetCreateCmd.Flags().String("smart-rule", "", "JSONata expression for dynamic fleet membership")
	fleetCreateCmd.Flags().Bool("connectivity-assurance", false, "Enable connectivity assurance for this fleet")

	// Add flags for fleet update
	fleetUpdateCmd.Flags().String("name", "", "New name for the fleet")
	fleetUpdateCmd.Flags().String("smart-rule", "", "JSONata expression for dynamic fleet membership")
	fleetUpdateCmd.Flags().Bool("connectivity-assurance", false, "Enable or disable connectivity assurance")
	fleetUpdateCmd.Flags().Int("watchdog-mins", 0, "Watchdog timer in minutes (0 to disable)")

	addConfirmFlag(fleetDeleteCmd)
}
