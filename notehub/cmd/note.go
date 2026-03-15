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

// noteCmd represents the note command
var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage Notes on devices",
	Long: `Commands for reading, adding, updating, and deleting Notes in DB Notefiles,
and adding Notes to inbound queue (.qi) Notefiles.`,
}

// noteGetCmd represents the note get command
var noteGetCmd = &cobra.Command{
	Use:   "get [device-uid] [notefile-id] [note-id]",
	Short: "Get a Note from a DB Notefile",
	Long: `Get a specific Note from a DB Notefile on a device.

Examples:
  # Get a Note
  notehub note get dev:864475046552567 config.db settings

  # Get a Note including deleted
  notehub note get dev:864475046552567 config.db settings --deleted

  # Get with pretty JSON output
  notehub note get dev:864475046552567 config.db settings --pretty`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]
		noteID := args[2]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.DeviceAPI.GetDbNote(ctx, projectUID, deviceUID, notefileID, noteID)

		if deleted, _ := cmd.Flags().GetBool("deleted"); deleted {
			req = req.Deleted(deleted)
		}

		dbNote, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get Note: %w", err)
		}

		return printResult(cmd, dbNote)
	},
}

// noteAddCmd represents the note add command
var noteAddCmd = &cobra.Command{
	Use:   "add [device-uid] [notefile-id] [note-id]",
	Short: "Add a Note to a DB Notefile",
	Long: `Add a Note to a DB Notefile on a device. The Note body is provided as a JSON string
or from a file.

Examples:
  # Add a Note with inline JSON body
  notehub note add dev:864475046552567 config.db settings --body '{"interval":60}'

  # Add a Note with body from a file
  notehub note add dev:864475046552567 config.db settings --file note.json

  # Add a Note with a payload
  notehub note add dev:864475046552567 config.db settings --body '{"type":"firmware"}' --payload "base64data"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]
		noteID := args[2]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		noteInput, err := buildNoteInput(cmd)
		if err != nil {
			return err
		}

		_, err = client.DeviceAPI.AddDbNote(ctx, projectUID, deviceUID, notefileID, noteID).
			NoteInput(*noteInput).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to add Note: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "add",
			"note_id":     noteID,
			"notefile_id": notefileID,
			"device_uid":  deviceUID,
		}, fmt.Sprintf("Note '%s' added to %s on %s", noteID, notefileID, deviceUID))
	},
}

// noteUpdateCmd represents the note update command
var noteUpdateCmd = &cobra.Command{
	Use:   "update [device-uid] [notefile-id] [note-id]",
	Short: "Update a Note in a DB Notefile",
	Long: `Update an existing Note in a DB Notefile on a device.

Examples:
  # Update a Note with inline JSON body
  notehub note update dev:864475046552567 config.db settings --body '{"interval":120}'

  # Update a Note with body from a file
  notehub note update dev:864475046552567 config.db settings --file note.json`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]
		noteID := args[2]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		noteInput, err := buildNoteInput(cmd)
		if err != nil {
			return err
		}

		_, err = client.DeviceAPI.UpdateDbNote(ctx, projectUID, deviceUID, notefileID, noteID).
			NoteInput(*noteInput).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to update Note: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "update",
			"note_id":     noteID,
			"notefile_id": notefileID,
			"device_uid":  deviceUID,
		}, fmt.Sprintf("Note '%s' updated in %s on %s", noteID, notefileID, deviceUID))
	},
}

// noteDeleteCmd represents the note delete command
var noteDeleteCmd = &cobra.Command{
	Use:   "delete [device-uid] [notefile-id] [note-id]",
	Short: "Delete a Note from a DB Notefile",
	Long: `Delete a specific Note from a DB Notefile on a device.

Examples:
  # Delete a Note
  notehub note delete dev:864475046552567 config.db settings`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]
		noteID := args[2]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		_, err = client.DeviceAPI.DeleteNote(ctx, projectUID, deviceUID, notefileID, noteID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete Note: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "delete",
			"note_id":     noteID,
			"notefile_id": notefileID,
			"device_uid":  deviceUID,
		}, fmt.Sprintf("Note '%s' deleted from %s on %s", noteID, notefileID, deviceUID))
	},
}

// notePushCmd represents the note push command
var notePushCmd = &cobra.Command{
	Use:   "push [device-uid] [notefile-id]",
	Short: "Push a Note to an inbound queue Notefile",
	Long: `Add a Note to an inbound queue (.qi) Notefile on a device. The Note is queued
for delivery to the device on its next sync.

Examples:
  # Push a Note to an inbound queue
  notehub note push dev:864475046552567 commands.qi --body '{"action":"reboot"}'

  # Push a Note with body from a file
  notehub note push dev:864475046552567 commands.qi --file command.json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceUID := args[0]
		notefileID := args[1]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		noteInput, err := buildNoteInput(cmd)
		if err != nil {
			return err
		}

		_, err = client.DeviceAPI.AddQiNote(ctx, projectUID, deviceUID, notefileID).
			NoteInput(*noteInput).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to push Note: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "push",
			"notefile_id": notefileID,
			"device_uid":  deviceUID,
		}, fmt.Sprintf("Note pushed to %s on %s", notefileID, deviceUID))
	},
}

// buildNoteInput constructs a NoteInput from --body/--file and --payload flags.
func buildNoteInput(cmd *cobra.Command) (*notehub.NoteInput, error) {
	bodyStr, _ := cmd.Flags().GetString("body")
	filePath, _ := cmd.Flags().GetString("file")
	payload, _ := cmd.Flags().GetString("payload")

	if bodyStr == "" && filePath == "" {
		return nil, fmt.Errorf("either --body or --file is required")
	}
	if bodyStr != "" && filePath != "" {
		return nil, fmt.Errorf("use either --body or --file, not both")
	}

	var bodyJSON []byte
	if filePath != "" {
		var err error
		bodyJSON, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		bodyJSON = []byte(bodyStr)
	}

	var body map[string]interface{}
	if err := note.JSONUnmarshal(bodyJSON, &body); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}

	noteInput := notehub.NewNoteInput()
	noteInput.SetBody(body)
	if payload != "" {
		noteInput.SetPayload(payload)
	}

	return noteInput, nil
}

func init() {
	rootCmd.AddCommand(noteCmd)
	noteCmd.AddCommand(noteGetCmd)
	noteCmd.AddCommand(noteAddCmd)
	noteCmd.AddCommand(noteUpdateCmd)
	noteCmd.AddCommand(noteDeleteCmd)
	noteCmd.AddCommand(notePushCmd)

	// Get flags
	noteGetCmd.Flags().Bool("deleted", false, "Include deleted Notes")

	// Add flags
	noteAddCmd.Flags().String("body", "", "Note body as JSON string")
	noteAddCmd.Flags().String("file", "", "Path to JSON file containing the Note body")
	noteAddCmd.Flags().String("payload", "", "Base64-encoded payload")

	// Update flags
	noteUpdateCmd.Flags().String("body", "", "Note body as JSON string")
	noteUpdateCmd.Flags().String("file", "", "Path to JSON file containing the Note body")
	noteUpdateCmd.Flags().String("payload", "", "Base64-encoded payload")

	// Push flags
	notePushCmd.Flags().String("body", "", "Note body as JSON string")
	notePushCmd.Flags().String("file", "", "Path to JSON file containing the Note body")
	notePushCmd.Flags().String("payload", "", "Base64-encoded payload")
}
