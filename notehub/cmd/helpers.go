// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/blues/note-go/note"
	notegoapi "github.com/blues/note-go/notehub/api"
)

// Type definitions
type Vars map[string]string

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
	// Get project info using V1 API
	// First we need to determine the project UID from global flags
	projectUID := GetProject()
	if projectUID == "" {
		product := GetProduct()
		if product != "" {
			projectUID = product
		}
	}

	if projectUID == "" {
		return appMetadata, fmt.Errorf("project or product UID required")
	}

	// Get project information using V1 API: GET /v1/projects/{projectOrProductUID}
	projectRsp := map[string]interface{}{}
	projectURL := fmt.Sprintf("/v1/projects/%s", projectUID)
	err = reqHubV1(flagVerbose, GetAPIHub(), "GET", projectURL, nil, &projectRsp)
	if err != nil {
		return
	}

	// App info
	appMetadata.App.UID, _ = projectRsp["uid"].(string)
	appMetadata.App.Name, _ = projectRsp["label"].(string)
	appMetadata.App.BA, _ = projectRsp["billing_account_uid"].(string)

	// Get fleets using V1 API: GET /v1/projects/{projectOrProductUID}/fleets
	fleetsRsp := map[string]interface{}{}
	fleetsURL := fmt.Sprintf("/v1/projects/%s/fleets", appMetadata.App.UID)
	err = reqHubV1(flagVerbose, GetAPIHub(), "GET", fleetsURL, nil, &fleetsRsp)
	if err == nil {
		fleetsList, exists := fleetsRsp["fleets"].([]interface{})
		if exists {
			items := []Metadata{}
			for _, v := range fleetsList {
				fleet, ok := v.(map[string]interface{})
				if ok {
					fleetUID, _ := fleet["uid"].(string)
					fleetLabel, _ := fleet["label"].(string)
					i := Metadata{Name: fleetLabel, UID: fleetUID}

					if flagVars {
						varsRsp := notegoapi.GetFleetEnvironmentVariablesResponse{}
						url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, fleetUID)
						err = reqHubV1(flagVerbose, GetAPIHub(), "GET", url, nil, &varsRsp)
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

	// Get routes using V1 API: GET /v1/projects/{projectOrProductUID}/routes
	routesRsp := []map[string]interface{}{}
	routesURL := fmt.Sprintf("/v1/projects/%s/routes", appMetadata.App.UID)
	err = reqHubV1(flagVerbose, GetAPIHub(), "GET", routesURL, nil, &routesRsp)
	if err == nil {
		items := []Metadata{}
		for _, route := range routesRsp {
			routeUID, _ := route["uid"].(string)
			routeLabel, _ := route["label"].(string)
			i := Metadata{Name: routeLabel, UID: routeUID}
			items = append(items, i)
		}
		appMetadata.Routes = items
	}

	// Get products using V1 API: GET /v1/projects/{projectOrProductUID}/products
	productsRsp := map[string]interface{}{}
	productsURL := fmt.Sprintf("/v1/projects/%s/products", appMetadata.App.UID)
	err = reqHubV1(flagVerbose, GetAPIHub(), "GET", productsURL, nil, &productsRsp)
	if err == nil {
		pi, exists := productsRsp["products"].([]interface{})
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

	return
}

// Get a device list given a scope
func appGetScope(scope string, flagVerbose bool) (appMetadata AppMetadata, scopeDevices []string, scopeFleets []string, err error) {
	// Process special scopes
	switch scope {
	case "devices":
		scope = "@"
	case "fleets":
		scope = "-"
	}

	// Get the metadata
	appMetadata, err = appGetMetadata(flagVerbose, false)
	if err != nil {
		return
	}

	// On the command line we allow comma-separated lists
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

	return
}

// ResolveScopeWithValidation is a convenience wrapper around appGetScope that:
// 1. Automatically uses GetVerbose() for the verbose flag
// 2. Validates that at least one device or fleet was found
// 3. Returns a more user-friendly error message
//
// This reduces boilerplate in commands that use scope resolution.
func ResolveScopeWithValidation(scope string) (appMetadata AppMetadata, scopeDevices []string, scopeFleets []string, err error) {
	verbose := GetVerbose()
	appMetadata, scopeDevices, scopeFleets, err = appGetScope(scope, verbose)
	if err != nil {
		return
	}

	if len(scopeDevices) == 0 && len(scopeFleets) == 0 {
		err = fmt.Errorf("no devices or fleets found within the specified scope")
		return
	}

	return
}

// Recursively add scope
func addScope(scope string, appMetadata *AppMetadata, scopeDevices *[]string, scopeFleets *[]string, flagVerbose bool) (err error) {
	if strings.HasPrefix(scope, "dev:") {
		*scopeDevices = append(*scopeDevices, scope)
		return
	}

	if strings.HasPrefix(scope, "imei:") || strings.HasPrefix(scope, "burn:") {
		*scopeDevices = append(*scopeDevices, scope)
		return
	}

	if strings.HasPrefix(scope, "fleet:") {
		*scopeFleets = append(*scopeFleets, scope)
		return
	}

	// See if this is a fleet name
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

	// Process indirection
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
			err = reqHubV1(flagVerbose, GetAPIHub(), "GET", url, nil, &devices)
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
					err = reqHubV1(flagVerbose, GetAPIHub(), "GET", url, nil, &devices)
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

// Sort and remove duplicates
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

// Load env vars from devices
func varsGetFromDevices(appMetadata AppMetadata, uids []string, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	for _, deviceUID := range uids {
		varsRsp := notegoapi.GetDeviceEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/environment_variables", appMetadata.App.UID, deviceUID)
		err = reqHubV1(flagVerbose, GetAPIHub(), "GET", url, nil, &varsRsp)
		if err != nil {
			return
		}
		vars[deviceUID] = varsRsp.EnvironmentVariables
	}

	return
}

// Load env vars from fleets
func varsGetFromFleets(appMetadata AppMetadata, uids []string, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	for _, fleetUID := range uids {
		varsRsp := notegoapi.GetFleetEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, fleetUID)
		err = reqHubV1(flagVerbose, GetAPIHub(), "GET", url, nil, &varsRsp)
		if err != nil {
			return
		}
		vars[fleetUID] = varsRsp.EnvironmentVariables
	}

	return
}

// Set env vars for devices
func varsSetFromDevices(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	for _, deviceUID := range uids {
		req := notegoapi.PutDeviceEnvironmentVariablesRequest{EnvironmentVariables: Vars{}}
		for k, v := range template {
			req.EnvironmentVariables[k] = v
		}

		var reqJSON []byte
		reqJSON, err = note.JSONMarshal(req)
		if err != nil {
			return
		}

		rspPut := notegoapi.PutDeviceEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/environment_variables", appMetadata.App.UID, deviceUID)
		err = reqHubV1(flagVerbose, GetAPIHub(), "PUT", url, reqJSON, &rspPut)
		if err != nil {
			return
		}

		vars[deviceUID] = rspPut.EnvironmentVariables
	}

	return
}

// Set env vars for fleets
func varsSetFromFleets(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	for _, fleetUID := range uids {
		req := notegoapi.PutFleetEnvironmentVariablesRequest{EnvironmentVariables: Vars{}}
		for k, v := range template {
			req.EnvironmentVariables[k] = v
		}

		var reqJSON []byte
		reqJSON, err = note.JSONMarshal(req)
		if err != nil {
			return
		}

		rspPut := notegoapi.PutFleetEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, fleetUID)
		err = reqHubV1(flagVerbose, GetAPIHub(), "PUT", url, reqJSON, &rspPut)
		if err != nil {
			return
		}

		vars[fleetUID] = rspPut.EnvironmentVariables
	}

	return
}

// Provision devices
func varsProvisionDevices(appMetadata AppMetadata, uids []string, productUID string, deviceSN string, flagVerbose bool) (err error) {
	for _, deviceUID := range uids {
		req := notegoapi.ProvisionDeviceRequest{ProductUID: productUID, DeviceSN: deviceSN}

		var reqJSON []byte
		reqJSON, err = note.JSONMarshal(req)
		if err != nil {
			return
		}

		url := fmt.Sprintf("/v1/projects/%s/devices/%s/provision", appMetadata.App.UID, deviceUID)
		err = reqHubV1(flagVerbose, GetAPIHub(), "POST", url, reqJSON, nil)
		if err != nil {
			return
		}
	}

	return
}
