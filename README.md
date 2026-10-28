# Notecard & Notehub CLI

Two command-line tools for working with Blues devices and services:

- **`notecard`** communicates with a Notecard attached to your computer.
- **`notehub`** manages Notehub projects, devices, data, firmware, and project skills.

## Install

Install both tools with Homebrew:

```sh
brew install --cask blues/note-cli/note-cli
```

Or download the appropriate binary archive from the
[releases page](https://github.com/blues/note-cli/releases).

> [!IMPORTANT]
> When upgrading a Homebrew installation older than v1.9.1, first run
> `brew uninstall note-cli`.

This README describes the code in this branch. To use changes that are not yet in a
release, [build from source](#build-from-source). Check your installed version with
`notehub --version` or `notecard --version`.

## Using Notehub from an AI agent

Start by reading the guide embedded in the installed CLI:

```sh
notehub --help-agent
```

The same guide is available in [notehub/help-agent.md](notehub/help-agent.md).
It covers authentication, API discovery, request bodies, responses, and modification
cursors. An agent with command execution can use `notehub` directly; an MCP connection
is not required.

### 1. Check authentication

```sh
notehub auth status --json
```

Read `signed_in` in the returned object. **Exit code 0 alone does not mean the user
is signed in.** If `signed_in` is true, reuse that connection. Otherwise inspect
`reason` and `action`; a network failure calls for retrying the check, not starting
a new login.

### 2. Guide the user through sign-in

If authorization is needed, start this command in a persistent execution session:

```sh
notehub login --name "My Agent" --json
```

The CLI does not open a browser or prompt for input. The user approves access in
their own browser:

1. Read stdout while the command is running. It emits one JSON object per line.
2. As soon as an object contains `url`, give the user that exact link and ask them
   to open it, sign in, and approve access for your named agent. They do not need
   to copy anything back.
3. Keep monitoring the **same process**. Progress messages contain
   `expires_in_seconds`; allow that window, normally up to ten minutes. Do not
   wait for the process to exit before showing the link or start duplicate logins.
4. Continue with the task only after a final object contains `success: true` and
   the command exits successfully. On failure, explain `status`. Respect
   cancellation; if the link expires, start again only if the user wants to proceed.

An existing valid login can produce the final success object immediately. The CLI
saves credentials and renews OAuth access tokens when the service provides a refresh
token. Reuse the login across tasks.

For unattended use, the user can supply a personal access token through the agent
runtime's secret settings as `NOTEHUB_TOKEN`. It overrides saved credentials;
check `auth status --json` again, and no login command is needed. Have the user
replace or remove a rejected environment token before trying browser sign-in.
Keep credentials out of chat and command output; let the CLI add authentication.

If the service reports that polling sign-in is unavailable, follow its instructions
for interactive sign-in on the machine running the CLI, or use `NOTEHUB_TOKEN`.

### 3. Discover the API and the target project

```sh
notehub api --spec > notehub-openapi.yaml
notehub api GET /v1/projects
```

`--spec` fetches the selected hub's public OpenAPI specification without requiring
login. On the default service, it is available at
[api.notefile.net/openapi.yaml](https://api.notefile.net/openapi.yaml).
Read the relevant operation and its referenced schemas before constructing a request.
Use project and device identifiers returned by Notehub, and resolve ambiguous targets
with the user before making changes.

If the project has skills, read them for context about its devices and data:

```sh
notehub skills --project app:PROJECT_UID
```

Examples below use placeholder identifiers such as `app:PROJECT_UID` and
`dev:DEVICE_UID`; replace them with real identifiers. For additional guidance about
working with a user's Notehub project, `notehub --agent-chat` fetches the public
guide from [chat.notehub.io](https://chat.notehub.io).

## Notehub commands

```text
notehub <mode> [options] [arguments]
notehub [options] [request]
```

| Mode | Purpose |
| --- | --- |
| `login`, `logout` | Sign in to or out of a hub |
| `auth` | Check and manage saved logins |
| `api` | Make authenticated HTTP requests to the Notehub API |
| `firmware` | Upload host firmware and manage device updates |
| `skills` | Read, edit, back up, and restore a project's skills |
| No mode | Send Notecard-format requests and manage notefiles, variables, and provisioning |

Run `notehub` for a command overview and saved settings, `notehub --help` for the
default mode's options, or `notehub <mode> --help` for a mode's commands and examples.
Put the mode first. Within a mode, options may appear before or after arguments;
in the default mode, put options before the request. A standalone `--` ends option
parsing. Values accept either `--project app:PROJECT_UID` or
`--project=app:PROJECT_UID`; booleans accept `--pretty` or `--pretty=false`.

### Sign-in and hub selection

For interactive use:

```sh
notehub login --name "My laptop"
notehub auth status
notehub auth status --all
notehub logout
```

Browser logins appear in Notehub's API Access screen as `Notehub CLI - <name>`.
Without `--name`, the CLI uses the computer's name. `notehub auth login` and
`notehub auth logout` are aliases for `login` and `logout`.

The CLI stores one login per hub. `--hub` selects a hub for one command, and logging
in to another hub does not change the active one:

```sh
notehub login --hub example.com
notehub auth switch example.com
```

`notehub --hub example.com` by itself also saves the active hub. `auth status --all`
checks every saved login; `--all --json` returns an object with `active` and a `hubs`
array. `--all` cannot be combined with `--hub`.

To save a personal access token supplied through a secret variable, pass it on stdin:

```sh
printf '%s\n' "$SECRET" | notehub login --with-token
```

`NOTEHUB_TOKEN` takes precedence over saved logins and must be cleared before
changing them or signing out. OAuth logins renew automatically when possible;
personal access tokens are not refreshed. A revoked login requires new authorization.

### API requests

Name the HTTP method explicitly so adding fields does not change the operation:

```sh
notehub api GET '/v1/projects/{project}/devices' --project app:PROJECT_UID
notehub api GET '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID
notehub api PUT '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID --input vars.json
```

Pass a path, not a full URL. Quote paths containing braces or query strings.
`--project`, `--product`, and `--device` fill their corresponding placeholders;
actual identifiers can also be written directly into the path. Without an explicit
method, the default is GET, or POST when fields or a body are supplied.

- `--field key=value` converts numbers, booleans, and `null`; other values remain
  strings. `@file` and `@-` read a file or stdin as a string value.
- `--raw-field key=value` always supplies a literal string. Use it for cursors.
- With GET or HEAD, fields become query parameters. With other methods, they form
  a JSON body. Use `--input FILE` or `--input -` for nested JSON, arrays, or a complete
  body; it cannot be combined with field options.
- `--header 'Name: value'` adds a header. JSON is the default body content type.

The response body goes to stdout and diagnostics to stderr. JSON responses need no
`--json` flag. HTTP errors retain their response body and return a nonzero exit code;
successful responses may be empty. Check the exit code before treating a request as
successful. After an uncertain write result, inspect the resource before retrying.

`--include` adds the HTTP status and headers. `--pretty` formats JSON but buffers the
response; omit it for streaming responses. `--verbose` writes request details and
response status to stderr.

### Large collections and modification cursors

Use `--paginate` on GET endpoints whose specification declares a `cursor` parameter,
including project events and devices:

```sh
notehub api GET '/v1/projects/{project}/events' --project app:PROJECT_UID \
  --paginate --field pageSize=1000
```

The CLI starts at `cursor=0`, follows the returned `next_cursor` or `X-Next-Cursor`
header, and stops when `has_more` or `X-Has-More` is false. It writes each response
separately, preserving the response bodies and continuation metadata rather than
combining them into one JSON document. Use `--include` to retain cursor headers for
endpoints whose bodies are arrays or other formats.

To start at a particular modification time, supply a UTC Unix epoch timestamp in
**microseconds**. For subsequent reads, pass the exact returned cursor as a string:

```sh
notehub api GET '/v1/projects/{project}/events' --project app:PROJECT_UID \
  --paginate --raw-field cursor="$CURSOR" --field pageSize=1000
```

Results arrive in ascending modification order. A returned cursor may include more
than the timestamp; preserve it exactly so records sharing a timestamp are not skipped.
Save the final cursor after processing the results, then reuse it with the same
endpoint and filters to read later changes. An empty response can still carry a useful
cursor. Apply updates by resource identifier and handle `deleted: true` where supported.

For processing and checkpointing one batch at a time, omit `--paginate` and advance
the cursor yourself after each successful batch. Automatic pagination stops with an
error on missing continuation metadata or a repeated cursor while more results remain.
Do not combine it with a page number beyond 1 or ordering options that conflict with
the endpoint's modification order.

### Firmware

```sh
notehub firmware list --project app:PROJECT_UID --json
notehub firmware upload ./build/app.bin --version 1.2.3 --project app:PROJECT_UID
notehub firmware show 1.2.3 --project app:PROJECT_UID
notehub firmware get 1.2.3 ./app-1.2.3.bin --project app:PROJECT_UID
notehub firmware update 1.2.3 --scope "Field Units" --project app:PROJECT_UID --dry-run
notehub firmware update 1.2.3 --scope "Field Units" --project app:PROJECT_UID
notehub firmware status --scope "Field Units" --project app:PROJECT_UID --json
```

Identify firmware by its stored filename, original filename, or an unambiguous version.
Quote stored names containing `$`, such as `'app$20260324190911.bin'`. Upload reads the
version from the binary when available, skips identical content, and replaces firmware
with the same name and version unless `--no-replace` is set. `set` is an alias for
`upload`. Use `edit` to change metadata, `delete` to remove firmware, `cancel` to cancel
an update, and `history` to inspect past updates.

`--scope` accepts device or fleet UIDs, fleet names, or `@file` containing targets.
`--tag`, `--sn`, and `--sku` narrow the selection. Updating or cancelling across
`--scope all` requires `--force`. `--dry-run` previews uploads, edits, deletes, updates,
and cancellations. Downloads verify MD5 and require `--force` to overwrite a different
local file. `--json` is available for listings, metadata, status, and history.

### Project skills

Skills are Markdown files that explain a project's product and data to an agent.
To create them, point an agent at [notehub.md](https://notehub.md) and ask it to train
your project.

```sh
notehub skills --project app:PROJECT_UID
notehub skills list --project app:PROJECT_UID
notehub skills show index.md --project app:PROJECT_UID
notehub skills get all ./skills --project app:PROJECT_UID
notehub skills set ./skills --project app:PROJECT_UID --dry-run
notehub skills set ./skills --project app:PROJECT_UID
notehub skills backup skills.zip --project app:PROJECT_UID
notehub skills restore skills.zip --project app:PROJECT_UID --dry-run
```

With no command, `skills --project ...` assembles the full set into one linked document.
`show` displays one skill; `get <name> -` writes its exact contents to stdout.
`set` uploads changed files without deleting other skills. `rename` changes a skill's
name, and `delete` removes it. Kinds are stored as tags from the `kind:`, `kinds:`, or
`tags:` field in Markdown front matter and preserved in backups.

**`restore` makes the project match the ZIP archive, deleting skills absent from it.**
Preview it with `--dry-run`, then omit that flag to apply the changes. `--dry-run` also
works with `set`, `rename`, `delete`, and `backup`. Overwriting different local files
with `get` or `backup`, replacing another skill with `rename`, and `delete all` require
`--force`.

The CLI uses the current Skills API and handles names with nested paths and punctuation.
Writes check the revision that was read; on a conflict, reread and review before
retrying. Rename and restore involve multiple operations and can finish only partially
if an operation fails. Restore verifies the resulting set before reporting success.
If a full-set read exceeds the service's size limit, use `list` and named reads.

### Notefiles, environment variables, and provisioning

The default mode accepts Notecard-format JSON requests and device-management options:

```sh
notehub --projects --pretty
notehub --project app:PROJECT_UID --device dev:DEVICE_UID \
  '{"req":"note.get","file":"hub.db","note":"config"}'
notehub --project app:PROJECT_UID --device dev:DEVICE_UID --explore
notehub --project app:PROJECT_UID --scope "Field Units" --get-vars --pretty
notehub --project app:PROJECT_UID --scope dev:DEVICE_UID \
  --set-vars '{"interval":"60","debug":"-"}'
notehub --product com.example:sensor --scope dev:DEVICE_UID --provision --sn SN-0042
```

Give the request as the last argument or with `--req`; `@file` reads it from a file.
Responses are JSON, formatted with `--pretty` when requested. `--scope` accepts device
or fleet UIDs, fleet names, `@fleet` for a fleet's devices, `@file` listing targets, and
`devices` or `fleets` for all of them. Separate multiple targets with commas. Setting
an environment variable to `-` removes it.

## Using the Notecard CLI

```text
notecard [options] ['{"req":"..."}']
```

Run `notecard` to see saved settings and available ports, or `notecard --help` for
all options and examples. Select your attached Notecard's actual port:

```sh
notecard --interface serial --port /dev/ttyUSB0
notecard '{"req":"card.version"}'
notecard --pretty --req '{"req":"card.version"}'
notecard --info
notecard --explore
notecard --trace
```

Setting only the interface and port saves them for later commands. When combined
with a request or another action, they apply only to that invocation.
`NOTE_INTERFACE`, `NOTE_PORT`, and `NOTE_PORT_CONFIG` override saved settings.

Put options before the JSON request. Replies use the Notecard's JSON format; check
both the exit code and any `err` field. A standalone `--` ends option parsing.

## Build from source

From the repository root with Go 1.25 or later, build both tools using the published
dependencies pinned in the module files:

```sh
GOWORK=off go build -o ./dist/notecard ./notecard
GOWORK=off go build -o ./dist/notehub ./notehub
./dist/notehub --help-agent
```

The examples use POSIX shell syntax. Add `dist` to your PATH or invoke the binaries
by their paths. `GOWORK=off` ensures the build uses the pinned dependencies even if
you have a local development workspace.

To run the audit checks and tests, including the separate library module:

```sh
GOWORK=off go mod verify
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./...
(cd lib && GOWORK=off go mod verify && GOWORK=off go vet ./... && GOWORK=off go test ./...)
```

## Additional resources

- [Blues](https://blues.com)
- [Developer documentation](https://dev.blues.io)
- [Notehub](https://notehub.io)

## License

Copyright (c) 2017 Blues Inc. Released under the MIT license. See [LICENSE](LICENSE).
