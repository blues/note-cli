// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
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
	Long: `List events in the current project, with optional filtering by device, fleet, or notefile.

By default, returns up to 50 events. Use --limit to change the number, or --all to fetch every event.

Examples:
  # List first 50 events (default)
  notehub event list

  # List first 100 events
  notehub event list --limit 100

  # List all events
  notehub event list --all

  # Filter by device
  notehub event list --device dev:864475046552567

  # Continue from a cursor
  notehub event list --cursor <cursor-string>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		// Get flags
		limit, _ := cmd.Flags().GetInt32("limit")
		fetchAll, _ := cmd.Flags().GetBool("all")
		cursor, _ := cmd.Flags().GetString("cursor")
		deviceFilter, _ := cmd.Flags().GetString("device")
		fleetFilter, _ := cmd.Flags().GetString("fleet")
		fileFilter, _ := cmd.Flags().GetString("file")
		sortOrder, _ := cmd.Flags().GetString("sort-order")

		maxResults := int(limit)
		pageSize := limit
		if fetchAll {
			pageSize = 500
			maxResults = 0
		}

		var allEvents []notehub.Event
		for {
			req := client.EventAPI.GetEventsByCursor(ctx, projectUID)
			req = req.Limit(pageSize)
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

			allEvents = append(allEvents, eventsRsp.Events...)

			if !eventsRsp.HasMore || eventsRsp.NextCursor == "" {
				break
			}
			if maxResults > 0 && len(allEvents) >= maxResults {
				allEvents = allEvents[:maxResults]
				break
			}
			cursor = eventsRsp.NextCursor
		}

		return printListResult(cmd, allEvents, "No events found.", func() bool {
			return len(allEvents) == 0
		})
	},
}

func init() {
	rootCmd.AddCommand(eventCmd)
	eventCmd.AddCommand(eventListCmd)

	eventListCmd.Flags().Int32("limit", 50, "Maximum number of events to return")
	eventListCmd.Flags().Bool("all", false, "Fetch all events (may be slow for large datasets)")
	eventListCmd.Flags().String("cursor", "", "Cursor for pagination")
	eventListCmd.Flags().String("device", "", "Filter by device UID")
	eventListCmd.Flags().String("fleet", "", "Filter by fleet UID")
	eventListCmd.Flags().String("file", "", "Filter by notefile name")
	eventListCmd.Flags().String("sort-order", "desc", "Sort order: asc or desc")
}
