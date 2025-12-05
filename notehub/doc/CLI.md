# Notehub CLI Documentation

The Notehub CLI is a command-line tool for interacting with Blues Notehub. It provides commands for authentication, managing projects, devices, fleets, products, and firmware updates.

## Table of Contents

- [Global Flags](#global-flags)
- [Device Scoping](#device-scoping)
- [Authentication](#authentication)
- [Project Management](#project-management)
- [Product Management](#product-management)
- [Fleet Management](#fleet-management)
- [Route Management](#route-management)
- [Device Management](#device-management)
- [Firmware Updates (DFU)](#firmware-updates-dfu)
- [Configuration](#configuration)
- [Examples](#examples)
- [Tips](#tips)
- [Error Handling](#error-handling)
- [Getting Help](#getting-help)

---

## Global Flags

These flags are available for all commands:

| Flag        | Short | Description                             |
| ----------- | ----- | --------------------------------------- |
| `--project` | `-p`  | Project UID to use for the command      |
| `--product` |       | Product UID to use for the command      |
| `--device`  | `-d`  | Device UID to use for the command       |
| `--verbose` | `-v`  | Display requests and responses          |
| `--json`    |       | Output only JSON (strip non-JSON lines) |
| `--pretty`  |       | Pretty print JSON output                |

---

## Device Scoping

Many commands support flexible device scoping to target one or more devices. The scope syntax is consistent across all device and firmware update commands.

### Scope Formats

| Format | Description | Example |
|--------|-------------|---------|
| `dev:xxxx` | Single device by UID | `dev:864475046552567` |
| `imei:xxxx` | Single device by IMEI | `imei:123456789012345` |
| `fleet:xxxx` | All devices in fleet (by UID) | `fleet:abc123...` |
| `production` | All devices in named fleet | `production` |
| `prod*` | Fleet wildcard matching | `prod*` (matches prod-east, prod-west) |
| `@fleet-name` | Fleet indirection (explicit) | `@production` |
| `@` | All devices in project | `@` |
| `@devices.txt` | Device UIDs from file (one per line) | `@devices.txt` |
| `dev:a,dev:b` | Multiple scopes (comma-separated) | `dev:111,dev:222` |

### Fleet Name vs Fleet Indirection

Both `production` and `@production` target all devices in the "production" fleet:

- **Fleet name** (`production`) - Direct, supports wildcards (`prod*`)
- **Fleet indirection** (`@production`) - Explicit, clearer intent

Use whichever style you prefer - they work identically for single fleet names.

### File-Based Scoping

Create a text file with device UIDs (one per line):

```
dev:864475046552567
dev:864475046552568
dev:864475046552569
```

Then reference it with `@devices.txt` in any command that accepts scope.

---

## Authentication

Commands for signing in, signing out, and managing authentication tokens.

### `notehub auth signin`

Sign in to Notehub using browser-based OAuth2 flow.

```bash
# Basic signin
notehub auth signin

# Sign in and automatically set a project
notehub auth signin --set-project "My Project"
notehub auth signin --set-project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

**Flags:**
- `--set-project`: Automatically set project after signin (name or UID)

### `notehub auth signin-token [token]`

Sign in to Notehub using a personal access token.

```bash
# Sign in with a personal access token
notehub auth signin-token your-personal-access-token

# Sign in with token and set project
notehub auth signin-token your-token --set-project "My Project"
```

**Flags:**
- `--set-project`: Automatically set project after signin (name or UID)

### `notehub auth signout`

Sign out of Notehub and remove stored credentials.

```bash
notehub auth signout
```

### `notehub auth token`

Display the current authentication token.

```bash
notehub auth token
```

---

## Project Management

Commands for listing and selecting Notehub projects.

### `notehub project list`

List all projects for the authenticated user.

```bash
# List all projects
notehub project list

# List with JSON output
notehub project list --json

# List with pretty JSON
notehub project list --pretty
```

### `notehub project get [project-name-or-uid]`

Get detailed information about a specific project. If no project is specified, uses the active project.

```bash
# Get information about active project
notehub project get

# Get information about specific project by UID
notehub project get app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Get information about specific project by name
notehub project get "My Project"

# Get with JSON output
notehub project get --json

# Get with pretty JSON
notehub project get --pretty
```

### `notehub project set [project-name-or-uid]`

Set the active project in the configuration.

```bash
# Set project by name
notehub project set "My Project"

# Set project by UID
notehub project set app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### `notehub project clear`

Clear the active project from the configuration.

```bash
notehub project clear
```

---

## Product Management

Commands for listing and managing products in Notehub projects.

### `notehub product list`

List all products in the current or specified project.

```bash
# List products in current project
notehub product list

# List products in specific project
notehub product list --project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# List with JSON output
notehub product list --json

# List with pretty JSON
notehub product list --pretty
```

### `notehub product get [product-uid-or-name]`

Get detailed information about a specific product.

```bash
# Get product by name
notehub product get "My Product"

# Get product by UID
notehub product get com.company.user:product-name

# Get with JSON output
notehub product get "My Product" --json
```

---

## Fleet Management

Commands for listing and managing fleets in Notehub projects.

### `notehub fleet list`

List all fleets in the current or specified project.

```bash
# List all fleets
notehub fleet list

# List fleets in specific project
notehub fleet list --project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# List with JSON output
notehub fleet list --json
```

### `notehub fleet get [fleet-uid-or-name]`

Get detailed information about a specific fleet.

```bash
# Get fleet by name
notehub fleet get production

# Get fleet by UID
notehub fleet get fleet:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Get with pretty JSON
notehub fleet get production --pretty
```

### `notehub fleet create [name]`

Create a new fleet in the current project.

```bash
# Create a simple fleet
notehub fleet create production

# Create fleet with smart rule
notehub fleet create production --smart-rule '$contains(environment_variables.stage, "prod")'

# Create fleet with connectivity assurance enabled
notehub fleet create production --connectivity-assurance
```

**Flags:**
- `--smart-rule`: JSONata expression for dynamic fleet membership
- `--connectivity-assurance`: Enable connectivity assurance for this fleet

### `notehub fleet update [fleet-uid-or-name]`

Update a fleet's properties.

```bash
# Update fleet name
notehub fleet update production --name production-fleet

# Update smart rule
notehub fleet update production --smart-rule '$contains(tags, "prod")'

# Enable connectivity assurance
notehub fleet update production --connectivity-assurance true

# Set watchdog timer
notehub fleet update production --watchdog-mins 60
```

**Flags:**
- `--name`: New name for the fleet
- `--smart-rule`: JSONata expression for dynamic fleet membership
- `--connectivity-assurance`: Enable or disable connectivity assurance
- `--watchdog-mins`: Watchdog timer in minutes (0 to disable)

### `notehub fleet delete [fleet-uid-or-name]`

Delete a fleet from the current project.

```bash
# Delete fleet by name
notehub fleet delete production

# Delete fleet by UID
notehub fleet delete fleet:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

---

## Route Management

Commands for creating, updating, deleting, and viewing routes in Notehub projects.

### `notehub route list`

List all routes in the current or specified project.

```bash
# List all routes
notehub route list

# List routes in specific project
notehub route list --project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# List with JSON output
notehub route list --json

# List with pretty JSON
notehub route list --pretty
```

### `notehub route get [route-uid-or-name]`

Get detailed information about a specific route.

```bash
# Get route by UID
notehub route get route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Get route by name
notehub route get "My Route"

# Get with pretty JSON
notehub route get "My Route" --pretty
```

### `notehub route create [label]`

Create a new route in the current project. Requires a JSON configuration file.

```bash
# Create route from JSON file
notehub route create "My Route" --config route.json
```

**Example route.json for HTTP route:**

```json
{
  "label": "My HTTP Route",
  "http": {
    "url": "https://example.com/webhook",
    "fleets": ["fleet:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"],
    "throttle_ms": 100,
    "timeout": 5000,
    "http_headers": {
      "X-Custom-Header": "value"
    }
  }
}
```

**Flags:**

- `--config`: Path to JSON configuration file (required)

### `notehub route update [route-uid-or-name]`

Update an existing route. Requires a JSON configuration file.

```bash
# Update route from JSON file
notehub route update "My Route" --config route-update.json

# Update by UID
notehub route update route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx --config route-update.json
```

**Example route-update.json (partial update):**

```json
{
  "http": {
    "url": "https://newexample.com/webhook",
    "throttle_ms": 50
  }
}
```

**Flags:**

- `--config`: Path to JSON configuration file (required)

### `notehub route delete [route-uid-or-name]`

Delete a route from the current project.

```bash
# Delete route by UID
notehub route delete route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Delete route by name
notehub route delete "My Route"
```

### `notehub route logs [route-uid-or-name]`

Get logs for a specific route, showing delivery attempts, errors, and status information.

```bash
# Get logs for route
notehub route logs "My Route"

# Get logs by UID
notehub route logs route:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Get logs with pagination
notehub route logs "My Route" --page-size 100 --page-num 1

# Filter logs by device
notehub route logs "My Route" --device dev:864475046552567

# Get logs with JSON output
notehub route logs "My Route" --json
```

**Flags:**

- `--page-size`: Number of logs to return per page (default: 50)
- `--page-num`: Page number to retrieve (default: 1)
- `--device`: Filter logs by device UID

---

## Device Management

Commands for listing and managing devices in Notehub projects.

### `notehub device list`

List all devices in the current or specified project.

```bash
# List all devices
notehub device list

# List devices in specific project
notehub device list --project app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# List with JSON output
notehub device list --json
```

### `notehub device enable [scope]`

Enable one or more devices, allowing them to communicate with Notehub.

See [Device Scoping](#device-scoping) for all supported scope formats.

```bash
# Enable a single device
notehub device enable dev:864475046552567

# Enable all devices in a fleet (by name)
notehub device enable production

# Enable all devices in a fleet (explicit indirection)
notehub device enable @production

# Enable all devices in project
notehub device enable @

# Enable devices from a file
notehub device enable @devices.txt

# Enable multiple devices
notehub device enable dev:111,dev:222,dev:333

# Enable devices with wildcard fleet matching
notehub device enable prod*
```

### `notehub device disable [scope]`

Disable one or more devices, preventing them from communicating with Notehub.

See [Device Scoping](#device-scoping) for all supported scope formats.

```bash
# Disable a single device
notehub device disable dev:864475046552567

# Disable all devices in a fleet
notehub device disable @production

# Disable all devices in project
notehub device disable @

# Disable devices from a file
notehub device disable @devices.txt
```

### `notehub device move [scope] [fleet-uid-or-name]`

Move one or more devices to a fleet.

See [Device Scoping](#device-scoping) for all supported scope formats.

```bash
# Move a single device to a fleet
notehub device move dev:864475046552567 production

# Move a device to a fleet by UID
notehub device move dev:864475046552567 fleet:xxxx

# Move all devices from one fleet to another
notehub device move @old-fleet new-fleet

# Move devices from a file to a fleet
notehub device move @devices.txt production

# Move multiple devices to a fleet
notehub device move dev:111,dev:222 production
```

### `notehub device health [device-uid]`

Get the health log for a specific device, showing boot events, DFU completions, and other health-related information.

```bash
# Get health log for a device
notehub device health dev:864475046552567

# Get health log with JSON output
notehub device health dev:864475046552567 --json

# Get health log with pretty JSON
notehub device health dev:864475046552567 --pretty
```

### `notehub device session [device-uid]`

Get the session log for a specific device, showing connection history, network information, and session statistics.

```bash
# Get session log for a device
notehub device session dev:864475046552567

# Get session log with JSON output
notehub device session dev:864475046552567 --json

# Get session log with pretty JSON
notehub device session dev:864475046552567 --pretty
```

---

## Firmware Updates (DFU)

Commands for scheduling and managing firmware updates for Notecards and host MCUs.

### `notehub dfu list`

List all firmware files available in the current project.

```bash
# List all firmware files
notehub dfu list

# List only host firmware
notehub dfu list --type host

# List only notecard firmware
notehub dfu list --type notecard

# Filter by version
notehub dfu list --version "1.2.3"

# Filter by target
notehub dfu list --target "stm32"

# List with pretty JSON
notehub dfu list --pretty
```

**Flags:**
- `--type`: Filter by firmware type (host or notecard)
- `--product`: Filter by product UID
- `--version`: Filter by version
- `--target`: Filter by target device
- `--filename`: Filter by filename

### `notehub dfu update [firmware-type] [filename] [scope]`

Schedule a firmware update for devices. Firmware type must be either `host` or `notecard`.

See [Device Scoping](#device-scoping) for all supported scope formats.

**Additional Filter Flags:**

These filters narrow down the scope further:

- `--tag`: Filter by device tags (comma-separated)
- `--serial`: Filter by serial numbers (comma-separated)
- `--location`: Filter by location
- `--notecard-firmware`: Filter by Notecard firmware version
- `--host-firmware`: Filter by host firmware version
- `--product`: Filter by product UID
- `--sku`: Filter by SKU

```bash
# Schedule notecard firmware update for a specific device
notehub dfu update notecard notecard-6.2.1.bin dev:864475046552567

# Schedule host firmware update for all devices in a fleet
notehub dfu update host app-v1.2.3.bin @production

# Schedule update for multiple devices
notehub dfu update notecard notecard-6.2.1.bin dev:111,dev:222,dev:333

# Schedule update for all devices in project
notehub dfu update notecard notecard-6.2.1.bin @

# Schedule update for devices from a file
notehub dfu update host app-v1.2.3.bin @devices.txt

# Schedule update for fleet with wildcard
notehub dfu update notecard notecard-6.2.1.bin prod*

# Schedule update with additional SKU filter
notehub dfu update notecard notecard-6.2.1.bin @production --sku NOTE-WBEX

# Schedule update with additional location filter
notehub dfu update host app-v1.2.3.bin @production --location "San Francisco"

# Combine scope with multiple additional filters
notehub dfu update notecard notecard-6.2.1.bin @production --tag outdoor --sku NOTE-WBEX
```

### `notehub dfu cancel [firmware-type] [scope]`

Cancel pending firmware updates for devices. Firmware type must be either `host` or `notecard`.

See [Device Scoping](#device-scoping) for all supported scope formats.

**Additional Filter Flags:**

- `--tag`: Filter by device tags (comma-separated)
- `--serial`: Filter by serial numbers (comma-separated)

```bash
# Cancel notecard firmware update for a specific device
notehub dfu cancel notecard dev:864475046552567

# Cancel host firmware updates for all devices in a fleet
notehub dfu cancel host @production

# Cancel updates for multiple devices
notehub dfu cancel notecard dev:111,dev:222,dev:333

# Cancel updates for all devices in project
notehub dfu cancel notecard @

# Cancel updates for devices from a file
notehub dfu cancel host @devices.txt

# Cancel updates with additional tag filter
notehub dfu cancel host @production --tag outdoor
```

---

## Configuration

The CLI stores configuration in `~/.notehub/config.yaml`, including:

- Active project UID
- API hub URL (default: notehub.io)
- Authentication credentials (stored securely)

You can also use environment variables:

```bash
export NOTEHUB_PROJECT=app:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export NOTEHUB_VERBOSE=true
```

---

## Examples

### Common Workflows

**1. Initial Setup**

```bash
# Sign in to Notehub
notehub auth signin

# List available projects
notehub project list

# Set active project
notehub project set "My Project"

# List devices in project
notehub device list
```

**2. Managing a Fleet**

```bash
# Create a production fleet
notehub fleet create production

# Move devices to the fleet
notehub device move dev:123456 production
notehub device move dev:789012 production

# Or move multiple devices at once
notehub device move dev:123456,dev:789012 production

# View fleet details
notehub fleet get production

# Enable all devices in the fleet
notehub device enable @production
```

**3. Firmware Update Workflow**

```bash
# List available firmware
notehub dfu list --type notecard

# Schedule firmware update for a fleet
notehub dfu update notecard notecard-6.2.1.bin @production

# Schedule update with additional filtering
notehub dfu update notecard notecard-6.2.1.bin @production --sku NOTE-WBEX

# Check device health after update
notehub device health dev:864475046552567

# Cancel pending updates if needed
notehub dfu cancel notecard @production
```

**4. Device Troubleshooting**

```bash
# Check device health
notehub device health dev:864475046552567

# View session history
notehub device session dev:864475046552567

# Get verbose output for debugging
notehub device session dev:864475046552567 --verbose
```

**5. Using File-Based Scoping**

```bash
# Create a file with device UIDs
cat > devices.txt <<EOF
dev:864475046552567
dev:864475046552568
dev:864475046552569
EOF

# Enable devices from file
notehub device enable @devices.txt

# Move devices to a fleet
notehub device move @devices.txt production

# Schedule firmware update
notehub dfu update notecard notecard-6.2.1.bin @devices.txt

# Disable devices
notehub device disable @devices.txt
```

**6. Wildcard Fleet Operations**

```bash
# Target all fleets starting with "prod"
notehub device enable prod*

# Schedule updates for multiple fleets
notehub dfu update notecard notecard-6.2.1.bin prod*
```

---

## Tips

1. **Use `--pretty` for human-readable JSON output:**
   ```bash
   notehub device list --pretty | less
   ```

2. **Combine with jq for advanced JSON processing:**
   ```bash
   notehub device list --json | jq '.devices[] | select(.serial_number != "")'
   ```

3. **Use environment variables for automation:**
   ```bash
   export NOTEHUB_PROJECT=app:my-project-uid
   export NOTEHUB_VERBOSE=true
   ./my-script.sh
   ```

4. **Save authentication token for CI/CD:**
   ```bash
   export NOTEHUB_TOKEN=$(notehub auth token)
   ```

5. **Use verbose mode for debugging:**
   ```bash
   notehub device list --verbose
   ```

6. **Scope consistency across commands:**
   ```bash
   # The same scope works for all device-related commands
   SCOPE="@production"
   notehub device enable $SCOPE
   notehub dfu update notecard fw.bin $SCOPE
   notehub device health $(notehub device list --json | jq -r ".devices[0].uid")
   ```

---

## Error Handling

If you encounter authentication errors:

```bash
# Sign out and sign in again
notehub auth signout
notehub auth signin
```

If you get "no project set" errors:

```bash
# List available projects
notehub project list

# Set a project
notehub project set "My Project"
```

If you get "no devices found in scope" errors:

```bash
# Verify devices exist in the scope
notehub device list

# Check fleet membership
notehub fleet get production
```

For API errors, use `--verbose` to see the full request/response:

```bash
notehub device list --verbose
```

---

## Getting Help

For any command, use the `--help` flag:

```bash
notehub --help
notehub auth --help
notehub device enable --help
notehub dfu update --help
```

For more information, visit:
- [Blues Wireless Documentation](https://dev.blues.io)
- [Notehub](https://notehub.io)
- [GitHub Repository](https://github.com/blues/note-cli)
