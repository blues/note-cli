// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	flagType      string
	flagTags      string
	flagNotes     string
	flagOverwrite bool
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload [file]",
	Short: "Upload firmware to Notehub",
	Long: `Upload host or notecard firmware to Notehub using the V1 API.

The firmware type must be specified as either 'host' or 'notecard' using the --type flag.

Example:
  notehub upload firmware.bin --type host --project app:xxxx
  notehub upload notecard-fw.bin --type notecard --product com.company:product --notes "Bug fixes"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		filename := args[0]

		// Determine project UID
		projectUID := GetProject()
		if projectUID == "" {
			product := GetProduct()
			if product != "" {
				projectUID = product
			}
		}
		if projectUID == "" {
			return fmt.Errorf("--project or --product flag is required")
		}

		// Validate firmware type
		if flagType == "" {
			return fmt.Errorf("--type flag is required (must be 'host' or 'notecard')")
		}
		if flagType != "host" && flagType != "notecard" {
			return fmt.Errorf("--type must be either 'host' or 'notecard', got: %s", flagType)
		}

		// Upload using V1 API
		err := uploadFirmwareV1(projectUID, flagType, filename, flagNotes, GetVerbose())
		if err != nil {
			return err
		}

		fmt.Printf("Successfully uploaded %s firmware: %s\n", flagType, filename)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringVar(&flagType, "type", "", "Firmware type: 'host' or 'notecard' (required)")
	uploadCmd.Flags().StringVar(&flagNotes, "notes", "", "Notes describing the firmware")
	uploadCmd.MarkFlagRequired("type")
}

// uploadFirmwareV1 uploads firmware using the V1 API
// PUT /v1/projects/{projectOrProductUID}/firmware/{firmwareType}/{filename}
func uploadFirmwareV1(projectUID, firmwareType, filename, notes string, verbose bool) error {
	// Read the file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Close the writer to set the terminating boundary
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Build URL
	hub := GetAPIHub()
	url := fmt.Sprintf("https://%s/v1/projects/%s/firmware/%s/%s",
		hub, projectUID, firmwareType, filepath.Base(filename))

	// Add query parameters if provided
	if notes != "" {
		url += fmt.Sprintf("?notes=%s", notes)
	}

	// Create HTTP request
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "notehub-client")

	// Add authentication
	err = AddAuthenticationHeader(req)
	if err != nil {
		return fmt.Errorf("failed to set auth header: %w", err)
	}

	if verbose {
		fmt.Printf("PUT %s (file size: %d bytes)\n", url, fileInfo.Size())
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	if verbose {
		fmt.Printf("Upload successful (status: %d)\n", resp.StatusCode)
	}

	return nil
}
