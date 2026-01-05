// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blues/note-go/notehub"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Auth command flags
var (
	flagSetProject string
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Commands for signing in, signing out, and managing authentication tokens.`,
}

// signinCmd represents the signin command
var signinCmd = &cobra.Command{
	Use:   "signin",
	Short: "Sign in to Notehub",
	Long:  `Sign in to Notehub using browser-based OAuth2 flow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		credentials, err := GetHubCredentials()
		if err != nil {
			return err
		}

		// if signed in with an access token via OAuth, then revoke the access token
		// we don't want to revoke a PAT because the user explicitly set an
		// expiration date on that token
		if credentials != nil && credentials.IsOAuthAccessToken() {
			if err := RemoveHubCredentials(); err != nil {
				return err
			}
		}

		// initiate the browser-based OAuth2 login flow
		hub := GetHub()
		accessToken, err := notehub.InitiateBrowserBasedLogin(hub)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// save the credentials
		if err := SetHubCredentials(accessToken.AccessToken, accessToken.Email, &accessToken.ExpiresAt); err != nil {
			return err
		}

		// print out information about the session
		if accessToken != nil {
			fmt.Printf("%s\n", banner())
			fmt.Printf("signed in as %s\n", accessToken.Email)
			fmt.Printf("token expires at %s\n", accessToken.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		}

		// Set project if provided via flag or prompt for selection
		if err := handleProjectSelection(flagSetProject); err != nil {
			return err
		}

		return nil
	},
}

// signinTokenCmd represents the signin-token command
var signinTokenCmd = &cobra.Command{
	Use:   "signin-token [token]",
	Short: "Sign in with a personal access token",
	Long:  `Sign in to Notehub using a personal access token.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		personalAccessToken := args[0]

		hub := GetHub()
		// Print hub if not the default
		fmt.Printf("notehub: %s\n", hub)

		email, err := IntrospectToken(hub, personalAccessToken)
		if err != nil {
			return err
		}

		if err := SetHubCredentials(personalAccessToken, email, nil); err != nil {
			return err
		}

		// Done
		fmt.Printf("signed in successfully with token\n")

		// Set project if provided via flag or prompt for selection
		if err := handleProjectSelection(flagSetProject); err != nil {
			return err
		}

		return nil
	},
}

// signoutCmd represents the signout command
var signoutCmd = &cobra.Command{
	Use:   "signout",
	Short: "Sign out of Notehub",
	Long:  `Sign out of Notehub and remove stored credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := RemoveHubCredentials(); err != nil {
			return err
		}

		// Also clear project setting
		viper.Set("project", "")
		if err := SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("signed out successfully\n")
		return nil
	},
}

// tokenCmd represents the token command
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Display the current authentication token",
	Long:  `Display the current authentication token for the signed-in account.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		credentials, err := GetHubCredentials()
		if err != nil {
			return err
		}

		if credentials == nil {
			return fmt.Errorf("please sign in using 'notehub auth signin' or 'notehub auth signin-token'")
		}

		fmt.Printf("%s\n", credentials.Token)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(signinCmd)
	authCmd.AddCommand(signinTokenCmd)
	authCmd.AddCommand(signoutCmd)
	authCmd.AddCommand(tokenCmd)

	// Add --set-project flag to signin commands
	signinCmd.Flags().StringVar(&flagSetProject, "set-project", "", "Automatically set project after signin (name or UID)")
	signinTokenCmd.Flags().StringVar(&flagSetProject, "set-project", "", "Automatically set project after signin (name or UID)")
}

// Banner for authentication
// http://patorjk.com/software/taag
// "Big" font
func banner() (s string) {
	s += "             _       _           _       \r\n"
	s += "            | |     | |         | |      \r\n"
	s += " _ __   ___ | |_ ___| |__  _   _| |__    \r\n"
	s += "| '_ \\ / _ \\| __/ _ \\ '_ \\| | | | '_ \\   \r\n"
	s += "| | | | (_) | ||  __/ | | | |_| | |_) |  \r\n"
	s += "|_| |_|\\___/ \\__\\___|_| |_|\\__,_|_.__/   \r\n"
	s += "\r\n"
	return
}

// handleProjectSelection handles project selection after signin via flag or interactive prompt
func handleProjectSelection(projectFlag string) error {
	// Check if a project is already set
	currentProject := GetProject()
	if currentProject != "" {
		// Project already configured, no need to prompt
		return nil
	}

	// If project flag was provided, set it directly
	if projectFlag != "" {
		return setProjectByIdentifier(projectFlag)
	}

	// Otherwise, offer interactive selection
	return interactiveProjectSelection()
}

// setProjectByIdentifier sets a project by name or UID (from project.go logic)
func setProjectByIdentifier(identifier string) error {
	// Get SDK client
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return err
	}

	// First, try to use it directly as a UID
	project, resp, err := client.ProjectAPI.GetProject(ctx, identifier).Execute()

	// If that failed, it might be a project name - fetch all projects and search
	if err != nil || (resp != nil && resp.StatusCode == 404) {
		projectsRsp, _, err := client.ProjectAPI.GetProjects(ctx).Execute()
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		// Search for project by name (exact match)
		found := false
		for _, proj := range projectsRsp.Projects {
			if proj.Label == identifier {
				project = &proj
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("project '%s' not found", identifier)
		}
	}

	// Save to config
	viper.Set("project", project.Uid)
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nActive project set to: %s\n", project.Label)
	fmt.Printf("Project UID: %s\n\n", project.Uid)

	return nil
}

// interactiveProjectSelection prompts the user to select a project interactively
func interactiveProjectSelection() error {
	// Get SDK client
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return err
	}

	// Fetch all projects
	projectsRsp, _, err := client.ProjectAPI.GetProjects(ctx).Execute()
	if err != nil {
		// If we can't fetch projects, just show instructions
		fmt.Println()
		fmt.Println("To get started, you'll need to select a project to work with.")
		fmt.Println("Run 'notehub project list' to see your available projects,")
		fmt.Println("then 'notehub project set <name-or-uid>' to select one.")
		fmt.Println()
		return nil
	}

	if len(projectsRsp.Projects) == 0 {
		fmt.Println()
		fmt.Println("No projects found. You can create a new project at https://notehub.io")
		fmt.Println()
		return nil
	}

	// Display projects with numbers
	fmt.Println()
	fmt.Println("Select a project to work with:")
	fmt.Println()
	for i, project := range projectsRsp.Projects {
		fmt.Printf("  %d) %s\n", i+1, project.Label)
	}
	fmt.Println()
	fmt.Printf("Enter project number (1-%d), or press Enter to skip: ", len(projectsRsp.Projects))

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil // Skip on error
	}

	input = strings.TrimSpace(input)
	if input == "" {
		// User pressed Enter, skip selection
		fmt.Println()
		fmt.Println("Skipped project selection. You can set a project later with 'notehub project set <name-or-uid>'")
		fmt.Println()
		return nil
	}

	// Parse selection
	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(projectsRsp.Projects) {
		fmt.Println()
		fmt.Printf("Invalid selection. You can set a project later with 'notehub project set <name-or-uid>'\n")
		fmt.Println()
		return nil
	}

	// Set the selected project
	selectedProject := projectsRsp.Projects[selection-1]
	viper.Set("project", selectedProject.Uid)
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("Active project set to: %s\n", selectedProject.Label)
	fmt.Printf("Project UID: %s\n\n", selectedProject.Uid)

	return nil
}
