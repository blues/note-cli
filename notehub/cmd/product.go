// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"

	notehub "github.com/blues/notehub-go"
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
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to list products: %w", err)
		}

		return printListResult(cmd, productsRsp, "No products found in this project.", func() bool {
			return len(productsRsp.Products) == 0
		})
	},
}

// productGetCmd represents the product get command
var productGetCmd = &cobra.Command{
	Use:   "get [product-uid-or-name]",
	Short: "Get details about a specific product",
	Long:  `Get detailed information about a specific product by UID or name. If no argument is provided, uses the active product (set with 'product set'). If no active product is configured, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var productIdentifier string
		if len(args) > 0 {
			productIdentifier = args[0]
		} else if def := GetProduct(); def != "" {
			productIdentifier = def
		} else {
			productIdentifier, err = pickProduct(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		foundProduct, err := resolveProduct(client, ctx, projectUID, productIdentifier)
		if err != nil {
			return err
		}

		return printResult(cmd, foundProduct)
	},
}

var productCreateCmd = &cobra.Command{
	Use:   "create [label] [product-uid]",
	Short: "Create a new product",
	Long:  `Create a new product in the current project. The product UID will be prefixed with the user's reversed email.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := args[0]
		productUID := args[1]

		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		createReq := notehub.NewCreateProductRequest(label, productUID)

		createdProduct, _, err := client.ProjectAPI.CreateProduct(ctx, projectUID).
			CreateProductRequest(*createReq).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to create product: %w", err)
		}

		return printMutationResult(cmd, createdProduct, "Product created")
	},
}

var productDeleteCmd = &cobra.Command{
	Use:   "delete [product-uid-or-name]",
	Short: "Delete a product",
	Long:  `Delete a product from the current project. If no argument is provided, an interactive picker will be shown.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var productUID string
		if len(args) > 0 {
			product, err := resolveProduct(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
			productUID = product.Uid
		} else {
			productUID, err = pickProduct(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
		}

		_, err = client.ProjectAPI.DeleteProduct(ctx, projectUID, productUID).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete product: %w", err)
		}

		return printActionResult(cmd, map[string]any{
			"action":      "delete",
			"product_uid": productUID,
		}, fmt.Sprintf("Product '%s' deleted", productUID))
	},
}

// productSetCmd represents the product set command
var productSetCmd = &cobra.Command{
	Use:   "set [product-uid-or-name]",
	Short: "Set the active product",
	Long: `Set the active product in the configuration. You can specify either the product name or UID.
If no argument is provided, an interactive picker will be shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, projectUID, err := initCommand()
		if err != nil {
			return err
		}

		var selectedProduct *notehub.Product
		if len(args) > 0 {
			selectedProduct, err = resolveProduct(client, ctx, projectUID, args[0])
			if err != nil {
				return err
			}
		} else {
			productUID, err := pickProduct(client, ctx, projectUID)
			if err == errPickCancelled {
				return nil
			}
			if err != nil {
				return err
			}
			selectedProduct, err = resolveProduct(client, ctx, projectUID, productUID)
			if err != nil {
				return err
			}
		}

		return setDefault(cmd, "product", selectedProduct.Uid, selectedProduct.Label)
	},
}

// productClearCmd represents the product clear command
var productClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active product",
	Long:  `Clear the active product from the configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearDefault(cmd, "product", "notehub product set <name-or-uid>")
	},
}

func init() {
	rootCmd.AddCommand(productCmd)
	productCmd.AddCommand(productListCmd)
	productCmd.AddCommand(productGetCmd)
	productCmd.AddCommand(productCreateCmd)
	productCmd.AddCommand(productDeleteCmd)
	productCmd.AddCommand(productSetCmd)
	productCmd.AddCommand(productClearCmd)
}
