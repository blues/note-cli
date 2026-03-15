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

// Shared scope flag variables used by provision, vars, and explore commands.
var (
	flagScope string
	flagSn    string
)

// scopeHelpLong is shared scope format documentation appended to Long descriptions.
const scopeHelpLong = `
Scope Formats:
  dev:xxxx           Single device UID
  imei:xxxx          Device by IMEI
  fleet:xxxx         All devices in fleet (by UID)
  production         All devices in named fleet
  @fleet-name        All devices in fleet (indirection)
  @                  All devices in project
  @devices.txt       Device UIDs from file (one per line)
  dev:aaa,dev:bbb    Multiple scopes (comma-separated)`

// addScopeFlag adds the standard --scope/-s flag to a command.
func addScopeFlag(cmd *cobra.Command, description string) {
	cmd.Flags().StringVarP(&flagScope, "scope", "s", "", description)
}

// confirmAction prompts the user to confirm a destructive action. Returns nil
// if confirmed, errPickCancelled if declined. Skips the prompt if --yes/-y is set.
func confirmAction(cmd *cobra.Command, message string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return nil
	}

	var confirmed bool
	err := huh.NewConfirm().
		Title(message).
		Value(&confirmed).
		WithTheme(huh.ThemeBase()).
		Run()
	if err != nil || !confirmed {
		return errPickCancelled
	}
	return nil
}

// addConfirmFlag adds the --yes/-y flag to a command for skipping confirmation prompts.
func addConfirmFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
}

// validateAuth checks that the user is signed in and returns an error if not.
// Use this for commands that need auth but don't use the SDK client (e.g. V0
// commands like request/trace). For commands that also need the SDK client and
// project, use initCommand() instead.
func validateAuth() error {
	creds, err := GetHubCredentials()
	if err != nil {
		return fmt.Errorf("error getting credentials: %s", err)
	}
	if creds == nil || creds.Token == "" {
		return fmt.Errorf("please sign in using 'notehub auth signin' or 'notehub auth signin-token'")
	}
	return nil
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

// printListResult handles the standard list output pattern: JSON if requested,
// otherwise check for empty list and show a message, or render human-readable.
// The isEmpty func checks whether the data is empty (e.g. len(resp.Items) == 0).
func printListResult(cmd *cobra.Command, v any, emptyMsg string, isEmpty func() bool) error {
	if wantJSON() {
		return printJSON(cmd, v)
	}
	if isEmpty() {
		cmd.Println(emptyMsg)
		return nil
	}
	return printHuman(cmd, v)
}

// printMutationResult handles output for create/update mutations: JSON if requested,
// otherwise print a success message followed by the human-readable result.
func printMutationResult(cmd *cobra.Command, v any, successMsg string) error {
	if wantJSON() {
		return printJSON(cmd, v)
	}
	cmd.Println(successMsg)
	return printHuman(cmd, v)
}

// printActionResult handles output for action commands (enable, disable, delete, etc.)
// that don't return structured data from the API but should support --json output.
// The result map is printed as JSON when requested, otherwise the successMsg is shown.
func printActionResult(cmd *cobra.Command, result map[string]any, successMsg string) error {
	if wantJSON() {
		return printJSON(cmd, result)
	}
	cmd.Println(successMsg)
	return nil
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

// resolveProduct looks up a product by UID or label and returns the full Product object.
func resolveProduct(client *notehub.APIClient, ctx context.Context, projectUID, identifier string) (*notehub.Product, error) {
	productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	for _, p := range productsRsp.Products {
		if p.Uid == identifier || p.Label == identifier {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("product '%s' not found in project", identifier)
}

// resolveMonitor looks up a monitor by UID or name and returns its UID and name.
func resolveMonitor(client *notehub.APIClient, ctx context.Context, projectUID, identifier string) (uid string, name string, err error) {
	// Try direct UID lookup first
	monitor, resp, getErr := client.MonitorAPI.GetMonitor(ctx, projectUID, identifier).Execute()
	if getErr == nil && resp != nil && resp.StatusCode != 404 {
		if monitor.Uid != nil {
			uid = *monitor.Uid
		}
		if monitor.Name != nil {
			name = *monitor.Name
		}
		return uid, name, nil
	}

	// Fall back to list search
	monitors, _, err := client.MonitorAPI.GetMonitors(ctx, projectUID).Execute()
	if err != nil {
		return "", "", fmt.Errorf("failed to list monitors: %w", err)
	}

	for _, m := range monitors {
		mUID := ""
		mName := ""
		if m.Uid != nil {
			mUID = *m.Uid
		}
		if m.Name != nil {
			mName = *m.Name
		}
		if mUID == identifier || mName == identifier {
			return mUID, mName, nil
		}
	}

	return "", "", fmt.Errorf("monitor '%s' not found in project", identifier)
}

// PickerItem represents a single item in an interactive picker.
type PickerItem struct {
	Label string // Display label shown to the user
	Value string // Value returned when selected (e.g., UID)
}

// errPickCancelled is returned when the user cancels an interactive picker.
// Commands should treat this as a no-op (not an error to display).
var errPickCancelled = fmt.Errorf("selection cancelled")

// PickerPage holds a page of picker items and whether more pages exist.
type PickerPage struct {
	Items   []PickerItem
	HasMore bool
}

// pickPaginated displays a paginated interactive picker. The fetchPage callback
// is called with the current page number (1-based) and should return items for
// that page. For non-paginated APIs, return all items with HasMore=false.
// Returns the selected item's Value, or errPickCancelled if the user cancels.
func pickPaginated(title string, emptyMsg string, fetchPage func(page int32) (PickerPage, error)) (string, error) {
	pageNum := int32(1)
	for {
		result, err := fetchPage(pageNum)
		if err != nil {
			return "", err
		}
		if len(result.Items) == 0 && pageNum == 1 {
			return "", fmt.Errorf("%s", emptyMsg)
		}

		// Build picker items with navigation
		items := make([]PickerItem, 0, len(result.Items)+2)
		if pageNum > 1 {
			items = append(items, PickerItem{Label: "← Previous page", Value: "__prev__"})
		}
		items = append(items, result.Items...)
		if result.HasMore {
			items = append(items, PickerItem{Label: "Next page →", Value: "__next__"})
		}

		// Show picker
		pickerTitle := title
		if result.HasMore || pageNum > 1 {
			pickerTitle = fmt.Sprintf("%s (page %d)", title, pageNum)
		}

		options := make([]huh.Option[int], len(items))
		for i, item := range items {
			options[i] = huh.NewOption(item.Label, i)
		}

		theme := huh.ThemeBase()
		var selected int
		err = huh.NewSelect[int]().
			Title(pickerTitle).
			Options(options...).
			Value(&selected).
			WithTheme(theme).
			Run()
		if err != nil {
			return "", errPickCancelled
		}

		switch items[selected].Value {
		case "__next__":
			pageNum++
		case "__prev__":
			pageNum--
		default:
			return items[selected].Value, nil
		}
	}
}

// resolveDeviceArg returns a device UID from the command args, the --device flag,
// or an interactive picker if neither is provided.
func resolveDeviceArg(client *notehub.APIClient, ctx context.Context, projectUID string, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if d := GetDevice(); d != "" {
		return d, nil
	}
	return pickDevice(client, ctx, projectUID)
}

// pickDevice presents a paginated device picker.
func pickDevice(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	return pickPaginated("Select a device", "no devices found in this project", func(page int32) (PickerPage, error) {
		devicesResp, _, err := client.DeviceAPI.GetDevices(ctx, projectUID).
			PageSize(50).
			PageNum(page).
			Execute()
		if err != nil {
			return PickerPage{}, fmt.Errorf("failed to list devices: %w", err)
		}
		items := make([]PickerItem, len(devicesResp.Devices))
		for i, d := range devicesResp.Devices {
			label := d.Uid
			if d.SerialNumber != nil && *d.SerialNumber != "" {
				label = fmt.Sprintf("%s (%s)", d.Uid, *d.SerialNumber)
			}
			items[i] = PickerItem{Label: label, Value: d.Uid}
		}
		return PickerPage{Items: items, HasMore: devicesResp.HasMore}, nil
	})
}

// pickFleet presents a fleet picker.
func pickFleet(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	return pickPaginated("Select a fleet", "no fleets found in this project. Create one with 'notehub fleet create <name>'", func(page int32) (PickerPage, error) {
		fleetsRsp, _, err := client.ProjectAPI.GetFleets(ctx, projectUID).Execute()
		if err != nil {
			return PickerPage{}, fmt.Errorf("failed to list fleets: %w", err)
		}
		items := make([]PickerItem, len(fleetsRsp.Fleets))
		for i, f := range fleetsRsp.Fleets {
			items[i] = PickerItem{Label: f.Label, Value: f.Uid}
		}
		return PickerPage{Items: items, HasMore: false}, nil
	})
}

// pickRoute presents a route picker.
func pickRoute(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	return pickPaginated("Select a route", "no routes found in this project. Create one with 'notehub route create <label> --config <file>'", func(page int32) (PickerPage, error) {
		routes, _, err := client.RouteAPI.GetRoutes(ctx, projectUID).Execute()
		if err != nil {
			return PickerPage{}, fmt.Errorf("failed to list routes: %w", err)
		}
		items := make([]PickerItem, 0, len(routes))
		for _, r := range routes {
			uid := ""
			label := ""
			if r.Uid != nil {
				uid = *r.Uid
			}
			if r.Label != nil {
				label = *r.Label
			}
			if uid != "" {
				if label == "" {
					label = uid
				}
				items = append(items, PickerItem{Label: label, Value: uid})
			}
		}
		return PickerPage{Items: items, HasMore: false}, nil
	})
}

// pickMonitor presents a monitor picker.
func pickMonitor(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	return pickPaginated("Select a monitor", "no monitors found in this project. Create one with 'notehub monitor create <name> --config <file>'", func(page int32) (PickerPage, error) {
		monitors, _, err := client.MonitorAPI.GetMonitors(ctx, projectUID).Execute()
		if err != nil {
			return PickerPage{}, fmt.Errorf("failed to list monitors: %w", err)
		}
		items := make([]PickerItem, 0, len(monitors))
		for _, m := range monitors {
			uid := ""
			label := ""
			if m.Uid != nil {
				uid = *m.Uid
			}
			if m.Name != nil {
				label = *m.Name
			}
			if uid != "" {
				if label == "" {
					label = uid
				}
				items = append(items, PickerItem{Label: label, Value: uid})
			}
		}
		return PickerPage{Items: items, HasMore: false}, nil
	})
}

// pickProduct presents a product picker.
func pickProduct(client *notehub.APIClient, ctx context.Context, projectUID string) (string, error) {
	return pickPaginated("Select a product", "no products found in this project. Create one with 'notehub product create <label> <uid>'", func(page int32) (PickerPage, error) {
		productsRsp, _, err := client.ProjectAPI.GetProducts(ctx, projectUID).Execute()
		if err != nil {
			return PickerPage{}, fmt.Errorf("failed to list products: %w", err)
		}
		items := make([]PickerItem, len(productsRsp.Products))
		for i, p := range productsRsp.Products {
			items[i] = PickerItem{Label: p.Label, Value: p.Uid}
		}
		return PickerPage{Items: items, HasMore: false}, nil
	})
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
	if label != uid {
		cmd.Printf("Active %s set to: %s (%s)\n", key, label, uid)
	} else {
		cmd.Printf("Active %s set to: %s\n", key, uid)
	}
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
