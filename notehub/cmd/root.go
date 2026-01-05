// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CLI Version - Set by ldflags during build/release
var version = "development"

// Global flags
var (
	flagProject string
	flagProduct string
	flagDevice  string
	flagVerbose bool
	flagPretty  bool
	flagJson    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "notehub",
	Short: "Notehub CLI - Command-line tool for interacting with Notehub",
	Long: `Notehub CLI is a command-line tool for interacting with Blues Notehub.

It provides commands for authentication, managing projects and devices,
setting environment variables, and making API requests.`,
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help (config shown via HelpFunc)
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Initialize config before running any command
	cobra.OnInitialize(initConfig)

	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Project UID")
	rootCmd.PersistentFlags().StringVar(&flagProduct, "product", "", "Product UID")
	rootCmd.PersistentFlags().StringVarP(&flagDevice, "device", "d", "", "Device UID")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Display requests and responses")
	rootCmd.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "Pretty print JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagJson, "json", false, "Strip all non-JSON lines from output")

	// Bind flags to Viper (allows flags to override config file values)
	viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
	viper.BindPFlag("product", rootCmd.PersistentFlags().Lookup("product"))
	viper.BindPFlag("device", rootCmd.PersistentFlags().Lookup("device"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("pretty", rootCmd.PersistentFlags().Lookup("pretty"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))

	// Enable environment variable support (NOTEHUB_PROJECT, NOTEHUB_DEVICE, etc.)
	viper.SetEnvPrefix("NOTEHUB")
	viper.AutomaticEnv()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Get user home directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %s\n", err)
		os.Exit(1)
	}

	// Set config file location
	configDir := filepath.Join(home, ".notehub")
	viper.AddConfigPath(configDir)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("hub", "notehub.io")

	// Attempt to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; create with defaults
			if err := os.MkdirAll(configDir, 0755); err != nil {
				fmt.Printf("Error creating config directory: %s\n", err)
				os.Exit(1)
			}
			// Write default config
			if err := SaveConfig(); err != nil {
				fmt.Printf("Error creating config file: %s\n", err)
				os.Exit(1)
			}
		} else {
			// Config file was found but another error was produced
			fmt.Printf("Error reading config file: %s\n", err)
			os.Exit(1)
		}
	}
}

// GetCredentials returns validated credentials or exits with error
func GetCredentials() *Credentials {
	credentials, err := GetHubCredentials()
	if err != nil {
		fmt.Printf("error getting credentials: %s\n", err)
		os.Exit(1)
	}

	if credentials == nil {
		fmt.Printf("please sign in using 'notehub auth signin' or 'notehub auth signin-token'\n")
		os.Exit(1)
	}

	if err := credentials.Validate(); err != nil {
		hub := GetHub()
		fmt.Printf("invalid credentials for %s: %s\n", hub, err)
		fmt.Printf("please use 'notehub auth signin' or 'notehub auth signin-token' to sign into Notehub\n")
		os.Exit(1)
	}

	return credentials
}

// Helper functions to get flag values from Viper
// These allow flags, config file, and environment variables to work together

func GetProject() string {
	return viper.GetString("project")
}

func GetProduct() string {
	return viper.GetString("product")
}

func GetDevice() string {
	return viper.GetString("device")
}

func GetVerbose() bool {
	return viper.GetBool("verbose")
}

func GetPretty() bool {
	return viper.GetBool("pretty")
}

func GetJson() bool {
	return viper.GetBool("json")
}
