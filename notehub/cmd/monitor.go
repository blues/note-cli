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

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, monitors)
		}

		if len(monitors) == 0 {
			cmd.Println("No monitors found in this project.")
			return nil
		}
		return printHuman(cmd, monitors)
	},
}

// monitorGetCmd represents the monitor get command
var monitorGetCmd = &cobra.Command{
	Use:   "get [monitor-uid]",
	Short: "Get details about a specific monitor",
	Long:  `Get detailed information about a specific monitor by UID. If no argument is provided, uses the active monitor (set with 'monitor set'). If no active monitor is configured, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var monitorUID string
		if len(args) > 0 {
			monitorUID = args[0]
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

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, createdMonitor)
		}

		cmd.Println("Monitor created successfully!")
		return printHuman(cmd, createdMonitor)
	},
}

// monitorUpdateCmd represents the monitor update command
var monitorUpdateCmd = &cobra.Command{
	Use:   "update [monitor-uid]",
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
			monitorUID = args[0]
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

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, updatedMonitor)
		}

		cmd.Println("Monitor updated successfully!")
		return printHuman(cmd, updatedMonitor)
	},
}

// monitorDeleteCmd represents the monitor delete command
var monitorDeleteCmd = &cobra.Command{
	Use:   "delete [monitor-uid]",
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
			monitorUID = args[0]
		} else {
			monitorUID, err = pickMonitor(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		// Delete monitor using SDK
		_, _, err = client.MonitorAPI.DeleteMonitor(ctx, projectUID, monitorUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete monitor: %w", err)
		}

		cmd.Printf("\nMonitor '%s' deleted successfully.\n\n", monitorUID)

		return nil
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

		monitors, _, err := client.MonitorAPI.GetMonitors(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list monitors: %w", err)
		}

		var selectedUID, selectedLabel string
		if len(args) > 0 {
			for _, m := range monitors {
				mUID := ""
				mLabel := ""
				if m.Uid != nil {
					mUID = *m.Uid
				}
				if m.Name != nil {
					mLabel = *m.Name
				}
				if mUID == args[0] || mLabel == args[0] {
					selectedUID = mUID
					selectedLabel = mLabel
					break
				}
			}
			if selectedUID == "" {
				return fmt.Errorf("monitor '%s' not found in project", args[0])
			}
		} else {
			if len(monitors) == 0 {
				return fmt.Errorf("no monitors found in this project. Create one with 'notehub monitor create <name> --config <file>'")
			}
			items := make([]PickerItem, 0, len(monitors))
			for _, m := range monitors {
				label := ""
				uid := ""
				if m.Name != nil {
					label = *m.Name
				}
				if m.Uid != nil {
					uid = *m.Uid
				}
				if uid != "" {
					if label == "" {
						label = uid
					}
					items = append(items, PickerItem{Label: label, Value: uid})
				}
			}
			picked := pickOne("Select a monitor", items)
			if picked == nil {
				return nil
			}
			selectedUID = picked.Value
			selectedLabel = picked.Label
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
}
