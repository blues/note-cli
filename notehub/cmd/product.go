// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

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

		// Define product types
		type Product struct {
			UID   string `json:"uid"`
			Label string `json:"label"`
		}

		type ProductsResponse struct {
			Products []Product `json:"products"`
		}

		// Get products using V1 API: GET /v1/projects/{projectUID}/products
		productsRsp := ProductsResponse{}
		url := fmt.Sprintf("/v1/projects/%s/products", projectUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &productsRsp)
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
			fmt.Printf("  %s\n\n", product.UID)
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

		// Define product type
		type Product struct {
			UID   string `json:"uid"`
			Label string `json:"label"`
		}

		type ProductsResponse struct {
			Products []Product `json:"products"`
		}

		// Get all products and find the matching one
		productsRsp := ProductsResponse{}
		url := fmt.Sprintf("/v1/projects/%s/products", projectUID)
		err := reqHubV1(GetVerbose(), GetAPIHub(), "GET", url, nil, &productsRsp)
		if err != nil {
			return fmt.Errorf("failed to list products: %w", err)
		}

		// Find the product by UID or name
		var foundProduct *Product
		for _, product := range productsRsp.Products {
			if product.UID == productIdentifier || product.Label == productIdentifier {
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
		fmt.Printf("UID: %s\n", foundProduct.UID)
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(productCmd)
	productCmd.AddCommand(productListCmd)
	productCmd.AddCommand(productGetCmd)
}
