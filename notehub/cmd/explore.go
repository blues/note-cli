// Copyright 2021 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"sort"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
	"github.com/blues/note-go/notehub"
	"github.com/spf13/cobra"
)

var (
	flagReserved bool
)

// exploreCmd represents the explore command
var exploreCmd = &cobra.Command{
	Use:   "explore",
	Short: "Explore the contents of a device",
	Long: `Explore the notefiles and notes on a device.

By default, reserved notefiles are not shown. Use --reserved to include them.

Example:
  notehub explore --device dev:xxxx --pretty
  notehub explore --scope @production --reserved`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		device := GetDevice()
		if flagScope == "" && device == "" {
			return fmt.Errorf("use --device to specify a device or --scope to specify multiple devices")
		}

		// If scope is specified, iterate over multiple devices
		if flagScope != "" {
			appMetadata, scopeDevices, _, err := ResolveScopeWithValidation(flagScope)
			if err != nil {
				return err
			}

			verbose := GetVerbose()
			pretty := GetPretty()

			for _, deviceUID := range scopeDevices {
				reqFlagDevice = deviceUID
				err = exploreDevice(flagReserved, verbose, pretty)
				if err != nil {
					return err
				}
			}

			// Set the project for the request
			reqFlagApp = appMetadata.App.UID
		} else {
			// Single device exploration
			reqFlagDevice = device
			reqFlagApp = GetProject()
			err := exploreDevice(flagReserved, GetVerbose(), GetPretty())
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(exploreCmd)

	exploreCmd.Flags().BoolVarP(&flagReserved, "reserved", "r", false, "Include reserved notefiles")
	exploreCmd.Flags().StringVarP(&flagScope, "scope", "s", "", "Device scope (alternative to --device)")
}

// Explore the contents of a device
// Note: This function intentionally uses V0 Notecard APIs (file.changes, note.changes)
// These are device-specific APIs for communicating with Notecard hardware, distinct from
// the Notehub project management APIs which have been migrated to V1 REST endpoints.
func exploreDevice(includeReserved bool, verbose bool, pretty bool) (err error) {
	// Get the list of notefiles using file.changes API
	req := notehub.HubRequest{}
	req.Req = notecard.ReqFileChanges
	req.Allow = includeReserved
	var rsp notehub.HubRequest
	rsp, err = hubTransactionRequest(req, verbose)
	if err != nil {
		return
	}

	// Exit if no notefiles
	fmt.Printf("%s\n", reqFlagDevice)
	if rsp.FileInfo == nil || len(*rsp.FileInfo) == 0 {
		fmt.Printf("    no notefiles\n")
		return
	}

	// Sort the notefiles
	notefileIDs := []string{}
	for notefileID := range *rsp.FileInfo {
		notefileIDs = append(notefileIDs, notefileID)
	}
	sort.Strings(notefileIDs)

	// Iterate over each file
	for _, notefileID := range notefileIDs {
		fmt.Printf("    %s\n", notefileID)

		// Get the notes using note.changes API
		req = notehub.HubRequest{}
		req.Req = notecard.ReqNoteChanges
		req.Allow = includeReserved
		req.Deleted = true
		req.NotefileID = notefileID
		rsp, err = hubTransactionRequest(req, verbose)
		if err != nil {
			return
		}

		// Exit if no notefiles
		if rsp.Notes == nil || len(*rsp.Notes) == 0 {
			continue
		}

		// Show the notes
		for noteID, n := range *rsp.Notes {
			fmt.Printf("        %s", noteID)
			if n.Deleted {
				fmt.Printf(" (DELETED)")
			}
			fmt.Printf("\n")
			if n.Body != nil {
				prefix := "            "
				var bodyJSON []byte
				if pretty {
					bodyJSON, err = note.JSONMarshalIndent(*n.Body, prefix, "    ")
				} else {
					bodyJSON, err = note.JSONMarshal(*n.Body)
				}
				if err == nil {
					fmt.Printf("%s%s\n", prefix, string(bodyJSON))
				}
			}
			if n.Payload != nil {
				fmt.Printf("            Payload: %d bytes\n", len(*n.Payload))
			}
		}
	}

	return
}
