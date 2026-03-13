// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/blues/note-go/note"
	notehub "github.com/blues/notehub-go"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// isNetworkError checks whether an error is caused by a network connectivity
// issue (DNS resolution failure, connection refused, timeout, etc.) as opposed
// to an application-level error like invalid credentials.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error (timeout, DNS, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for DNS errors specifically
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for connection refused / dial errors via net.OpError
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Fallback: check the error string for common network failure patterns
	// (handles wrapped errors from HTTP client that may not preserve type info)
	msg := err.Error()
	networkPatterns := []string{
		"no such host",
		"connection refused",
		"network is unreachable",
		"i/o timeout",
		"dial tcp",
		"dial udp",
		"TLS handshake timeout",
		"no route to host",
	}
	for _, pattern := range networkPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}

// networkErrorMessage returns a user-friendly error message for network failures.
func networkErrorMessage(err error) string {
	hub := GetHub()
	return fmt.Sprintf("unable to connect to %s: %s\n\nPlease check your network connection and try again.", hub, err)
}

// initCommand handles the common command setup: auth validation, project UID
// resolution, and SDK client/context creation. Most commands need all of these
// and previously duplicated ~15 lines of boilerplate for this setup.
func initCommand() (client *notehub.APIClient, ctx context.Context, projectUID string, err error) {
	creds, err := GetHubCredentials()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error getting credentials: %s", err)
	}
	if creds == nil || creds.Token == "" {
		return nil, nil, "", fmt.Errorf("please sign in using 'notehub auth signin' or 'notehub auth signin-token'")
	}

	projectUID = GetProject()
	if projectUID == "" {
		return nil, nil, "", fmt.Errorf("no project set. Use 'notehub project set <name-or-uid>' or provide --project flag")
	}

	client = GetNotehubClient()
	ctx = context.WithValue(context.Background(), notehub.ContextAccessToken, creds.Token)
	return
}

// wantJSON returns true if the user requested JSON or pretty-printed output.
func wantJSON() bool {
	return GetJson() || GetPretty()
}

// printJSON marshals v as JSON (compact or pretty based on flags) and prints it
// to the command's configured output writer.
func printJSON(cmd *cobra.Command, v any) error {
	var output []byte
	var err error
	if GetPretty() {
		output, err = note.JSONMarshalIndent(v, "", "  ")
	} else {
		output, err = note.JSONMarshal(v)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	cmd.Printf("%s\n", output)
	return nil
}

// printResult handles the standard output pattern for all commands: if JSON
// output is requested, print as JSON; otherwise print as human-readable text.
func printResult(cmd *cobra.Command, v any) error {
	if wantJSON() {
		return printJSON(cmd, v)
	}
	return printHuman(cmd, v)
}

// printHuman renders a value as human-readable key-value text. It marshals to
// JSON first (ensuring the same fields as --json) then formats the output with
// readable key names and indentation.
func printHuman(cmd *cobra.Command, v any) error {
	jsonBytes, err := note.JSONMarshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("failed to parse: %w", err)
	}

	switch tok {
	case json.Delim('{'):
		humanRenderObject(cmd, dec, "")
	case json.Delim('['):
		humanRenderArray(cmd, dec, "")
	default:
		cmd.Printf("%v\n", tok)
	}
	return nil
}

// humanKeyAbbreviations maps lowercase words to their uppercase abbreviations.
var humanKeyAbbreviations = map[string]string{
	"uid": "UID", "api": "API", "url": "URL", "http": "HTTP",
	"sku": "SKU", "dfu": "DFU", "sn": "SN", "id": "ID",
	"ip": "IP", "tls": "TLS", "imei": "IMEI", "iccid": "ICCID",
	"rssi": "RSSI", "rsrp": "RSRP", "rsrq": "RSRQ", "sinr": "SINR",
}

// humanFormatKey converts a snake_case or camelCase JSON key to Title Case,
// with common abbreviations kept uppercase.
func humanFormatKey(key string) string {
	key = strings.ReplaceAll(key, "_", " ")
	words := strings.Fields(key)
	for i, w := range words {
		lower := strings.ToLower(w)
		if upper, ok := humanKeyAbbreviations[lower]; ok {
			words[i] = upper
		} else if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// humanRenderObject reads a JSON object from the decoder and prints it as
// indented key-value pairs. Null values and empty strings are omitted.
func humanRenderObject(cmd *cobra.Command, dec *json.Decoder, indent string) {
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		key := humanFormatKey(tok.(string))

		tok, err = dec.Token()
		if err != nil {
			return
		}

		switch v := tok.(type) {
		case json.Delim:
			if v == '{' {
				if !dec.More() {
					dec.Token() // consume empty }
				} else {
					cmd.Printf("%s%s:\n", indent, key)
					humanRenderObject(cmd, dec, indent+"  ")
				}
			} else if v == '[' {
				if !dec.More() {
					dec.Token() // consume empty ]
				} else {
					cmd.Printf("%s%s:\n", indent, key)
					humanRenderArray(cmd, dec, indent+"  ")
				}
			}
		case string:
			if v != "" {
				cmd.Printf("%s%s: %s\n", indent, key, v)
			}
		case json.Number:
			cmd.Printf("%s%s: %s\n", indent, key, v.String())
		case bool:
			cmd.Printf("%s%s: %v\n", indent, key, v)
		case nil:
			// skip null values
		}
	}
	dec.Token() // consume }
}

// humanRenderArray reads a JSON array from the decoder and prints its elements.
// Objects are separated by blank lines; scalars use "- value" format.
func humanRenderArray(cmd *cobra.Command, dec *json.Decoder, indent string) {
	first := true
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return
		}

		switch v := tok.(type) {
		case json.Delim:
			if v == '{' {
				if !first {
					cmd.Println()
				}
				first = false
				humanRenderObject(cmd, dec, indent)
			} else if v == '[' {
				humanRenderArray(cmd, dec, indent+"  ")
			}
		case string:
			cmd.Printf("%s- %s\n", indent, v)
		case json.Number:
			cmd.Printf("%s- %s\n", indent, v.String())
		case bool:
			cmd.Printf("%s- %v\n", indent, v)
		case nil:
			// skip
		}
	}
	dec.Token() // consume ]
}

// resolveFleet looks up a fleet by UID or name and returns the full Fleet object.
func resolveFleet(client *notehub.APIClient, ctx context.Context, projectUID, identifier string) (*notehub.Fleet, error) {
	// Try direct UID lookup first
	fleet, resp, err := client.ProjectAPI.GetFleet(ctx, projectUID, identifier).Execute()
	if err == nil && resp != nil && resp.StatusCode != 404 {
		return fleet, nil
	}

	// Fall back to name search
	fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list fleets: %w", err)
	}

	for _, f := range fleetsRsp.Fleets {
		if f.Label == identifier {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("fleet '%s' not found in project", identifier)
}

// PickerItem represents a single item in an interactive picker.
type PickerItem struct {
	Label string // Display label shown to the user
	Value string // Value returned when selected (e.g., UID)
}

// pickOne displays an interactive arrow-key picker and returns the selected item.
// Returns nil if the user cancels (Ctrl+C/Esc) or if items is empty.
func pickOne(title string, items []PickerItem) *PickerItem {
	if len(items) == 0 {
		return nil
	}

	options := make([]huh.Option[int], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.Label, i)
	}

	var selected int
	err := huh.NewSelect[int]().
		Title(title).
		Options(options...).
		Value(&selected).
		Run()

	if err != nil {
		return nil
	}

	return &items[selected]
}

// errPickCancelled is returned when the user cancels an interactive picker.
// Commands should treat this as a no-op (not an error to display).
var errPickCancelled = fmt.Errorf("selection cancelled")

// pickFleet fetches all fleets for the project and presents an interactive picker.
// Returns the selected fleet's UID, errPickCancelled if the user cancels, or an
// error with a helpful message if none exist.
func pickFleet(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to list fleets: %w", err)
	}
	if len(fleetsRsp.Fleets) == 0 {
		return "", fmt.Errorf("no fleets found in this project. Create one with 'notehub fleet create <name>'")
	}
	items := make([]PickerItem, len(fleetsRsp.Fleets))
	for i, f := range fleetsRsp.Fleets {
		items[i] = PickerItem{Label: f.Label, Value: f.Uid}
	}
	picked := pickOne("Select a fleet", items)
	if picked == nil {
		return "", errPickCancelled
	}
	return picked.Value, nil
}

// pickRoute fetches all routes for the project and presents an interactive picker.
// Returns the selected route's UID, errPickCancelled if the user cancels, or an
// error with a helpful message if none exist.
func pickRoute(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to list routes: %w", err)
	}
	if len(routes) == 0 {
		return "", fmt.Errorf("no routes found in this project. Create one with 'notehub route create <label> --config <file>'")
	}
	items := make([]PickerItem, 0, len(routes))
	for _, r := range routes {
		label := ""
		uid := ""
		if r.Label != nil {
			label = *r.Label
		}
		if r.Uid != nil {
			uid = *r.Uid
		}
		if uid != "" {
			if label == "" {
				label = uid
			}
			items = append(items, PickerItem{Label: label, Value: uid})
		}
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no routes found in this project. Create one with 'notehub route create <label> --config <file>'")
	}
	picked := pickOne("Select a route", items)
	if picked == nil {
		return "", errPickCancelled
	}
	return picked.Value, nil
}

// pickMonitor fetches all monitors for the project and presents an interactive picker.
// Returns the selected monitor's UID, errPickCancelled if the user cancels, or an
// error with a helpful message if none exist.
func pickMonitor(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	monitors, _, err := client.MonitorAPI.GetMonitors(ctx, projectUID).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to list monitors: %w", err)
	}
	if len(monitors) == 0 {
		return "", fmt.Errorf("no monitors found in this project. Create one with 'notehub monitor create <name> --config <file>'")
	}
	items := make([]PickerItem, 0, len(monitors))
	for _, m := range monitors {
		label := ""
		uid := ""
		if m.Name != nil {
			label = *m.Name
		}
		if m.Uid != nil {
			uid = *m.Uid
		}
		if uid != "" {
			if label == "" {
				label = uid
			}
			items = append(items, PickerItem{Label: label, Value: uid})
		}
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no monitors found in this project. Create one with 'notehub monitor create <name> --config <file>'")
	}
	picked := pickOne("Select a monitor", items)
	if picked == nil {
		return "", errPickCancelled
	}
	return picked.Value, nil
}

// pickProduct fetches all products for the project and presents an interactive picker.
// Returns the selected product's UID, errPickCancelled if the user cancels, or an
// error with a helpful message if none exist.
func pickProduct(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to list products: %w", err)
	}
	if len(productsRsp.Products) == 0 {
		return "", fmt.Errorf("no products found in this project. Create one with 'notehub product create <label> <uid>'")
	}
	items := make([]PickerItem, len(productsRsp.Products))
	for i, p := range productsRsp.Products {
		items[i] = PickerItem{Label: p.Label, Value: p.Uid}
	}
	picked := pickOne("Select a product", items)
	if picked == nil {
		return "", errPickCancelled
	}
	return picked.Value, nil
}

// addPaginationFlags adds --limit and --all flags to a command.
func addPaginationFlags(cmd *cobra.Command, defaultLimit int) {
	cmd.Flags().Int("limit", defaultLimit, "Maximum number of results to return")
	cmd.Flags().Bool("all", false, "Fetch all results (may be slow for large datasets)")
}

// getPaginationConfig reads --limit and --all flags from a command.
// Returns pageSize (for the API) and maxResults (total to collect, 0 = unlimited).
func getPaginationConfig(cmd *cobra.Command) (pageSize int32, maxResults int) {
	limit, _ := cmd.Flags().GetInt("limit")
	all, _ := cmd.Flags().GetBool("all")

	if all {
		return 500, 0
	}
	return int32(limit), limit
}

// setDefault saves a resource UID and label to the config file.
func setDefault(cmd *cobra.Command, key, uid, label string) error {
	viper.Set(key, uid)
	viper.Set(key+"_label", label)
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	cmd.Printf("Active %s set to: %s (%s)\n", key, label, uid)
	return nil
}

// clearDefault removes a resource default from the config file.
func clearDefault(cmd *cobra.Command, key, setHint string) error {
	current := viper.GetString(key)
	if current == "" {
		cmd.Printf("No %s is currently set.\n", key)
		return nil
	}
	viper.Set(key, "")
	viper.Set(key+"_label", "")
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	cmd.Printf("Active %s cleared.\n", key)
	cmd.Printf("You can set a new %s with '%s'\n", key, setHint)
	return nil
}

// resolveRoute looks up a route by UID or name. It first tries GetRoute for
// full details, then falls back to matching from the route list (which returns
// summaries but works with broader permissions).
func resolveRoute(client *notehub.APIClient, ctx context.Context, projectUID, identifier string) (*notehub.NotehubRoute, *notehub.NotehubRouteSummary, error) {
	// Try direct lookup for full details (may fail with 403 depending on permissions)
	route, _, getErr := client.RouteAPI.GetRoute(ctx, projectUID, identifier).Execute()
	if getErr == nil && route != nil {
		return route, nil, nil
	}

	// Fall back to list search (returns summaries, works with broader permissions)
	routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list routes: %w", err)
	}

	for _, r := range routes {
		if (r.Uid != nil && *r.Uid == identifier) || (r.Label != nil && *r.Label == identifier) {
			return nil, &r, nil
		}
	}

	return nil, nil, fmt.Errorf("route '%s' not found", identifier)
}

// resolveRouteUID resolves a route identifier (UID or name) to a route UID string.
func resolveRouteUID(client *notehub.APIClient, ctx context.Context, projectUID, identifier string) (string, error) {
	fullRoute, summary, err := resolveRoute(client, ctx, projectUID, identifier)
	if err != nil {
		return "", err
	}
	if fullRoute != nil && fullRoute.Uid != nil {
		return *fullRoute.Uid, nil
	}
	if summary != nil && summary.Uid != nil {
		return *summary.Uid, nil
	}
	return "", fmt.Errorf("route '%s' has no UID", identifier)
}

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
