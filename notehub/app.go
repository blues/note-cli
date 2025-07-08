// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
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

	// Determine the project UID to use
	projectUID := flagApp
	if projectUID == "" {
		projectUID = flagProduct
	}
	if projectUID == "" {
		err = fmt.Errorf("project UID must be specified via -project or -product flag")
		return
	}

	// Get the specific project using V1 API
	projectRsp := map[string]interface{}{}
	err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", "/v1/project/"+projectUID, nil, &projectRsp)
	if err != nil {
		return
	}

	// App info
	appMetadata.App.UID, _ = projectRsp["uid"].(string)
	appMetadata.App.Name, _ = projectRsp["label"].(string)
	// Placeholder for billing account since V1 API doesn't return it
	appMetadata.App.BA = "" // TODO: billing_account_uid not available in V1 project API

	// Fleet info - Get fleets using V1 API
	fleetsRsp := map[string]interface{}{}
	err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", "/v1/projects/"+appMetadata.App.UID+"/fleets", nil, &fleetsRsp)
	if err == nil {
		fleets, exists := fleetsRsp["fleets"].([]interface{})
		if exists {
			items := []Metadata{}
			for _, v := range fleets {
				fleet, ok := v.(map[string]interface{})
				if ok {
					name, nameExists := fleet["label"].(string)
					uid, uidExists := fleet["uid"].(string)
					if nameExists && uidExists {
						i := Metadata{Name: name, UID: uid}
						if flagVars {
							varsRsp := notegoapi.GetFleetEnvironmentVariablesResponse{}
							url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, i.UID)
							err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &varsRsp)
							if err != nil {
								return
							}
							i.Vars = varsRsp.EnvironmentVariables
						}
						items = append(items, i)
					}
				}
			}
			appMetadata.Fleets = items
		}
	}

	// Routes - Get routes using V1 API
	routesRsp := map[string]interface{}{}
	err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", "/v1/projects/"+appMetadata.App.UID+"/routes", nil, &routesRsp)
	if err == nil {
		routes, exists := routesRsp["routes"].([]interface{})
		if exists {
			items := []Metadata{}
			for _, v := range routes {
				route, ok := v.(map[string]interface{})
				if ok {
					name, nameExists := route["label"].(string)
					uid, uidExists := route["uid"].(string)
					if nameExists && uidExists {
						i := Metadata{Name: name, UID: uid}
						items = append(items, i)
					}
				}
			}
			appMetadata.Routes = items
		}
	}
	// Don't fail the entire function if routes fail, just continue without them

	// Products
	productsRsp := map[string]interface{}{}
	err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", "/v1/projects/"+appMetadata.App.UID+"/products", nil, &productsRsp)
	if err == nil {
		pi, exists := productsRsp["products"].([]interface{})
		if exists {
			items := []Metadata{}
			for _, v := range pi {
				p, ok := v.(map[string]interface{})
				if ok {
					name, nameExists := p["label"].(string)
					uid, uidExists := p["uid"].(string)
					if nameExists && uidExists {
						i := Metadata{Name: name, UID: uid}
						items = append(items, i)
					}
				}
			}
			appMetadata.Products = items
		}
	}

	// Done
	return

}

// Get a device list given
func appGetScope(scope string, flagVerbose bool) (appMetadata AppMetadata, scopeDevices []string, scopeFleets []string, err error) {

	// Process special scopes, which are handled inside addScope
	switch scope {
	case "devices":
		scope = "@"
	case "fleets":
		scope = "-"
	}

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

	if strings.HasPrefix(scope, "imei:") || strings.HasPrefix(scope, "burn:") {
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
		found := false
		for _, fleet := range (*appMetadata).Fleets {
			if fleetMatchesScope(fleet.Name, scope) {
				*scopeFleets = append(*scopeFleets, fleet.UID)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("'%s' does not appear to be a device, fleet, @fleet indirection, or @file.ext indirection", scope)
		}
		return
	}

	// Process a fleet indirection.  First, find the fleet.
	indirectScope := strings.TrimPrefix(scope, "@")
	foundFleet := false
	lookingFor := strings.TrimSpace(indirectScope)

	// Looking for "all devices" or a named fleet
	if indirectScope == "" {
		// All devices

		pageSize := 500
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
			if lookingFor == fleet.UID || fleetMatchesScope(fleet.Name, lookingFor) {
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
	contents, err = os.ReadFile(indirectScope)
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

// See if a fleet name matches a scope name
func fleetMatchesScope(fleetName string, scope string) bool {
	normalizedScope := strings.ToLower(scope)
	scopeWildcard := false
	if strings.HasSuffix(normalizedScope, "*") {
		normalizedScope = strings.TrimSuffix(normalizedScope, "*")
		scopeWildcard = true
	}
	normalizedName := strings.ToLower(fleetName)
	match := scope == "-" || normalizedName == normalizedScope
	if scopeWildcard {
		if strings.HasPrefix(normalizedName, normalizedScope) {
			match = true
		}
	}
	return match
}
