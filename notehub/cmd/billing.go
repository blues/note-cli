// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// billingCmd represents the billing command
var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Manage billing accounts",
	Long:  `Commands for listing billing accounts.`,
}

// billingListCmd represents the billing list command
var billingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List billing accounts",
	Long:  `List all billing accounts accessible by the current authentication.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		billingRsp, _, err := client.BillingAccountAPI.GetBillingAccounts(ctx).Execute()
		if err != nil {
			return fmt.Errorf("failed to list billing accounts: %w", err)
		}

		return printListResult(cmd, billingRsp, "No billing accounts found.", func() bool {
			return len(billingRsp.BillingAccounts) == 0
		})
	},
}

func init() {
	rootCmd.AddCommand(billingCmd)
	billingCmd.AddCommand(billingListCmd)
}
