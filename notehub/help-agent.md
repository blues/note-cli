---
name: notehub-api
description: Use the Notehub CLI to authenticate with a user's Blues Notehub account, discover the deployed public API, and work with projects, devices, fleets, environment variables, and events. Intended for agents with command execution capability and no Notehub MCP connection.
---

# Notehub API skill for agents

## 1. Introduction

`notehub` gives agents that lack Model Context Protocol (MCP) capability rich
access to the public APIs of the Blues Notehub service through command execution.
Notehub is the cloud service that connects Blues Notecard devices to applications
and provides device management, configuration, and access to their data.
Use the default Blues Notehub service, with the API at `api.notefile.net`.

Use this skill when a user asks you to inspect or manage their Notehub resources.
The CLI handles authentication and sends HTTP requests on the user's behalf.
You can use the service's public API without implementing an authentication flow,
handling access tokens yourself, or installing a Notehub SDK. Your access is
limited to the resources and operations the authenticated user is allowed to use.

The working sequence is: establish authentication, read the deployed API spec,
identify the user's target resources, perform the requested operations, and
inspect the responses. Use the spec to discover capabilities and exact schemas
instead of guessing endpoints or relying on remembered API definitions.

This document is self-contained and available as `notehub --help-agent`.
`notehub --help` lists CLI modes and options with examples; `notehub api --help`
and `notehub login --help` describe those modes specifically.

## 2. Authentication

There are two authentication workflows for this skill. Prefer the browser-based
OAuth2 workflow, which lets the user grant access directly to the CLI. Use a
Personal Access Token (PAT) supplied through `NOTEHUB_TOKEN` when browser approval
is unsuitable, such as in unattended automation.

### Check the current authentication state

```sh
notehub auth status --json
```

The result is a single object describing the active connection. Inspect its
`signed_in` field directly. A successful check also identifies the `user` and
authentication `method`; an unsuccessful one provides a `reason` and sometimes
an `action`.

**`auth status --json` can exit with code 0 even when `signed_in` is false.** Read
the report. This command checks credentials with the service; a network failure
does not establish that a login is invalid. If Notehub accepts the
credentials, reuse them.

An expired OAuth access token can still be renewed using a saved refresh token.
Every authenticated operation, including this status check, attempts renewal
when needed. The login command requests new browser approval if renewal is
unavailable or the service rejects the saved session.

### Preferred: orchestrate OAuth2 approval with the user

Start the agent login flow with a recognizable name for your agent:

```sh
notehub login --name "My Agent" --json
```

`--json` makes this a prompts-disabled flow. The CLI supplies an authorization
link and polls Notehub while the user completes approval in their own browser.
It uses outbound HTTPS and does not require a local callback server. You do not
need the user's password, an OAuth client secret, or the resulting access token.

If the CLI reports that polling sign-in is unavailable, ask the user to run
`notehub login` on the machine running the CLI, with a browser on that same
machine. Interactive login automatically uses the compatible local callback.
Then check authentication and reuse the saved session. If that machine cannot
support browser approval, use the PAT workflow below. The agent's `--json` flow
never opens a browser or silently switches to a local callback.

1. Start the command in an execution session that lets you read output while the
   process is still running. Keep that process alive while talking to the user;
   waiting for the command to finish before reading its output would prevent you
   from delivering the sign-in link in time.
2. Read stdout as newline-delimited JSON: one object per line. When an object
   contains `url`, immediately give that exact URL to the user and ask them to
   open it, sign in if necessary, and approve the named CLI session. Do not
   reconstruct the URL or ask the user to return browser credentials or tokens.
3. Continue reading the same running process. Objects with
   `status: "waiting for authorization"` report progress. The optional
   `expires_in_seconds` is the time remaining for browser approval, not the
   lifetime of the eventual access token.
4. Wait for the terminal object containing the boolean `success`. Proceed only
   when it is `true` and the process exits successfully. When it is `false`, use
   `status` to explain the failure. If approval expires, start a new login to
   obtain a new URL. If the user declines or cancels, respect that decision.

A typical stdout stream has this shape; the URL and countdown are illustrative:

```json
{"status":"please instruct the user to open their browser to this URL to complete sign in","url":"https://...","expires_in_seconds":600}
{"status":"waiting for authorization","expires_in_seconds":590}
{"status":"signed in successfully","success":true}
```

The first two objects are progress, not failures: they have no `success` member.
Use the fields to drive the workflow; `status` is explanatory text. A reusable
login may produce only a terminal success object saying that you are already
signed in. The CLI verifies that session with Notehub before reusing it.

The default request allows up to ten minutes for approval, subject to the hub's
limit. `--expires-in SECONDS` requests a shorter approval window. Allow sufficient
execution time for the reported window, and reuse the running session instead of
launching duplicate login commands. Interrupt it if the user cancels.

After approval, the CLI stores the access token, its expiration, and any refresh
token the service issues. Its shared authentication layer supplies the header
for every authenticated request and silently refreshes an expired access token
or one within a minute of expiration. This covers API calls, built-in commands,
and authentication checks. After HTTP 401, it attempts renewal and retries once
if the request body can be replayed. Rotated tokens are saved for subsequent
calls; concurrent CLI processes coordinate renewal. Refresh emits no output and
needs no browser interaction. Login progress and success output contain no tokens.

Refresh requires Notehub's CLI OAuth registration to permit `offline_access`
and the refresh-token grant. Login remains usable when refresh is unavailable;
that session needs browser approval again when its access token expires. Once
refresh is enabled, create a new login to obtain a refresh token. Regular use
can keep renewing a session, subject to Notehub's expiration and inactivity
policies. Deleting its OAuth entry in Notehub's API Access ends its access;
refresh cannot restore the deleted sign-in.

The current service configuration gives each refresh token a 72-hour lifetime
starting at issuance or the last successful renewal. Ordinary API calls extend
that window only when they trigger renewal. If the window expires while the
CLI is idle, browser approval is needed again.

If the refresh grant has expired or been revoked, the CLI reports that new
authorization is needed. Repeat the login flow to obtain new user approval.
An API operation never starts browser approval automatically. To replace a
still-valid older login that has no refresh token, use `notehub logout` followed
by a new login once refresh is enabled.

### Alternative: a Personal Access Token in NOTEHUB_TOKEN

Have the user create a PAT in their Notehub account's **API Access** screen,
choosing a recognizable name and suitable expiration. Ask them to supply it
through the execution environment's secret configuration or a secret manager,
under the exact environment-variable name `NOTEHUB_TOKEN`. The
[Notehub authentication reference](https://dev.blues.io/api-reference/notehub-api/#creating-a-personal-access-token)
describes PAT creation and revocation.

The variable must be inherited by the actual `notehub` processes you launch.
Setting it in an unrelated terminal does not update an already-running agent's
environment. Do not ask the user to paste the token into the conversation or put
the literal token in commands, source files, or logs.

Once the secret has been provisioned, verify access without printing it:

```sh
notehub auth status --json
```

For this workflow, the status object should have `signed_in: true`,
`method: "pat"`, and `source: "NOTEHUB_TOKEN"`. No `login` command is needed.
Run API commands normally; the CLI reads the variable and supplies authentication.

A nonempty `NOTEHUB_TOKEN` overrides saved credentials. The
CLI uses it even if an OAuth login is already stored. If it is rejected or has
expired, have the user replace or remove it through the same secret mechanism.
Running the browser login cannot override it. To return to saved OAuth credentials,
remove it from the environment inherited by subsequent CLI invocations. Logging
out is refused while the environment token is present, and removing the variable
does not revoke the PAT at Notehub; the user manages that in API Access.

### Credential handling

Let the CLI own credential handling. Do not inspect its credential file, print
`NOTEHUB_TOKEN`, invoke `notehub auth token` or `notehub --token`, or supply an
`Authorization` header yourself. Normal login, status, and API operations do not
require the agent to see the token.

Storage behavior in this build: credentials are persisted in a local JSON
configuration file. There is no OS keychain integration or encryption of that
file. Treat it as private credential storage and reauthenticate when renewal is
no longer possible. The absence of tokens from normal
agent output is not a guarantee that another process with filesystem access
cannot read them.

Preserve reusable logins between tasks unless the user wants the session ended.
Use `notehub logout` to remove the saved login and attempt to revoke its OAuth
session.

## 3. Calling the Notehub API through this tool

`notehub api` is a curl-like authenticated HTTP client for Blues Notehub:

```text
notehub api [options] [METHOD] <path>
```

Supply an endpoint path such as `/v1/projects`, including any query string.
The CLI supplies the HTTPS API host and authentication. Full URLs are rejected.
Supported methods are `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, and `OPTIONS`.
Options may appear before or after the method and path. Examples below use
`app:PROJECT_UID` and `dev:DEVICE_UID` as stand-ins: replace them with identifiers
discovered from the user's actual resources.

### Read resources and supply path parameters

```sh
notehub api GET /v1/projects
notehub api GET '/v1/projects/{project}/devices' --project app:PROJECT_UID
notehub api GET '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID
```

You may write actual UIDs directly into the path instead. Quote paths containing
braces or query strings so the shell passes them unchanged.

| Path placeholder | CLI value |
| --- | --- |
| `{project}`, `{projectUID}`, `{projectOrProductUID}` | `--project`; falls back to `--product` when no project UID is supplied. |
| `{product}`, `{productUID}` | `--product` |
| `{device}`, `{deviceUID}` | `--device` |

These options substitute placeholders; they do not automatically scope a path
that contains none. Other path parameters, such as `{fleetUID}` or `{eventUID}`,
must be replaced with actual values before invoking the CLI. Follow the endpoint's
accepted identifiers and URL-encode values where needed.

### Choose the method explicitly when adding parameters

With no method, the default is `GET` unless `--field`, `--raw-field`, or `--input`
supplies data, which changes the default to `POST`. For predictable operations,
name the method explicitly, especially when adding query parameters to a read:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --raw-field cursor=0
```

For `GET` and `HEAD`, field options become URL-encoded query parameters. For
other methods, they form a JSON object in the request body.

| Option | Interpretation |
| --- | --- |
| `--field key=value` | Converts `true`, `false`, `null`, and JSON numbers to those JSON types; other values remain strings. |
| `--field key=@file` | Reads the file's contents as a string value for that key. |
| `--field key=@-` | Reads stdin as a string value for that key. |
| `--raw-field key=value` | Always uses a literal string, including values resembling numbers, booleans, or `@file`. |

Repeat field options for distinct keys. They build a flat object: bracketed keys
do not create nested objects, JSON-looking strings are not parsed as objects or
arrays, and repeating a key does not build an array. For query parameters that
require repeated keys, write the encoded query string directly in the quoted
path, following the spec's parameter serialization.

For example, when the user requests a new fleet, a simple JSON body can be built
from a field:

```sh
notehub api POST '/v1/projects/{project}/fleets' \
  --project app:PROJECT_UID --raw-field label='Field Units'
```

### Send a complete request body

Use `--input FILE` to send a file's bytes as the entire body, or `--input -` to
read the body from stdin. This is the appropriate mechanism for nested JSON,
arrays, or an exact payload from the API spec. For the same fleet-creation example:

```sh
notehub api POST '/v1/projects/{project}/fleets' \
  --project app:PROJECT_UID --input - <<'JSON'
{"label":"Field Units"}
JSON
```

`--input` cannot be combined with `--field` or `--raw-field`. If a body request
also needs query parameters, put those parameters in its path. The CLI defaults
the body's `Content-Type` to `application/json`; override it when the endpoint
requires another media type:

```text
--header 'Content-Type: application/octet-stream'
```

`--header 'Name: value'` adds a request header and can be repeated. Authentication
is supplied by the CLI. Use endpoint-specific request schemas from the spec;
the CLI does not validate your body against them or infer update semantics.

### Read responses and handle failures

By default, stdout contains the response body, with a trailing newline added
when needed. Diagnostics go to stderr. Keep the streams separate when parsing
the response. JSON is returned as JSON without requiring a `--json` option;
`--json` is not an option of the `api` mode.

| Option | Output behavior |
| --- | --- |
| `--pretty` | Indents JSON after buffering the whole response. |
| `--include` | Prepends the HTTP status line and response headers; stdout is then not a bare JSON document. |
| `--verbose` | Writes request details, including the body, and response status to stderr; omits the outgoing Authorization header. |

Ordinary successful responses stream as they arrive. Avoid `--pretty` for a
long-lived stream: it waits for the entire response. A successful request can
have an empty body, for example an HTTP 204 response.

Check the process exit status as well as its output. HTTP responses with status
400 or greater retain their response body on stdout, produce a diagnostic on
stderr, and exit with code 1. Request-building and transport failures also fail;
command-line parsing can use another nonzero code. Treat every nonzero exit as
a failure rather than assuming that JSON output means success.

For an authentication failure, check the authentication report.
For an authorization failure, check the user's access to the resource.
For an invalid request, inspect the response and the endpoint schema. Respect
rate-limit responses and any retry guidance from the service. After a timeout or
connection failure during a write, inspect the resulting resource state before
retrying an operation that could create duplicates or apply a change twice.

### Read changes with modification-time cursors

Many Notehub APIs support a modification-time cursor. Check the endpoint's
description in the deployed OpenAPI spec; the query parameter is `cursor`.
Supplying it returns records in ascending order of modification time. Start
with a Unix epoch timestamp in **microseconds, UTC**, or use `0` when the task
requires reading from the beginning:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --raw-field cursor=0
```

Process the returned records and read `next_cursor` and `has_more` from this
endpoint's response. Use the exact returned cursor in the next request, keeping
the same resource and filters:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --raw-field cursor="$NEXT_CURSOR"
```

Here `NEXT_CURSOR` is the string returned by the preceding successful request.
Treat returned cursors as opaque strings: preserve the entire value, including
any suffix, without converting it to a number, rounding it, or incrementing it.
The suffix distinguishes records with the same modification timestamp.

Continue while `has_more` is true. When it is false, save the returned cursor
after successfully processing the records; reuse it later to read subsequent
changes. An empty result can still return a useful cursor. Keep saved cursors
associated with their endpoint, resource, and filters. Updated records may
reappear; apply them by resource identifier, and handle `deleted: true` where
the endpoint supports deletion records.

The equivalent response headers are `X-Next-Cursor` and `X-Has-More`. For an
endpoint that supplies its continuation metadata in headers, add `--include`
and parse the headers separately from the body. Follow the endpoint's documented
cursor semantics and supported filters. Choose a starting time appropriate to
the user's task rather than reading an entire history unnecessarily.

## 4. Discovering the API's capabilities

Fetch the full public OpenAPI specification of the currently deployed Notehub service:

```sh
notehub api --spec
```

For a substantial task, save it locally so you can search and parse it repeatedly:

```sh
notehub api --spec > notehub-openapi.yaml
```

This fetch is public and does not need a login. It retrieves the published YAML
from `https://api.notefile.net/openapi.yaml`, falling back to
`https://notehub.io/openapi.yaml`; it is not a schema bundled into the CLI.
Do not supply an endpoint path with `--spec`.

Treat the returned specification as the API contract for Notehub. Find
operations by path, method, tags, `operationId`, and description. For each operation,
read its path and query parameters, required request fields, request media types,
response schemas, cursor behavior, and documented access requirements. Resolve `$ref`
entries into `components` rather than reading only an operation's short description.

Discover the operation that matches the user's intent, fill its parameters with
actual resource identifiers, then call it through `notehub api`. The spec is the
capability catalog; this skill deliberately does not duplicate an endpoint list
that could become stale. Publication of an operation does not grant the current
user permission to execute it. Refresh the spec when the service's behavior
differs from a previously saved copy.

## 5. Understanding the user's Notehub resources

Use the following model to navigate the user's account:

| Resource | Meaning for your work |
| --- | --- |
| Projects | Containers for the user's applications and resources. Start with `GET /v1/projects` to discover accessible projects and their UIDs. A user may have access to several projects. |
| Devices | The connected Notecard devices within a project, identified by device UIDs. Use actual returned identifiers when inspecting or configuring a device. |
| Fleets | Named groups of devices within a project. A device can belong to multiple fleets; use fleet membership to understand the scope of group operations. |
| Environment variables | Configuration key/value pairs set at project, fleet, or device level and synchronized to devices. These are Notehub settings, separate from shell variables such as `NOTEHUB_TOKEN`. |
| Events | Records of data and activity from devices. Query the relevant project's events with filters appropriate to the device, time period, and question. |

The [Project API reference](https://dev.blues.io/api-reference/notehub-api/project-api/)
describes project and fleet relationships. Configuration values can override one
another across levels; inspect effective values and their origin before changing
settings, following the
[environment-variable hierarchy](https://dev.blues.io/guides-and-tutorials/notecard-guides/understanding-environment-variables/).
A successful cloud-side update does not by itself establish that an offline
device has received it.

Identify the intended project first, then narrow to the relevant devices or
fleets. If the user's target is ambiguous, resolve that ambiguity before changing
resources. Use existing authorization to carry out the requested work, inspect
the resulting state, and report what succeeded and what remains unresolved.
The deployed OpenAPI spec supplies the operations available for each resource.

For more guidance on how the user might want to interact with their work on
Notehub, run:

```sh
notehub --agent-chat
```

This fetches the public entry guide at `https://chat.notehub.io` and writes its
contents to stdout. It requires no login. Consult it and the documentation it
links to for additional context about Notehub conversations and workflows.
