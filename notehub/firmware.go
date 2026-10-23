// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub firmware': the host firmware a project holds, and applying it to devices,
// through the hub's v1 firmware and DFU calls.  With --type notecard, which isn't
// documented, the same verbs manage the hub's own notecard firmware. Uploads detect
// Notecard binaries and use Airnote when no project is supplied for the v1 upload API.
// Catalog operations use the project-less v0 calls; DFU calls need the devices' project.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notehub"
)

// The firmware types, as the hub's API names them
const (
	firmwareHost     = "host"
	firmwareNotecard = "notecard"

	// The v1 upload endpoint needs a project even for the hub's Notecard firmware.
	firmwareNotecardProject = "app:2606f411-dea6-44a0-9743-1130f57d77d8" // Airnote
)

// Switches used in this mode
var (
	flagFirmwareType    string
	flagFirmwareVersion string
	flagFirmwareInfo    string
	flagFirmwareTags    cli.StringList
	flagFirmwareSNs     cli.StringList
	flagFirmwareSKUs    cli.StringList
)

// The modes in which these switches may be used
var inFirmware = []string{modeFirmware}

// firmwareSwitches returns the switches of this mode
func firmwareSwitches() []*cli.Switch {
	return []*cli.Switch{
		// The notecard type is Blues' to manage, so it isn't described
		{Name: "type", Target: &flagFirmwareType, Group: "firmware", Modes: inFirmware, Hidden: true,
			Usage: "the firmware `type`: host or notecard; uploads detect Notecard binaries, other commands default to host"},
		{Name: "version", Target: &flagFirmwareVersion, Group: "firmware", Modes: inFirmware,
			Usage: "the `version` to record for a firmware, when its binary doesn't carry one"},
		{Name: "notes", Target: &flagUploadsNotes, Group: "firmware", Modes: inFirmware,
			Usage: "`notes` describing what's different about a firmware, or @file for a file's contents"},
		{Name: "info", Target: &flagFirmwareInfo, Group: "firmware", Modes: inFirmware,
			Usage: "metadata of your own to record with a firmware, as `json` or @file"},
		{Name: "scope", Target: &flagScope, Group: "firmware", Modes: inFirmware,
			Usage: "the `scope` of an update or query: dev:xx, fleet:xx, a fleet's name, @file of them, or all"},
		{Name: "tag", Target: &flagFirmwareTags, Group: "firmware", Modes: inFirmware,
			Usage: "also only the devices with this `tag`"},
		{Name: "sn", Target: &flagFirmwareSNs, Group: "firmware", Modes: inFirmware,
			Usage: "also only the device with this `serial` number"},
		{Name: "sku", Target: &flagFirmwareSKUs, Group: "firmware", Modes: inFirmware,
			Usage: "also only the devices of this `sku`"},
		{Name: "no-replace", Target: &flagUploadsNoReplace, Group: "firmware", Modes: inFirmware,
			Usage: "refuse to replace a firmware the project already holds under the same name and version"},
		{Name: "dry-run", Target: &flagSkillsDryRun, Group: "firmware", Modes: inFirmware,
			Usage: "say what upload, edit, delete, update or cancel would do, without doing it"},
		{Name: "force", Target: &flagSkillsForce, Group: "firmware", Modes: inFirmware,
			Usage: "let get overwrite a local file that differs, and update or cancel address every device in the project"},
		{Name: "json", Target: &flagJson, Group: "output", Modes: inFirmware,
			Usage: "write the listing, metadata, status or history as JSON"},
	}
}

// firmwareExamples returns the examples shown in this mode's help
func firmwareExamples() []cli.Example {
	return []cli.Example{
		{What: "List the host firmware the project holds", Command: cliName + " firmware --project <projectUID>"},
		{What: "The same as JSON, for a program", Command: cliName + " firmware list --json --project <projectUID>"},
		{What: "Upload a build, with its version read from the binary and release notes from a file", Command: cliName + " firmware upload ./build/app.bin --notes @RELEASE.md --project <projectUID>"},
		{What: "The same, naming the version when the binary doesn't carry one", Command: cliName + " firmware upload ./build/app.bin --version 1.2.3 --project <projectUID>"},
		{What: "See what would be uploaded, without uploading", Command: cliName + " firmware upload ./build/app.bin --dry-run --project <projectUID>"},
		{What: "Download a firmware, verified against its md5", Command: cliName + " firmware get 1.2.3 ./app-1.2.3.bin --project <projectUID>"},
		{What: "Apply a firmware to a fleet, and then watch it go", Command: cliName + ` firmware update 1.2.3 --scope "Field Units" --project <projectUID>`},
		{What: "Each device's update, as it goes", Command: cliName + ` firmware status --scope "Field Units" --project <projectUID>`},
		{What: "Apply a firmware to every device in the project", Command: cliName + " firmware update 1.2.3 --scope all --force --project <projectUID>"},
		{What: "Change a firmware's notes after the fact", Command: cliName + ` firmware edit 1.2.3 --notes "fixes the sensor timeout" --project <projectUID>`},
	}
}

// firmwareCommands returns the commands accepted by this mode
func firmwareCommands() []cli.Command {
	return []cli.Command{
		{Name: "list", Summary: "list the firmware the project holds, by name, version, date and notes",
			Run: firmwareListCommand},
		{Name: "show", Args: "<firmware>", Summary: "one firmware's metadata, including what was read from the binary",
			Run: firmwareShowCommand},
		{Name: "get", Args: "<firmware> [path|-]", Summary: "download a firmware to a file or stdout, verified against its md5",
			Run: firmwareGetCommand},
		{Name: "upload", Args: "<path> [name]", Summary: "upload a firmware, skipping one the project holds already, and replacing one of the same name and version",
			Run: firmwareUploadCommand},
		{Name: "set", Args: "<path> [name]", Summary: "alias for upload",
			Run: firmwareUploadCommand},
		{Name: "edit", Args: "<firmware>", Summary: "change a firmware's --version, --notes or --info",
			Run: firmwareEditCommand},
		{Name: "delete", Args: "<firmware>", Summary: "remove a firmware from the project",
			Run: firmwareDeleteCommand},
		{Name: "update", Args: "<firmware>", Summary: "ask the devices in --scope to install a firmware",
			Run: firmwareUpdateCommand},
		{Name: "cancel", Summary: "call off the update under way for the devices in --scope",
			Run: firmwareCancelCommand},
		{Name: "status", Summary: "each device's update as it goes: versions, phase, and when",
			Run: firmwareStatusCommand},
		{Name: "history", Summary: "each device's past updates",
			Run: firmwareHistoryCommand},
	}
}

// runFirmware is the handler for 'notehub firmware'.  With a project and no command it
// lists the project's firmware; with nothing at all, the help is shown.
func runFirmware(config *lib.ConfigSettings) error {
	args := flag.Args()
	if len(args) == 0 {
		if flagApp == "" && flagProduct == "" {
			cli.PrintHelp(cli.ModeNamed(modeFirmware), false)
			return nil
		}
		return cli.Dispatch(cli.ModeNamed(modeFirmware), config, []string{"list"})
	}
	if !strings.EqualFold(args[0], modeHelp) && cli.CommandNamed(firmwareCommands(), args[0]) == nil {
		args = append([]string{"show"}, args...)
	}
	return cli.Dispatch(cli.ModeNamed(modeFirmware), config, args)
}

// firmwareType returns the firmware type in play, taking the v0 name of host firmware as
// meaning it
func firmwareType() (string, error) {
	switch strings.ToLower(flagFirmwareType) {
	case "", firmwareHost, "firmware":
		return firmwareHost, nil
	case firmwareNotecard:
		return firmwareNotecard, nil
	}
	return "", fmt.Errorf("'%s' is not a firmware type - the types are host and notecard", flagFirmwareType)
}

// firmwareProject returns the project named by --project or --product.  Host firmware
// always needs one; notecard firmware needs one only where devices are concerned.
func firmwareProject(ftype string, devices bool) (string, error) {
	project := flagApp
	if project == "" {
		project = flagProduct
	}
	if project == "" {
		if ftype == firmwareNotecard {
			if !devices {
				return "", nil
			}
			return "", errors.New("name the project the devices are in, with --project or --product")
		}
		return "", errors.New("specify the project with --project or --product")
	}
	return project, nil
}

// firmwareItem is one firmware as it is listed and shown, whichever API it came from.  Its
// JSON form is the v1 API's, which is also what --json writes.
type firmwareItem struct {
	Filename     string         `json:"filename"`
	Type         string         `json:"type,omitempty"`
	Version      string         `json:"version,omitempty"`
	MD5          string         `json:"md5,omitempty"`
	Length       int            `json:"length,omitempty"`
	Created      string         `json:"created,omitempty"`
	Built        string         `json:"built,omitempty"`
	Organization string         `json:"organization,omitempty"`
	Product      string         `json:"product,omitempty"`
	Target       string         `json:"target,omitempty"`
	Published    bool           `json:"published,omitempty"`
	Tags         string         `json:"tags,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	Info         map[string]any `json:"info,omitempty"`
}

// firmwareCall performs one v1 call and returns the hub's answer, or the error its status
// says
func firmwareCall(method string, path string, query url.Values, body []byte, contentType string) (*apiResponse, error) {
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	rsp, err := apiSend(uploadsClient, &apiRequest{Method: method, URL: &url.URL{Path: path, RawQuery: query.Encode()}, Header: header, Body: body})
	if err != nil {
		return nil, err
	}
	if err := apiFailure(rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

// firmwareScope is the uploads scope of the hub's notecard firmware
var firmwareScope = uploadsScope{Type: firmwareNotecard, Global: true}

// firmwareList returns the firmware of a type: a project's host firmware through the v1
// call, newest first, or the hub's notecard firmware through the v0 one
func firmwareList(ftype string, project string) ([]firmwareItem, error) {

	if ftype == firmwareNotecard {
		uploads, err := uploadsQuery(firmwareScope, false)
		if err != nil {
			return nil, err
		}
		items := []firmwareItem{}
		for _, upload := range uploads {
			items = append(items, firmwareItemFromUpload(upload))
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].Created > items[j].Created })
		return items, nil
	}

	query := url.Values{"firmwareType": {ftype}, "sortBy": {"created"}, "sortOrder": {"desc"}}
	rsp, err := firmwareCall(http.MethodGet, "/v1/projects/"+project+"/firmware", query, nil, "")
	if err != nil {
		return nil, err
	}
	items := []firmwareItem{}
	body := bytes.TrimSpace(rsp.Body)
	if bytes.HasPrefix(body, []byte("[")) {
		err = note.JSONUnmarshal(body, &items)
	} else {
		var wrapped map[string]json.RawMessage
		if err = note.JSONUnmarshal(body, &wrapped); err == nil {
			for _, value := range wrapped {
				if bytes.HasPrefix(bytes.TrimSpace(value), []byte("[")) {
					err = note.JSONUnmarshal(value, &items)
					break
				}
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("the firmware listing can't be read: %w", err)
	}
	for i := range items {
		if items[i].Type == "" {
			items[i].Type = ftype
		}
	}
	return items, nil

}

// firmwareItemFromUpload is a notecard firmware as the v0 call describes it, in the v1
// call's terms
func firmwareItemFromUpload(upload notehub.UploadMetadata) firmwareItem {
	item := firmwareItem{
		Filename:  upload.Name,
		Type:      firmwareNotecard,
		Version:   upload.Version,
		MD5:       upload.MD5,
		Length:    upload.Length,
		Published: strings.Contains(upload.Tags, skillsReservedTag),
		Tags:      upload.Tags,
		Notes:     upload.Notes,
		Info:      upload.Info,
	}
	if when := uploadsWhen(upload); when != 0 {
		item.Created = time.Unix(when, 0).UTC().Format(time.RFC3339)
	}
	if upload.Firmware != nil {
		if item.Version == "" {
			item.Version = upload.Firmware.VersionString()
			if item.Version == "0.0.0.0" {
				item.Version = upload.Firmware.Version
			}
		}
		item.Built, item.Organization, item.Product, item.Target = upload.Firmware.Built, upload.Firmware.Organization, upload.Firmware.Product, upload.Firmware.Target
	}
	return item
}

// firmwareStampRE matches the moment the hub adds to a filename when it stores it
var firmwareStampRE = regexp.MustCompile(`\$\d+`)

// firmwareOriginal is a stored filename as it was uploaded, without the hub's stamp
func firmwareOriginal(filename string) string {
	return firmwareStampRE.ReplaceAllString(filename, "")
}

// firmwareWhen is how a firmware's date is displayed
func firmwareWhen(created string) string {
	if when, err := time.Parse(time.RFC3339, created); err == nil {
		return when.Local().Format("2006-01-02 15:04")
	}
	if seconds, err := strconv.ParseInt(created, 10, 64); err == nil && seconds != 0 {
		return time.Unix(seconds, 0).Format("2006-01-02 15:04")
	}
	return created
}

// firmwareResolve returns the firmware a name refers to: the one stored under it, or the
// only one uploaded under it or carrying it as a version
func firmwareResolve(items []firmwareItem, name string) (firmwareItem, error) {
	if len(items) == 0 {
		return firmwareItem{}, fmt.Errorf("there is no firmware named '%s', or any other - none is held", name)
	}
	wanted := strings.TrimSpace(name)
	candidates := []firmwareItem{}
	for _, item := range items {
		if strings.EqualFold(item.Filename, wanted) {
			return item, nil
		}
		if strings.EqualFold(firmwareOriginal(item.Filename), wanted) || strings.EqualFold(item.Version, wanted) {
			candidates = append(candidates, item)
		}
	}
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		names := []string{}
		for _, item := range items {
			names = append(names, item.Filename)
		}
		return firmwareItem{}, fmt.Errorf("'%s' is not one of the firmware - they are: %s", name, strings.Join(names, ", "))
	}
	names := []string{}
	for _, item := range candidates {
		names = append(names, fmt.Sprintf("%s (version %s, %s)", item.Filename, item.Version, firmwareWhen(item.Created)))
	}
	return firmwareItem{}, fmt.Errorf("'%s' names more than one firmware - give the stored filename of the one meant: %s", name, strings.Join(names, ", "))
}

// firmwareListCommand lists what the project holds
func firmwareListCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if len(args) != 0 {
		return fmt.Errorf("'%s firmware list' takes no arguments", cliName)
	}
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	if flagJson {
		return filesWriteJSON(items)
	}
	holder := project
	if ftype == firmwareNotecard {
		holder = "the hub"
	}
	if len(items) == 0 {
		fmt.Printf("%s holds no %s firmware.\n", holder, ftype)
		return nil
	}
	width, versionWidth := 0, 7
	for _, item := range items {
		width, versionWidth = max(width, len(item.Filename)), max(versionWidth, len(item.Version))
	}
	fmt.Printf("%s firmware of %s\n\n", ftype, holder)
	for _, item := range items {
		line := fmt.Sprintf("  %-*s  %-*s  %-16s %s", width, item.Filename, versionWidth, item.Version, firmwareWhen(item.Created), uploadsFirstLine(item.Notes))
		fmt.Println(strings.TrimRight(line, " "))
	}
	fmt.Printf("\n%d firmware(s).  Name one by its stored filename, or by its original name or version when that names just one.\n", len(items))
	return nil
}

// firmwareShowCommand writes one firmware's metadata
func firmwareShowCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s firmware show <firmware>", cliName)
	}
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	item, err := firmwareResolve(items, args[0])
	if err != nil {
		return err
	}
	if flagJson {
		return filesWriteJSON(item)
	}
	fmt.Printf("filename:     %s\n", item.Filename)
	fmt.Printf("type:         %s\n", item.Type)
	fmt.Printf("version:      %s\n", item.Version)
	for _, field := range []struct{ label, value string }{
		{"built", item.Built}, {"organization", item.Organization}, {"product", item.Product}, {"target", item.Target},
		{"md5", item.MD5}, {"uploaded", firmwareWhen(item.Created)}, {"tags", item.Tags},
	} {
		if field.value != "" {
			fmt.Printf("%-13s %s\n", field.label+":", field.value)
		}
	}
	if item.Length != 0 {
		fmt.Printf("length:       %d\n", item.Length)
	}
	if ftype == firmwareNotecard {
		fmt.Printf("published:    %v\n", item.Published)
	}
	if len(item.Info) != 0 {
		info, _ := note.JSONMarshal(item.Info)
		fmt.Printf("info:         %s\n", info)
	}
	if item.Notes != "" {
		fmt.Printf("notes:\n%s\n", item.Notes)
	}
	return nil
}

// firmwareRead returns a firmware's bytes, checked against its md5
func firmwareRead(ftype string, project string, item firmwareItem) ([]byte, error) {
	if ftype == firmwareNotecard {
		uploads, err := uploadsQuery(firmwareScope, false)
		if err != nil {
			return nil, err
		}
		for _, upload := range uploads {
			if upload.Name == item.Filename {
				return uploadsRead(firmwareScope, upload)
			}
		}
		return nil, fmt.Errorf("%s is no longer there", item.Filename)
	}
	rsp, err := firmwareCall(http.MethodGet, "/v1/projects/"+project+"/firmware/"+ftype+"/"+item.Filename, nil, nil, "")
	if err != nil {
		return nil, err
	}
	if item.MD5 != "" && !strings.EqualFold(uploadsMD5(rsp.Body), item.MD5) {
		return nil, fmt.Errorf("%s: the bytes read don't match the hub's md5 of them", item.Filename)
	}
	return rsp.Body, nil
}

// firmwareGetCommand downloads a firmware to a file, named as it was uploaded unless a
// path is given, or to stdout.  A local file that differs is kept unless --force is given.
func firmwareGetCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s firmware get <firmware> [path|-]", cliName)
	}
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	item, err := firmwareResolve(items, args[0])
	if err != nil {
		return err
	}
	contents, err := firmwareRead(ftype, project, item)
	if err != nil {
		return err
	}
	filename := firmwareOriginal(item.Filename)
	if len(args) == 2 {
		if args[1] == "-" {
			_, err = os.Stdout.Write(contents)
			return err
		}
		filename = args[1]
		if info, statErr := os.Stat(filename); statErr == nil && info.IsDir() {
			filename = filepath.Join(filename, firmwareOriginal(item.Filename))
		}
	}
	state, err := skillsSaveFile(filename, contents, flagSkillsForce)
	if err != nil {
		return err
	}
	fmt.Printf("  %-9s %s -> %s (%d bytes, version %s)\n", state, item.Filename, filename, len(contents), item.Version)
	if state == "kept" {
		return fmt.Errorf("%s holds something different from the firmware and was kept - use --force to overwrite it", filename)
	}
	return nil
}

// firmwareSameVersion reports whether two versions are the same, counting a missing
// trailing number as zero, since the hub writes 7.1.3 as 7.1.3.0 in one place and not
// another.  Two firmware with no version at all are the same in this respect too.
func firmwareSameVersion(a string, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	partsA, partsB := strings.Split(a, "."), strings.Split(b, ".")
	for len(partsA) < len(partsB) {
		partsA = append(partsA, "0")
	}
	for len(partsB) < len(partsA) {
		partsB = append(partsB, "0")
	}
	for i := range partsA {
		if strings.TrimLeft(partsA[i], "0") != strings.TrimLeft(partsB[i], "0") {
			return false
		}
	}
	return true
}

// firmwareInfoPrefix marks the information a Blues-style binary carries about itself
var firmwareInfoPrefix = []byte("firmware::info:")

type firmwareBinaryInfo struct {
	Product string `json:"product"`
	Version string `json:"version"`
	Major   int    `json:"ver_major"`
	Minor   int    `json:"ver_minor"`
	Patch   int    `json:"ver_patch"`
	Build   int    `json:"ver_build"`
}

// firmwareEmbeddedInfo reads metadata from the binary, independent of its filename.
func firmwareEmbeddedInfo(contents []byte) (info firmwareBinaryInfo) {
	at := bytes.Index(contents, firmwareInfoPrefix)
	if at < 0 || json.NewDecoder(bytes.NewReader(contents[at+len(firmwareInfoPrefix):])).Decode(&info) != nil {
		return firmwareBinaryInfo{}
	}
	return info
}

// firmwareEmbeddedVersion reads the version a binary carries, as the hub does: the
// numbers in the JSON object after "firmware::info:", or its version string
func firmwareEmbeddedVersion(contents []byte) string {
	info := firmwareEmbeddedInfo(contents)
	if info.Major > 0 || info.Minor > 0 || info.Patch > 0 || info.Build > 0 {
		version := fmt.Sprintf("%d.%d.%d", info.Major, info.Minor, info.Patch)
		if info.Build > 0 {
			version += fmt.Sprintf(".%d", info.Build)
		}
		return version
	}
	return info.Version
}

// firmwareUploadCommand uploads a firmware. One the project already holds, byte for byte, is
// skipped; one it holds under the same name and version is replaced, the new one going
// up before the old one comes down, unless --no-replace refuses that.
func firmwareUploadCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: %s firmware upload <path> [name]", cliName)
	}
	contents, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	if flagFirmwareType == "" && strings.EqualFold(firmwareEmbeddedInfo(contents).Product, "Notecard") {
		ftype = firmwareNotecard
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if ftype == firmwareNotecard {
		if flagFirmwareVersion != "" {
			return errors.New("a notecard firmware's version is in the binary, so --version doesn't apply")
		}
		if project == "" {
			project = firmwareNotecardProject
		}
	}
	name := filepath.Base(args[0])
	if len(args) == 2 {
		name = args[1]
	}
	if err = skillsCheckPath(name); err != nil || strings.Contains(name, "/") {
		return fmt.Errorf("'%s' is not a name a firmware may have, which is a filename", name)
	}
	notes, err := uploadsNotes()
	if err != nil {
		return err
	}
	version := flagFirmwareVersion
	if version == "" {
		version = firmwareEmbeddedVersion(contents)
	}

	// What becomes of it: skipped, new, or a replacement of what has its name and version
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	md5 := uploadsMD5(contents)
	state := "new"
	superseded := []string{}
	for _, item := range items {
		if !strings.EqualFold(firmwareOriginal(item.Filename), name) {
			continue
		}
		if strings.EqualFold(item.MD5, md5) {
			state = "unchanged"
			break
		}
		if firmwareSameVersion(item.Version, version) {
			state = "replaced"
			superseded = append(superseded, item.Filename)
		}
	}
	if state == "replaced" && flagUploadsNoReplace {
		return fmt.Errorf("%s already holds %s at version %s, with other bytes - without --no-replace it would be replaced", project, name, version)
	}
	label := fmt.Sprintf("%d bytes", len(contents))
	if version != "" {
		label = "version " + version + ", " + label
	}
	skillsReport(state, len(name), name, label)
	if state == "unchanged" {
		skillsSummary("%s already holds it.", project)
		return nil
	}
	if flagSkillsDryRun {
		skillsSummary("%s would be %s in %s.", name, map[string]string{"new": "stored", "replaced": "replaced"}[state], project)
		return nil
	}

	// Up it goes, and then the old ones come down
	query := url.Values{}
	if flagFirmwareVersion != "" {
		query.Set("version", flagFirmwareVersion)
	}
	if notes != "" {
		query.Set("notes", notes)
	}
	rsp, err := firmwareCall(http.MethodPut, "/v1/projects/"+project+"/firmware/"+ftype+"/"+name, query, contents, "application/octet-stream")
	if err != nil {
		return err
	}
	var stored firmwareItem
	if err = note.JSONUnmarshal(rsp.Body, &stored); err != nil {
		return fmt.Errorf("the hub's answer can't be read: %w", err)
	}
	for _, old := range superseded {
		if old == stored.Filename {
			continue
		}
		if ftype == firmwareNotecard {
			err = uploadsRemove(firmwareScope, old)
		} else {
			_, err = firmwareCall(http.MethodDelete, "/v1/projects/"+project+"/firmware/"+ftype+"/"+old, nil, nil, "")
		}
		if err != nil {
			return fmt.Errorf("%s: %s", old, err)
		}
	}
	skillsSummary("%s is stored in %s as %s, version %s.", name, project, stored.Filename, stored.Version)
	return nil

}

// firmwareEditCommand changes a host firmware's version, notes or info
func firmwareEditCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	if ftype != firmwareHost {
		return fmt.Errorf("%s firmware can't be edited - its version is in the binary", ftype)
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s firmware edit <firmware> --version <version> --notes <notes> --info <json>", cliName)
	}
	changes := map[string]any{}
	if flagFirmwareVersion != "" {
		changes["version"] = flagFirmwareVersion
	}
	if flagUploadsNotes != "" {
		notes, notesErr := uploadsNotes()
		if notesErr != nil {
			return notesErr
		}
		changes["notes"] = notes
	}
	if flagFirmwareInfo != "" {
		info, infoErr := firmwareInfoGiven()
		if infoErr != nil {
			return infoErr
		}
		changes["info"] = info
	}
	if len(changes) == 0 {
		return errors.New("give what to change: --version, --notes or --info")
	}
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	item, err := firmwareResolve(items, args[0])
	if err != nil {
		return err
	}
	body, err := note.JSONMarshal(changes)
	if err != nil {
		return err
	}
	if !flagSkillsDryRun {
		if _, err = firmwareCall(http.MethodPost, "/v1/projects/"+project+"/firmware/"+ftype+"/"+item.Filename, nil, body, "application/json"); err != nil {
			return err
		}
	}
	fmt.Printf("  edited    %s: %s\n", item.Filename, body)
	skillsSummary("%s %s in %s.", item.Filename, skillsVerb("is changed", "would be changed"), project)
	return nil
}

// firmwareInfoGiven returns the --info metadata, given as JSON or as @file
func firmwareInfoGiven() (map[string]any, error) {
	text := flagFirmwareInfo
	if strings.HasPrefix(text, "@") {
		contents, err := os.ReadFile(text[1:])
		if err != nil {
			return nil, fmt.Errorf("--info: %w", err)
		}
		text = string(contents)
	}
	info := map[string]any{}
	if err := note.JSONUnmarshal([]byte(text), &info); err != nil {
		return nil, fmt.Errorf("--info must be a JSON object: %w", err)
	}
	return info, nil
}

// firmwareDeleteCommand removes one firmware
func firmwareDeleteCommand(config *lib.ConfigSettings, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, false)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: %s firmware delete <firmware>", cliName)
	}
	items, err := firmwareList(ftype, project)
	if err != nil {
		return err
	}
	item, err := firmwareResolve(items, args[0])
	if err != nil {
		return err
	}
	if !flagSkillsDryRun {
		if ftype == firmwareNotecard {
			err = uploadsRemove(firmwareScope, item.Filename)
		} else {
			_, err = firmwareCall(http.MethodDelete, "/v1/projects/"+project+"/firmware/"+ftype+"/"+item.Filename, nil, nil, "")
		}
		if err != nil {
			return err
		}
	}
	fmt.Printf("  removed   %s (version %s)\n", item.Filename, item.Version)
	holder := project
	if ftype == firmwareNotecard {
		holder = "the hub"
	}
	skillsSummary("%s %s from %s.", item.Filename, skillsVerb("is removed", "would be removed"), holder)
	return nil
}

// dfuTarget is one set of devices a DFU call concerns, as the API filters them
type dfuTarget struct {
	Label string
	Query url.Values
}

// firmwareTargets returns the devices --scope names, as the targets of DFU calls: the
// devices named in one, and each fleet in its own.  With no scope, an action is refused,
// and a query concerns every device.
func firmwareTargets(project string, action bool) ([]dfuTarget, error) {

	scope := strings.TrimSpace(flagScope)
	targets := []dfuTarget{}
	switch {
	case scope == "" && action:
		return nil, errors.New("use --scope to name the devices or fleets concerned, or all")
	case scope == "" || strings.EqualFold(scope, "all"):
		if action && !strings.EqualFold(scope, "all") {
			break
		}
		if action && !flagSkillsForce {
			return nil, errors.New("--scope all concerns every device in the project - add --force to mean it")
		}
		targets = append(targets, dfuTarget{Label: "every device in " + project, Query: url.Values{}})
	default:
		devices, fleets, err := firmwareScopeItems(project, scope)
		if err != nil {
			return nil, err
		}
		if len(devices) != 0 {
			targets = append(targets, dfuTarget{Label: fmt.Sprintf("%d device(s): %s", len(devices), strings.Join(devices, ", ")), Query: url.Values{"deviceUID": devices}})
		}
		for _, fleet := range fleets {
			targets = append(targets, dfuTarget{Label: "fleet " + fleet, Query: url.Values{"fleetUID": {fleet}}})
		}
	}

	// The filters that narrow every target
	for _, target := range targets {
		for _, tag := range flagFirmwareTags {
			target.Query.Add("tag", tag)
		}
		for _, sn := range flagFirmwareSNs {
			target.Query.Add("serialNumber", sn)
		}
		for _, sku := range flagFirmwareSKUs {
			target.Query.Add("sku", sku)
		}
	}
	return targets, nil

}

// firmwareScopeItems reads a scope: devices and fleets by UID, fleets by name, and files
// of them, separated by commas or lines
func firmwareScopeItems(project string, scope string) (devices []string, fleets []string, err error) {
	var fleetNames map[string]string
	for _, item := range strings.Split(scope, ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
		case strings.HasPrefix(item, "dev:") || strings.HasPrefix(item, "imei:"):
			devices = append(devices, item)
		case strings.HasPrefix(item, "fleet:"):
			fleets = append(fleets, item)
		case strings.HasPrefix(item, "@") && firmwareIsFile(item[1:]):
			contents, readErr := os.ReadFile(item[1:])
			if readErr != nil {
				return nil, nil, readErr
			}
			for _, line := range strings.Split(string(contents), "\n") {
				moreDevices, moreFleets, lineErr := firmwareScopeItems(project, line)
				if lineErr != nil {
					return nil, nil, lineErr
				}
				devices, fleets = append(devices, moreDevices...), append(fleets, moreFleets...)
			}
		default:
			if fleetNames == nil {
				if fleetNames, err = firmwareFleets(project); err != nil {
					return nil, nil, err
				}
			}
			name := strings.TrimPrefix(item, "@")
			found := false
			for label, uid := range fleetNames {
				if fleetMatchesScope(label, name) {
					fleets, found = append(fleets, uid), true
				}
			}
			if !found {
				return nil, nil, fmt.Errorf("'%s' is not a device, a fleet, or the name of one of %s's fleets", item, project)
			}
		}
	}
	return sortAndRemoveDuplicates(devices), sortAndRemoveDuplicates(fleets), nil
}

// firmwareIsFile reports whether a name is a file that exists
func firmwareIsFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

// firmwareFleets returns a project's fleets, by label, through the v1 call
func firmwareFleets(project string) (map[string]string, error) {
	rsp, err := firmwareCall(http.MethodGet, "/v1/projects/"+project+"/fleets", nil, nil, "")
	if err != nil {
		return nil, err
	}
	var listing struct {
		Fleets []struct {
			UID   string `json:"uid"`
			Label string `json:"label"`
		} `json:"fleets"`
	}
	if err = note.JSONUnmarshal(rsp.Body, &listing); err != nil {
		return nil, fmt.Errorf("the fleet listing can't be read: %w", err)
	}
	fleets := map[string]string{}
	for _, fleet := range listing.Fleets {
		fleets[fleet.Label] = fleet.UID
	}
	return fleets, nil
}

// firmwareUpdateCommand asks the devices in --scope to install a firmware
func firmwareUpdateCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: %s firmware update <firmware> --scope <devices>", cliName)
	}
	return firmwareAction("update", args[0])
}

// firmwareCancelCommand calls off the update under way for the devices in --scope
func firmwareCancelCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s firmware cancel --scope <devices>", cliName)
	}
	return firmwareAction("cancel", "")
}

// firmwareAction performs a DFU action for each target --scope names
func firmwareAction(action string, name string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, true)
	if err != nil {
		return err
	}
	request := map[string]any{}
	what := "the update under way"
	if action == "update" {
		items, listErr := firmwareList(ftype, project)
		if listErr != nil {
			return listErr
		}
		item, resolveErr := firmwareResolve(items, name)
		if resolveErr != nil {
			return resolveErr
		}
		request["filename"] = item.Filename
		what = fmt.Sprintf("%s (version %s)", item.Filename, item.Version)
	}
	targets, err := firmwareTargets(project, true)
	if err != nil {
		return err
	}
	body, err := note.JSONMarshal(request)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if !flagSkillsDryRun {
			if _, err = firmwareCall(http.MethodPost, "/v1/projects/"+project+"/dfu/"+ftype+"/"+action, target.Query, body, "application/json"); err != nil {
				return fmt.Errorf("%s: %s", target.Label, err)
			}
		}
		fmt.Printf("  %-9s %s: %s\n", action, target.Label, what)
	}
	verb := map[string]string{"update": "requested", "cancel": "called off"}[action]
	skillsSummary("%s %s for %d target(s) in %s.", what, skillsVerb(verb, "would be "+verb), len(targets), project)
	return nil
}

// dfuNode is one step of a device's update
type dfuNode struct {
	Phase       string `json:"phase"`
	Datetime    string `json:"datetime"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// dfuMachine is one update of a device, from a version to another, step by step
type dfuMachine struct {
	RequestedVersion string    `json:"requested_version"`
	CurrentVersion   string    `json:"current_version"`
	Initiated        string    `json:"initiated"`
	Updates          []dfuNode `json:"updates"`
}

// dfuDevice is a device's standing in the DFU status and history calls
type dfuDevice struct {
	DeviceUID  string `json:"device_uid"`
	InProgress bool   `json:"dfu_in_progress"`
	Current    struct {
		Version string `json:"version"`
	} `json:"current"`
	Status  *dfuMachine  `json:"status"`
	History []dfuMachine `json:"history"`
}

// firmwareDevices fetches every page of a DFU query for each target
func firmwareDevices(ftype string, project string, call string, targets []dfuTarget) ([]json.RawMessage, []dfuDevice, error) {
	raw := []json.RawMessage{}
	devices := []dfuDevice{}
	for _, target := range targets {
		for pageNum := 1; ; pageNum++ {
			query := url.Values{}
			for key, values := range target.Query {
				query[key] = values
			}
			query.Set("pageSize", "100")
			query.Set("pageNum", strconv.Itoa(pageNum))
			rsp, err := firmwareCall(http.MethodGet, "/v1/projects/"+project+"/dfu/"+ftype+"/"+call, query, nil, "")
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", target.Label, err)
			}
			var page struct {
				Devices []json.RawMessage `json:"devices"`
				HasMore bool              `json:"has_more"`
			}
			if err = note.JSONUnmarshal(rsp.Body, &page); err != nil {
				return nil, nil, fmt.Errorf("the hub's answer can't be read: %w", err)
			}
			for _, entry := range page.Devices {
				var device dfuDevice
				if err = note.JSONUnmarshal(entry, &device); err != nil {
					return nil, nil, fmt.Errorf("the hub's answer can't be read: %w", err)
				}
				raw, devices = append(raw, entry), append(devices, device)
			}
			if !page.HasMore {
				break
			}
		}
	}
	return raw, devices, nil
}

// firmwareStatusCommand shows each device's update as it goes
func firmwareStatusCommand(config *lib.ConfigSettings, args []string) error {
	return firmwareQuery("status", args)
}

// firmwareHistoryCommand shows each device's past updates
func firmwareHistoryCommand(config *lib.ConfigSettings, args []string) error {
	return firmwareQuery("history", args)
}

// firmwareQuery runs a DFU query for the devices --scope names, or every device
func firmwareQuery(call string, args []string) error {
	ftype, err := firmwareType()
	if err != nil {
		return err
	}
	project, err := firmwareProject(ftype, true)
	if err != nil {
		return err
	}
	if len(args) != 0 {
		return fmt.Errorf("'%s firmware %s' takes no arguments - name the devices with --scope", cliName, call)
	}
	targets, err := firmwareTargets(project, false)
	if err != nil {
		return err
	}
	raw, devices, err := firmwareDevices(ftype, project, call, targets)
	if err != nil {
		return err
	}
	if flagJson {
		return filesWriteJSON(map[string]any{"devices": raw})
	}
	if len(devices) == 0 {
		fmt.Printf("no devices.\n")
		return nil
	}
	width := 0
	for _, device := range devices {
		width = max(width, len(device.DeviceUID))
	}
	for _, device := range devices {
		if call == "status" {
			fmt.Printf("  %-*s  %s\n", width, device.DeviceUID, dfuStatusLine(device))
			continue
		}
		fmt.Printf("  %s (running %s)\n", device.DeviceUID, dfuVersion(device.Current.Version))
		if len(device.History) == 0 {
			fmt.Printf("    no updates\n")
		}
		for _, machine := range device.History {
			fmt.Printf("    %s\n", dfuMachineLine(machine))
		}
	}
	return nil
}

// dfuVersion is a version as displayed, when there may be none
func dfuVersion(version string) string {
	if version == "" {
		return "unknown version"
	}
	return version
}

// dfuWhen is a DFU timestamp as displayed
func dfuWhen(datetime string) string {
	if when, err := time.Parse(time.RFC3339, datetime); err == nil {
		return when.Local().Format("2006-01-02 15:04")
	}
	return datetime
}

// dfuStatusLine describes where a device's update stands
func dfuStatusLine(device dfuDevice) string {
	line := "running " + dfuVersion(device.Current.Version)
	if device.Status == nil || (!device.InProgress && len(device.Status.Updates) == 0) {
		return line + ", no update under way"
	}
	machine := device.Status
	state := "done"
	if device.InProgress {
		state = "under way"
	}
	line += fmt.Sprintf(", update to %s %s", dfuVersion(machine.RequestedVersion), state)
	if len(machine.Updates) != 0 {
		last := machine.Updates[len(machine.Updates)-1]
		line += fmt.Sprintf(": %s at %s", last.Phase, dfuWhen(last.Datetime))
		if last.Description != "" {
			line += ", " + last.Description
		}
	}
	return line
}

// dfuMachineLine describes one update of a device, step by step
func dfuMachineLine(machine dfuMachine) string {
	steps := []string{}
	for _, node := range machine.Updates {
		steps = append(steps, fmt.Sprintf("%s (%s)", node.Phase, dfuWhen(node.Datetime)))
	}
	line := fmt.Sprintf("%s  %s -> %s", dfuWhen(machine.Initiated), dfuVersion(machine.CurrentVersion), dfuVersion(machine.RequestedVersion))
	if len(steps) != 0 {
		line += ": " + strings.Join(steps, ", ")
	}
	return line
}
