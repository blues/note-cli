// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/blues/note-go/note"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Notehub projects",
	Long:  `Commands for listing and selecting Notehub projects to work with.`,
}

// projectListCmd represents the project list command
var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long:  `List all Notehub projects for the authenticated user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		credentials := GetCredentials() // Validates and exits if not authenticated

		// Get all projects using V1 API: GET /v1/projects
		type Project struct {
			UID                string `json:"uid"`
			Label              string `json:"label"`
			BillingAccountUID  string `json:"billing_account_uid"`
			DisableDevicesByDefault bool `json:"disable_devices_by_default"`
		}

		type ProjectsResponse struct {
			Projects []Project `json:"projects"`
		}

		projectsRsp := ProjectsResponse{}
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", "/v1/projects", nil, &projectsRsp)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(projectsRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(projectsRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(projectsRsp.Projects) == 0 {
			fmt.Println("No projects found.")
			fmt.Println("\nYou can create a new project at https://notehub.io")
			return nil
		}

		// Check current project
		currentProject := GetProject()

		// Display projects in human-readable format
		fmt.Println("\nAvailable Projects:")
		fmt.Println("===================")
		for _, project := range projectsRsp.Projects {
			if project.UID == currentProject {
				fmt.Printf("* %s (current)\n", project.Label)
				fmt.Printf("  %s\n\n", project.UID)
			} else {
				fmt.Printf("  %s\n", project.Label)
				fmt.Printf("  %s\n\n", project.UID)
			}
		}

		if currentProject == "" {
			fmt.Println("No project selected. Use 'notehub project set <name-or-uid>' to select one.\n")
		}

		// Show credentials user
		fmt.Printf("Signed in as: %s\n\n", credentials.User)

		return nil
	},
}

// projectSetCmd represents the project set command
var projectSetCmd = &cobra.Command{
	Use:   "set [project-name-or-uid]",
	Short: "Set the active project",
	Long:  `Set the active project in the configuration. You can specify either the project name or UID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		projectIdentifier := args[0]

		// Define project types
		type Project struct {
			UID   string `json:"uid"`
			Label string `json:"label"`
		}

		type ProjectsResponse struct {
			Projects []Project `json:"projects"`
		}

		// First, try to use it directly as a UID
		var selectedProject Project
		url := fmt.Sprintf("/v1/projects/%s", projectIdentifier)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &selectedProject)

		// If that failed or returned empty UID, it might be a project name - fetch all projects and search
		if err != nil || selectedProject.UID == "" {
			projectsRsp := ProjectsResponse{}
			err = reqHubV1(GetVerbose(), GetAPIHub(), "GET", "/v1/projects", nil, &projectsRsp)
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}

			// Search for project by name (exact match)
			found := false
			for _, project := range projectsRsp.Projects {
				if project.Label == projectIdentifier {
					selectedProject = project
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("project '%s' not found. Use 'notehub project list' to see available projects", projectIdentifier)
			}
		}

		// Save to config
		viper.Set("project", selectedProject.UID)
		if err := SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Active project set to: %s\n", selectedProject.Label)
		fmt.Printf("Project UID: %s\n", selectedProject.UID)
		fmt.Println("\nThis project will now be used as the default for all commands.")

		return nil
	},
}

// projectGetCmd represents the project get command
var projectGetCmd = &cobra.Command{
	Use:   "get [project-name-or-uid]",
	Short: "Get detailed information about a project",
	Long: `Get detailed information about a specific project. If no project is specified, uses the active project.

Examples:
  # Get information about active project
  notehub project get

  # Get information about specific project by UID
  notehub project get app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Get information about specific project by name
  notehub project get "My Project"

  # Get with JSON output
  notehub project get --json

  # Get with pretty JSON
  notehub project get --pretty`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		var projectUID string

		// If project specified, use that; otherwise use active project
		if len(args) > 0 {
			projectIdentifier := args[0]

			// First try to use it directly as a UID
			if len(projectIdentifier) > 4 && projectIdentifier[:4] == "app:" {
				projectUID = projectIdentifier
			} else {
				// It might be a project name - fetch all projects and search
				type Project struct {
					UID   string `json:"uid"`
					Label string `json:"label"`
				}

				type ProjectsResponse struct {
					Projects []Project `json:"projects"`
				}

				projectsRsp := ProjectsResponse{}
				err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", "/v1/projects", nil, &projectsRsp)
				if err != nil {
					return fmt.Errorf("failed to list projects: %w", err)
				}

				// Search for project by name (exact match)
				found := false
				for _, project := range projectsRsp.Projects {
					if project.Label == projectIdentifier {
						projectUID = project.UID
						found = true
						break
					}
				}

				if !found {
					return fmt.Errorf("project '%s' not found. Use 'notehub project list' to see available projects", projectIdentifier)
				}
			}
		} else {
			// Use active project
			projectUID = GetProject()
			if projectUID == "" {
				return fmt.Errorf("no project specified and no active project set. Use 'notehub project set <name-or-uid>' to set an active project")
			}
		}

		verbose := GetVerbose()

		// Get project details using V1 API: GET /v1/projects/{projectUID}
		var project map[string]interface{}
		url := fmt.Sprintf("/v1/projects/%s", projectUID)
		err := reqHubV1(verbose, GetAPIHub(), "GET", url, nil, &project)
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(project, "", "  ")
			} else {
				output, err = note.JSONMarshal(project)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display project in human-readable format
		uid, _ := project["uid"].(string)
		label, _ := project["label"].(string)
		billingAccountUID, _ := project["billing_account_uid"].(string)
		disableDevicesByDefault, _ := project["disable_devices_by_default"].(bool)

		// Check if this is the active project
		currentProject := GetProject()
		isActive := (uid == currentProject)

		fmt.Printf("\nProject Details:\n")
		fmt.Printf("================\n\n")
		fmt.Printf("Name: %s", label)
		if isActive {
			fmt.Printf(" (active)")
		}
		fmt.Println()
		fmt.Printf("UID: %s\n", uid)
		if billingAccountUID != "" {
			fmt.Printf("Billing Account: %s\n", billingAccountUID)
		}
		if disableDevicesByDefault {
			fmt.Printf("Disable Devices by Default: Yes\n")
		} else {
			fmt.Printf("Disable Devices by Default: No\n")
		}
		fmt.Println()

		return nil
	},
}

// projectClearCmd represents the project clear command
var projectClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active project",
	Long:  `Clear the active project from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject := GetProject()
		if currentProject == "" {
			fmt.Println("No project is currently set.")
			return nil
		}

		// Clear from config
		viper.Set("project", "")
		if err := SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("Active project cleared.")
		fmt.Println("You can set a new project with 'notehub project set <name-or-uid>'")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectSetCmd)
	projectCmd.AddCommand(projectClearCmd)
}
