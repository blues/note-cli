// Copyright 2021 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// traceCmd represents the trace command
var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Enter interactive trace mode",
	Long: `Enter an interactive trace mode to send requests to Notehub.

In trace mode, you can:
  - Set project, product, and device context
  - Send JSON requests
  - Make HTTPS GET, POST, PUT, DELETE requests
  - Ping the Notehub
  - Type ? for help

Example:
  notehub trace --project app:xxxx --device dev:yyyy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validate credentials

		// Set initial context
		reqFlagApp = GetProject()
		reqFlagProduct = GetProduct()
		reqFlagDevice = GetDevice()

		return traceMode()
	},
}

func init() {
	rootCmd.AddCommand(traceCmd)
}

// Command definitions
type cmdDef struct {
	Command string
	Desc    string
}

func validCommands() []cmdDef {
	return []cmdDef{
		{"product", "set productUID for requests made in this session"},
		{"project", "set projectUID (appUID) for requests made in this session"},
		{"device", "set deviceUID for requests made in this session"},
		{"hub", "set notehub domain for requests made in this session"},
		{"get", "HTTPS GET from specified URL"},
		{"put", "HTTPS PUT to specified URL of the JSON prompted for on next line"},
		{"post", "HTTPS POST of specified URL of the JSON prompted for on next line"},
		{"delete", "HTTPS DELETE of resource at specified URL"},
		{"ping", "ping the notehub"},
		{"q", "quit"},
	}
}

// Enter a diagnostic trace mode
func traceMode() error {
	// Create a scanner to watch stdin
	scanner := bufio.NewScanner(os.Stdin)
	var cmd string

traceloop:
	for {
		// Get next text line
		fmt.Print("> ")
		scanner.Scan()
		cmd = scanner.Text()

		// Parse into arguments
		r := regexp.MustCompile(`[^\s"']+|"([^"]*)"|'([^']*)`)
		args := r.FindAllString(cmd, -1)
		for i := 0; i < 10; i++ {
			args = append(args, "")
		}
		cmdAfter0 := strings.TrimPrefix(cmd, args[0]+" ")

		// Process JSON requests
		if strings.HasPrefix(cmd, "{") {
			_, err := reqHubV0JSON(true, GetAPIHub(), []byte(cmd), "", "", "", "", false, false, nil)
			if err != nil {
				fmt.Printf("error: %s\n", err)
			}
			continue
		}

		// Create clean IDs to work with in the commands
		cleanProduct := reqFlagProduct
		if cleanProduct != "" && !strings.HasPrefix(cleanProduct, "product:") {
			cleanProduct = "product:" + reqFlagProduct
		}
		cleanApp := reqFlagApp
		if !strings.HasPrefix(cleanApp, "app:") {
			if cleanApp == "" {
				cleanApp = cleanProduct
			} else {
				cleanApp = "app:" + reqFlagApp
			}
		}
		cleanDevice := reqFlagDevice
		if !strings.HasPrefix(cleanDevice, "dev:") {
			cleanDevice = "dev:" + reqFlagDevice
		}
		cmdAfter0 = strings.Replace(cmdAfter0, "{productUID}", cleanProduct, -1)
		cmdAfter0 = strings.Replace(cmdAfter0, "{projectUID}", cleanApp, -1)
		cmdAfter0 = strings.Replace(cmdAfter0, "{deviceUID}", cleanDevice, -1)

		// Dispatch command
		switch args[0] {
		case "?":
			fmt.Printf("Trace commands:\n")
			for _, c := range validCommands() {
				fmt.Printf("%s: %s\n", c.Command, c.Desc)
			}
			fmt.Printf("{\"req\":...} for a JSON request\n")
		case "product":
			if args[1] != "" {
				if args[1] == "-" {
					args[1] = ""
				}
				reqFlagProduct = args[1]
			}
			fmt.Printf("productUID is %s\n", reqFlagProduct)

		case "project":
			fallthrough
		case "app":
			if args[1] != "" {
				if args[1] == "-" {
					args[1] = ""
				}
				reqFlagApp = args[1]
			}
			fmt.Printf("projectUID is %s\n", reqFlagApp)

		case "device":
			if args[1] != "" {
				if args[1] == "-" {
					args[1] = ""
				}
				reqFlagDevice = args[1]
			}
			fmt.Printf("deviceUID is %s\n", reqFlagDevice)

		case "hub":
			if args[1] != "" {
				if args[1] == "-" {
					args[1] = ""
				}
				SetHub(args[1])
				SaveConfig()
			}
			fmt.Printf("hub is %s\n", GetHub())

		case "get":
			fallthrough
		case "delete":
			fallthrough
		case "put":
			fallthrough
		case "post":
			// Get the body to post/put
			var bodyJSON []byte
			if args[0] == "put" || args[0] == "post" {
				fmt.Print("JSON> ")
				scanner.Scan()
				bodyJSON = []byte(scanner.Text())
			}

			// Make sure that it's a well-formed URL for our API
			url := cmdAfter0
			if !strings.HasPrefix(url, "/") {
				url = "/" + url
			}
			if !strings.HasPrefix(url, "/v1/") {
				url = "/v1" + url
			}

			// Perform the transaction
			_, err := reqHubV1JSON(true, GetAPIHub(), args[0], url, bodyJSON)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return err
			}
		case "ping":
			_, err := reqHubV1JSON(true, GetAPIHub(), "GET", "/ping", nil)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return err
			}
			if cleanApp != "" {
				url := "/v1/products/" + cleanApp + "/products"
				_, err = reqHubV1JSON(true, GetAPIHub(), "GET", url, nil)
				if err != nil {
					fmt.Printf("error: %s\n", err)
					return err
				}
			}
		case "q":
			break traceloop
		case "":
			// ignore
		default:
			fmt.Printf("%s ???\n", args[0])
		}
	}
	return nil
}
