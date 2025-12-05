// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/blues/note-go/note"
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

		verbose := GetVerbose()

		// Get routes using V1 API: GET /v1/projects/{projectUID}/routes
		var routes []map[string]interface{}
		url := fmt.Sprintf("/v1/projects/%s/routes", projectUID)
		err := reqHubV1(verbose, GetAPIHub(), "GET", url, nil, &routes)
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
			uid, _ := route["uid"].(string)
			label, _ := route["label"].(string)
			routeType, _ := route["type"].(string)
			modified, _ := route["modified"].(string)
			disabled, _ := route["disabled"].(bool)

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

		verbose := GetVerbose()

		// Get metadata to resolve route name to UID if needed
		appMetadata, err := appGetMetadata(verbose, false)
		if err != nil {
			return fmt.Errorf("failed to get project metadata: %w", err)
		}

		// Find route UID (handle both UID and name)
		var routeUID string
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
		} else {
			// Search for route by name
			for _, route := range appMetadata.Routes {
				if route.Name == routeIdentifier {
					routeUID = route.UID
					break
				}
			}
			if routeUID == "" {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Get route using V1 API: GET /v1/projects/{projectUID}/routes/{routeUID}
		var route map[string]interface{}
		url := fmt.Sprintf("/v1/projects/%s/routes/%s", projectUID, routeUID)
		err = reqHubV1(verbose, GetAPIHub(), "GET", url, nil, &route)
		if err != nil {
			return fmt.Errorf("failed to get route: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(route, "", "  ")
			} else {
				output, err = note.JSONMarshal(route)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display route in human-readable format
		uid, _ := route["uid"].(string)
		label, _ := route["label"].(string)
		routeType, _ := route["type"].(string)
		modified, _ := route["modified"].(string)
		disabled, _ := route["disabled"].(bool)

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
		if routeType == "http" {
			if httpConfig, ok := route["http"].(map[string]interface{}); ok {
				fmt.Printf("HTTP Configuration:\n")
				if httpURL, ok := httpConfig["url"].(string); ok {
					fmt.Printf("  URL: %s\n", httpURL)
				}
				if fleets, ok := httpConfig["fleets"].([]interface{}); ok && len(fleets) > 0 {
					fmt.Printf("  Fleets: %v\n", fleets)
				}
				if throttle, ok := httpConfig["throttle_ms"].(float64); ok {
					fmt.Printf("  Throttle: %.0f ms\n", throttle)
				}
				if timeout, ok := httpConfig["timeout"].(float64); ok && timeout > 0 {
					fmt.Printf("  Timeout: %.0f ms\n", timeout)
				}
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

		verbose := GetVerbose()

		// Read config file
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		var configData map[string]interface{}
		err = note.JSONUnmarshal(configBytes, &configData)
		if err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}

		// Override label if provided
		configData["label"] = label

		// Marshal config to JSON
		reqJSON, err := note.JSONMarshal(configData)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		// Create route using V1 API: POST /v1/projects/{projectUID}/routes
		var route map[string]interface{}
		url := fmt.Sprintf("/v1/projects/%s/routes", projectUID)
		err = reqHubV1(verbose, GetAPIHub(), "POST", url, reqJSON, &route)
		if err != nil {
			return fmt.Errorf("failed to create route: %w", err)
		}

		uid, _ := route["uid"].(string)
		routeType, _ := route["type"].(string)

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

		verbose := GetVerbose()

		// Get metadata to resolve route name to UID if needed
		appMetadata, err := appGetMetadata(verbose, false)
		if err != nil {
			return fmt.Errorf("failed to get project metadata: %w", err)
		}

		// Find route UID (handle both UID and name)
		var routeUID string
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
		} else {
			// Search for route by name
			for _, route := range appMetadata.Routes {
				if route.Name == routeIdentifier {
					routeUID = route.UID
					break
				}
			}
			if routeUID == "" {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Read config file
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		var configData map[string]interface{}
		err = note.JSONUnmarshal(configBytes, &configData)
		if err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}

		// Marshal config to JSON
		reqJSON, err := note.JSONMarshal(configData)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		// Update route using V1 API: PUT /v1/projects/{projectUID}/routes/{routeUID}
		var route map[string]interface{}
		url := fmt.Sprintf("/v1/projects/%s/routes/%s", projectUID, routeUID)
		err = reqHubV1(verbose, GetAPIHub(), "PUT", url, reqJSON, &route)
		if err != nil {
			return fmt.Errorf("failed to update route: %w", err)
		}

		label, _ := route["label"].(string)
		routeType, _ := route["type"].(string)

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

		verbose := GetVerbose()

		// Get metadata to resolve route name to UID if needed
		appMetadata, err := appGetMetadata(verbose, false)
		if err != nil {
			return fmt.Errorf("failed to get project metadata: %w", err)
		}

		// Find route UID (handle both UID and name)
		var routeUID string
		var routeName string
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
			// Try to find name for better output
			for _, route := range appMetadata.Routes {
				if route.UID == routeUID {
					routeName = route.Name
					break
				}
			}
		} else {
			// Search for route by name
			routeName = routeIdentifier
			for _, route := range appMetadata.Routes {
				if route.Name == routeIdentifier {
					routeUID = route.UID
					break
				}
			}
			if routeUID == "" {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Delete route using V1 API: DELETE /v1/projects/{projectUID}/routes/{routeUID}
		url := fmt.Sprintf("/v1/projects/%s/routes/%s", projectUID, routeUID)
		err = reqHubV1(verbose, GetAPIHub(), "DELETE", url, nil, nil)
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

		verbose := GetVerbose()
		pageSize, _ := cmd.Flags().GetInt("page-size")
		pageNum, _ := cmd.Flags().GetInt("page-num")
		deviceUID, _ := cmd.Flags().GetString("device")

		// Get metadata to resolve route name to UID if needed
		appMetadata, err := appGetMetadata(verbose, false)
		if err != nil {
			return fmt.Errorf("failed to get project metadata: %w", err)
		}

		// Find route UID (handle both UID and name)
		var routeUID string
		if len(routeIdentifier) > 6 && routeIdentifier[:6] == "route:" {
			routeUID = routeIdentifier
		} else {
			// Search for route by name
			for _, route := range appMetadata.Routes {
				if route.Name == routeIdentifier {
					routeUID = route.UID
					break
				}
			}
			if routeUID == "" {
				return fmt.Errorf("route '%s' not found", routeIdentifier)
			}
		}

		// Build URL with query parameters
		url := fmt.Sprintf("/v1/projects/%s/routes/%s/route-logs", projectUID, routeUID)

		// Add query parameters
		firstParam := true
		if pageSize > 0 {
			if firstParam {
				url += "?"
				firstParam = false
			} else {
				url += "&"
			}
			url += fmt.Sprintf("pageSize=%d", pageSize)
		}
		if pageNum > 0 {
			if firstParam {
				url += "?"
				firstParam = false
			} else {
				url += "&"
			}
			url += fmt.Sprintf("pageNum=%d", pageNum)
		}
		if deviceUID != "" {
			if firstParam {
				url += "?"
				firstParam = false
			} else {
				url += "&"
			}
			url += fmt.Sprintf("deviceUID=%s", deviceUID)
		}

		// Get route logs using V1 API: GET /v1/projects/{projectUID}/routes/{routeUID}/route-logs
		var logs map[string]interface{}
		err = reqHubV1(verbose, GetAPIHub(), "GET", url, nil, &logs)
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
		logEntries, _ := logs["logs"].([]interface{})
		hasMore, _ := logs["has_more"].(bool)

		if len(logEntries) == 0 {
			fmt.Println("No logs found.")
			return nil
		}

		fmt.Printf("\nRoute Logs (%d entries):\n", len(logEntries))
		fmt.Printf("========================\n\n")

		for i, entry := range logEntries {
			if logMap, ok := entry.(map[string]interface{}); ok {
				when, _ := logMap["when"].(string)
				deviceID, _ := logMap["device_uid"].(string)
				status, _ := logMap["status"].(string)
				message, _ := logMap["message"].(string)

				fmt.Printf("%d. %s\n", i+1, when)
				if deviceID != "" {
					fmt.Printf("   Device: %s\n", deviceID)
				}
				if status != "" {
					fmt.Printf("   Status: %s\n", status)
				}
				if message != "" {
					fmt.Printf("   Message: %s\n", message)
				}
				fmt.Println()
			}
		}

		if hasMore {
			fmt.Printf("More logs available. Use --page-num %d to see next page.\n", pageNum+1)
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
