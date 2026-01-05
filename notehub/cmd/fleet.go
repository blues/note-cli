// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
	"github.com/blues/note-go/note"
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
		GetCredentials() // Validates and exits if not authenticated

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get fleets using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list fleets: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(fleetsRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(fleetsRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(fleetsRsp.Fleets) == 0 {
			fmt.Println("No fleets found in this project.")
			return nil
		}

		// Display fleets in human-readable format
		fmt.Printf("\nFleets in Project:\n")
		fmt.Printf("==================\n\n")

		for _, fleet := range fleetsRsp.Fleets {
			fmt.Printf("Fleet: %s\n", fleet.Label)
			fmt.Printf("  UID: %s\n", fleet.Uid)
			if !fleet.Created.IsZero() {
				fmt.Printf("  Created: %s\n", fleet.Created.Format("2006-01-02 15:04:05 MST"))
			}
			if fleet.HasSmartRule() {
				fmt.Printf("  Smart Rule: %s\n", *fleet.SmartRule)
			}
			if fleet.HasConnectivityAssurance() {
				status := "disabled"
				if ca := fleet.ConnectivityAssurance.Get(); ca != nil && ca.Enabled.IsSet() {
					if enabled := ca.Enabled.Get(); enabled != nil && *enabled {
						status = "enabled"
					}
				}
				fmt.Printf("  Connectivity Assurance: %s\n", status)
			}
			if fleet.HasWatchdogMins() && *fleet.WatchdogMins > 0 {
				fmt.Printf("  Watchdog: %d minutes\n", *fleet.WatchdogMins)
			}
			fmt.Println()
		}

		fmt.Printf("Total fleets: %d\n\n", len(fleetsRsp.Fleets))

		return nil
	},
}

// fleetGetCmd represents the fleet get command
var fleetGetCmd = &cobra.Command{
	Use:   "get [fleet-uid-or-name]",
	Short: "Get details about a specific fleet",
	Long:  `Get detailed information about a specific fleet by UID or name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		fleetIdentifier := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// First, try to use it directly as a UID
		var selectedFleet *notehub.Fleet
		fleet, resp, err := client.ProjectAPI.GetFleet(ctx, projectUID, fleetIdentifier).Execute()

		// If that failed or returned 404, it might be a fleet name - fetch all fleets and search
		if err != nil || (resp != nil && resp.StatusCode == 404) {
			fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, f := range fleetsRsp.Fleets {
				if f.Label == fleetIdentifier {
					selectedFleet = &f
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
		} else {
			selectedFleet = fleet
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(selectedFleet, "", "  ")
			} else {
				output, err = note.JSONMarshal(selectedFleet)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display fleet in human-readable format
		fmt.Printf("\nFleet Details:\n")
		fmt.Printf("==============\n\n")
		fmt.Printf("Name: %s\n", selectedFleet.Label)
		fmt.Printf("UID: %s\n", selectedFleet.Uid)
		if !selectedFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", selectedFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if selectedFleet.HasSmartRule() {
			fmt.Printf("Smart Rule: %s\n", *selectedFleet.SmartRule)
		}
		if selectedFleet.HasConnectivityAssurance() {
			status := "disabled"
			if ca := selectedFleet.ConnectivityAssurance.Get(); ca != nil && ca.Enabled.IsSet() {
				if enabled := ca.Enabled.Get(); enabled != nil && *enabled {
					status = "enabled"
				}
			}
			fmt.Printf("Connectivity Assurance: %s\n", status)
		}
		if selectedFleet.HasWatchdogMins() && *selectedFleet.WatchdogMins > 0 {
			fmt.Printf("Watchdog: %d minutes\n", *selectedFleet.WatchdogMins)
		}

		// Display environment variables if any
		if selectedFleet.HasEnvironmentVariables() {
			envVars := selectedFleet.GetEnvironmentVariables()
			if len(envVars) > 0 {
				fmt.Printf("\nEnvironment Variables:\n")
				for key, value := range envVars {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}
		}

		fmt.Println()

		return nil
	},
}

// fleetCreateCmd represents the fleet create command
var fleetCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new fleet",
	Long:  `Create a new fleet in the current project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		fleetName := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get optional flags
		smartRule, _ := cmd.Flags().GetString("smart-rule")
		connectivityAssurance, _ := cmd.Flags().GetBool("connectivity-assurance")

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

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

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(createdFleet, "", "  ")
			} else {
				output, err = note.JSONMarshal(createdFleet)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display success message
		fmt.Printf("\nFleet created successfully!\n\n")
		fmt.Printf("Name: %s\n", createdFleet.Label)
		fmt.Printf("UID: %s\n", createdFleet.Uid)
		if !createdFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", createdFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if createdFleet.HasSmartRule() {
			fmt.Printf("Smart Rule: %s\n", *createdFleet.SmartRule)
		}
		if createdFleet.HasConnectivityAssurance() {
			status := "disabled"
			if ca := createdFleet.ConnectivityAssurance.Get(); ca != nil && ca.Enabled.IsSet() {
				if enabled := ca.Enabled.Get(); enabled != nil && *enabled {
					status = "enabled"
				}
			}
			fmt.Printf("Connectivity Assurance: %s\n", status)
		}
		fmt.Println()

		return nil
	},
}

// fleetDeleteCmd represents the fleet delete command
var fleetDeleteCmd = &cobra.Command{
	Use:   "delete [fleet-uid-or-name]",
	Short: "Delete a fleet",
	Long:  `Delete a fleet from the current project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		fleetIdentifier := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Determine the fleet UID
		var fleetUID string
		var fleetName string

		// First, try to use it directly as a UID
		fleet, resp, err := client.ProjectAPI.GetFleet(ctx, projectUID, fleetIdentifier).Execute()

		if err == nil && resp != nil && resp.StatusCode != 404 {
			// It's a valid UID
			fleetUID = fleet.Uid
			fleetName = fleet.Label
		} else {
			// Try to find by name
			fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, f := range fleetsRsp.Fleets {
				if f.Label == fleetIdentifier {
					fleetUID = f.Uid
					fleetName = f.Label
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
		}

		// Delete fleet using SDK
		_, err = client.ProjectAPI.DeleteFleet(ctx, projectUID, fleetUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete fleet: %w", err)
		}

		fmt.Printf("\nFleet '%s' (UID: %s) deleted successfully.\n\n", fleetName, fleetUID)

		return nil
	},
}

// fleetUpdateCmd represents the fleet update command
var fleetUpdateCmd = &cobra.Command{
	Use:   "update [fleet-uid-or-name]",
	Short: "Update a fleet",
	Long:  `Update a fleet's properties such as name, smart rule, connectivity assurance, or watchdog timer.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		fleetIdentifier := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
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
			return fmt.Errorf("no update flags provided. Use --name, --smart-rule, --connectivity-assurance, or --watchdog-mins")
		}

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Determine the fleet UID
		var fleetUID string

		// First, try to use it directly as a UID
		fleet, resp, err := client.ProjectAPI.GetFleet(ctx, projectUID, fleetIdentifier).Execute()

		if err == nil && resp != nil && resp.StatusCode != 404 {
			// It's a valid UID
			fleetUID = fleet.Uid
		} else {
			// Try to find by name
			fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, f := range fleetsRsp.Fleets {
				if f.Label == fleetIdentifier {
					fleetUID = f.Uid
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
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
		updatedFleet, _, err := client.ProjectAPI.UpdateFleet(ctx, projectUID, fleetUID).
			UpdateFleetRequest(*updateReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to update fleet: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(updatedFleet, "", "  ")
			} else {
				output, err = note.JSONMarshal(updatedFleet)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display success message
		fmt.Printf("\nFleet updated successfully!\n\n")
		fmt.Printf("Name: %s\n", updatedFleet.Label)
		fmt.Printf("UID: %s\n", updatedFleet.Uid)
		if !updatedFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", updatedFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if updatedFleet.HasSmartRule() {
			fmt.Printf("Smart Rule: %s\n", *updatedFleet.SmartRule)
		}
		if updatedFleet.HasConnectivityAssurance() {
			status := "disabled"
			if ca := updatedFleet.ConnectivityAssurance.Get(); ca != nil && ca.Enabled.IsSet() {
				if enabled := ca.Enabled.Get(); enabled != nil && *enabled {
					status = "enabled"
				}
			}
			fmt.Printf("Connectivity Assurance: %s\n", status)
		}
		if updatedFleet.HasWatchdogMins() && *updatedFleet.WatchdogMins > 0 {
			fmt.Printf("Watchdog: %d minutes\n", *updatedFleet.WatchdogMins)
		}
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fleetCmd)
	fleetCmd.AddCommand(fleetListCmd)
	fleetCmd.AddCommand(fleetGetCmd)
	fleetCmd.AddCommand(fleetCreateCmd)
	fleetCmd.AddCommand(fleetDeleteCmd)
	fleetCmd.AddCommand(fleetUpdateCmd)

	// Add flags for fleet create
	fleetCreateCmd.Flags().String("smart-rule", "", "JSONata expression for dynamic fleet membership")
	fleetCreateCmd.Flags().Bool("connectivity-assurance", false, "Enable connectivity assurance for this fleet")

	// Add flags for fleet update
	fleetUpdateCmd.Flags().String("name", "", "New name for the fleet")
	fleetUpdateCmd.Flags().String("smart-rule", "", "JSONata expression for dynamic fleet membership")
	fleetUpdateCmd.Flags().Bool("connectivity-assurance", false, "Enable or disable connectivity assurance")
	fleetUpdateCmd.Flags().Int("watchdog-mins", 0, "Watchdog timer in minutes (0 to disable)")
}
