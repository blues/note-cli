---
name: notehub-api
description: Use the Notehub CLI to authenticate with a user and call Blues Notehub public APIs. For agents with command execution capability and no Notehub MCP connection.
---

# Notehub API skill for agents

## 1. Introduction

`notehub` gives agents without MCP access the public APIs of Blues Notehub,
the cloud service for managing Notecard devices and their data. Use the default
Blues Notehub service. Your workflow is: check authentication, discover the API,
identify the user's resources, perform their task, and report the result.

## 2. Authentication

Check whether you can already work:

```sh
notehub auth status --json
```

Read the single result object's `signed_in` boolean; exit code 0 alone does not
mean signed in. If true, continue with the task. Otherwise inspect `reason` and
`action`. A network failure calls for retrying the check, not a new login.

### Preferred: guide the user through sign-in

Use a recognizable name for your agent:

```sh
notehub login --name "My Agent" --json
```

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

```json
{"status":"signed in successfully","success":true}
{"status":"sign-in was cancelled","success":false}
```

Progress messages have no `success` field. Login can also finish immediately
with `success: true` when an existing sign-in is reusable. A process failure or
exit without a final success message does not establish authentication.

If the command reports that polling sign-in is unavailable, ask the user to run
`notehub login` on the machine running the CLI, using a browser on that machine;
then check status again. If that is unsuitable, use the token method below.

The CLI saves and renews credentials for you. Reuse the sign-in across tasks;
repeat this workflow only when the CLI says new authorization is needed.
Use `notehub logout` when the user wants to end the session.

### Alternative: a Personal Access Token

Have the user create a token in Notehub's **API Access** screen and provide it
as `NOTEHUB_TOKEN` through their environment's secret settings. It must be
available to the `notehub` processes you run. Check `notehub auth status --json`
again; expect `signed_in: true`, `method: "pat"`, and `source: "NOTEHUB_TOKEN"`.
Then call the API normally; no login command is required.

`NOTEHUB_TOKEN` takes precedence over a saved sign-in. If rejected, ask the user
to replace it or remove it before using the preferred login workflow.

Let the CLI handle secrets. Never ask the user to paste credentials into chat,
print tokens, inspect credential files, use `auth token` or `--token`, or supply
an `Authorization` header yourself.

## 3. Calling the Notehub API

Use the CLI as an authenticated HTTP client:

```text
notehub api [options] METHOD <path>
```

Supply an endpoint path, including any query string, rather than a full URL.
Always name the method explicitly: adding fields otherwise changes the default
from GET to POST. Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS.
Quote paths containing braces or query strings.

```sh
notehub api GET /v1/projects
notehub api GET '/v1/projects/{project}/devices/{device}/environment_variables' \
  --project app:PROJECT_UID --device dev:DEVICE_UID
```

Replace example UIDs with identifiers returned by Notehub. `--project` fills
`{project}`, `{projectUID}`, or `{projectOrProductUID}`; `--device` fills
`{device}` or `{deviceUID}`. You can also put actual identifiers directly in
the path. Fill other placeholders, such as a fleet UID, yourself.

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
content types. Follow the spec for the exact request schema.

### Responses and failures

Stdout contains the response body; diagnostics go to stderr. Keep them separate
and check the exit status. JSON responses need no `--json` option. HTTP errors
retain their response body and exit nonzero. Successful responses may be empty.

`--include` adds the HTTP status and headers before the body. `--pretty` formats
JSON but buffers the complete response, so avoid it for streams. `--verbose`
writes request details and response status to stderr.

On authentication failure, check status and follow section 2. On permission or
request errors, inspect the response and spec. Respect rate-limit retry guidance.
After an uncertain write result, inspect the resource before repeating the write.

### Read changes with cursors

For endpoints documenting a modification-time `cursor`, results arrive oldest
modification first. Start at a UTC Unix timestamp in **microseconds**, or `0`
when you need to read from the beginning:

```sh
notehub api GET '/v1/projects/{project}/devices' \
  --project app:PROJECT_UID --raw-field cursor=0
```

Process the records, then send the exact returned `next_cursor` as the next
`cursor`, preserving it as an opaque string. Continue while `has_more` is true.
Save the final cursor after processing; reuse it later to read new changes,
keeping the same endpoint, resource, and filters. Empty results may still carry
a useful cursor. Apply updates by resource identifier and handle `deleted: true`
where supported. If the endpoint provides continuation in headers, use
`--include` to read `X-Next-Cursor` and `X-Has-More` separately from the body.

## 4. Discovering API capabilities

Fetch the deployed public OpenAPI specification:

```sh
notehub api --spec > notehub-openapi.yaml
```

This needs no login. Search the saved spec for the user's task instead of
guessing endpoints. Read the operation's path, method, parameters, request and
response schemas, cursor behavior, and access requirements; resolve `$ref`
definitions. Use it as the full capability catalog and refresh it when needed.

## 5. Understanding the user's resources

| Resource | How to use it |
| --- | --- |
| Projects | Start with `GET /v1/projects` and identify the user's intended project. |
| Devices | Notecard devices within a project; use their returned device UIDs. |
| Fleets | Groups of devices; a device can belong to multiple fleets. |
| Environment variables | Device configuration set at project, fleet, or device level. Inspect effective values and overrides before changing them. |
| Events | Data and activity generated by devices; filter by the relevant devices, time period, and task. |

Resolve ambiguous targets before changing resources. A cloud-side configuration
update does not establish that an offline device has received it. The OpenAPI
spec describes the available operations for all these resources.

For guidance on how the user might want to interact with their Notehub work, run:

```sh
notehub --agent-chat
```

It prints the guide from `https://chat.notehub.io`; no login is needed.
