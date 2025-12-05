// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/blues/note-go/note"
	"github.com/spf13/cobra"
)

var (
	flagOut string
)

// requestCmd represents the request command
var requestCmd = &cobra.Command{
	Use:     "request [json]",
	Aliases: []string{"req"},
	Short:   "Send an API request to Notehub",
	Long: `Send a raw API request to Notehub.

The JSON argument can be a JSON object or @filename to read from a file.

Example:
  notehub request '{"req":"hub.app.get"}' --project app:xxxx
  notehub req @request.json --device dev:xxxx --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		reqArg := args[0]

		// Process request starting with @ as a filename
		if strings.HasPrefix(reqArg, "@") {
			fn := strings.TrimPrefix(reqArg, "@")
			contents, err := os.ReadFile(fn)
			if err != nil {
				return fmt.Errorf("can't read request file '%s': %w", fn, err)
			}
			reqArg = string(contents)
		}

		// Set flags for the request functions
		reqFlagApp = GetProject()
		reqFlagProduct = GetProduct()
		reqFlagDevice = GetDevice()

		// Perform the request
		rsp, err := reqHubV0JSON(GetVerbose(), GetAPIHub(), []byte(reqArg), "", "", "", "", false, GetJson(), nil)
		if err != nil {
			return err
		}

		// Handle output
		if flagOut == "" {
			if GetPretty() {
				var rspo map[string]interface{}
				err = note.JSONUnmarshal(rsp, &rspo)
				if err != nil {
					fmt.Printf("%s", rsp)
				} else {
					rsp, _ = note.JSONMarshalIndent(rspo, "", "    ")
					fmt.Printf("%s", rsp)
				}
			} else {
				fmt.Printf("%s", rsp)
			}
		} else {
			outfile, err := os.Create(flagOut)
			if err != nil {
				return err
			}
			defer outfile.Close()
			outfile.Write(rsp)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(requestCmd)

	requestCmd.Flags().StringVarP(&flagOut, "out", "o", "", "Output filename")
}
