// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/spf13/cobra"
)

var (
	flagReserved bool
)

// exploreCmd represents the explore command
var exploreCmd = &cobra.Command{
	Use:   "explore [device-uid]",
	Short: "Explore the contents of a device",
	Long: `Explore the notefiles and notes on a device.

By default, reserved notefiles (starting with '_') are not shown.
Use --reserved to include them.

Examples:
  # Explore a device
  notehub explore dev:864475046552567

  # Explore with pretty output
  notehub explore dev:864475046552567 --pretty

  # Include reserved notefiles
  notehub explore dev:864475046552567 --reserved

  # Explore multiple devices via scope
  notehub explore --scope @production --reserved`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		pretty := GetPretty()

		// Determine device(s) to explore
		if flagScope != "" {
			_, scopeDevices, _, err := ResolveScopeWithValidation(flagScope)
			if err != nil {
				return err
			}
			for _, deviceUID := range scopeDevices {
				if err := exploreDevice(cmd, client, ctx, projectUID, deviceUID, flagReserved, pretty); err != nil {
					return err
				}
			}
			return nil
		}

		// Single device
		var deviceUID string
		if len(args) > 0 {
			deviceUID = args[0]
		} else {
			deviceUID = GetDevice()
		}
		if deviceUID == "" {
			return fmt.Errorf("device UID argument or --scope is required")
		}

		return exploreDevice(cmd, client, ctx, projectUID, deviceUID, flagReserved, pretty)
	},
}

func init() {
	rootCmd.AddCommand(exploreCmd)

	exploreCmd.Flags().BoolVarP(&flagReserved, "reserved", "r", false, "Include reserved notefiles")
	addScopeFlag(exploreCmd, "Device scope (alternative to positional arg)")
}

// exploreDevice lists all notefiles on a device and displays their notes.
func exploreDevice(cmd *cobra.Command, client *notehub.APIClient, ctx context.Context, projectUID, deviceUID string, includeReserved, pretty bool) error {
	// List notefiles on the device
	notefiles, _, err := client.DeviceAPI.ListNotefiles(ctx, projectUID, deviceUID).Execute()
	if err != nil {
		return fmt.Errorf("failed to list Notefiles on %s: %w", deviceUID, err)
	}

	cmd.Printf("%s\n", deviceUID)

	if len(notefiles) == 0 {
		cmd.Printf("    no notefiles\n")
		return nil
	}

	// Collect and sort notefile IDs, filtering reserved if needed
	var notefileIDs []string
	for _, nf := range notefiles {
		id := nf.GetId()
		if !includeReserved && strings.HasPrefix(id, "_") {
			continue
		}
		notefileIDs = append(notefileIDs, id)
	}
	sort.Strings(notefileIDs)

	if len(notefileIDs) == 0 {
		cmd.Printf("    no notefiles\n")
		return nil
	}

	// Get and display notes for each notefile
	for _, notefileID := range notefileIDs {
		cmd.Printf("    %s\n", notefileID)

		resp, _, err := client.DeviceAPI.GetNotefile(ctx, projectUID, deviceUID, notefileID).
			Deleted(true).
			Execute()
		if err != nil {
			cmd.Printf("        (error: %s)\n", err)
			continue
		}

		notes := resp.GetNotes()
		if len(notes) == 0 {
			continue
		}

		// Sort note IDs for consistent output
		noteIDs := make([]string, 0, len(notes))
		for noteID := range notes {
			noteIDs = append(noteIDs, noteID)
		}
		sort.Strings(noteIDs)

		for _, noteID := range noteIDs {
			noteData := notes[noteID]
			cmd.Printf("        %s", noteID)

			// Try to extract deleted flag from the note data
			if noteMap, ok := noteData.(map[string]interface{}); ok {
				if deleted, ok := noteMap["deleted"].(bool); ok && deleted {
					cmd.Printf(" (DELETED)")
				}
				cmd.Printf("\n")

				// Display body
				if body, ok := noteMap["body"]; ok && body != nil {
					prefix := "            "
					var bodyJSON []byte
					if pretty {
						bodyJSON, _ = note.JSONMarshalIndent(body, prefix, "    ")
					} else {
						bodyJSON, _ = note.JSONMarshal(body)
					}
					if len(bodyJSON) > 0 {
						cmd.Printf("%s%s\n", prefix, string(bodyJSON))
					}
				}

				// Display payload info
				if payload, ok := noteMap["payload"].(string); ok && payload != "" {
					cmd.Printf("            Payload: %d bytes\n", len(payload))
				}
			} else {
				cmd.Printf("\n")
			}
		}
	}

	return nil
}
