// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Credentials represent Notehub authentication credentials
type Credentials struct {
	User      string     `json:"user,omitempty" mapstructure:"user"`
	Token     string     `json:"token,omitempty" mapstructure:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" mapstructure:"expires_at"`
	Hub       string     `json:"-" mapstructure:"-"`
}

// IsOAuthAccessToken checks if the token is an OAuth access token (vs PAT)
func (creds Credentials) IsOAuthAccessToken() bool {
	personalAccessTokenPrefixes := []string{"ory_st_", "api_key_"}
	for _, prefix := range personalAccessTokenPrefixes {
		if strings.HasPrefix(creds.Token, prefix) {
			return false
		}
	}
	return true
}

// AddHttpAuthHeader adds the authorization header to an HTTP request
func (creds Credentials) AddHttpAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+creds.Token)
}

// IntrospectToken validates a token and returns the associated email
func IntrospectToken(hub string, token string) (string, error) {
	if !strings.HasPrefix(hub, "api.") {
		hub = "api." + hub
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/userinfo", hub), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	userinfo := map[string]interface{}{}
	if err := note.JSONUnmarshal(body, &userinfo); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		err := userinfo["err"]
		return "", fmt.Errorf("%s (http %d)", err, resp.StatusCode)
	}

	if email, ok := userinfo["email"].(string); !ok || email == "" {
		fmt.Printf("response: %s\n", userinfo)
		return "", fmt.Errorf("error introspecting token: no email in response")
	} else {
		return email, nil
	}
}

// Validate checks if credentials are valid
func (creds *Credentials) Validate() error {
	if creds == nil {
		return errors.New("no credentials specified")
	}
	_, err := IntrospectToken(creds.Hub, creds.Token)
	return err
}

// GetHub returns the currently configured Notehub hub
func GetHub() string {
	hub := viper.GetString("hub")
	if hub == "" {
		hub = "notehub.io" // default
	}
	return hub
}

// SetHub sets the Notehub hub
func SetHub(hub string) {
	viper.Set("hub", hub)
}

// GetCredentials returns credentials for the current hub
func GetHubCredentials() (*Credentials, error) {
	hub := GetHub()

	// Viper treats dots in keys as nested paths, so "notehub.io" becomes "notehub.io"
	// We need to access it using the dot notation that Viper creates
	credsMap := viper.GetStringMap(fmt.Sprintf("credentials.%s", hub))
	if len(credsMap) == 0 {
		return nil, nil
	}

	creds := &Credentials{
		Hub: hub,
	}

	if user, ok := credsMap["user"].(string); ok {
		creds.User = user
	}
	if token, ok := credsMap["token"].(string); ok {
		creds.Token = token
	}
	if expiresAt, ok := credsMap["expires_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
			creds.ExpiresAt = &t
		}
	}

	if creds.User == "" || creds.Token == "" {
		return nil, nil
	}

	return creds, nil
}

// SetCredentials sets credentials for the current hub
func SetHubCredentials(token, user string, expiresAt *time.Time) error {
	hub := GetHub()

	// Viper treats dots as path separators, so we use dot notation to set nested values
	// For "notehub.io", this creates credentials.notehub.io structure
	viper.Set(fmt.Sprintf("credentials.%s.user", hub), user)
	viper.Set(fmt.Sprintf("credentials.%s.token", hub), token)
	if expiresAt != nil {
		viper.Set(fmt.Sprintf("credentials.%s.expires_at", hub), expiresAt.Format(time.RFC3339))
	} else {
		viper.Set(fmt.Sprintf("credentials.%s.expires_at", hub), nil)
	}

	return SaveConfig()
}

// RemoveCredentials removes credentials for the current hub
func RemoveHubCredentials() error {
	hub := GetHub()

	credentials, err := GetHubCredentials()
	if err != nil {
		return err
	}
	if credentials == nil {
		return fmt.Errorf("not signed in to %s", hub)
	}

	// If OAuth access token, revoke it
	if credentials.IsOAuthAccessToken() {
		// Revoke token logic would go here if needed
		// For now, we just remove it from config
	}

	// Remove credentials by clearing each field explicitly
	viper.Set(fmt.Sprintf("credentials.%s.user", hub), "")
	viper.Set(fmt.Sprintf("credentials.%s.token", hub), "")
	viper.Set(fmt.Sprintf("credentials.%s.expires_at", hub), "")

	return SaveConfig()
}

// SaveConfig writes the current viper configuration to disk
func SaveConfig() error {
	configPath := getConfigPath()

	// Ensure the config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write the config file
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	// Use the same config directory as lib/config.go
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".notehub", "config.yaml")
	}
	return filepath.Join(home, ".notehub", "config.yaml")
}

// GetAPIHub returns the API hub URL
func GetAPIHub() string {
	hub := GetHub()
	if !strings.HasPrefix(hub, "api.") {
		hub = "api." + hub
	}
	return hub
}

// AddAuthenticationHeader adds authentication header to an HTTP request
func AddAuthenticationHeader(httpReq *http.Request) error {
	credentials, err := GetHubCredentials()
	if err != nil {
		return err
	}

	if credentials == nil {
		hub := GetHub()
		return fmt.Errorf("not authenticated to %s: please use 'notehub auth signin' to sign into the Notehub service", hub)
	}

	// Set the header
	httpReq.Header.Set("Authorization", "Bearer "+credentials.Token)

	return nil
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display current configuration",
	Long:  `Display the current configuration including hub, credentials, and flag values.`,
	Run: func(cmd *cobra.Command, args []string) {
		displayConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// displayConfig prints the current configuration in a readable format
func displayConfig() {
	fmt.Println("\nCurrent Configuration:")
	fmt.Println("=====================")

	// Display hub
	hub := GetHub()
	fmt.Printf("\nHub: %s\n", hub)

	// Display credentials
	credentials, _ := GetHubCredentials()
	if credentials != nil && credentials.User != "" {
		fmt.Println("\nCredentials:")
		fmt.Printf("  %s:\n", hub)
		fmt.Printf("    User: %s\n", credentials.User)

		// Determine token type
		tokenType := "OAuth"
		if !credentials.IsOAuthAccessToken() {
			tokenType = "Personal Access Token"
		}

		// Check expiration
		expires := ""
		if credentials.ExpiresAt != nil {
			if credentials.ExpiresAt.Before(time.Now()) {
				expires = " [EXPIRED]"
			} else {
				expires = fmt.Sprintf(" (expires: %s)", credentials.ExpiresAt.Format("2006-01-02 15:04"))
			}
		}

		fmt.Printf("    Type: %s%s\n", tokenType, expires)
	} else {
		fmt.Println("\nCredentials: None (not signed in)")
	}

	// Display active flag values (only non-empty ones)
	fmt.Println("\nActive Settings:")

	settings := []struct {
		name  string
		value string
	}{
		{"project", viper.GetString("project")},
		{"product", viper.GetString("product")},
		{"device", viper.GetString("device")},
	}

	hasSettings := false
	for _, setting := range settings {
		if setting.value != "" {
			fmt.Printf("  %s: %s\n", setting.name, setting.value)
			hasSettings = true
		}
	}

	if !hasSettings {
		fmt.Println("  (none)")
	}

	// Display config file location
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = getConfigPath()
	}
	fmt.Printf("\nConfig file: %s\n\n", configFile)
}
