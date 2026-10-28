// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// The modes understood by this CLI.  A mode is an optional first-argument keyword that
// selects which switches apply and how the remaining arguments are read.  To add one, add
// it to the table below, write its handler, and list it in the Modes of each switch it
// accepts (see switches.go).

package main

import "github.com/blues/note-cli/lib/cli"

// The name of each mode, as typed on the command line
const (
	// modeDefault is the CLI's original behavior, and what runs when no mode is given
	modeDefault = cli.ModeDefault

	// modeLogin logs in to Notehub, the same as 'auth login'
	modeLogin = "login"

	// modeLogout logs out of Notehub, the same as 'auth logout'
	modeLogout = "logout"

	// modeAuth manages logins: login, logout, status, switch and token
	modeAuth = "auth"

	// modeApi sends one request to the hub's API, like gh api
	modeApi = "api"

	// modeFirmware manages a project's host firmware, and applies it to devices
	modeFirmware = "firmware"

	// modeFiles manages the files a project holds; it works but isn't listed yet
	modeFiles = "files"

	// modeSkills reads and manages the skills a project holds
	modeSkills = "skills"

	// modeHelp is the framework's hidden alias of --help
	modeHelp = cli.Help
)

// notehubModes returns every mode, in the order help displays them
func notehubModes() []*cli.Mode {
	return []*cli.Mode{
		{
			Name:     modeDefault,
			Summary:  "send requests, read notefiles, and manage environment variables and provisioning",
			Args:     "[request]",
			Preamble: apiAccessPreamble(),
			Examples: []cli.Example{
				{What: "Sign into the hub in your browser", Command: cliName + " login"},
				{What: "Sign in from an agent, under its own name", Command: agentLoginForms()[0]},
				{What: "Sign in from an agent with a personal access token", Command: agentLoginForms()[1]},
				{What: "Check the active connection's login", Command: cliName + " auth status"},
				{What: "List the projects you belong to, with their products", Command: cliName + " --projects --pretty"},
				{What: "Send an API request, authorized as you", Command: cliName + " api /v1/projects"},
				{What: "Send a request to a device's notefiles, in the Notecard's own JSON form", Command: cliName + ` --product <productUID> --device <deviceUID> '{"req":"note.get","file":"hub.db","note":"config"}'`},
				{What: "List a device's notefiles and notes", Command: cliName + " --product <productUID> --device <deviceUID> --explore"},
				{What: "Get the environment variables of a fleet", Command: cliName + ` --product <productUID> --scope "Field Units" --get-vars --pretty`},
				{What: "Set one on a device, and remove another", Command: cliName + ` --product <productUID> --scope <deviceUID> --set-vars '{"interval":"60","debug":"-"}'`},
				{What: "Read a project's skills as one document", Command: cliName + " skills --project <projectUID>"},
			},
			Detail: "With no mode, notehub works on one project, named by --project or --product,\n" +
				"and often one device, named by --device.  A request in the Notecard's own JSON\n" +
				"form, such as {\"req\":\"note.get\",\"file\":\"hub.db\",\"note\":\"config\"}, is sent\n" +
				"to the hub as the Notecard would send it, and the hub's answer is written as\n" +
				"JSON.  The request is the last argument, or --req, or @file, and the options\n" +
				"come before it.\n\n" +
				"--scope names the devices or fleets that --get-vars, --set-vars, --provision\n" +
				"and --explore concern: dev: and fleet: UIDs, a fleet's name, @fleet for a\n" +
				"fleet's devices, @file for a file listing any of these one per line, or\n" +
				"devices or fleets for all of them, separated by commas.\n\n" +
				"Each mode has help of its own: notehub <mode> --help.",
			Run: runDefault,
		},
		{
			Name:     modeLogin,
			Summary:  "sign into hub: in your browser, with a token, or from an agent",
			Preamble: apiAccessPreamble(),
			Examples: []cli.Example{
				{What: "Sign into the active hub in your browser", Command: cliName + " login"},
				{What: "The same, listed in API Access under a name of your choosing", Command: cliName + ` login --name "Ray's laptop"`},
				{What: "Sign in with a personal access token, kept out of your shell history", Command: `echo "$SECRET" | ` + cliName + " login --with-token"},
				{What: "Sign in from an agent: the URL, progress and result as JSON", Command: cliName + ` login --name "DotBot" --json`},
				{What: "Sign in from an agent with a personal access token", Command: `echo "$SECRET" | ` + cliName + " login --with-token --json"},
				{What: "Sign into another hub, leaving the active one as it is", Command: cliName + " login --hub preprod.blues.tools"},
			},
			Detail: "'notehub login' opens your browser to sign into Notehub, and saves the login\n" +
				"for the requests that follow.  With --with-token, it signs in with a personal\n" +
				"access token instead, given after the switch or on stdin, which keeps it out\n" +
				"of your shell history.  The login is listed in Notehub's API Access as\n" +
				"'Notehub CLI - <name>', with the computer's name unless --name gives another.\n" +
				"Quote names with spaces.\n\n" +
				"A login that Notehub still accepts is kept: 'notehub login' checks it and exits with\n" +
				"success, without opening anything, so it is safe to run just to make sure you\n" +
				"are signed in.\n\n" +
				"With --json, nothing opens and nothing is typed: the URL for the user to open,\n" +
				"progress, and the result are written to stdout as JSON, one object per line,\n" +
				"which is how an agent signs in, under a --name of its own; with a login already\n" +
				"in place, the result is all that is written.  --expires-in asks the hub for a\n" +
				"shorter window than its usual ten minutes.\n\n" +
				"A login is to the active hub, or to the one given by --hub, which stays as it\n" +
				"is otherwise: 'notehub auth switch' makes a hub the active one.  To see every\n" +
				"login, use 'notehub auth status --all'.",
			Run: runLogin,
		},
		{
			Name:     modeLogout,
			Summary:  "sign out of hub",
			Preamble: apiAccessPreamble(),
			Examples: []cli.Example{
				{What: "Sign out of the active hub", Command: cliName + " logout"},
				{What: "Sign out of another hub", Command: cliName + " logout --hub preprod.blues.tools"},
			},
			Detail: "'notehub logout' signs out of the active hub, or of the one given by --hub:\n" +
				"the saved credentials are removed, and a browser login is revoked at\n" +
				"Notehub, so its token is refused from then on.  A personal access token is\n" +
				"left as it is at Notehub, where its expiration is yours to manage.",
			Run: runLogout,
		},
		{
			Name:     modeAuth,
			Summary:  "manage logins: login, logout, status, switch, token",
			Preamble: apiAccessPreamble(),
			Args:     "<command>",
			Commands: authCommands(),
			Examples: []cli.Example{
				{What: "Check the active connection's login", Command: cliName + " auth status"},
				{What: "The same as JSON, nicely formatted", Command: cliName + " auth status --json --pretty"},
				{What: "Check every saved login and show which hub is active", Command: cliName + " auth status --all"},
				{What: "Make a hub you have signed into the active one", Command: cliName + " auth switch preprod.blues.tools"},
				{What: "Print the token in use, for a program that needs it", Command: cliName + " auth token"},
				{What: "Sign in from an agent, under its own name", Command: cliName + ` auth login --name "DotBot" --json`},
			},
			Detail: "'notehub auth' manages logins, of which there is one per hub.  'login' and\n" +
				"'logout' are the same as 'notehub login' and 'notehub logout'.  'status' checks\n" +
				"only the active connection, or the one given by --hub.  With --json it writes\n" +
				"that connection's status as one object, including 'signed_in'.  Add --all to\n" +
				"check every saved login; its JSON has 'active' and a 'hubs' array.  --all and\n" +
				"--hub cannot be combined.  Its exit code is 0 when the connection in use\n" +
				"accepts its login, and 1 otherwise, except with --json, where it is 0.\n\n" +
				"'switch' makes a hub you have signed into the active one, for every command\n" +
				"that doesn't name one with --hub, and 'token' prints the token in use, for a\n" +
				"program that needs it.  With NOTEHUB_TOKEN set, that token is used for every\n" +
				"request, and is what a login finds already in place; signing out is refused\n" +
				"until it is cleared.",
			Run: runAuth,
		},
		{
			Name:     modeApi,
			Summary:  "send an API request to hub authorized using saved creds",
			Args:     "[METHOD] <path>",
			Preamble: apiAccessPreamble(),
			Examples: apiExamples(),
			Detail: "'notehub api' sends one request to the hub's API, as you, and writes the\n" +
				"response body to stdout: a GET unless a method is given, or a body is sent,\n" +
				"which makes it a POST.  The path is as the API documents it, with the UIDs\n" +
				"written in, as in /v1/projects/app:123/devices.  Or write {project},\n" +
				"{product} or {device} in their place, and --project, --product or --device\n" +
				"fills in the placeholder; with none in the path, those switches aren't needed.\n\n" +
				"Parameters given with --field or --raw-field make the request a POST unless\n" +
				"the method is given, and go in a JSON body, or in the query string of a GET;\n" +
				"--input sends a file, or stdin with -, as the body.  The response is written\n" +
				"as it arrives, for as long as the hub keeps sending, unless --pretty must read\n" +
				"it whole.  The exit code is 0 when the hub answered with success, and 1 when\n" +
				"it answered with an error, whose body is still written, or couldn't be\n" +
				"reached.\n\n" +
				"--paginate reads a GET as a modification feed, starting at cursor=0 unless\n" +
				"a cursor is supplied.  It follows next_cursor and has_more from the JSON\n" +
				"response or X-Next-Cursor and X-Has-More headers, writing each response.\n" +
				"Use --raw-field cursor=<value> to start at Unix epoch microseconds or resume\n" +
				"from an exact returned cursor; --include retains continuation headers.\n\n" +
				"The hub's OpenAPI spec describes every endpoint, and --spec shows it, from\n" +
				"https://<hub>/openapi.yaml.",
			Run: runApi,
		},
		{
			Name:     modeFirmware,
			Summary:  "manage a project's host firmware, and apply it to devices",
			Args:     "[command]",
			Commands: firmwareCommands(),
			Examples: firmwareExamples(),
			Detail: "'notehub firmware' manages the host firmware a project holds, and applies it\n" +
				"to devices, through the hub's firmware and DFU calls.  With no command, it\n" +
				"lists what the project holds; a firmware's name in place of a command shows\n" +
				"it.  A firmware is named by its stored filename, such as\n" +
				"app$20260324190911.bin, or by its original name or its version when that\n" +
				"names just one.\n\n" +
				"'upload' uploads a binary, skipping one the project already holds byte for byte,\n" +
				"and replacing one of the same name and version, the new one going up before\n" +
				"the old comes down, unless --no-replace refuses that.  The version is read\n" +
				"from the binary when it carries one, or given with --version.  'edit' changes\n" +
				"the version, notes or info of a firmware already uploaded.  'set' is an alias\n" +
				"for 'upload'.\n\n" +
				"'update' asks the devices in --scope to install a firmware, 'cancel' calls\n" +
				"that off, and 'status' and 'history' show each device's update as it goes\n" +
				"and as it went.  --scope names devices, fleets, a fleet's name, or @file of\n" +
				"them, and 'all' with --force addresses every device in the project; --tag,\n" +
				"--sn and --sku narrow it further.  --dry-run says what upload, edit, delete,\n" +
				"update or cancel would do, without doing it.",
			Run: runFirmware,
		},
		{
			Name:     modeSkills,
			Summary:  "read and manage the skills a project holds",
			Args:     "[command]",
			Commands: skillsCommands(),
			Examples: []cli.Example{
				{What: "Read every skill, assembled into one linked document", Command: cliName + " skills --project <projectUID>"},
				{What: "List what the project holds", Command: cliName + " skills list --project <projectUID>"},
				{What: "Copy every skill into a directory", Command: cliName + " skills get all ./skills --project <projectUID>"},
				{What: "Store the skills in a directory that differ from the project's", Command: cliName + " skills set ./skills --project <projectUID>"},
				{What: "Save every skill into a zip file", Command: cliName + " skills backup skills.zip --project <projectUID>"},
			},
			Detail: "With no command, 'notehub skills' writes the whole set to stdout as one\n" +
				"document, index first, with cross-references turned into links.  A skill's\n" +
				"name in place of a command shows just that skill.  'get' and 'set' copy\n" +
				"skills between the project and local files, one file or a directory at a\n" +
				"time, and 'get <name> -' writes one to stdout.  'set' never removes a\n" +
				"skill; only 'delete' and 'restore' do.\n\n" +
				"'backup' saves every skill into a zip file, each at its path, and 'restore'\n" +
				"makes the project hold exactly what a zip file holds.  --dry-run says what\n" +
				"set, rename, delete, backup or restore would do, without doing it.\n\n" +
				"A skill's kinds are stored as its tags, taken from the 'kind:', 'kinds:' or\n" +
				"'tags:' line of its front matter.  To change them, edit that line and set\n" +
				"the file again.  A backup writes the kinds a skill is stored under into its\n" +
				"front matter when they aren't already there, so a restore keeps them.\n\n" +
				"To write a project's skills, point any AI agent at https://notehub.md and\n" +
				"ask it to train the project.",
			Run: runSkills,
		},
		{
			Name:     modeFiles,
			Summary:  "manage the files a project holds",
			Args:     "[command]",
			Commands: filesCommands(),
			Examples: filesExamples(),
			Hidden:   true,
			Detail: "'notehub files' manages the files a project holds: user data objects, each\n" +
				"stored under its name.  With no command, it lists them; a file's name in\n" +
				"place of a command shows its metadata.  'get' and 'set' copy files between\n" +
				"the project and local files, one file or a directory at a time, and 'get\n" +
				"<name> -' writes one to stdout.  'set' skips a file the project already holds\n" +
				"byte for byte, and replaces one it holds under the same name with other\n" +
				"bytes, unless --no-replace refuses that.  Nothing is removed except by\n" +
				"'delete'.  --dry-run says what set or delete would do, without doing it.",
			Run: runFiles,
		},
	}
}
