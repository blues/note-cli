---
name: notehub-api
description: Guide a user through Notehub authentication, discover the public API through its OpenAPI specification, and use the Notehub CLI as an authenticated HTTP request proxy.
---

# Notehub API skill for agents

## 1. What the CLI provides

`notehub` lets an agent with command execution work with Blues Notehub, the
cloud service for managing Notecard devices and their data. Use `notehub api`
as a curl-like **authenticated request proxy**: supply an HTTP method, API
path, parameters, and optional body; the CLI adds the user's authentication,
sends the request to Notehub, and returns the response. The CLI manages saved
credentials and OAuth renewal when available.

Your workflow is: lead the user through authentication, fetch the deployed
OpenAPI specification, identify the relevant operations and resources, make
requests, and report the results. The default API base is
`https://api.notefile.net`. Use the selected hub consistently for authentication,
discovery, and requests; select another hub only when the user's task calls for it.

## 2. Lead the user through authentication

Check whether you can already work:

```sh
notehub auth status --json
```

Read the single result object's `signed_in` boolean; exit code 0 alone does not
mean signed in. If true, check the reported `hub` and `user`, then continue with
the task. Otherwise inspect `reason` and `action`. A network or temporary service
failure calls for retrying the check, not a new login. When sign-in is needed,
the suggested `action` is `notehub login`; as an agent, use the `--json` workflow
below.

### Preferred: guide the user through sign-in

Tell the user that you will provide a link for them to approve Notehub access.
Use a recognizable name for your agent:

```sh
notehub login --name "My Agent" --json
```

This command opens no browser and requires no terminal input. The user opens
the approval link in their own browser, which can be on another machine.

1. **Start a persistent execution session and read stdout while it runs.**
   Output is one JSON object per line. Keep the process alive while interacting
   with the user; do not wait for it to exit before reading its output.
2. **When a message contains `url`, immediately give that exact link to the
   user.** In your own friendly words, ask them to open it in their browser,
   sign in, and approve access for your named agent. For example:
   “Please open this link in your browser to complete sign-in for My Agent.
   I'll keep watching here and continue once it's approved.”
   The user does not need to copy anything back to you.
3. **Keep monitoring the same process's stdout.** Messages with
   `status: "waiting for authorization"` are progress, not failures.
   `expires_in_seconds` is the remaining approval time. Reassure the user if
   needed; you need not relay every countdown message. Allow the reported
   window, normally up to ten minutes, and do not launch duplicate logins.
4. **Wait for a final message containing `success`.** If it is `true` and the
   command exits successfully, tell the user sign-in succeeded and resume
   their task. If it is `false`, explain the `status` in plain language.
   Respect cancellation or refusal; do not immediately ask for approval again.
   If the link expires and the user still wants to proceed, start a new login
   and give them its new URL. If the user asks you to stop, interrupt the process.

Example final messages (alternative outcomes):

```jsonl
{"status":"signed in successfully","success":true}
{"status":"sign-in was cancelled","success":false}
```

Progress messages have no `success` field. Login can also finish immediately
with `success: true` when an existing sign-in is reusable. A process failure or
exit without a final success message does not establish authentication.

If the command reports that polling sign-in is unavailable, ask the user to run
`notehub login` on the machine running the CLI, using a browser on that machine;
then check status again. If that is unsuitable, use the token method below.

The CLI saves credentials and renews OAuth access tokens when a refresh token
is available. Run subsequent commands in the same user environment so they
can reuse that login. Repeat sign-in only when new authorization is needed;
use `notehub logout` when the user wants to end the session.

### Alternative: a Personal Access Token

Have the user create a token in Notehub's **API Access** screen and provide it
as `NOTEHUB_TOKEN` through their environment's secret settings. It must be
available to the `notehub` processes you run. Check `notehub auth status --json`
again; expect `signed_in: true`, `method: "pat"`, and `source: "NOTEHUB_TOKEN"`.
Then call the API normally; no login command is required. Personal access
tokens are not refreshed automatically.

`NOTEHUB_TOKEN` takes precedence over a saved sign-in. If rejected, ask the user
to replace it or remove it before using the preferred login workflow.

Let the CLI handle secrets. Never ask the user to paste credentials into chat,
print tokens, inspect credential files, use `auth token` or `--token`, or supply
an `Authorization` header yourself.

## 3. Discover the entire public API

Fetch the deployed OpenAPI YAML from
[https://api.notefile.net/openapi.yaml](https://api.notefile.net/openapi.yaml):

```sh
curl -fsSL 'https://api.notefile.net/openapi.yaml' -o notehub-openapi.yaml
```

The CLI can fetch the selected hub's specification too:

```sh
notehub api --spec > notehub-openapi.yaml
```

Both fetches are public and need no login. For the default hub they retrieve
the same specification. If the task targets another hub, fetch that hub's spec
with `--spec --hub HUB` and use the same `--hub HUB` for subsequent commands.
The spec must describe the service you are actually calling.

**Use the live YAML as the complete catalog of public API capabilities.**
The CLI's named modes are convenient entry points; `notehub api` can call the
other operations described in the spec too, without a dedicated CLI command.
Do not infer the available API from the few examples in this guide.

First survey `paths` and their HTTP methods, operation IDs, descriptions, and
tags to learn the available resources and actions. The catalog covers projects,
products, members, devices, fleets, events, notes and notefiles, environment
variables, routes and webhooks, firmware and DFU, skills, monitors and alerts,
jobs and reports, organizations, and usage. Use the fetched spec to determine
the actual operations available for each resource.

Then locate the operations needed for the user's task. A text search can help:

```sh
rg -n 'operationId:|environment_variables|firmware|skills' notehub-openapi.yaml
```

Read the full operation, using a YAML parser when helpful:

1. Find the exact path and HTTP method. Read parameters declared on both the
   path and the operation, including required values, defaults, and limits.
2. Resolve `$ref` entries under `components/parameters`, `requestBodies`,
   `schemas`, `responses`, and `headers`. Follow nested references and schema
   alternatives to understand the complete request and response shapes.
3. Check the request's media type, required fields, types, and enumerated values.
   Check successful and error status codes, response bodies, and headers.
4. Read pagination and modification-cursor behavior, security requirements,
   and any permission or resource annotations. Keep using the CLI's login
   rather than implementing a separate credential exchange from the spec.

Cache the YAML for the task and refresh it when changing hubs or when an
operation appears to have changed. Discover paths and schemas from this file
instead of inventing endpoint names or relying on deprecated APIs.

## 4. Use the CLI as an authenticated request proxy

For example, an authenticated GET to
`https://api.notefile.net/v1/projects` becomes:

```sh
notehub api GET /v1/projects
```

The general form is:

```text
notehub api [options] METHOD <path>
```

Supply an endpoint path, including any query string, rather than a full URL.
Always name the method explicitly: adding fields otherwise changes the default
from GET to POST. Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS.
Quote paths containing braces or query strings.

| HTTP request element | CLI form |
| --- | --- |
| Method and URL path | `notehub api METHOD /path` |
| Query parameter | With GET or HEAD, `--raw-field key=value` or `--field key=value` |
| Header | `--header 'Name: value'` (repeatable) |
| Complete request body | `--input request.json`, or `--input -` for stdin |
| Response status and headers | `--include` |
| Save the response body | Shell redirection, such as `> response.json` |

The CLI supplies authentication for each request using the selected hub's saved
login or `NOTEHUB_TOKEN`. You do not need to extract a token, add an authorization
header, or invoke curl for authenticated Notehub operations.

### Identify resources and fill paths

Start with `GET /v1/projects` and identify the user's intended project from the
response. Inspect that project's devices, fleets, or other resources as needed.

Replace example UIDs with identifiers returned by Notehub. `--project` fills
`{project}`, `{projectUID}`, or `{projectOrProductUID}`; `--product` fills
`{product}` or `{productUID}` and can also supply the project when `--project`
is absent. `--device` fills `{device}` or `{deviceUID}`. These options substitute
path values; they do not add a project or device filter to an unrelated path.
You can also put identifiers directly in the path. Fill other placeholders,
such as a fleet UID, yourself, encoding path values as the API requires.

For example, read a device's environment variables:

```sh
notehub api GET '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID
```

### Parameters and bodies

- `--field key=value` supplies a typed value: numbers, booleans, and null are
  converted; other values are strings. `@file` reads a file as a string and
  `@-` reads stdin.
- `--raw-field key=value` always supplies a literal string. Use this for cursors.
- With GET or HEAD, fields become URL-encoded query parameters. With other
  methods, they form a JSON body. Repeat options for distinct keys; fields
  do not build nested objects or arrays. For repeated query keys, write the
  encoded query string in the path as specified by the API.
- `--input FILE` sends a complete body; `--input -` reads it from stdin.
  Use this for nested JSON or arrays. Do not combine it with field options.
  Put any additional query parameters in the path.

For example, when the user requests a new fleet:

```sh
notehub api POST '/v1/projects/{project}/fleets' \
  --project app:PROJECT_UID --input - <<'JSON'
{"label":"Field Units"}
JSON
```

Bodies default to `Content-Type: application/json`. Use repeatable
`--header 'Name: value'` options when the endpoint requires other headers or
content types. Follow the spec for the exact request schema. The example above
creates a fleet; run it when that is the user's requested action and use the
returned identifier for later operations.

To send a prepared JSON file, for example when setting device variables:

```sh
notehub api PUT '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID --input vars.json
```

Construct `vars.json` according to that operation's request schema. Check whether
a write merges with or replaces existing values before choosing its payload.

### Responses and failures

Stdout contains the response body; diagnostics go to stderr. Keep them separate
and check the exit status. JSON responses need no `--json` option. HTTP errors
retain their response body and exit nonzero. Successful responses may be empty.

For example, save a response for inspection or processing:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --field pageSize=10 > devices.json
```

Check the command's exit status before treating `devices.json` as a successful
response; it can contain an API error body. Parse the body using the operation's
response schema rather than assuming every endpoint returns the same shape.

`--include` adds the HTTP status and headers before the body. `--pretty` formats
JSON but buffers the complete response, so avoid it for streams. `--verbose`
writes request details and response status to stderr. When using `--include`,
separate the headers from the body before parsing JSON. For binary firmware
downloads, use `notehub firmware get`; `api` can append a trailing newline to
its output.

On a 401, check authentication status and follow section 2 if needed. A 403 calls
for checking permissions; a 400 or 404 calls for checking the request, resource
identifiers, and spec. Respect rate-limit retry guidance, including `Retry-After`
when provided. After an uncertain write result, inspect the resource before
repeating the write.

## 5. Read large collections with modification cursors

For endpoints documenting a modification-time `cursor`, results arrive oldest
modification first. Start at a UTC Unix timestamp in **microseconds**, or `0`
when you need to read from the beginning. Use the collection endpoint documented
in the deployed spec, such as `/v1/projects/{project}/events`.

To read all available batches automatically:

```sh
notehub api GET '/v1/projects/{project}/events' \
  --project app:PROJECT_UID --paginate --field pageSize=1000
```

`--paginate` starts at `cursor=0` unless you supply `--raw-field cursor=VALUE`.
It follows `next_cursor`/`has_more` or the `X-Next-Cursor`/`X-Has-More` headers
until no more results remain. Each response is written separately; the output
is a sequence of responses, not one combined JSON document. Add `--include`
when you need to retain cursor headers. Missing continuation metadata or a
repeated cursor with more results causes a nonzero exit; do not treat partial
output as a complete read. Do not combine automatic pagination with a page
number beyond 1 or conflicting sort options.

For processing and checkpointing one batch at a time, omit `--paginate`:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --raw-field cursor=0
```

Process the records, then send the exact returned `next_cursor` as the next
`cursor`, preserving it as an opaque string. A returned cursor may include a
tie breaker; never truncate it to a timestamp or convert it to a floating-point
number. Continue while `has_more` is true.
Save the final cursor after processing; reuse it later to read new changes,
keeping the same endpoint, resource, and filters. Empty results may still carry
a useful cursor. Apply updates by resource identifier and handle `deleted: true`
where supported. If the endpoint provides continuation in headers, use
`--include` to read `X-Next-Cursor` and `X-Has-More` separately from the body.

## 6. Use the results to complete the user's task

| Resource | How to use it |
| --- | --- |
| Projects | Start with `GET /v1/projects` and identify the user's intended project. |
| Devices | Notecard devices within a project; use their returned device UIDs. |
| Fleets | Groups of devices; a device can belong to multiple fleets. |
| Environment variables | Device configuration set at project, fleet, or device level. Inspect effective values and overrides before changing them. |
| Events | Data and activity generated by devices; filter by the relevant devices, time period, and task. |

Resolve ambiguous targets before changing resources. A cloud-side configuration
update does not establish that an offline device has received it. The OpenAPI
spec describes the available operations for all these resources. Read the result
of each write and, when the task requires it, verify the resulting state with a
follow-up read. Report what changed and distinguish queued device work from work
the device has actually completed.

If the project has skills, read them for context about the product and its data:

```sh
notehub skills --project app:PROJECT_UID
```

For guidance on how the user might want to interact with their Notehub work, run:

```sh
notehub --agent-chat
```

It prints the guide from `https://chat.notehub.io`; no login is needed.
