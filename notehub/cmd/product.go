// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
	"github.com/blues/note-go/note"
	"github.com/spf13/cobra"
)

// productCmd represents the product command
var productCmd = &cobra.Command{
	Use:   "product",
	Short: "Manage Notehub products",
	Long:  `Commands for listing and managing products in Notehub projects.`,
}

// productListCmd represents the product list command
var productListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all products in a project",
	Long:  `List all products in the current project or a specified project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get products using SDK
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list products: %w", err)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(productsRsp, "", "  ")
			} else {
				output, err = note.JSONMarshal(productsRsp)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		if len(productsRsp.Products) == 0 {
			fmt.Println("No products found in this project.")
			return nil
		}

		// Display products in human-readable format
		fmt.Printf("\nProducts in Project:\n")
		fmt.Printf("====================\n\n")

		for _, product := range productsRsp.Products {
			fmt.Printf("  %s\n", product.Label)
			fmt.Printf("  %s\n\n", product.Uid)
		}

		fmt.Printf("Total products: %d\n\n", len(productsRsp.Products))

		return nil
	},
}

// productGetCmd represents the product get command
var productGetCmd = &cobra.Command{
	Use:   "get [product-uid-or-name]",
	Short: "Get details about a specific product",
	Long:  `Get detailed information about a specific product by UID or name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		GetCredentials() // Validates and exits if not authenticated

		productIdentifier := args[0]

		// Get project UID (from config or --project flag)
		projectUID := GetProject()
		if projectUID == "" {
			return fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
		}

		// Get SDK client
		client := GetNotehubClient()
		ctx, err := GetNotehubContext()
		if err != nil {
			return err
		}

		// Get all products and find the matching one
		productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list products: %w", err)
		}

		// Find the product by UID or name
		var foundProduct *notehub.Product
		for _, product := range productsRsp.Products {
			if product.Uid == productIdentifier || product.Label == productIdentifier {
				foundProduct = &product
				break
			}
		}

		if foundProduct == nil {
			return fmt.Errorf("product '%s' not found in project", productIdentifier)
		}

		// Handle JSON output
		if GetJson() || GetPretty() {
			var output []byte
			var err error
			if GetPretty() {
				output, err = note.JSONMarshalIndent(foundProduct, "", "  ")
			} else {
				output, err = note.JSONMarshal(foundProduct)
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Printf("%s\n", output)
			return nil
		}

		// Display product in human-readable format
		fmt.Printf("\nProduct Details:\n")
		fmt.Printf("================\n\n")
		fmt.Printf("Name: %s\n", foundProduct.Label)
		fmt.Printf("UID: %s\n", foundProduct.Uid)
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(productCmd)
	productCmd.AddCommand(productListCmd)
	productCmd.AddCommand(productGetCmd)
}
