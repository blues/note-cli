// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Manage Notehub monitors",
	Long:  `Commands for listing and managing monitors in Notehub projects.`,
}

// monitorListCmd represents the monitor list command
var monitorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitors",
	Long:  `List all monitors in the current project or a specified project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		monitors, _, err := client.MonitorAPI.GetMonitors(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list monitors: %w", err)
		}

		return printListResult(cmd, monitors, "No monitors found in this project.", func() bool {
			return len(monitors) == 0
		})
	},
}

// monitorGetCmd represents the monitor get command
var monitorGetCmd = &cobra.Command{
	Use:   "get [monitor-uid-or-name]",
	Short: "Get details about a specific monitor",
	Long:  `Get detailed information about a specific monitor by UID or name. If no argument is provided, uses the active monitor (set with 'monitor set'). If no active monitor is configured, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var monitorUID string
		if len(args) > 0 {
			monitorUID, _, err = resolveMonitor(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else if def := GetMonitor(); def != "" {
			monitorUID = def
		} else {
			monitorUID, err = pickMonitor(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		monitor, _, err := client.MonitorAPI.GetMonitor(ctx, projectUID, monitorUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to get monitor: %w", err)
		}

		return printResult(cmd, monitor)
	},
}

// monitorCreateCmd represents the monitor create command
var monitorCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new monitor",
	Long:  `Create a new monitor in the current project from a JSON configuration file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		monitorName := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		// Read config file
		configPath, _ := cmd.Flags().GetString("config")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		var monitor notehub.Monitor
		if err := json.Unmarshal(configData, &monitor); err != nil {
			return fmt.Errorf("failed to parse config JSON: %w", err)
		}

		// Set the name from args
		monitor.Name = &monitorName

		// Create monitor using SDK
		createdMonitor, _, err := client.MonitorAPI.CreateMonitor(ctx, projectUID).
			Body(monitor).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create monitor: %w", err)
		}

		return printMutationResult(cmd, createdMonitor, "Monitor created")
	},
}

// monitorUpdateCmd represents the monitor update command
var monitorUpdateCmd = &cobra.Command{
	Use:   "update [monitor-uid-or-name]",
	Short: "Update a monitor",
	Long:  `Update a monitor's configuration from a JSON configuration file. If no argument is provided, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var monitorUID string
		if len(args) > 0 {
			monitorUID, _, err = resolveMonitor(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else {
			monitorUID, err = pickMonitor(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		// Read config file
		configPath, _ := cmd.Flags().GetString("config")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		var monitor notehub.Monitor
		if err := json.Unmarshal(configData, &monitor); err != nil {
			return fmt.Errorf("failed to parse config JSON: %w", err)
		}

		// Update monitor using SDK
		updatedMonitor, _, err := client.MonitorAPI.UpdateMonitor(ctx, projectUID, monitorUID).
			Monitor(monitor).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to update monitor: %w", err)
		}

		return printMutationResult(cmd, updatedMonitor, "Monitor updated")
	},
}

// monitorDeleteCmd represents the monitor delete command
var monitorDeleteCmd = &cobra.Command{
	Use:   "delete [monitor-uid-or-name]",
	Short: "Delete a monitor",
	Long:  `Delete a monitor from the current project. If no argument is provided, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var monitorUID string
		if len(args) > 0 {
			monitorUID, _, err = resolveMonitor(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else {
			monitorUID, err = pickMonitor(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		if err := confirmAction(cmd, fmt.Sprintf("Delete monitor '%s'?", monitorUID)); err != nil {
			return nil
		}

		_, _, err = client.MonitorAPI.DeleteMonitor(ctx, projectUID, monitorUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete monitor: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "delete",
			"monitor_uid": monitorUID,
		}, fmt.Sprintf("Monitor '%s' deleted", monitorUID))
	},
}

// monitorSetCmd represents the monitor set command
var monitorSetCmd = &cobra.Command{
	Use:   "set [monitor-uid-or-name]",
	Short: "Set the active monitor",
	Long: `Set the active monitor in the configuration. You can specify either the monitor name or UID.
If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var selectedUID, selectedLabel string
		if len(args) > 0 {
			selectedUID, selectedLabel, err = resolveMonitor(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else {
			selectedUID, err = pickMonitor(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
			selectedUID, selectedLabel, err = resolveMonitor(client, ctx, projectUID, selectedUID)
			if err != nil {
				return err
			}
		}

		return setDefault(cmd, "monitor", selectedUID, selectedLabel)
	},
}

// monitorClearCmd represents the monitor clear command
var monitorClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active monitor",
	Long:  `Clear the active monitor from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearDefault(cmd, "monitor", "notehub monitor set <name-or-uid>")
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.AddCommand(monitorListCmd)
	monitorCmd.AddCommand(monitorGetCmd)
	monitorCmd.AddCommand(monitorCreateCmd)
	monitorCmd.AddCommand(monitorDeleteCmd)
	monitorCmd.AddCommand(monitorUpdateCmd)
	monitorCmd.AddCommand(monitorSetCmd)
	monitorCmd.AddCommand(monitorClearCmd)

	monitorCreateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	monitorCreateCmd.MarkFlagRequired("config")

	monitorUpdateCmd.Flags().String("config", "", "Path to JSON configuration file (required)")
	monitorUpdateCmd.MarkFlagRequired("config")

	addConfirmFlag(monitorDeleteCmd)
}
