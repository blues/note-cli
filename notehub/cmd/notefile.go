// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

// notefileCmd represents the notefile command
var notefileCmd = &cobra.Command{
	Use:   "notefile",
	Short: "Manage Notefiles on devices",
	Long:  `Commands for listing, inspecting, and deleting Notefiles on Notehub devices.`,
}

// notefileListCmd represents the notefile list command
var notefileListCmd = &cobra.Command{
	Use:   "list [device-uid]",
	Short: "List Notefiles on a device",
	Long: `List all Notefiles on a device.

Examples:
  # List all Notefiles
  notehub notefile list dev:864475046552567

  # List only Notefiles with pending changes
  notehub notefile list dev:864475046552567 --pending

  # List specific Notefiles
  notehub notefile list dev:864475046552567 --files data.qo,config.db`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.DeviceAPI.ListNotefiles(ctx, projectUID, deviceUID)

		if pending, _ := cmd.Flags().GetBool("pending"); pending {
			req = req.Pending(pending)
		}
		if files, _ := cmd.Flags().GetStringSlice("files"); len(files) > 0 {
			req = req.Files(files)
		}

		notefiles, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to list Notefiles: %w", err)
		}

		return printListResult(cmd, notefiles, "No Notefiles found on this device.", func() bool {
			return len(notefiles) == 0
		})
	},
}

// notefileGetCmd represents the notefile get command
var notefileGetCmd = &cobra.Command{
	Use:   "get [device-uid] [notefile-id]",
	Short: "Get Notes from a Notefile",
	Long: `Get the Notes contained in a Notefile on a device.

Examples:
  # Get all Notes in a Notefile
  notehub notefile get dev:864475046552567 data.qo

  # Get up to 10 Notes
  notehub notefile get dev:864475046552567 data.qo --max 10

  # Include deleted Notes
  notehub notefile get dev:864475046552567 data.qo --deleted`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.DeviceAPI.GetNotefile(ctx, projectUID, deviceUID, notefileID)

		if max, _ := cmd.Flags().GetInt32("max"); max > 0 {
			req = req.Max(max)
		}
		if deleted, _ := cmd.Flags().GetBool("deleted"); deleted {
			req = req.Deleted(deleted)
		}

		notefile, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get Notefile: %w", err)
		}

		return printResult(cmd, notefile)
	},
}

// notefileDeleteCmd represents the notefile delete command
var notefileDeleteCmd = &cobra.Command{
	Use:   "delete [device-uid] [notefile-ids...]",
	Short: "Delete Notefiles from a device",
	Long: `Delete one or more Notefiles from a device.

Examples:
  # Delete a single Notefile
  notehub notefile delete dev:864475046552567 data.qo

  # Delete multiple Notefiles
  notehub notefile delete dev:864475046552567 data.qo config.db`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileIDs := args[1:]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		if err := confirmAction(cmd, fmt.Sprintf("Delete %d Notefile(s) from %s?", len(notefileIDs), deviceUID)); err != nil {
			return nil
		}

		deleteReq := notehub.NewDeleteNotefilesRequest()
		deleteReq.SetFiles(notefileIDs)

		_, err = client.DeviceAPI.DeleteNotefiles(ctx, projectUID, deviceUID).
			DeleteNotefilesRequest(*deleteReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to delete Notefiles: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":     "delete",
			"notefiles":  notefileIDs,
			"count":      len(notefileIDs),
			"device_uid": deviceUID,
		}, fmt.Sprintf("Deleted %d Notefile(s) from %s", len(notefileIDs), deviceUID))
	},
}

func init() {
	rootCmd.AddCommand(notefileCmd)
	notefileCmd.AddCommand(notefileListCmd)
	notefileCmd.AddCommand(notefileGetCmd)
	notefileCmd.AddCommand(notefileDeleteCmd)

	notefileListCmd.Flags().Bool("pending", false, "Show only Notefiles with pending changes")
	notefileListCmd.Flags().StringSlice("files", nil, "Filter by specific Notefile names (comma-separated)")

	notefileGetCmd.Flags().Int32("max", 0, "Maximum number of Notes to return")
	notefileGetCmd.Flags().Bool("deleted", false, "Include deleted Notes")

	addConfirmFlag(notefileDeleteCmd)
}
