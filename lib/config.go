// Copyright 2019 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
	"github.com/blues/note-go/notehub"
)

// ConfigCreds are the credentials for a given Notehub
type ConfigCreds struct {
	User      string     `json:"user,omitempty"`
	Token     string     `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Hub       string     `json:"-"`
}

func (creds ConfigCreds) IsOAuthAccessToken() bool {
	personalAccessTokenPrefixes := []string{"ory_st_", "api_key_"}
	for _, prefix := range personalAccessTokenPrefixes {
		if strings.HasPrefix(creds.Token, prefix) {
			return false
		}
	}
	return true
}

// ExpiredAt reports whether the credentials have expired as of the given time.  Expired
// credentials are as good as none, because the hub will reject them.
func (creds ConfigCreds) ExpiredAt(now time.Time) bool {
	return creds.ExpiresAt != nil && !creds.ExpiresAt.After(now)
}

func (creds ConfigCreds) AddHttpAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+creds.Token)
}

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

func (creds *ConfigCreds) Validate() error {
	if creds == nil {
		return errors.New("no credentials specified")
	}
	_, err := IntrospectToken(creds.Hub, creds.Token)
	return err
}

// Port/PortConfig on a per-interface basis
type ConfigPort struct {
	Port       string `json:"port,omitempty"`
	PortConfig int    `json:"port_config,omitempty"`
}

// ConfigSettings defines the config file that maintains the command processor's state
type ConfigSettings struct {
	When      string                 `json:"when,omitempty"`
	Hub       string                 `json:"hub,omitempty"`
	HubCreds  map[string]ConfigCreds `json:"creds,omitempty"`
	Interface string                 `json:"interface,omitempty"`
	IPort     map[string]ConfigPort  `json:"iport,omitempty"`
}

// Config are the master config settings
var config *ConfigSettings
var configFlagHub string
var configFlagInterface string
var configFlagPort string
var configFlagPortConfig int

func (config *ConfigSettings) Write() error {
	// Marshal it
	configJSON, err := note.JSONMarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("can't marshal configuration: %s", err)
	}

	// Write the file
	configPath := configSettingsPath()
	fd, err := os.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	if _, err := fd.Write(configJSON); err != nil {
		return fmt.Errorf("can't write %s: %s", configPath, err)
	}
	return fd.Close()
}

func (config *ConfigSettings) Print() {
	fmt.Printf("\nCurrently saved values:\n")

	if config.Hub != "" {
		fmt.Printf("       hub: %s\n", config.Hub)
	}
	// Expired credentials are not shown, because we are effectively not signed in to
	// that hub, and a hub we are not signed in to is not a saved value worth showing
	now := time.Now()
	shownCreds := false
	for hub, cred := range config.HubCreds {
		if cred.ExpiredAt(now) {
			continue
		}
		if !shownCreds {
			fmt.Printf("     creds:\n")
			shownCreds = true
		}

		tokenType := "PAT"
		if cred.IsOAuthAccessToken() {
			tokenType = "OAuth"
		}

		expires := ""
		if cred.ExpiresAt != nil {
			expires = fmt.Sprintf(" (expires at %s)", cred.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		}
		fmt.Printf("            %s: %s (%s)%s\n", hub, cred.User, tokenType, expires)
	}
	// These are shown as the options that would set them, so that they can be copied
	// straight onto a command line.  They are spelled with two hyphens because that is
	// the spelling we now document, and the single-hyphen spelling means the same thing
	if config.Interface != "" {
		fmt.Printf("   --interface %s\n", config.Interface)

		configPort := config.IPort[config.Interface]
		if configPort.Port == "" {
			fmt.Printf("   --port -\n")
			fmt.Printf("   --portconfig -\n")
		} else {
			fmt.Printf("   --port %s\n", configPort.Port)
			fmt.Printf("   --portconfig %d\n", configPort.PortConfig)
		}
	}
}

func (config *ConfigSettings) DefaultCredentials() *ConfigCreds {
	if creds, present := config.HubCreds[config.Hub]; present && creds.Token != "" && creds.User != "" {
		creds.Hub = config.Hub
		return &creds
	}
	return nil
}

// clear credentials for the currently config.Hub value
// if the credentials are from OAuth, then revoke the access token
func (config *ConfigSettings) RemoveDefaultCredentials() error {
	credentials, present := config.HubCreds[config.Hub]
	if !present {
		return fmt.Errorf("not signed in to %s", config.Hub)
	}

	if !credentials.IsOAuthAccessToken() {
		notehub.RevokeAccessToken(credentials.Hub, credentials.Token)
	}

	// remove the credentials, and write the credentials file
	delete(config.HubCreds, config.Hub)
	return config.Write()
}

func (config *ConfigSettings) SetDefaultCredentials(token string, email string, expiresAt *time.Time) {
	config.HubCreds[config.Hub] = ConfigCreds{
		Hub:       config.Hub,
		User:      email,
		Token:     token,
		ExpiresAt: expiresAt,
	}
}

func defaultConfig() *ConfigSettings {
	iface, port, portConfig := notecard.Defaults()
	return &ConfigSettings{
		When:     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Hub:      notehub.DefaultAPIService,
		HubCreds: map[string]ConfigCreds{},
		IPort: map[string]ConfigPort{
			iface: {
				Port:       port,
				PortConfig: portConfig,
			},
		},
	}
}

// returns (nil, nil) if there's no config file
// returns (non-nil, nil) if a config file was read from the filesystem successfully
// returns (nil, non-nil) if there was some any other error
func readConfigFromFile() (*ConfigSettings, error) {
	// Read the config file
	configPath := configSettingsPath()
	contents, err := os.ReadFile(configPath)

	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("can't read %s: %s", configPath, err)
	}

	var configFromFile ConfigSettings
	if err := note.JSONUnmarshal(contents, &configFromFile); err != nil {
		return nil, fmt.Errorf("can't parse %s: %s", configPath, err)
	}

	return &configFromFile, nil
}

func GetConfig() (*ConfigSettings, error) {
	if config == nil {
		// try reading it from the filesystem
		// otherwise, use a new default config
		if configFromFile, err := readConfigFromFile(); err != nil {
			return nil, err
		} else if configFromFile != nil {
			config = configFromFile
		} else {
			config = defaultConfig()
		}
	}
	if config.IPort == nil {
		config.IPort = make(map[string]ConfigPort)
	}
	if config.HubCreds == nil {
		config.HubCreds = make(map[string]ConfigCreds)
	}
	return config, nil
}

// ConfigRead reads the current info from config file
func ConfigRead() error {

	// As a convenience to all tools, generate a new random seed for each iteration
	rand.Seed(time.Now().UnixNano())
	rand.Seed(rand.Int63() ^ time.Now().UnixNano())

	// Read the config file
	configPath := configSettingsPath()
	contents, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		// If no interface has been provided and no saved config,
		// set it to a default value and write it
		config = defaultConfig()
		return config.Write()
	} else if err != nil {
		return fmt.Errorf("can't read %s: %s", configPath, err)
	}

	var newConfig ConfigSettings
	if err := note.JSONUnmarshal(contents, &newConfig); err != nil {
		return fmt.Errorf("can't parse %s: %s", configPath, err)
	}
	config = &newConfig
	if config.IPort == nil {
		config.IPort = make(map[string]ConfigPort)
	}
	if config.HubCreds == nil {
		config.HubCreds = make(map[string]ConfigCreds)
	}

	return nil
}

// load current config, apply CLI flag values, and save if appropriate
func ConfigFlagsProcess() (err error) {
	config, err := GetConfig()
	if err != nil {
		return
	}

	if configFlagInterface == "-" {
		config = defaultConfig()
	} else if configFlagInterface != "" {
		config.Interface = configFlagInterface
	}

	// Set or reset the flags as desired
	if configFlagHub != "" {
		ConfigSetHub(configFlagHub)
	}
	if config.Hub == "" {
		config.Hub = notehub.DefaultAPIService
	}

	defaultPort := config.IPort[config.Interface]

	if configFlagPort == "-" {
		defaultPort.Port = ""
	} else if configFlagPort != "" {
		defaultPort.Port = configFlagPort
	}

	if configFlagPortConfig < 0 {
		defaultPort.PortConfig = 0
	} else if configFlagPortConfig != 0 {
		defaultPort.PortConfig = configFlagPortConfig
	}

	config.IPort[config.Interface] = defaultPort

	// Done
	return nil
}

// ConfigFlagsRegister registers the config-related flags
func ConfigFlagsRegister(notecardFlags bool, notehubFlags bool) {

	// Process the commands
	if notecardFlags {
		flag.StringVar(&configFlagInterface, "interface", "", "select 'serial' or 'i2c' interface for Notecard")
		flag.StringVar(&configFlagPort, "port", "", "select serial or i2c port for Notecard")
		flag.IntVar(&configFlagPortConfig, "portconfig", 0, "set serial device speed or i2c address for Notecard")
	}
	if notehubFlags {
		flag.StringVar(&configFlagHub, "hub", "", "set Notehub domain")
	}

}

// FlagParse is a wrapper around flag.Parse that handles our config flags
func FlagParse(notecardFlags bool, notehubFlags bool) (err error) {

	// Register our flags
	ConfigFlagsRegister(notecardFlags, notehubFlags)

	// Parse them
	flag.Parse()

	// Process our flags
	err = ConfigFlagsProcess()
	if err != nil {
		return
	}

	// Save only when configuration flags are the entire request.  Use parsed names
	// so -hub value, --hub value and --hub=value all have the same behavior.
	if configFlagsOnly(flag.CommandLine) && config.Interface != "lease" {
		if err := config.Write(); err != nil {
			return fmt.Errorf("could not write config file: %w", err)
		}
	}

	// Override, just for this session, with env vars
	if iface := os.Getenv("NOTE_INTERFACE"); iface != "" {
		config.Interface = iface
	}

	// Override via env vars if specified
	if port := os.Getenv("NOTE_PORT"); port != "" {
		temp := config.IPort[config.Interface]
		temp.Port = port
		if portConfig, err := strconv.Atoi(os.Getenv("NOTE_PORT_CONFIG")); err == nil {
			temp.PortConfig = portConfig
		}
		config.IPort[config.Interface] = temp
	}

	// Done
	return

}

// configFlagsOnly reports whether the command line only sets configuration, with
// no operation flags or positional arguments that would make the settings temporary.
func configFlagsOnly(flags *flag.FlagSet) bool {
	if flags.NFlag() == 0 || flags.NArg() != 0 {
		return false
	}
	configOnly := true
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "interface", "port", "portconfig", "hub":
		default:
			configOnly = false
		}
	})
	return configOnly
}

// ConfigSignedIn returns info about whether or not we're signed in
//
// TODO: check credentials by issuing an HTTP request
// maybe this should return an error if a PAT is expired?
// what happens if an access token is expired?
func ConfigSignedIn() *ConfigCreds {
	if creds, present := config.HubCreds[config.Hub]; present && creds.Token != "" && creds.User != "" {
		creds.Hub = config.Hub
		return &creds
	}
	return nil
}

// ConfigAuthenticationHeader sets the authorization field in the header as appropriate
func ConfigAuthenticationHeader(httpReq *http.Request) error {
	// Exit if not signed in
	credentials := ConfigSignedIn()
	if credentials == nil {
		return fmt.Errorf("not authenticated to %s: please use 'notehub --signin' to sign into the Notehub service", config.Hub)
	}

	// Set the header
	httpReq.Header.Set("Authorization", "Bearer "+credentials.Token)

	// Done
	return nil

}

// ConfigAPIHub returns the configured notehub, for use by the HTTP API.  If none is configured it returns
// the default Blues API service.  Regardless, it always makes sure that the host has "api." as a prefix.
// This enables flexibility in what's configured.
func ConfigAPIHub() (hub string) {
	hub = config.Hub
	if hub == "" || hub == "-" {
		hub = notehub.DefaultAPIService
	}
	if !strings.HasPrefix(hub, "api.") {
		hub = "api." + hub
	}
	return
}

// ConfigNotecardHub returns the configured notehub, for use as the Notecard host.  If none is configured
// it returns "".  Regardless, it always makes sure that the host does NOT have "api." as a prefix.
func ConfigNotecardHub() (hub string) {
	hub = config.Hub
	if hub == "" || hub == "-" {
		hub = notehub.DefaultAPIService
	}
	hub = strings.TrimPrefix(hub, "api.")
	return
}

// ConfigSetHub clears the hub
func ConfigSetHub(hub string) {
	if hub == "-" || hub == "" {
		hub = notehub.DefaultAPIService
	}
	config.Hub = hub
}
