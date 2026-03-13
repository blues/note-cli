// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// usageCmd represents the usage command
var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "View Notehub usage data",
	Long:  `Commands for viewing usage data in Notehub projects.`,
}

// usageDataCmd represents the usage data command
var usageDataCmd = &cobra.Command{
	Use:   "data",
	Short: "Get data usage",
	Long:  `Get data usage statistics for the current project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.UsageAPI.GetDataUsage(ctx, projectUID)

		if period, _ := cmd.Flags().GetString("period"); period != "" {
			req = req.Period(period)
		}
		if device, _ := cmd.Flags().GetString("device"); device != "" {
			req = req.DeviceUID([]string{device})
		}
		if fleet, _ := cmd.Flags().GetString("fleet"); fleet != "" {
			req = req.FleetUID([]string{fleet})
		}

		result, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get data usage: %w", err)
		}

		// Usage data is best viewed as JSON
		return printJSON(cmd, result)
	},
}

// usageEventsCmd represents the usage events command
var usageEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Get events usage",
	Long:  `Get events usage statistics for the current project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.UsageAPI.GetEventsUsage(ctx, projectUID)

		if period, _ := cmd.Flags().GetString("period"); period != "" {
			req = req.Period(period)
		}
		if device, _ := cmd.Flags().GetString("device"); device != "" {
			req = req.DeviceUID([]string{device})
		}
		if fleet, _ := cmd.Flags().GetString("fleet"); fleet != "" {
			req = req.FleetUID([]string{fleet})
		}

		result, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get events usage: %w", err)
		}

		// Usage data is best viewed as JSON
		return printJSON(cmd, result)
	},
}

// usageRoutesCmd represents the usage routes command
var usageRoutesCmd = &cobra.Command{
	Use:   "routes",
	Short: "Get route logs usage",
	Long:  `Get route logs usage statistics for the current project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.UsageAPI.GetRouteLogsUsage(ctx, projectUID)

		if period, _ := cmd.Flags().GetString("period"); period != "" {
			req = req.Period(period)
		}
		if route, _ := cmd.Flags().GetString("route"); route != "" {
			req = req.RouteUID([]string{route})
		}

		result, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get route logs usage: %w", err)
		}

		// Usage data is best viewed as JSON
		return printJSON(cmd, result)
	},
}

// usageSessionsCmd represents the usage sessions command
var usageSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Get sessions usage",
	Long:  `Get sessions usage statistics for the current project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		req := client.UsageAPI.GetSessionsUsage(ctx, projectUID)

		if period, _ := cmd.Flags().GetString("period"); period != "" {
			req = req.Period(period)
		}
		if device, _ := cmd.Flags().GetString("device"); device != "" {
			req = req.DeviceUID([]string{device})
		}
		if fleet, _ := cmd.Flags().GetString("fleet"); fleet != "" {
			req = req.FleetUID([]string{fleet})
		}

		result, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("failed to get sessions usage: %w", err)
		}

		// Usage data is best viewed as JSON
		return printJSON(cmd, result)
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.AddCommand(usageDataCmd)
	usageCmd.AddCommand(usageEventsCmd)
	usageCmd.AddCommand(usageRoutesCmd)
	usageCmd.AddCommand(usageSessionsCmd)

	// Add shared flags to data, events, sessions subcommands
	for _, cmd := range []*cobra.Command{usageDataCmd, usageEventsCmd, usageSessionsCmd} {
		cmd.Flags().String("period", "", "Time period (e.g., 30d, 7d)")
		cmd.Flags().String("device", "", "Filter by device UID")
		cmd.Flags().String("fleet", "", "Filter by fleet UID")
	}

	// Route logs usage has different filters
	usageRoutesCmd.Flags().String("period", "", "Time period (e.g., 30d, 7d)")
	usageRoutesCmd.Flags().String("route", "", "Filter by route UID")
}
