---
name: notehub
description: reach Notehub across its full surface area - the classic HTTP API and Notehub IQ
---

# Notehub

Notehub is the cloud service that Blues Notecards synchronize with. It holds projects,
fleets, devices, Notefiles, events, routes, environment variables, firmware images, and
jobs.

This skill, and the skills listed at the end of it, teach you how to reach Notehub
across its entire surface area: which interface to use for a given task, the vocabulary
each one expects, and how to confirm that what you did actually took effect. Load a
skill before performing an unfamiliar Notehub operation rather than guessing at an
endpoint or a schema.

## The two API surface areas

Notehub is reached through two quite different APIs. Choosing the wrong one is the most
common way to make a hard job out of an easy one.

### The classic HTTP API

The operational surface, at `https://api.notefile.net`. It is transactional: it reads
and changes the state of projects, fleets, devices, Notefiles, routes, environment
variables, and firmware. Requests carry `Authorization: Bearer <token>`, and every
operation names the thing it acts upon by its identifier.

This is the surface the `notehub` CLI itself wraps, so anything the CLI does can also be
done directly over HTTP. Use it when you need to **change** something, or to read the
configuration or contents of a specific project, fleet, or device.

### Notehub IQ

The analytics surface, and the intelligence layer for Notehub. Events that arrive in a
Notefile are mirrored into datasets, which are queryable tables inside a per-project
repository, and newly arriving data is synced automatically. It is queried with
read-only SQL and cannot add, update, or delete anything.

Use it when the question spans **many events, many devices, or a stretch of time** -
aggregates, histories, comparisons, fleet-wide behavior, cost attribution, or anything
you would otherwise answer by paging through events and adding them up yourself.

Notehub IQ is reached through the Notehub MCP server. See `notehub skills mcp`.

### Choosing between them

| The task | The surface |
|---|---|
| Change a setting, variable, route, or firmware | Classic HTTP API |
| Read the configuration of a named device or fleet | Classic HTTP API |
| Send a request to a device, or read one Notefile | Classic HTTP API |
| Aggregate, compare, or trend event data over time | Notehub IQ |
| Answer a question about many devices at once | Notehub IQ |
| Anything that must not mutate state | Notehub IQ (it is read-only by construction) |

## Vocabulary

- **Project** - the top-level container, identified by a ProjectUID. A project may also
  be referred to by a ProductUID.
- **Fleet** - a named group of devices within a project, identified by a FleetUID.
- **Device** - one Notecard, identified by a DeviceUID of the form `dev:<imei>`.
- **Notefile** - a named collection of notes that synchronize as a unit. An outbound
  queue ends in `.qo`, an inbound queue in `.qi`, and a database in `.db`.
- **Event** - the record Notehub creates when a note arrives from a device.
- **Environment variable** - configuration set at the project, fleet, or device level,
  with the most specific level winning.
- **System Notefile** - a Notefile Notehub maintains itself, such as `_session.qo` and
  `_health.qo`, which carry device and connection metadata rather than application data.

## Loading a skill

Each skill below is a Markdown document describing how to do one kind of job. Run its
command to load it, and prefer loading the skill over inferring the procedure.
