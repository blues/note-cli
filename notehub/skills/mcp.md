---
name: mcp
description: install the Notehub MCP server, which is the interface to Notehub IQ
---

# Installing the Notehub MCP

The Notehub MCP is a hosted Model Context Protocol server, and it is the interface to
Notehub IQ. It turns a question into SQL against your project's event data, runs it, and
returns the result. It is read-only: no action taken through it can add, update, or
delete event data in Notehub.

Install it when the work involves analyzing event data rather than changing the
configuration of a project, a fleet, or a device. For that, use the classic HTTP API or
the `notehub` CLI instead.

## The server

```
https://mcp.notehub.io/mcp
```

It is a remote MCP server, so there is nothing to download and nothing to run locally.
Authentication happens in the browser the first time the server is reached, and later
requests reuse that authorization.

## Installing it in a client

Add the server URL above as a remote MCP server, following your client's own procedure:

| Client | Where the procedure is documented |
|---|---|
| Claude Desktop and Claude web | https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp - or search the Connectors directory for "Notehub" |
| Claude Code | https://code.claude.com/docs/en/mcp#installing-mcp-servers |
| GitHub Copilot in VS Code | https://code.visualstudio.com/docs/agent-customization/mcp-servers |
| Codex | https://learn.chatgpt.com/docs/extend/mcp |
| Cursor | https://cursor.com/docs/mcp#installing-mcp-servers |

## Onboarding a project, once

Before a project can be queried, it must be onboarded, and onboarding a project requires
more permission than querying it does. To onboard, you must be an **Owner** of the
project and hold an organizational role of **Project Creator**, **Billing Manager**, or
**Admin**. That requirement applies to onboarding only, not to asking questions
afterward.

The flow is:

1. **Onboard the project.** `list_onboardable_projects` shows what you may onboard. A
   repository, which is the container that holds the project's queryable data, is created
   for it automatically.
2. **Create a dataset for each Notefile you care about.** A dataset mirrors a Notefile's
   JSON events as a table whose columns follow the event structure. Datasets are created
   only for the Notefiles you ask for, so a Notefile without a dataset is invisible to
   every query.
3. **Include the System Notefiles you need.** Questions about device health, sessions, or
   connectivity are answered from `_health.qo` and `_session.qo`, so those need datasets
   of their own.
4. **Wait for the datasets to become ready.** This can take minutes to hours on first
   creation. Afterward, new events sync automatically and immediately.
5. **Ask.**

## The tools it exposes

**Discovery**

- `list_onboardable_projects` - projects that may be onboarded
- `list_project_fleets` - the fleets within a project
- `list_project_notefile_schemas` - the Notefiles available to become datasets
- `list_repositories` - the repositories that already exist

**Repositories and datasets**

- `create_storage_repository`, `delete_storage_repository`
- `infer_dataset` - derive a dataset's shape from a Notefile's events
- `create_repository_dataset`, `replace_repository_dataset`, `delete_repository_dataset`

**Query and analytics**

- `describe_schema` - the columns available to query
- `get_table_sample` - a sample of rows, for orienting before writing a query
- `query_repository` - read-only SQL, limited to `SELECT`, `WITH`, `EXPLAIN`, `SHOW`,
  and `DESCRIBE`

## Working with it

Inspect the schema before writing a query. `list_project_notefile_schemas`,
`describe_schema`, and `get_table_sample` exist so that a query can be written against
the columns that are actually there rather than the ones an event payload appears to
imply.

The questions worth bringing here are the ones that span events, devices, and time:

- Did any shipment breach 8°C for more than 15 continuous minutes last week?
- Which devices are actually down, as opposed to merely syncing on a long interval?
- Given lat/lon, which devices left the warehouse, and how long did each one sit at each
  stop?
- Which devices are drifting toward a failure, and which are driving data cost?

The MCP is most useful as one tool among several. Pairing it with the tools that can act
on what it finds - the `notehub` CLI, the classic HTTP API, a ticketing system, a chat
workspace, or a scheduled routine - is what turns an answer into an outcome.
