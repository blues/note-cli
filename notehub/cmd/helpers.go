// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	notehub "github.com/blues/notehub-go"
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

// GetNotehubClient creates and returns a configured Notehub API client with authentication
func GetNotehubClient() *notehub.APIClient {
	cfg := notehub.NewConfiguration()

	// Set the API server if configured
	apiHub := GetAPIHub()
	if apiHub != "" {
		cfg.Host = apiHub
		cfg.Scheme = "https"
	}

	return notehub.NewAPIClient(cfg)
}

// GetNotehubContext creates a context with authentication for Notehub API calls
func GetNotehubContext() (context.Context, error) {
	// Get the authentication credentials
	creds, err := GetHubCredentials()
	if err != nil {
		return nil, err
	}
	if creds == nil || creds.Token == "" {
		return nil, fmt.Errorf("not authenticated: please use 'notehub auth signin' to sign in")
	}

	// Create context with bearer token authentication
	ctx := context.WithValue(context.Background(), notehub.ContextAccessToken, creds.Token)

	return ctx, nil
}

// Load metadata for the app
func appGetMetadata(flagVerbose bool, flagVars bool) (appMetadata AppMetadata, err error) {
	// Get project info using SDK
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

	// Initialize SDK client and context
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return appMetadata, err
	}

	// Get project information using SDK
	project, _, err := client.ProjectAPI.GetProject(ctx, projectUID).Execute()
	if err != nil {
		return appMetadata, fmt.Errorf("failed to get project: %w", err)
	}

	// App info - fields are direct values, not pointers
	appMetadata.App.UID = project.Uid
	appMetadata.App.Name = project.Label
	// Note: BillingAccountUid is not in the Project model

	// Get fleets using SDK
	fleetsResp, _, err := client.ProjectAPI.GetFleets(ctx, appMetadata.App.UID).Execute()
	if err == nil && fleetsResp.Fleets != nil {
		items := []Metadata{}
		for _, fleet := range fleetsResp.Fleets {
			i := Metadata{Name: fleet.Label, UID: fleet.Uid}

			if flagVars && fleet.Uid != "" {
				varsResp, _, err := client.ProjectAPI.GetFleetEnvironmentVariables(ctx, appMetadata.App.UID, fleet.Uid).Execute()
				if err != nil {
					return appMetadata, fmt.Errorf("failed to get fleet environment variables: %w", err)
				}
				i.Vars = varsResp.EnvironmentVariables
			}
			items = append(items, i)
		}
		appMetadata.Fleets = items
	}

	// Get routes using SDK
	routes, _, err := client.RouteAPI.GetRoutes(ctx, appMetadata.App.UID).Execute()
	if err == nil && routes != nil {
		items := []Metadata{}
		for _, route := range routes {
			routeUID := ""
			routeLabel := ""
			if route.Uid != nil {
				routeUID = *route.Uid
			}
			if route.Label != nil {
				routeLabel = *route.Label
			}
			i := Metadata{Name: routeLabel, UID: routeUID}
			items = append(items, i)
		}
		appMetadata.Routes = items
	}

	// Get products using SDK
	productsResp, _, err := client.ProjectAPI.GetProducts(ctx, appMetadata.App.UID).Execute()
	if err == nil && productsResp.Products != nil {
		items := []Metadata{}
		for _, product := range productsResp.Products {
			i := Metadata{Name: product.Label, UID: product.Uid}
			items = append(items, i)
		}
		appMetadata.Products = items
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

	// Get SDK client for device queries
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	// Looking for "all devices" or a named fleet
	if indirectScope == "" {
		// All devices - use SDK
		pageSize := int32(500)
		pageNum := int32(0)
		for {
			pageNum++

			devicesResp, _, err := client.DeviceAPI.GetDevices(ctx, appMetadata.App.UID).
				PageSize(pageSize).
				PageNum(pageNum).
				Execute()
			if err != nil {
				return err
			}

			for _, device := range devicesResp.Devices {
				err = addScope(device.Uid, appMetadata, scopeDevices, scopeFleets, flagVerbose)
				if err != nil {
					return err
				}
			}

			if !devicesResp.HasMore {
				break
			}
		}
		return
	} else {
		// Fleet - use SDK
		for _, fleet := range (*appMetadata).Fleets {
			if lookingFor == fleet.UID || fleetMatchesScope(fleet.Name, lookingFor) {
				foundFleet = true

				pageSize := int32(100)
				pageNum := int32(0)
				for {
					pageNum++

					devicesResp, _, err := client.DeviceAPI.GetFleetDevices(ctx, appMetadata.App.UID, fleet.UID).
						PageSize(pageSize).
						PageNum(pageNum).
						Execute()
					if err != nil {
						return err
					}

					for _, device := range devicesResp.Devices {
						err = addScope(device.Uid, appMetadata, scopeDevices, scopeFleets, flagVerbose)
						if err != nil {
							return err
						}
					}

					if !devicesResp.HasMore {
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

	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	for _, deviceUID := range uids {
		varsResp, _, err := client.DeviceAPI.GetDeviceEnvironmentVariables(ctx, appMetadata.App.UID, deviceUID).Execute()
		if err != nil {
			return vars, err
		}
		vars[deviceUID] = varsResp.EnvironmentVariables
	}

	return
}

// Load env vars from fleets
func varsGetFromFleets(appMetadata AppMetadata, uids []string, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	for _, fleetUID := range uids {
		varsResp, _, err := client.ProjectAPI.GetFleetEnvironmentVariables(ctx, appMetadata.App.UID, fleetUID).Execute()
		if err != nil {
			return vars, err
		}
		vars[fleetUID] = varsResp.EnvironmentVariables
	}

	return
}

// Set env vars for devices
func varsSetFromDevices(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	for _, deviceUID := range uids {
		envVars := notehub.NewEnvironmentVariables(template)

		varsResp, _, err := client.DeviceAPI.SetDeviceEnvironmentVariables(ctx, appMetadata.App.UID, deviceUID).
			EnvironmentVariables(*envVars).
			Execute()
		if err != nil {
			return vars, err
		}

		vars[deviceUID] = varsResp.EnvironmentVariables
	}

	return
}

// Set env vars for fleets
func varsSetFromFleets(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {
	vars = map[string]Vars{}

	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	for _, fleetUID := range uids {
		envVars := notehub.NewEnvironmentVariables(template)

		varsResp, _, err := client.ProjectAPI.SetFleetEnvironmentVariables(ctx, appMetadata.App.UID, fleetUID).
			EnvironmentVariables(*envVars).
			Execute()
		if err != nil {
			return vars, err
		}

		vars[fleetUID] = varsResp.EnvironmentVariables
	}

	return
}

// Provision devices
func varsProvisionDevices(appMetadata AppMetadata, uids []string, productUID string, deviceSN string, flagVerbose bool) (err error) {
	client := GetNotehubClient()
	ctx, err := GetNotehubContext()
	if err != nil {
		return
	}

	for _, deviceUID := range uids {
		provReq := notehub.NewProvisionDeviceRequest(productUID)
		if deviceSN != "" {
			provReq.SetDeviceSn(deviceSN)
		}

		_, _, err = client.DeviceAPI.ProvisionDevice(ctx, appMetadata.App.UID, deviceUID).
			ProvisionDeviceRequest(*provReq).
			Execute()
		if err != nil {
			return
		}
	}

	return
}
