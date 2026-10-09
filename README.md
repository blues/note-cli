# Notecard & Notehub CLI

This repository holds two command-line tools: `notecard`, for a Notecard attached to this
computer, and `notehub`, for the Notehub service.

## Installing

Install with Homebrew, or download a binary from the
[releases page](https://github.com/blues/note-cli/releases).

```bash
brew install --cask blues/note-cli/note-cli
```

> [!IMPORTANT]
> If you are upgrading with Homebrew from a version older than v1.9.1, uninstall first with
> `brew uninstall note-cli`.

Each release has builds of both tools for several operating systems and architectures. If
yours is missing, file an issue and we'll add it.

## Using the Notecard CLI

```
notecard [options] ['{"req":"..."}']
```

`notecard` by itself shows how to get going, the saved settings, and the ports it can see,
with the saved one marked. `notecard --help` describes every option, with examples.

```bash
notecard --interface serial --port /dev/ttyUSB0   # save the port the Notecard is on
notecard '{"req":"card.version"}'                 # send a request to the Notecard
notecard --req '{"req":"card.version"}'           # the same, with the request as an option
notecard --info                                   # what the Notecard knows about itself
notecard --explore                                # its notefiles, and the notes in each
notecard --trace                                  # watch its trace output
```

A request is in the Notecard's own JSON form, given as the last argument or with `--req`,
and the Notecard's answer is written as JSON, indented with `--pretty`. Put options before
the request, since option parsing stops at the first argument that isn't one; a standalone
`--` ends it too. Value options take `--port /dev/ttyUSB0` or `--port=/dev/ttyUSB0`, and
boolean options take `--verbose` or `--verbose=false`.

Running `notecard --interface serial --port /dev/ttyUSB0` alone saves the interface and
port for the commands that follow. With any other option, or a request, they apply to that
one command. `NOTE_INTERFACE`, `NOTE_PORT` and `NOTE_PORT_CONFIG` in the environment
override the saved ones.

## Using the Notehub CLI

```
notehub [options] [request]
notehub <mode> [options] [arguments]
```

Everything `notehub` does needs Notehub API access. Sign in once with `notehub login`,
which opens your browser. An agent signs in without a browser, either with
`notehub login --json` or by setting the `NOTEHUB_TOKEN` environment variable to a
personal access token; while that variable is set, its token is used for every request,
whatever login is saved.

The first word on the command line may name a mode. Without one, `notehub` sends a request
to the hub in the Notecard's own JSON form, or works on a device's notefiles, environment
variables, and provisioning.

| Mode | Purpose |
| --- | --- |
| *(none)* | Send a request, list a device's notefiles, manage environment variables, provision devices |
| `login`, `logout` | Sign into a hub, or out of it |
| `auth` | Manage logins: `login`, `logout`, `status`, `switch`, `token` |
| `api` | Send one request to the Notehub API, the way `gh api` does for GitHub |
| `firmware` | Manage a project's host firmware, and apply it to devices |
| `skills` | Read and manage the skills a project holds |

`notehub` by itself lists the modes and the saved settings. `notehub --help` adds every
option of the default mode, and `notehub <mode> --help` describes a mode, with examples.
A mode given nothing to do shows its help too.

Value options take `--project app:123` or `--project=app:123`, boolean options take
`--pretty` or `--pretty=false`, and a standalone `--` ends option parsing. In the default
mode, options come before the request. In a mode, they may come before or after a
command's arguments.

Running `notehub --hub example.com` alone makes that hub the active one for the commands
that follow. With anything else on the command line, `--hub` applies to that command only.

### Signing in

The CLI keeps one login per hub. Each is listed in Notehub's API Access as
`Notehub CLI - <name>`, with your computer's name unless `--name` gives another.

```bash
notehub login                                  # in your browser
notehub login --name "Ray's laptop"            # under a name of your choosing
echo "$SECRET" | notehub login --with-token    # with a personal access token, kept out of your shell history
notehub login --hub preprod.blues.tools        # into another hub, which stays inactive
notehub logout                                 # out of the active hub
notehub auth status                            # every login, and which hub is active
notehub auth switch preprod.blues.tools        # make a hub you have signed into the active one
notehub auth token                             # print the token in use, for a program
```

`notehub auth login` and `notehub auth logout` are the same commands as `notehub login`
and `notehub logout`. A login leaves the active hub as it is; `auth switch` changes it.
A login that is still good is kept: `notehub login` says so and exits with success,
without opening anything, so it is safe to run just to make sure you are signed in.
`auth status` checks every login against its hub, and exits 0 when the active hub, or the
one given with `--hub`, accepts its login, and 1 otherwise. With `--json` it writes one
object instead, and exits 0.

An agent signs in with `--json`, which opens nothing and types nothing:

```bash
notehub login --name "DotBot" --json                 # the URL to give the user, progress, and the result
echo "$SECRET" | notehub login --with-token --json   # with a token: the result alone
```

The output is one JSON object per line: the URL to hand to the user, with
`expires_in_seconds` for how long the hub will honor it; a progress line every ten seconds
with the seconds remaining; and a final line with `success` and `status`. With a login
already in place, the final line is all there is. `--expires-in` asks the hub for a
shorter window than its usual ten minutes.

### Requests, notefiles, environment variables and provisioning

With no mode, `notehub` works on one project, named by `--project` or `--product`, and
often one device, named by `--device`.

```bash
notehub --projects --pretty                                                        # the projects you belong to
notehub --product com.example:sensor --device dev:864475040518622 '{"req":"note.get","file":"hub.db","note":"config"}'
notehub --product com.example:sensor --device dev:864475040518622 --explore        # the device's notefiles and notes
notehub --product com.example:sensor --scope "Field Units" --get-vars --pretty     # a fleet's environment variables
notehub --product com.example:sensor --scope dev:864475040518622 --set-vars '{"interval":"60","debug":"-"}'
notehub --product com.example:sensor --scope dev:864475040518622 --provision --sn SN-0042
```

A request is in the Notecard's own JSON form, given as the last argument or with `--req`,
and `@file` reads it from a file. The hub answers as the Notecard would, and the answer is
written as JSON, indented with `--pretty`.

`--scope` names the devices or fleets that `--get-vars`, `--set-vars`, `--provision` and
`--explore` concern: `dev:` and `fleet:` UIDs, a fleet's name, `@fleet` for a fleet's
devices, `@file` for a file listing any of these one per line, or `devices` and `fleets`
for all of them, separated by commas. A variable set to `-` is removed.

### API requests

`notehub api` sends one request to the Notehub API, authorized as you, and writes the
response body to stdout, the way `gh api` does for GitHub:

```bash
notehub api /v1/projects                                                          # a GET
notehub api /v1/projects/app:123/devices                                          # the project written into the path
notehub api /v1/projects/{project}/devices --project app:123                      # or --project filling in {project}
notehub api /v1/projects/{project}/devices --paginate --project app:123           # every page
notehub api PUT /v1/projects/{project}/devices/{device}/environment_variables --input vars.json --project app:123 --device dev:abc
cat fleet.json | notehub api POST /v1/projects/{project}/fleets --input - --project app:123   # a body from stdin
notehub api /v1/projects/{project}/fleets --field label=Field --project app:123   # parameters make a POST
notehub api GET /v1/projects/{project}/events --field pageSize=5 --project app:123 # or a GET's query string
notehub api /v1/projects --include                                                # the status line and headers too
notehub api --spec                                                                # the hub's OpenAPI spec
```

Write the path as the API documents it, with the UIDs in it, or write `{project}`,
`{product}` or `{device}` in their place and let `--project`, `--product` or `--device`
fill them in. Without a placeholder, those options aren't needed. The method is GET unless
one is given, or a body is sent, which makes it POST.

Parameters given with `--field key=value` are typed as `gh` types them: `true`, `false`,
`null` and numbers as such, `@file` for a file's contents and `@-` for stdin. `--raw-field`
keeps a string as it is. Parameters go in a JSON body, or in the query string when the
method given is GET. `--input` sends a file, or stdin with `-`, as the body, and `--header`
adds a header.

The response is written as it arrives, for as long as the hub keeps sending, unless
`--pretty` must read it whole. The exit code is 0 when the hub answered with success, and 1
when it answered with an error, whose body is still written, or when it couldn't be
reached. `--paginate` follows `has_more` through every page of a GET, writing each. `--spec`
shows the hub's OpenAPI spec, from `https://<hub>/openapi.yaml`, which describes every
endpoint.

### Firmware

`notehub firmware` manages the host firmware a project holds, and applies it to devices:

```bash
notehub firmware --project app:123                                             # what the project holds
notehub firmware show 1.2.3 --project app:123                                  # one firmware, with what its binary says
notehub firmware set ./build/app.bin --notes @RELEASE.md --project app:123     # upload, version read from the binary
notehub firmware set ./build/app.bin --version 1.2.3 --project app:123         # or given
notehub firmware get 1.2.3 ./app-1.2.3.bin --project app:123                   # download, verified against its md5
notehub firmware edit 1.2.3 --notes "fixes the sensor timeout" --project app:123
notehub firmware delete app$20260324190911.bin --project app:123
notehub firmware update 1.2.3 --scope "Field Units" --project app:123          # apply it to a fleet
notehub firmware status --scope "Field Units" --project app:123                # each device's update, as it goes
notehub firmware history --scope dev:864475040518622 --project app:123         # and as it went
notehub firmware update 1.2.3 --scope all --force --project app:123            # every device in the project
```

A firmware is named by its stored filename, such as `app$20260324190911.bin`, or by its
original name or its version when that names just one. `set` skips a binary the project
already holds byte for byte, and replaces one of the same name and version, uploading the
new one before removing the old, unless `--no-replace` refuses that.

`--scope` names devices, fleets, a fleet's name, or `@file` of them, and `all` with
`--force` addresses every device in the project; `--tag`, `--sn` and `--sku` narrow it
further. `--dry-run` says what `set`, `edit`, `delete`, `update` or `cancel` would do,
without doing it, and `--json` writes a listing, metadata, status or history as JSON.

### Skills

A project's skills are Markdown files, stored in the project, that tell an AI agent what
the product is and what its data means. To write them, point any AI agent at
https://notehub.md and ask it to train your project. The `skills` mode reads and manages
them:

```bash
notehub skills --project app:123                          # every skill, assembled into one linked document
notehub skills list --project app:123                     # what the project holds, by name, size, date and kind
notehub skills show index.md --project app:123            # one skill, exactly as stored
notehub skills get index.md - --project app:123           # the same, as 'get' writes it to stdout
notehub skills get all ./skills --project app:123         # copy every skill into ./skills
notehub skills set ./skills --dry-run --project app:123   # what storing them back would change
notehub skills set ./skills --project app:123             # store the ones that changed
notehub skills rename old.md new.md --project app:123     # store a skill under another name
notehub skills delete old.md --project app:123            # remove one skill from the project
notehub skills backup skills.zip --project app:123        # save every skill into a zip file
notehub skills restore skills.zip --project app:123       # make the project hold exactly the zip's skills
```

A skill's kinds, such as `product` or `recipe`, are stored as its tags, taken from the
`kind:` (or `kinds:` or `tags:`) line of its front matter. To change them, edit that line
and `set` the file again. A backup writes the kinds a skill is stored under into its front
matter when they aren't already there, so a restore keeps them.

`get` won't overwrite a local file that differs unless given `--force`, and neither will
`backup`. `set` stores only what differs, and never removes anything. `rename` won't
replace another skill without `--force`, and `delete all` requires it. `restore` stores
what the zip holds and then removes every other skill. `--dry-run` says what `set`,
`rename`, `delete`, `backup` or `restore` would do, without doing it.

## Building from source

With Go 1.25 or later installed:

```bash
cd notecard && go build .
cd notehub && go build .
```

## Additional resources

- [blues.com](https://blues.com), about Blues, the Notecard and Notehub
- [dev.blues.io](https://dev.blues.io), the developer documentation
- [notehub.io](https://notehub.io), the Notehub service

## License

Copyright (c) 2017 Blues Inc. Released under the MIT license. See [LICENSE](LICENSE) for details.
