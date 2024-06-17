// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/blues/note-cli/lib"
	notegoapi "github.com/blues/note-go/notehub/api"
)

type Metadata struct {
	Name string            `json:"name,omitempty"`
	UID  string            `json:"uid,omitempty"`
	BA   string            `json:"billing_account_uid,omitempty"`
	Vars map[string]string `json:"vars,omitempty"`
}

type AppMetadata struct {
	App      Metadata   `json:"app,omitempty"`
	Fleets   []Metadata `json:"fleets,omitempty"`
	Routes   []Metadata `json:"routes,omitempty"`
	Products []Metadata `json:"products,omitempty"`
}

// Load metadata for the app
func appGetMetadata(flagVerbose bool, flagVars bool) (appMetadata AppMetadata, err error) {

	rsp := map[string]interface{}{}
	err = reqHubV0(flagVerbose, lib.ConfigAPIHub(), []byte("{\"req\":\"hub.app.get\"}"), "", "", "", "", false, false, nil, &rsp)
	if err != nil {
		return
	}

	// App info
	appMetadata.App.UID = rsp["uid"].(string)
	appMetadata.App.Name = rsp["label"].(string)
	appMetadata.App.BA = rsp["billing_account_uid"].(string)

	// Fleet info
	settings, exists := rsp["info"].(map[string]interface{})
	if exists {
		fleets, exists := settings["fleet"].(map[string]interface{})
		if exists {
			items := []Metadata{}
			for k, v := range fleets {
				vj, ok := v.(map[string]interface{})
				if ok {
					i := Metadata{Name: vj["label"].(string), UID: k}
					if flagVars {
						varsRsp := notegoapi.GetFleetEnvironmentVariablesResponse{}
						url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, k)
						err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &varsRsp)
						if err != nil {
							return
						}
						i.Vars = varsRsp.EnvironmentVariables
					}
					items = append(items, i)
				}
			}
			appMetadata.Fleets = items
		}
	}

	// Enum routes
	rsp = map[string]interface{}{}
	err = reqHubV0(flagVerbose, lib.ConfigAPIHub(), []byte("{\"req\":\"hub.app.test.route\"}"), "", "", "", "", false, false, nil, &rsp)
	if err == nil {
		body, exists := rsp["body"].(map[string]interface{})
		if exists {
			items := []Metadata{}
			for k, v := range body {
				vs, ok := v.(string)
				if ok {
					components := strings.Split(k, "/")
					if len(components) > 1 {
						i := Metadata{Name: vs, UID: components[1]}
						items = append(items, i)
					}
				}
			}
			appMetadata.Routes = items
		}
	}

	// Products
	rsp = map[string]interface{}{}
	err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", "/v1/projects/"+appMetadata.App.UID+"/products", nil, &rsp)
	if err == nil {
		pi, exists := rsp["products"].([]interface{})
		if exists {
			items := []Metadata{}
			for _, v := range pi {
				p, ok := v.(map[string]interface{})
				if ok {
					i := Metadata{Name: p["label"].(string), UID: p["uid"].(string)}
					items = append(items, i)
				}
				appMetadata.Products = items
			}
		}
	}

	// Done
	return

}

// Get a device list given
func appGetScope(scope string, flagVerbose bool) (appMetadata AppMetadata, scopeDevices []string, scopeFleets []string, err error) {

	// Get the metadata before we begin, because at a minimum we need appUID
	appMetadata, err = appGetMetadata(flagVerbose, false)
	if err != nil {
		return
	}

	// On the command line (but not inside files) we allow comma-separated lists
	if strings.Contains(scope, ",") {
		scopeList := strings.Split(scope, ",")
		for _, scope := range scopeList {
			err = addScope(scope, &appMetadata, &scopeDevices, &scopeFleets, flagVerbose)
			if err != nil {
				return
			}
		}
	} else {
		err = addScope(scope, &appMetadata, &scopeDevices, &scopeFleets, flagVerbose)
		if err != nil {
			return
		}
	}

	// Remove duplicates
	scopeDevices = sortAndRemoveDuplicates(scopeDevices)
	scopeFleets = sortAndRemoveDuplicates(scopeFleets)

	// Done
	return

}

// Recursively add scope
func addScope(scope string, appMetadata *AppMetadata, scopeDevices *[]string, scopeFleets *[]string, flagVerbose bool) (err error) {

	if strings.HasPrefix(scope, "dev:") {
		*scopeDevices = append(*scopeDevices, scope)
		return
	}

	if strings.HasPrefix(scope, "imei:") {
		// This is a pre-V1 legacy that still exists in some ancient fleets
		*scopeDevices = append(*scopeDevices, scope)
		return
	}

	if strings.HasPrefix(scope, "fleet:") {
		*scopeFleets = append(*scopeFleets, scope)
		return
	}

	// See if this is a fleet name, and translate it to an ID
	if !strings.HasPrefix(scope, "@") {
		for _, fleet := range (*appMetadata).Fleets {
			if strings.EqualFold(scope, strings.TrimSpace(fleet.Name)) {
				*scopeFleets = append(*scopeFleets, fleet.UID)
				return
			}
		}
		return fmt.Errorf("'%s' does not appear to be a device, fleet, @fleet indirection, or @file.ext indirection", scope)
	}

	// Process a fleet indirection.  First, find the fleet.
	indirectScope := strings.TrimPrefix(scope, "@")
	foundFleet := false
	lookingFor := strings.TrimSpace(indirectScope)

	// Looking for "all devices" or a named fleet
	if indirectScope == "" {
		// All devices

		pageSize := 100
		pageNum := 0
		for {
			pageNum++

			devices := notegoapi.GetDevicesResponse{}
			url := fmt.Sprintf("/v1/projects/%s/devices?pageSize=%d&pageNum=%d", appMetadata.App.UID, pageSize, pageNum)
			err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &devices)
			if err != nil {
				return
			}

			for _, device := range devices.Devices {
				err = addScope(device.UID, appMetadata, scopeDevices, scopeFleets, flagVerbose)
				if err != nil {
					return err
				}
			}

			if !devices.HasMore {
				break
			}

		}

		return

	} else {

		// Fleet
		for _, fleet := range (*appMetadata).Fleets {
			if strings.EqualFold(lookingFor, strings.TrimSpace(fleet.UID)) || strings.EqualFold(lookingFor, strings.TrimSpace(fleet.Name)) || lookingFor == "" {
				foundFleet = true

				pageSize := 100
				pageNum := 0
				for {
					pageNum++

					devices := notegoapi.GetDevicesResponse{}
					url := fmt.Sprintf("/v1/projects/%s/fleets/%s/devices?pageSize=%d&pageNum=%d", appMetadata.App.UID, fleet.UID, pageSize, pageNum)
					err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &devices)
					if err != nil {
						return
					}

					for _, device := range devices.Devices {
						err = addScope(device.UID, appMetadata, scopeDevices, scopeFleets, flagVerbose)
						if err != nil {
							return err
						}
					}

					if !devices.HasMore {
						break
					}

				}

			}
		}
		if foundFleet {
			return
		}

	}

	// Process a file indirection
	var contents []byte
	contents, err = ioutil.ReadFile(indirectScope)
	if err != nil {
		return fmt.Errorf("%s: %s", indirectScope, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(contents))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if trimmedLine := strings.TrimSpace(line); trimmedLine != "" {
			err = addScope(trimmedLine, appMetadata, scopeDevices, scopeFleets, flagVerbose)
			if err != nil {
				return err
			}
		}
	}

	err = scanner.Err()
	return

}

// Sort and remove duplicates in a string slice
func sortAndRemoveDuplicates(strings []string) []string {

	sort.Strings(strings)

	unique := make(map[string]struct{})
	var result []string

	for _, v := range strings {
		if _, exists := unique[v]; !exists {
			unique[v] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}
