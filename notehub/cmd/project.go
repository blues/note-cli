// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
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
		// Get all projects using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		projectsRsp, _, err := client.ProjectAPI.GetProjects(ctx).Execute()
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, projectsRsp)
		}

		if len(projectsRsp.Projects) == 0 {
			cmd.Println("No projects found.")
			cmd.Println("\nYou can create a new project at https://notehub.io")
			return nil
		}
		return printHuman(cmd, projectsRsp)
	},
}

// projectSetCmd represents the project set command
var projectSetCmd = &cobra.Command{
	Use:   "set [project-name-or-uid]",
	Short: "Set the active project",
	Long: `Set the active project in the configuration. You can specify either the project name or UID.
If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no args, show interactive picker
		if len(args) == 0 {
			return interactiveProjectSelection(cmd)
		}

		projectIdentifier := args[0]

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// First, try to use it directly as a UID
		var selectedProject notehub.Project
		project, resp, err := client.ProjectAPI.GetProject(ctx, projectIdentifier).Execute()

		// If that failed, it might be a project name - fetch all projects and search
		if err != nil || (resp != nil && resp.StatusCode == 404) {
			projectsRsp, _, err := client.ProjectAPI.GetProjects(ctx).Execute()
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}

			// Search for project by name (exact match)
			found := false
			for _, proj := range projectsRsp.Projects {
				if proj.Label == projectIdentifier {
					selectedProject = proj
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("project '%s' not found. Use 'notehub project list' to see available projects", projectIdentifier)
			}
		} else {
			selectedProject = *project
		}

		// Save to config
		viper.Set("project", selectedProject.Uid)
		viper.Set("project_label", selectedProject.Label)
		if err := SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		cmd.Printf("Active project set to: %s\n", selectedProject.Label)
		cmd.Printf("Project UID: %s\n", selectedProject.Uid)
		cmd.Println("\nThis project will now be used as the default for all commands.")

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
		var projectUID string

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// If project specified, use that; otherwise use active project
		if len(args) > 0 {
			projectIdentifier := args[0]

			// First try to use it directly as a UID
			if len(projectIdentifier) > 4 && projectIdentifier[:4] == "app:" {
				projectUID = projectIdentifier
			} else {
				// It might be a project name - fetch all projects and search
				projectsRsp, _, err := client.ProjectAPI.GetProjects(ctx).Execute()
				if err != nil {
					return fmt.Errorf("failed to list projects: %w", err)
				}

				// Search for project by name (exact match)
				found := false
				for _, project := range projectsRsp.Projects {
					if project.Label == projectIdentifier {
						projectUID = project.Uid
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

		// Get project details using SDK
		project, _, err := client.ProjectAPI.GetProject(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		return printResult(cmd, project)
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
			cmd.Println("No project is currently set.")
			return nil
		}

		// Clear from config
		viper.Set("project", "")
		viper.Set("project_label", "")
		if err := SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		cmd.Println("Active project cleared.")
		cmd.Println("You can set a new project with 'notehub project set <name-or-uid>'")

		return nil
	},
}

// projectCreateCmd represents the project create command
var projectCreateCmd = &cobra.Command{
	Use:   "create [label] [billing-account-uid]",
	Short: "Create a new project",
	Long:  `Create a new Notehub project within a billing account.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := args[0]
		billingAccountUID := args[1]

		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		createReq := notehub.NewCreateProjectRequest(billingAccountUID, label)

		createdProject, _, err := client.ProjectAPI.CreateProject(ctx).
			CreateProjectRequest(*createReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		if wantJSON() {
			return printJSON(cmd, createdProject)
		}

		cmd.Println("Project created successfully!")
		return printHuman(cmd, createdProject)
	},
}

// projectDeleteCmd represents the project delete command
var projectDeleteCmd = &cobra.Command{
	Use:   "delete [project-uid]",
	Short: "Delete a project",
	Long:  `Delete a Notehub project. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectUID := args[0]

		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		_, err = client.ProjectAPI.DeleteProject(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete project: %w", err)
		}

		cmd.Printf("\nProject '%s' deleted successfully.\n\n", projectUID)

		return nil
	},
}

// projectCloneCmd represents the project clone command
var projectCloneCmd = &cobra.Command{
	Use:   "clone [source-project-uid] [new-label] [billing-account-uid]",
	Short: "Clone a project",
	Long: `Clone an existing Notehub project to create a new one.
By default, fleets and routes are cloned. Use flags to disable.`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceProjectUID := args[0]
		newLabel := args[1]
		billingAccountUID := args[2]

		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		cloneReq := notehub.NewCloneProjectRequest(billingAccountUID, newLabel)

		if noFleets, _ := cmd.Flags().GetBool("no-fleets"); noFleets {
			cloneReq.SetDisableCloneFleets(true)
		}
		if noRoutes, _ := cmd.Flags().GetBool("no-routes"); noRoutes {
			cloneReq.SetDisableCloneRoutes(true)
		}

		clonedProject, _, err := client.ProjectAPI.CloneProject(ctx, sourceProjectUID).
			CloneProjectRequest(*cloneReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to clone project: %w", err)
		}

		if wantJSON() {
			return printJSON(cmd, clonedProject)
		}

		cmd.Println("Project cloned successfully!")
		return printHuman(cmd, clonedProject)
	},
}

// projectMembersCmd represents the project members command
var projectMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List project members",
	Long: `List all members of the current project, showing their name, email, and role.

Examples:
  # List members of the active project
  notehub project members

  # List members with JSON output
  notehub project members --json

  # List members with pretty JSON
  notehub project members --pretty`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		membersRsp, _, err := client.ProjectAPI.GetProjectMembers(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get project members: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, membersRsp)
		}

		if len(membersRsp.Members) == 0 {
			cmd.Println("No members found for this project.")
			return nil
		}
		return printHuman(cmd, membersRsp)
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectSetCmd)
	projectCmd.AddCommand(projectClearCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectCloneCmd)
	projectCmd.AddCommand(projectMembersCmd)

	projectCloneCmd.Flags().Bool("no-fleets", false, "Do not clone fleets from the source project")
	projectCloneCmd.Flags().Bool("no-routes", false, "Do not clone routes from the source project")
}
