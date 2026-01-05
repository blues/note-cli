// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// routeCmd represents the route command
var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage routes",
	Long:  `Commands for creating, updating, deleting, and viewing routes in Notehub projects.`,
}

// routeListCmd represents the route list command
var routeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all routes",
	Long: `List all routes in the current or specified project.

Examples:
  # List all routes
  notehub route list

  # List routes in specific project
  notehub route list --project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # List with JSON output
  notehub route list --json

  # List with pretty JSON
  notehub route list --pretty`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get routes using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list routes: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(routes, "", "  ")
			} else {
				output, err = note.JSONMarshal(routes)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(routes) == 0 {
			fmt.Println("No routes found.")
			return nil
		}

		// Display routes in human-readable format
		fmt.Printf("\nRoutes (%d):\n", len(routes))
		fmt.Printf("============\n\n")

		for _, route := range routes {
			uid := ""
			if route.Uid != nil {
				uid = *route.Uid
			}
			label := ""
			if route.Label != nil {
				label = *route.Label
			}
			routeType := ""
			if route.Type != nil {
				routeType = *route.Type
			}
			modified := ""
			if route.Modified != nil {
				modified = route.Modified.Format("2006-01-02 15:04:05 MST")
			}
			disabled := false
			if route.Disabled != nil {
				disabled = *route.Disabled
			}

			fmt.Printf("UID: %s\n", uid)
			fmt.Printf("  Label: %s\n", label)
			fmt.Printf("  Type: %s\n", routeType)
			fmt.Printf("  Modified: %s\n", modified)
			if disabled {
				fmt.Printf("  Status: Disabled\n")
			} else {
				fmt.Printf("  Status: Enabled\n")
			}
			fmt.Println()
		}

		return nil
	},
}

// routeGetCmd represents the route get command
var routeGetCmd = &cobra.Command{
	Use:   "get [route-uid-or-name]",
	Short: "Get detailed information about a specific route",
	Long: `Get detailed information about a specific route by UID or name.

Examples:
  # Get route by UID
  notehub route get route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Get route by name
  notehub route get "My Route"

  # Get with pretty JSON
  notehub route get "My Route" --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		routeIdentifier := args[0]

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

		// Determine route UID (handle both UID and name)
		var routeUID string
		var selectedRoute *notehub.NotehubRoute

		// First try as UID
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			route, resp, err := client.RouteAPI.GetRoute(ctx, projectUID, routeIdentifier).Execute()
			if err == nil && resp != nil && resp.StatusCode != 404 {
				routeUID = routeIdentifier
				selectedRoute = route
			}
		}

		// If not found as UID, search by name
		if selectedRoute == nil {
			routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			for _, route := range routes {
				if route.Label != nil && *route.Label == routeIdentifier {
					if route.Uid != nil {
						routeUID = *route.Uid
						// Get full route details
						fullRoute, _, err := client.RouteAPI.GetRoute(ctx, projectUID, routeUID).Execute()
						if err != nil {
							return fmt.Errorf("failed to get route: %w", err)
						}
						selectedRoute = fullRoute
						break
					}
				}
			}
		}

		if selectedRoute == nil {
			return fmt.Errorf("route '%s' not found", routeIdentifier)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(selectedRoute, "", "  ")
			} else {
				output, err = note.JSONMarshal(selectedRoute)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display route in human-readable format
		uid := ""
		if selectedRoute.Uid != nil {
			uid = *selectedRoute.Uid
		}
		label := ""
		if selectedRoute.Label != nil {
			label = *selectedRoute.Label
		}
		routeType := ""
		if selectedRoute.Type != nil {
			routeType = *selectedRoute.Type
		}
		modified := ""
		if selectedRoute.Modified != nil {
			modified = selectedRoute.Modified.Format("2006-01-02 15:04:05 MST")
		}
		disabled := false
		if selectedRoute.Disabled != nil {
			disabled = *selectedRoute.Disabled
		}

		fmt.Printf("\nRoute Details:\n")
		fmt.Printf("==============\n\n")
		fmt.Printf("UID: %s\n", uid)
		fmt.Printf("Label: %s\n", label)
		fmt.Printf("Type: %s\n", routeType)
		fmt.Printf("Modified: %s\n", modified)
		if disabled {
			fmt.Printf("Status: Disabled\n")
		} else {
			fmt.Printf("Status: Enabled\n")
		}
		fmt.Println()

		// Display type-specific configuration
		if routeType == "http" && selectedRoute.Http != nil {
			fmt.Printf("HTTP Configuration:\n")
			if selectedRoute.Http.Url != nil {
				fmt.Printf("  URL: %s\n", *selectedRoute.Http.Url)
			}
			if selectedRoute.Http.Fleets != nil && len(selectedRoute.Http.Fleets) > 0 {
				fmt.Printf("  Fleets: %v\n", selectedRoute.Http.Fleets)
			}
			if selectedRoute.Http.ThrottleMs != nil {
				fmt.Printf("  Throttle: %d ms\n", *selectedRoute.Http.ThrottleMs)
			}
			if selectedRoute.Http.Timeout != nil && *selectedRoute.Http.Timeout > 0 {
				fmt.Printf("  Timeout: %d ms\n", *selectedRoute.Http.Timeout)
			}
		}

		return nil
	},
}

// routeCreateCmd represents the route create command
var routeCreateCmd = &cobra.Command{
	Use:   "create [label]",
	Short: "Create a new route",
	Long: `Create a new route in the current project.

Note: Route creation requires a JSON configuration file. Use --config to specify the file.

Examples:
  # Create route from JSON file
  notehub route create "My Route" --config route.json

  # Example route.json for HTTP route:
  {
    "label": "My HTTP Route",
    "http": {
      "url": "https://example.com/webhook",
      "fleets": ["fleet:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"],
      "throttle_ms": 100,
      "timeout": 5000,
      "http_headers": {
        "X-Custom-Header": "value"
      }
    }
  }`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		label := args[0]
		configFile, _ := cmd.Flags().GetString("config")

		if configFile == "" {
			return fmt.Errorf("--config flag is required to specify route configuration JSON file")
		}

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Read config file
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		// Unmarshal into NotehubRoute struct
		var routeConfig notehub.NotehubRoute
		err = note.JSONUnmarshal(configBytes, &routeConfig)
		if err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}

		// Override label if provided
		routeConfig.Label = &label

		// Create route using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		createdRoute, _, err := client.RouteAPI.CreateRoute(ctx, projectUID).
			NotehubRoute(routeConfig).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create route: %w", err)
		}

		uid := ""
		if createdRoute.Uid != nil {
			uid = *createdRoute.Uid
		}
		routeType := ""
		if createdRoute.Type != nil {
			routeType = *createdRoute.Type
		}

		fmt.Printf("\nRoute created successfully!\n\n")
		fmt.Printf("UID: %s\n", uid)
		fmt.Printf("Label: %s\n", label)
		fmt.Printf("Type: %s\n", routeType)
		fmt.Println()

		return nil
	},
}

// routeUpdateCmd represents the route update command
var routeUpdateCmd = &cobra.Command{
	Use:   "update [route-uid-or-name]",
	Short: "Update an existing route",
	Long: `Update an existing route by UID or name.

Note: Route updates require a JSON configuration file. Use --config to specify the file.

Examples:
  # Update route from JSON file
  notehub route update "My Route" --config route-update.json

  # Update by UID
  notehub route update route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx --config route-update.json

  # Example route-update.json (partial update):
  {
    "http": {
      "url": "https://newexample.com/webhook",
      "throttle_ms": 50
    }
  }`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		routeIdentifier := args[0]
		configFile, _ := cmd.Flags().GetString("config")

		if configFile == "" {
			return fmt.Errorf("--config flag is required to specify route configuration JSON file")
		}

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

		// Determine route UID (handle both UID and name)
		var routeUID string

		// First try as UID
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
		} else {
			// Search by name
			routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			found := false
			for _, route := range routes {
				if route.Label != nil && *route.Label == routeIdentifier {
					if route.Uid != nil {
						routeUID = *route.Uid
						found = true
						break
					}
				}
			}

			if !found {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Read config file
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		// Unmarshal into NotehubRoute struct
		var routeConfig notehub.NotehubRoute
		err = note.JSONUnmarshal(configBytes, &routeConfig)
		if err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}

		// Update route using SDK
		updatedRoute, _, err := client.RouteAPI.UpdateRoute(ctx, projectUID, routeUID).
			NotehubRoute(routeConfig).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to update route: %w", err)
		}

		label := ""
		if updatedRoute.Label != nil {
			label = *updatedRoute.Label
		}
		routeType := ""
		if updatedRoute.Type != nil {
			routeType = *updatedRoute.Type
		}

		fmt.Printf("\nRoute updated successfully!\n\n")
		fmt.Printf("UID: %s\n", routeUID)
		fmt.Printf("Label: %s\n", label)
		fmt.Printf("Type: %s\n", routeType)
		fmt.Println()

		return nil
	},
}

// routeDeleteCmd represents the route delete command
var routeDeleteCmd = &cobra.Command{
	Use:   "delete [route-uid-or-name]",
	Short: "Delete a route",
	Long: `Delete a route by UID or name.

Examples:
  # Delete route by UID
  notehub route delete route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Delete route by name
  notehub route delete "My Route"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		routeIdentifier := args[0]

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

		// Determine route UID and name (handle both UID and name)
		var routeUID string
		var routeName string

		// First try as UID
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
			// Try to get route details for name
			route, resp, err := client.RouteAPI.GetRoute(ctx, projectUID, routeUID).Execute()
			if err == nil && resp != nil && resp.StatusCode != 404 && route.Label != nil {
				routeName = *route.Label
			}
		} else {
			// Search by name
			routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			found := false
			for _, route := range routes {
				if route.Label != nil && *route.Label == routeIdentifier {
					if route.Uid != nil {
						routeUID = *route.Uid
						routeName = *route.Label
						found = true
						break
					}
				}
			}

			if !found {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Delete route using SDK
		_, err = client.RouteAPI.DeleteRoute(ctx, projectUID, routeUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}

		fmt.Printf("\nRoute deleted successfully!\n\n")
		fmt.Printf("UID: %s\n", routeUID)
		if routeName != "" {
			fmt.Printf("Label: %s\n", routeName)
		}
		fmt.Println()

		return nil
	},
}

// routeLogsCmd represents the route logs command
var routeLogsCmd = &cobra.Command{
	Use:   "logs [route-uid-or-name]",
	Short: "Get logs for a specific route",
	Long: `Get logs for a specific route by UID or name.

Examples:
  # Get logs for route
  notehub route logs "My Route"

  # Get logs by UID
  notehub route logs route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Get logs with pagination
  notehub route logs "My Route" --page-size 100 --page-num 1

  # Filter logs by device
  notehub route logs "My Route" --device dev:864475046552567

  # Get logs with JSON output
  notehub route logs "My Route" --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		routeIdentifier := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		pageSize, _ := cmd.Flags().GetInt("page-size")
		pageNum, _ := cmd.Flags().GetInt("page-num")
		deviceUID, _ := cmd.Flags().GetString("device")

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Determine route UID (handle both UID and name)
		var routeUID string

		// First try as UID
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
		} else {
			// Search by name
			routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			found := false
			for _, route := range routes {
				if route.Label != nil && *route.Label == routeIdentifier {
					if route.Uid != nil {
						routeUID = *route.Uid
						found = true
						break
					}
				}
			}

			if !found {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Get route logs using SDK
		req := client.RouteAPI.GetRouteLogsByRoute(ctx, projectUID, routeUID)

		if pageSize > 0 {
			req = req.PageSize(int32(pageSize))
		}
		if pageNum > 0 {
			req = req.PageNum(int32(pageNum))
		}
		if deviceUID != "" {
			req = req.DeviceUID([]string{deviceUID})
		}

		logs, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get route logs: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(logs, "", "  ")
			} else {
				output, err = note.JSONMarshal(logs)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display logs in human-readable format
		if len(logs) == 0 {
			fmt.Println("No logs found.")
			return nil
		}

		fmt.Printf("\nRoute Logs (%d entries):\n", len(logs))
		fmt.Printf("========================\n\n")

		for i, entry := range logs {
			fmt.Printf("%d. ", i+1)

			if entry.Date != nil {
				fmt.Printf("%s\n", *entry.Date)
			} else {
				fmt.Println()
			}

			if entry.EventUid != nil && *entry.EventUid != "" {
				fmt.Printf("   Event UID: %s\n", *entry.EventUid)
			}
			if entry.Status != nil && *entry.Status != "" {
				fmt.Printf("   Status: %s\n", *entry.Status)
			}
			if entry.Duration != nil {
				fmt.Printf("   Duration: %d ms\n", *entry.Duration)
			}
			if entry.Url != nil && *entry.Url != "" {
				fmt.Printf("   URL: %s\n", *entry.Url)
			}
			if entry.Text != nil && *entry.Text != "" {
				fmt.Printf("   Response: %s\n", *entry.Text)
			}
			if entry.Attn != nil && *entry.Attn {
				fmt.Printf("   ⚠ Attention Required\n")
			}
			fmt.Println()
		}

		// Note: SDK returns a simple array, pagination info is in response headers
		// For now, suggest using page-num to paginate
		if len(logs) >= pageSize && pageSize > 0 {
			fmt.Printf("Showing page %d (page size: %d). Use --page-num %d to see next page.\n", pageNum, pageSize, pageNum+1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(routeCmd)
	routeCmd.AddCommand(routeListCmd)
	routeCmd.AddCommand(routeGetCmd)
	routeCmd.AddCommand(routeCreateCmd)
	routeCmd.AddCommand(routeUpdateCmd)
	routeCmd.AddCommand(routeDeleteCmd)
	routeCmd.AddCommand(routeLogsCmd)

	// Add flags for route create
	routeCreateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	routeCreateCmd.MarkFlagRequired("config")

	// Add flags for route update
	routeUpdateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	routeUpdateCmd.MarkFlagRequired("config")

	// Add flags for route logs
	routeLogsCmd.Flags().Int("page-size", 50, "Number of logs to return per page")
	routeLogsCmd.Flags().Int("page-num", 1, "Page number to retrieve")
	routeLogsCmd.Flags().String("device", "", "Filter logs by device UID")
}
