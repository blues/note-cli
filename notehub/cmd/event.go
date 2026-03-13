// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// eventCmd represents the event command
var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Manage Notehub events",
	Long:  `Commands for listing and managing events in Notehub projects.`,
}

// eventListCmd represents the event list command
var eventListCmd = &cobra.Command{
	Use:   "list",
	Short: "List events",
	Long:  `List events in the current project, with optional filtering by device, fleet, or notefile.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		// Get flags
		limit, _ := cmd.Flags().GetInt32("limit")
		cursor, _ := cmd.Flags().GetString("cursor")
		deviceFilter, _ := cmd.Flags().GetString("device")
		fleetFilter, _ := cmd.Flags().GetString("fleet")
		fileFilter, _ := cmd.Flags().GetString("file")
		sortOrder, _ := cmd.Flags().GetString("sort-order")

		// Build request
		req := client.EventAPI.GetEventsByCursor(ctx, projectUID)
		if limit > 0 {
			req = req.Limit(limit)
		}
		if cursor != "" {
			req = req.Cursor(cursor)
		}
		if deviceFilter != "" {
			req = req.DeviceUID([]string{deviceFilter})
		}
		if fleetFilter != "" {
			req = req.FleetUID(fleetFilter)
		}
		if fileFilter != "" {
			req = req.Files(fileFilter)
		}
		if sortOrder != "" {
			req = req.SortOrder(sortOrder)
		}

		eventsRsp, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to list events: %w", err)
		}

		// Handle JSON output
		if wantJSON() {
			return printJSON(cmd, eventsRsp)
		}

		if len(eventsRsp.Events) == 0 {
			cmd.Println("No events found.")
			return nil
		}
		return printHuman(cmd, eventsRsp)
	},
}

func init() {
	rootCmd.AddCommand(eventCmd)
	eventCmd.AddCommand(eventListCmd)

	eventListCmd.Flags().Int32("limit", 50, "Maximum number of events to return")
	eventListCmd.Flags().String("cursor", "", "Cursor for pagination")
	eventListCmd.Flags().String("device", "", "Filter by device UID")
	eventListCmd.Flags().String("fleet", "", "Filter by fleet UID")
	eventListCmd.Flags().String("file", "", "Filter by notefile name")
	eventListCmd.Flags().String("sort-order", "desc", "Sort order: asc or desc")
}
