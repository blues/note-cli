// Copyright 2019 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// ConfigCreds are the credentials for a given notehub
type ConfigCreds struct {
	User  string `json:"user,omitempty"`
	Token string `json:"token,omitempty"`
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
	SchemaUrl string                 `json:"json-schema-url,omitempty"`
}

// Config are the master config settings
var Config ConfigSettings
var configFlagHub string
var configFlagInterface string
var configFlagPort string
var configFlagPortConfig int
var configFlagJsonSchemaUrl string

// ConfigRead reads the current info from config file
func ConfigRead() error {

	// As a convenience to all tools, generate a new random seed for each iteration
	rand.Seed(time.Now().UnixNano())
	rand.Seed(rand.Int63() ^ time.Now().UnixNano())

	// Read the config file
	contents, err := ioutil.ReadFile(configSettingsPath())
	if os.IsNotExist(err) {
		ConfigReset()
		err = nil
	} else if err == nil {
		err = note.JSONUnmarshal(contents, &Config)
		if err != nil || Config.When == "" {
			ConfigReset()
			if err != nil {
				err = fmt.Errorf("can't read configuration: %s", err)
			}
		}
	}

	return err

}

// ConfigWrite updates the file with the current config info
func ConfigWrite() error {

	// Marshal it
	configJSON, _ := note.JSONMarshalIndent(Config, "", "    ")

	// Write the file
	fd, err := os.OpenFile(configSettingsPath(), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	fd.Write(configJSON)
	fd.Close()

	// Done
	return err

}

// Reset the comms to default
func configResetInterface() {
	Config = ConfigSettings{}
	Config.HubCreds = map[string]ConfigCreds{}
	Config.IPort = map[string]ConfigPort{}
}

// ConfigReset updates the file with the default info
func ConfigReset() {
	configResetInterface()
	ConfigSetHub("-")
	Config.When = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	Config.SchemaUrl = ""
}

// ConfigShow displays all current config parameters
func ConfigShow() error {

	fmt.Printf("\nCurrently saved values:\n")

	if Config.Hub != "" {
		fmt.Printf("       hub: %s\n", Config.Hub)
	}
	if Config.IPort == nil {
		Config.IPort = map[string]ConfigPort{}
	}
	if Config.HubCreds == nil {
		Config.HubCreds = map[string]ConfigCreds{}
	}
	if len(Config.HubCreds) != 0 {
		fmt.Printf("     creds:\n")
		for hub, cred := range Config.HubCreds {
			fmt.Printf("            %s: %s\n", hub, cred.User)
		}
	}
	if Config.Interface != "" {
		fmt.Printf("   -interface %s\n", Config.Interface)
		if Config.IPort[Config.Interface].Port == "" {
			fmt.Printf("   -port -\n")
			fmt.Printf("   -portconfig -\n")
		} else {
			fmt.Printf("   -port %s\n", Config.IPort[Config.Interface].Port)
			fmt.Printf("   -portconfig %d\n", Config.IPort[Config.Interface].PortConfig)
		}
	}
	if Config.SchemaUrl != "" {
		fmt.Printf("   -json-schema-url %s\n", Config.SchemaUrl)
	}

	return nil

}

// ConfigFlagsProcess processes the registered config flags
func ConfigFlagsProcess() (err error) {

	// Create maps if they don't exist
	if Config.IPort == nil {
		Config.IPort = map[string]ConfigPort{}
	}
	if Config.HubCreds == nil {
		Config.HubCreds = map[string]ConfigCreds{}
	}

	// Read if not yet read
	if Config.When == "" {
		err = ConfigRead()
		if err != nil {
			return
		}
	}

	// Set or reset the flags as desired
	if configFlagHub != "" {
		ConfigSetHub(configFlagHub)
	}
	if configFlagInterface == "-" {
		configResetInterface()
	} else if configFlagInterface != "" {
		Config.Interface = configFlagInterface
	}
	if configFlagJsonSchemaUrl == "-" {
		Config.SchemaUrl = ""
	} else if configFlagJsonSchemaUrl != "" {
		Config.SchemaUrl = configFlagJsonSchemaUrl
	}
	if configFlagPort == "-" {
		temp := Config.IPort[Config.Interface]
		temp.Port = ""
		Config.IPort[Config.Interface] = temp
	} else if configFlagPort != "" {
		temp := Config.IPort[Config.Interface]
		temp.Port = configFlagPort
		Config.IPort[Config.Interface] = temp
	}
	if configFlagPortConfig < 0 {
		temp := Config.IPort[Config.Interface]
		temp.PortConfig = 0
		Config.IPort[Config.Interface] = temp
	} else if configFlagPortConfig != 0 {
		temp := Config.IPort[Config.Interface]
		temp.PortConfig = configFlagPortConfig
		Config.IPort[Config.Interface] = temp
	}
	if Config.Interface == "" {
		configFlagPort = ""
		configFlagPortConfig = 0
	}

	// Done
	return nil

}

// ConfigFlagsRegister registers the config-related flags
func ConfigFlagsRegister(notecardFlags bool, notehubFlags bool) {

	// Process the commands
	if notecardFlags {
		flag.StringVar(&configFlagInterface, "interface", "", "select 'serial' or 'i2c' interface for notecard")
		flag.StringVar(&configFlagJsonSchemaUrl, "json-schema-url", "", "set the schema URL for the notecard")
		flag.StringVar(&configFlagPort, "port", "", "select serial or i2c port for notecard")
		flag.IntVar(&configFlagPortConfig, "portconfig", 0, "set serial device speed or i2c address for notecard")
	}
	if notehubFlags {
		flag.StringVar(&configFlagHub, "hub", "", "set notehub domain")
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

	// If our flags were the only ones present, save them
	configOnly := true
	if len(os.Args) == 1 {
		configOnly = false
	} else {
		for i, arg := range os.Args {
			// Even arguments are parameters, odd args are flags
			if (i & 1) != 0 {
				switch arg {
				case "-interface":
				case "-port":
				case "-portconfig":
				case "-json-schema-url":
				case "-hub":
				// any odd argument that isn't one of our switches
				default:
					configOnly = false
				}
			}
		}
	}
	if configOnly && Config.Interface != "lease" {
		fmt.Printf("*** saving configuration ***")
		ConfigWrite()
		ConfigShow()
	}

	// Override, just for this session, with env vars
	str := os.Getenv("NOTE_INTERFACE")
	if str != "" {
		Config.Interface = str
	}

	str = os.Getenv("NOTE_JSON_SCHEMA_URL")
	if str != "" {
		Config.SchemaUrl = str
	}

	// Override via env vars if specified
	str = os.Getenv("NOTE_PORT")
	if str != "" {
		temp := Config.IPort[Config.Interface]
		temp.Port = str
		Config.IPort[Config.Interface] = temp
		str := os.Getenv("NOTE_PORT_CONFIG")
		strint, err2 := strconv.Atoi(str)
		if err2 != nil {
			strint = Config.IPort[Config.Interface].PortConfig
		}
		temp = Config.IPort[Config.Interface]
		temp.PortConfig = strint
		Config.IPort[Config.Interface] = temp
	}

	// Done
	return

}

// ConfigSignedIn returns info about whether or not we're signed in
func ConfigSignedIn() (username string, token string, authenticated bool) {
	if Config.IPort == nil {
		Config.IPort = map[string]ConfigPort{}
	}
	if Config.HubCreds == nil {
		Config.HubCreds = map[string]ConfigCreds{}
	}
	hub := Config.Hub
	if hub == "" {
		hub = notehub.DefaultAPIService
	}
	creds, present := Config.HubCreds[hub]
	if present {
		if creds.Token != "" && creds.User != "" {
			authenticated = true
			username = creds.User
			token = creds.Token
		}
	}

	return

}

// ConfigAuthenticationHeader sets the authorization field in the header as appropriate
func ConfigAuthenticationHeader(httpReq *http.Request) (err error) {

	// Exit if not signed in
	_, token, authenticated := ConfigSignedIn()
	if !authenticated {
		hub := Config.Hub
		if hub == "" {
			hub = notehub.DefaultAPIService
		}
		err = fmt.Errorf("not authenticated to %s: please use 'notehub -signin' to sign into the notehub service", hub)
		return
	}

	// Set the header
	httpReq.Header.Set("X-Session-Token", token)

	// Done
	return

}

// ConfigAPIHub returns the configured notehub, for use by the HTTP API.  If none is configured it returns
// the default Blues API service.  Regardless, it always makes sure that the host has "api." as a prefix.
// This enables flexibility in what's configured.
func ConfigAPIHub() (hub string) {
	hub = Config.Hub
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
	hub = Config.Hub
	if hub == "" || hub == "-" {
		hub = notehub.DefaultAPIService
	}
	hub = strings.TrimPrefix(hub, "api.")
	return
}

// ConfigSetHub clears the hub
func ConfigSetHub(hub string) {
	if hub == "-" {
		hub = ""
	}
	Config.Hub = hub
}
