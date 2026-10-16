// Copyright 2019 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
	"github.com/blues/note-go/notehub"
)

// ConfigCreds are the credentials for a given Notehub
type ConfigCreds struct {
	User         string     `json:"user,omitempty"`
	Token        string     `json:"token,omitempty"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Hub          string     `json:"-"`
	// Source is the environment variable the token came from, or "" for a saved sign-in
	Source string `json:"-"`
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

// ExpiredAt reports whether the credentials had expired by the given time
func (creds ConfigCreds) ExpiredAt(now time.Time) bool {
	return creds.ExpiresAt != nil && !creds.ExpiresAt.After(now)
}

// ConfigTimeFormat is how a time is displayed, as in "expires at 2026-01-02 15:04:05 MST"
const ConfigTimeFormat = "2006-01-02 15:04:05 MST"

// ExpirationDescription says when the credentials expire, as "expires at <time>", or
// expired, as "expired at <time>", or "" when no expiration is recorded, as for a
// personal access token, whose expiration Notehub keeps
func (creds ConfigCreds) ExpirationDescription(now time.Time) string {
	switch {
	case creds.ExpiresAt == nil:
		return ""
	case creds.ExpiredAt(now):
		return "expired at " + creds.ExpiresAt.Format(ConfigTimeFormat)
	}
	return "expires at " + creds.ExpiresAt.Format(ConfigTimeFormat)
}

func (creds ConfigCreds) AddHttpAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+creds.Token)
}

// ErrCredentialsRejected distinguishes an invalid login from a service or network
// failure. Only an explicit authentication rejection warrants a new sign-in.
var ErrCredentialsRejected = errors.New("credentials rejected")

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
	return introspectionResponse(resp)
}

func introspectionResponse(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("%w (http %d)", ErrCredentialsRejected, resp.StatusCode)
	}

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
	When              string                 `json:"when,omitempty"`
	Hub               string                 `json:"hub,omitempty"`
	HubCreds          map[string]ConfigCreds `json:"creds,omitempty"`
	Interface         string                 `json:"interface,omitempty"`
	IPort             map[string]ConfigPort  `json:"iport,omitempty"`
	credentialChanges map[string]bool
}

// Config are the master config settings
var config *ConfigSettings
var configFlagHub string
var configFlagInterface string
var configFlagPort string
var configFlagPortConfig int

func (config *ConfigSettings) Write() error {
	return withConfigLock(context.Background(), func() error {
		stored, err := readConfigFromFile()
		if err != nil {
			return err
		}
		updated := *config
		// An unrelated settings write must not restore tokens rotated by another CLI.
		if stored != nil {
			updated.HubCreds = stored.HubCreds
			if updated.HubCreds == nil {
				updated.HubCreds = map[string]ConfigCreds{}
			}
			for hub := range config.credentialChanges {
				updated.HubCreds[hub] = config.HubCreds[hub]
			}
		}
		contents, err := updated.savedJSON()
		if err != nil {
			return err
		}
		if err = writeConfigJSON(contents); err != nil {
			return err
		}
		config.HubCreds, config.credentialChanges = updated.HubCreds, nil
		return nil
	})
}

// configSavedHub is the hub the configuration file names, as read before any --hub given
// for this command alone, or "" when the flags haven't been processed
var configSavedHub string

// savedJSON is the configuration as it is written: with the saved hub, so that a --hub
// given for one command doesn't become the hub of the commands that follow unless
// AdoptHub says so
func (config *ConfigSettings) savedJSON() ([]byte, error) {
	saved := *config
	if configSavedHub != "" {
		saved.Hub = configSavedHub
	}
	return note.JSONMarshalIndent(&saved, "", "    ")
}

// AdoptHub makes the hub in use the saved one, for the commands that follow, when the
// configuration is next written
func (config *ConfigSettings) AdoptHub() {
	configSavedHub = config.Hub
}

// ConfigSavedHub returns the hub the configuration names: the one in use between
// commands, whatever --hub was given for this one
func ConfigSavedHub() string {
	if configSavedHub != "" {
		return configSavedHub
	}
	if config != nil {
		return config.Hub
	}
	return ""
}

// PrintSettings shows the saved settings a tool uses: the Notecard's interface and port
// for one that talks to a Notecard, and the hub and its logins for one that talks to
// Notehub
func (config *ConfigSettings) PrintSettings(notecardSettings bool, notehubSettings bool) {
	if notehubSettings && config.EnvCredentials() != nil {
		fmt.Printf("\n%s is set, and its token is used for every request in place of the saved credentials.\n", ConfigTokenEnvVar)
	}
	fmt.Printf("\nCurrently saved values:\n")
	shown := false
	if notehubSettings {
		shown = config.printNotehubSettings() || shown
	}
	if notecardSettings {
		shown = config.printNotecardSettings() || shown
	}
	if !shown {
		fmt.Printf("   none\n")
	}
}

// printNotehubSettings shows the hub and each hub's login, and reports whether there
// were any
func (config *ConfigSettings) printNotehubSettings() (shown bool) {
	if config.Hub != "" {
		fmt.Printf("       hub: %s\n", config.Hub)
		shown = true
	}
	// Each hub's login, in a fixed order, with when it expires, or expired
	now := time.Now()
	hubs := []string{}
	for hub := range config.HubCreds {
		hubs = append(hubs, hub)
	}
	sort.Strings(hubs)
	shownCreds := false
	for _, hub := range hubs {
		cred := config.HubCreds[hub]

		tokenType := "token"
		if cred.IsOAuthAccessToken() {
			tokenType = "login"
		}

		expires := ""
		if description := cred.ExpirationDescription(now); description != "" {
			expires = " " + description
		}
		if !shownCreds {
			shownCreds = true
			fmt.Printf("     creds: %s: %s (%s)%s\n", hub, cred.User, tokenType, expires)
		} else {
			fmt.Printf("            %s: %s (%s)%s\n", hub, cred.User, tokenType, expires)
		}
		shown = true
	}
	return shown
}

// printNotecardSettings shows the interface and port, as the options that set them, so
// they can be pasted onto a command line, and reports whether there were any
func (config *ConfigSettings) printNotecardSettings() (shown bool) {
	if config.Interface != "" {
		shown = true
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
	return shown
}

// ConfigTokenEnvVar names the environment variable that, when set, supplies the token for
// every request, in place of any saved sign-in
const ConfigTokenEnvVar = "NOTEHUB_TOKEN"

// EnvCredentials returns the credentials given by the NOTEHUB_TOKEN environment variable,
// for the configured hub, or nil when it isn't set
func (config *ConfigSettings) EnvCredentials() *ConfigCreds {
	token := strings.TrimSpace(os.Getenv(ConfigTokenEnvVar))
	if token == "" {
		return nil
	}
	return &ConfigCreds{Hub: config.Hub, Token: token, Source: ConfigTokenEnvVar}
}

// DefaultCredentials returns the credentials for the configured hub: those in the
// environment when NOTEHUB_TOKEN is set, and otherwise the saved sign-in, or nil if there
// is neither
func (config *ConfigSettings) DefaultCredentials() *ConfigCreds {
	if creds := config.EnvCredentials(); creds != nil {
		return creds
	}
	return config.SavedCredentials(config.Hub)
}

// CredentialsForHub uses the environment token only for the connection in use.
func (config *ConfigSettings) CredentialsForHub(hub string) *ConfigCreds {
	if hub == config.Hub {
		return config.DefaultCredentials()
	}
	return config.SavedCredentials(hub)
}

// SavedCredentials returns the login saved for a hub, or nil if there is none
func (config *ConfigSettings) SavedCredentials(hub string) *ConfigCreds {
	if creds, present := config.HubCreds[hub]; present && creds.Token != "" && creds.User != "" {
		creds.Hub = hub
		return &creds
	}
	return nil
}

// RemoveDefaultCredentials clears the credentials for config.Hub. An OAuth
// sign-in is also ended at Notehub and Hydra. A personal access token is left
// alone: the user set its expiration and manages it under API Access.
func (config *ConfigSettings) RemoveDefaultCredentials() error {
	// Renew before asking Notehub to remove the session. Logout must still clear
	// local credentials when the grant is revoked or the service is unavailable.
	_, _ = config.RefreshCredentials(context.Background(), "")
	var credentials *ConfigCreds
	err := config.withStoredCredentials(context.Background(), func(stored *ConfigSettings) (bool, error) {
		credentials = stored.SavedCredentials(config.Hub)
		if credentials == nil {
			return false, fmt.Errorf("not logged in to %s", config.Hub)
		}
		delete(stored.HubCreds, config.Hub)
		return true, nil
	})
	if err != nil {
		return err
	}

	if credentials.IsOAuthAccessToken() {
		signOutOAuthToken(config.Hub, credentials.Token)
		if credentials.RefreshToken != "" {
			_ = notehub.RevokeRefreshToken(config.Hub, credentials.RefreshToken)
		}
	}

	return nil
}

// signOutOAuthToken ends an OAuth sign-in. Notehub first: it deletes the
// sign-in that API Access lists, which refuses the token from then on, and a
// token Hydra has already revoked could no longer authenticate that. Both calls
// are best effort; the local credentials go regardless.
func signOutOAuthToken(hub, token string) {
	if err := deleteSignIn(apiBaseURL(hub), token); err != nil {
		fmt.Fprintf(os.Stderr, "logout: the login stays listed under API Access at %s until it lapses: %s\n", hub, err)
	}
	notehub.RevokeAccessToken(hub, token)
}

// deleteSignIn deletes the sign-in behind an OAuth token at the Notehub API
// rooted at apiBaseURL.
func deleteSignIn(apiBaseURL, token string) error {
	req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/auth/logout", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// apiBaseURL is the HTTPS root of the Notehub API for a hub, whichever way the
// hub was configured: with or without its "api." prefix.
func apiBaseURL(hub string) string {
	if !strings.HasPrefix(hub, "api.") {
		hub = "api." + hub
	}
	return "https://" + hub
}

func (config *ConfigSettings) SetDefaultCredentials(token string, email string, expiresAt *time.Time) {
	if config.HubCreds == nil {
		config.HubCreds = map[string]ConfigCreds{}
	}
	config.HubCreds[config.Hub] = ConfigCreds{
		Hub:       config.Hub,
		User:      email,
		Token:     token,
		ExpiresAt: expiresAt,
	}
	if config.credentialChanges == nil {
		config.credentialChanges = map[string]bool{}
	}
	config.credentialChanges[config.Hub] = true
}

// SetOAuthCredentials preserves the refresh token returned at login.
func (config *ConfigSettings) SetOAuthCredentials(token *notehub.AccessToken) {
	config.SetDefaultCredentials(token.AccessToken, token.Email, &token.ExpiresAt)
	credentials := config.HubCreds[config.Hub]
	credentials.RefreshToken = token.RefreshToken
	config.HubCreds[config.Hub] = credentials
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

	// Set or reset the flags as desired, remembering the saved hub, which a --hub given
	// for this command alone doesn't replace
	if config.Hub == "" {
		config.Hub = notehub.DefaultAPIService
	}
	if configSavedHub == "" {
		configSavedHub = config.Hub
	}
	if configFlagHub != "" {
		ConfigSetHub(configFlagHub)
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
		flag.StringVar(&configFlagHub, "hub", "", "set currently active Notehub domain")
	}

}

// FlagParse is a wrapper around flag.Parse that handles our config flags, saving them when
// the command line does nothing else
func FlagParse(notecardFlags bool, notehubFlags bool) (err error) {
	return FlagParseSaving(notecardFlags, notehubFlags, true)
}

// FlagParseSaving is FlagParse with a say in whether a command line that only sets
// configuration saves it.  A mode that acts on the hub named by --hub, rather than
// adopting it, passes false.
func FlagParseSaving(notecardFlags bool, notehubFlags bool, saveConfig bool) (err error) {

	// Register our flags
	ConfigFlagsRegister(notecardFlags, notehubFlags)

	// Parse them
	if err = flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return
	}

	// Process our flags
	err = ConfigFlagsProcess()
	if err != nil {
		return
	}

	// Save only when the command line does nothing but set configuration, adopting the
	// hub it names as the saved one
	if saveConfig && configFlagsOnly(flag.CommandLine) && config.Interface != "lease" {
		config.AdoptHub()
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

// configFlagsOnly reports whether the command line sets configuration and nothing else
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

// ConfigSignedIn returns the credentials in use, or nil if there are none
func ConfigSignedIn() *ConfigCreds {
	return config.DefaultCredentials()
}

// ConfigAuthenticationHeader sets the authorization field in the header as appropriate
func ConfigAuthenticationHeader(httpReq *http.Request) error {
	config, err := GetConfig()
	if err != nil {
		return err
	}
	return config.authenticationHeader(httpReq, config.Hub)
}

func (config *ConfigSettings) authenticationHeader(httpReq *http.Request, hub string) error {
	if _, err := config.RefreshCredentialsForHub(httpReq.Context(), hub, ""); err != nil {
		return err
	}
	// Exit if not signed in
	credentials := config.CredentialsForHub(hub)
	if credentials == nil {
		return fmt.Errorf("not logged in to %s: set %s, or run 'notehub login'", hub, ConfigTokenEnvVar)
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
