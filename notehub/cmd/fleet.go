// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"time"

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

		// Define fleet types
		type ConnectivityAssurance struct {
			Enabled bool `json:"enabled"`
		}

		type Fleet struct {
			UID                   string                 `json:"uid"`
			Label                 string                 `json:"label"`
			Created               time.Time              `json:"created,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          int                    `json:"watchdog_mins,omitempty"`
		}

		type FleetsResponse struct {
			Fleets []Fleet `json:"fleets"`
		}

		// Get fleets using V1 API: GET /v1/projects/{projectUID}/fleets
		fleetsRsp := FleetsResponse{}
		url := fmt.Sprintf("/v1/projects/%s/fleets", projectUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &fleetsRsp)
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
			fmt.Printf("  UID: %s\n", fleet.UID)
			if !fleet.Created.IsZero() {
				fmt.Printf("  Created: %s\n", fleet.Created.Format("2006-01-02 15:04:05 MST"))
			}
			if fleet.SmartRule != "" {
				fmt.Printf("  Smart Rule: %s\n", fleet.SmartRule)
			}
			if fleet.ConnectivityAssurance != nil {
				status := "disabled"
				if fleet.ConnectivityAssurance.Enabled {
					status = "enabled"
				}
				fmt.Printf("  Connectivity Assurance: %s\n", status)
			}
			if fleet.WatchdogMins > 0 {
				fmt.Printf("  Watchdog: %d minutes\n", fleet.WatchdogMins)
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

		// Define fleet types
		type ConnectivityAssurance struct {
			Enabled bool `json:"enabled"`
		}

		type Fleet struct {
			UID                   string                 `json:"uid"`
			Label                 string                 `json:"label"`
			Created               time.Time              `json:"created,omitempty"`
			EnvironmentVariables  map[string]string      `json:"environment_variables,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          int                    `json:"watchdog_mins,omitempty"`
		}

		type FleetsResponse struct {
			Fleets []Fleet `json:"fleets"`
		}

		// First, try to use it directly as a UID
		var selectedFleet Fleet
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s", projectUID, fleetIdentifier)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &selectedFleet)

		// If that failed or returned empty UID, it might be a fleet name - fetch all fleets and search
		if err != nil || selectedFleet.UID == "" {
			fleetsRsp := FleetsResponse{}
			listURL := fmt.Sprintf("/v1/projects/%s/fleets", projectUID)
			err = reqHubV1(GetVerbose(), GetAPIHub(), "GET", listURL, nil, &fleetsRsp)
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, fleet := range fleetsRsp.Fleets {
				if fleet.Label == fleetIdentifier {
					selectedFleet = fleet
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
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
		fmt.Printf("UID: %s\n", selectedFleet.UID)
		if !selectedFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", selectedFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if selectedFleet.SmartRule != "" {
			fmt.Printf("Smart Rule: %s\n", selectedFleet.SmartRule)
		}
		if selectedFleet.ConnectivityAssurance != nil {
			status := "disabled"
			if selectedFleet.ConnectivityAssurance.Enabled {
				status = "enabled"
			}
			fmt.Printf("Connectivity Assurance: %s\n", status)
		}
		if selectedFleet.WatchdogMins > 0 {
			fmt.Printf("Watchdog: %d minutes\n", selectedFleet.WatchdogMins)
		}

		// Display environment variables if any
		if len(selectedFleet.EnvironmentVariables) > 0 {
			fmt.Printf("\nEnvironment Variables:\n")
			for key, value := range selectedFleet.EnvironmentVariables {
				fmt.Printf("  %s: %s\n", key, value)
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

		// Define fleet types
		type ConnectivityAssurance struct {
			Enabled bool `json:"enabled"`
		}

		type CreateFleetRequest struct {
			Label                 string                 `json:"label"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
		}

		type Fleet struct {
			UID                   string                 `json:"uid"`
			Label                 string                 `json:"label"`
			Created               time.Time              `json:"created,omitempty"`
			EnvironmentVariables  map[string]string      `json:"environment_variables,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          int                    `json:"watchdog_mins,omitempty"`
		}

		// Build create request
		createReq := CreateFleetRequest{
			Label: fleetName,
		}

		if smartRule != "" {
			createReq.SmartRule = smartRule
		}

		if cmd.Flags().Changed("connectivity-assurance") {
			createReq.ConnectivityAssurance = &ConnectivityAssurance{
				Enabled: connectivityAssurance,
			}
		}

		// Marshal request to JSON
		reqBody, err := note.JSONMarshal(createReq)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Create fleet using V1 API: POST /v1/projects/{projectUID}/fleets
		createdFleet := Fleet{}
		url := fmt.Sprintf("/v1/projects/%s/fleets", projectUID)
		err = reqHubV1(GetVerbose(), GetAPIHub(), "POST", url, reqBody, &createdFleet)
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
		fmt.Printf("UID: %s\n", createdFleet.UID)
		if !createdFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", createdFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if createdFleet.SmartRule != "" {
			fmt.Printf("Smart Rule: %s\n", createdFleet.SmartRule)
		}
		if createdFleet.ConnectivityAssurance != nil {
			status := "disabled"
			if createdFleet.ConnectivityAssurance.Enabled {
				status = "enabled"
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

		// Define fleet types
		type ConnectivityAssurance struct {
			Enabled bool `json:"enabled"`
		}

		type Fleet struct {
			UID                   string                 `json:"uid"`
			Label                 string                 `json:"label"`
			Created               time.Time              `json:"created,omitempty"`
			EnvironmentVariables  map[string]string      `json:"environment_variables,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          int                    `json:"watchdog_mins,omitempty"`
		}

		type FleetsResponse struct {
			Fleets []Fleet `json:"fleets"`
		}

		// Determine the fleet UID
		var fleetUID string
		var fleetName string

		// First, try to use it directly as a UID
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s", projectUID, fleetIdentifier)
		var testFleet Fleet
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &testFleet)

		if err == nil && testFleet.UID != "" {
			// It's a valid UID
			fleetUID = testFleet.UID
			fleetName = testFleet.Label
		} else {
			// Try to find by name
			fleetsRsp := FleetsResponse{}
			listURL := fmt.Sprintf("/v1/projects/%s/fleets", projectUID)
			err = reqHubV1(GetVerbose(), GetAPIHub(), "GET", listURL, nil, &fleetsRsp)
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, fleet := range fleetsRsp.Fleets {
				if fleet.Label == fleetIdentifier {
					fleetUID = fleet.UID
					fleetName = fleet.Label
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
		}

		// Delete fleet using V1 API: DELETE /v1/projects/{projectUID}/fleets/{fleetUID}
		deleteURL := fmt.Sprintf("/v1/projects/%s/fleets/%s", projectUID, fleetUID)
		err = reqHubV1(GetVerbose(), GetAPIHub(), "DELETE", deleteURL, nil, nil)
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

		// Define fleet types
		type ConnectivityAssurance struct {
			Enabled bool `json:"enabled"`
		}

		type Fleet struct {
			UID                   string                 `json:"uid"`
			Label                 string                 `json:"label"`
			Created               time.Time              `json:"created,omitempty"`
			EnvironmentVariables  map[string]string      `json:"environment_variables,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          int                    `json:"watchdog_mins,omitempty"`
		}

		type FleetsResponse struct {
			Fleets []Fleet `json:"fleets"`
		}

		type UpdateFleetRequest struct {
			Label                 string                 `json:"label,omitempty"`
			SmartRule             string                 `json:"smart_rule,omitempty"`
			ConnectivityAssurance *ConnectivityAssurance `json:"connectivity_assurance,omitempty"`
			WatchdogMins          *int                   `json:"watchdog_mins,omitempty"`
		}

		// Determine the fleet UID
		var fleetUID string

		// First, try to use it directly as a UID
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s", projectUID, fleetIdentifier)
		var testFleet Fleet
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &testFleet)

		if err == nil && testFleet.UID != "" {
			// It's a valid UID
			fleetUID = testFleet.UID
		} else {
			// Try to find by name
			fleetsRsp := FleetsResponse{}
			listURL := fmt.Sprintf("/v1/projects/%s/fleets", projectUID)
			err = reqHubV1(GetVerbose(), GetAPIHub(), "GET", listURL, nil, &fleetsRsp)
			if err != nil {
				return fmt.Errorf("failed to list fleets: %w", err)
			}

			// Search for fleet by name (exact match)
			found := false
			for _, fleet := range fleetsRsp.Fleets {
				if fleet.Label == fleetIdentifier {
					fleetUID = fleet.UID
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("fleet '%s' not found in project", fleetIdentifier)
			}
		}

		// Build update request
		updateReq := UpdateFleetRequest{}

		if cmd.Flags().Changed("name") {
			updateReq.Label = newName
		}

		if cmd.Flags().Changed("smart-rule") {
			updateReq.SmartRule = smartRule
		}

		if cmd.Flags().Changed("connectivity-assurance") {
			updateReq.ConnectivityAssurance = &ConnectivityAssurance{
				Enabled: connectivityAssurance,
			}
		}

		if cmd.Flags().Changed("watchdog-mins") {
			updateReq.WatchdogMins = &watchdogMins
		}

		// Marshal request to JSON
		reqBody, err := note.JSONMarshal(updateReq)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Update fleet using V1 API: PUT /v1/projects/{projectUID}/fleets/{fleetUID}
		updatedFleet := Fleet{}
		updateURL := fmt.Sprintf("/v1/projects/%s/fleets/%s", projectUID, fleetUID)
		err = reqHubV1(GetVerbose(), GetAPIHub(), "PUT", updateURL, reqBody, &updatedFleet)
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
		fmt.Printf("UID: %s\n", updatedFleet.UID)
		if !updatedFleet.Created.IsZero() {
			fmt.Printf("Created: %s\n", updatedFleet.Created.Format("2006-01-02 15:04:05 MST"))
		}
		if updatedFleet.SmartRule != "" {
			fmt.Printf("Smart Rule: %s\n", updatedFleet.SmartRule)
		}
		if updatedFleet.ConnectivityAssurance != nil {
			status := "disabled"
			if updatedFleet.ConnectivityAssurance.Enabled {
				status = "enabled"
			}
			fmt.Printf("Connectivity Assurance: %s\n", status)
		}
		if updatedFleet.WatchdogMins > 0 {
			fmt.Printf("Watchdog: %d minutes\n", updatedFleet.WatchdogMins)
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
