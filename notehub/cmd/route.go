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
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list routes: %w", err)
		}

		return printListResult(cmd, routes, "No routes found.", func() bool {
			return len(routes) == 0
		})
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
  notehub route get "My Route" --pretty

If no argument is provided, uses the active route (set with 'route set'). If no active route is configured, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var routeIdentifier string
		if len(args) > 0 {
			routeIdentifier = args[0]
		} else if def := GetRoute(); def != "" {
			routeIdentifier = def
		} else {
			routeIdentifier, err = pickRoute(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		fullRoute, summary, err := resolveRoute(client, ctx, projectUID, routeIdentifier)
		if err != nil {
			return err
		}

		if fullRoute != nil {
			return printResult(cmd, fullRoute)
		}
		return printResult(cmd, summary)
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
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		label := args[0]
		configFile, _ := cmd.Flags().GetString("config")

		if configFile == "" {
			return fmt.Errorf("--config is required")
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
		createdRoute, _, err := client.RouteAPI.CreateRoute(ctx, projectUID).
			NotehubRoute(routeConfig).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create route: %w", err)
		}

		return printMutationResult(cmd, createdRoute, "Route created")
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
  }

If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var routeIdentifier string
		if len(args) > 0 {
			routeIdentifier = args[0]
		} else {
			routeIdentifier, err = pickRoute(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}
		configFile, _ := cmd.Flags().GetString("config")

		if configFile == "" {
			return fmt.Errorf("--config is required")
		}

		routeUID, err := resolveRouteUID(client, ctx, projectUID, routeIdentifier)
		if err != nil {
			return err
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

		return printMutationResult(cmd, updatedRoute, "Route updated")
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
  notehub route delete "My Route"

If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var routeIdentifier string
		if len(args) > 0 {
			routeIdentifier = args[0]
		} else {
			routeIdentifier, err = pickRoute(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		routeUID, err := resolveRouteUID(client, ctx, projectUID, routeIdentifier)
		if err != nil {
			return err
		}

		if err := confirmAction(cmd, fmt.Sprintf("Delete route '%s'?", routeUID)); err != nil {
			return nil
		}

		_, err = client.RouteAPI.DeleteRoute(ctx, projectUID, routeUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":    "delete",
			"route_uid": routeUID,
		}, fmt.Sprintf("Route '%s' deleted", routeUID))
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

  # Get more logs
  notehub route logs "My Route" --limit 100

  # Filter logs by device
  notehub route logs "My Route" --device dev:864475046552567

  # Get logs with JSON output
  notehub route logs "My Route" --json

If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var routeIdentifier string
		if len(args) > 0 {
			routeIdentifier = args[0]
		} else {
			routeIdentifier, err = pickRoute(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		deviceUID, _ := cmd.Flags().GetString("device")

		routeUID, err := resolveRouteUID(client, ctx, projectUID, routeIdentifier)
		if err != nil {
			return err
		}

		pageSize, maxResults := getPaginationConfig(cmd)

		var allLogs []notehub.RouteLog
		pageNum := int32(1)
		for {
			req := client.RouteAPI.GetRouteLogsByRoute(ctx, projectUID, routeUID).
				PageSize(pageSize).
				PageNum(pageNum)

			if deviceUID != "" {
				req = req.DeviceUID([]string{deviceUID})
			}

			logs, _, err := req.Execute()
			if err != nil {
				return fmt.Errorf("failed to get route logs: %w", err)
			}

			allLogs = append(allLogs, logs...)

			if len(logs) < int(pageSize) {
				break
			}
			if maxResults > 0 && len(allLogs) >= maxResults {
				allLogs = allLogs[:maxResults]
				break
			}
			pageNum++
		}

		return printListResult(cmd, allLogs, "No logs found.", func() bool {
			return len(allLogs) == 0
		})
	},
}

// routeSetCmd represents the route set command
var routeSetCmd = &cobra.Command{
	Use:   "set [route-uid-or-name]",
	Short: "Set the active route",
	Long: `Set the active route in the configuration. You can specify either the route name or UID.
If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var selectedUID, selectedLabel string
		if len(args) > 0 {
			fullRoute, summary, err := resolveRoute(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
			if fullRoute != nil {
				if fullRoute.Uid != nil {
					selectedUID = *fullRoute.Uid
				}
				if fullRoute.Label != nil {
					selectedLabel = *fullRoute.Label
				}
			} else if summary != nil {
				if summary.Uid != nil {
					selectedUID = *summary.Uid
				}
				if summary.Label != nil {
					selectedLabel = *summary.Label
				}
			}
		} else {
			selectedUID, err = pickRoute(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
			// Resolve to get the label
			_, summary, _ := resolveRoute(client, ctx, projectUID, selectedUID)
			if summary != nil && summary.Label != nil {
				selectedLabel = *summary.Label
			}
		}
		if selectedLabel == "" {
			selectedLabel = selectedUID
		}

		return setDefault(cmd, "route", selectedUID, selectedLabel)
	},
}

// routeClearCmd represents the route clear command
var routeClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active route",
	Long:  `Clear the active route from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearDefault(cmd, "route", "notehub route set <name-or-uid>")
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
	routeCmd.AddCommand(routeSetCmd)
	routeCmd.AddCommand(routeClearCmd)

	// Add flags for route create
	routeCreateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	routeCreateCmd.MarkFlagRequired("config")

	// Add flags for route update
	routeUpdateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	routeUpdateCmd.MarkFlagRequired("config")

	addConfirmFlag(routeDeleteCmd)

	// Add flags for route logs
	addPaginationFlags(routeLogsCmd, 50)
	routeLogsCmd.Flags().String("device", "", "Filter logs by device UID")
}
