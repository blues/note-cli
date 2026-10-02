# Notecard & Notehub CLI

This repository contains command-line tools for working with the Notecard and Notehub CLI utilities.

## Installing

The Notecard & Notehub CLIs can be installed either with a package manager (`homebrew`) or by downloading the binaries from the [releases page](https://github.com/blues/note-cli/releases).

### Homebrew

```bash
brew install --cask blues/note-cli/note-cli
```

> [!IMPORTANT]
If you are upgrading from a version older than v1.9.1 to a newer version, using `brew`, you will need to uninstall first using `brew uninstall note-cli`.

### Downloading the binaries

For all releases, we have compiled the Notecard and Notehub utilities for different OS and architectures [here](https://github.com/blues/note-cli/releases).

If you don't see your OS and architecture supported, please file an issue and we'll add it to new releases.

## Using the Notecard CLI

```
notecard [options] ['{"req":"..."}']
```

Value options accept either `--port /dev/ttyUSB0` or `--port=/dev/ttyUSB0`; boolean
options accept `--verbose` or `--verbose=false`.

```bash
notecard                                    # the options, the saved settings, and the available ports
notecard --help                             # the same option list
notecard --req '{"req":"card.version"}'     # send a request to the Notecard
notecard '{"req":"card.version"}'           # the same, with the request as the sole argument
```

A single JSON request may be given as the last argument instead of with `--req`. Place
options before it, because option parsing stops at the first argument that is not an
option. A standalone `--` also ends option parsing.

Running `notecard --interface serial --port /dev/ttyUSB0` alone saves the interface and
port for future invocations. Combined with any other option or with a request, those
settings apply only to that one invocation.

## Using the Notehub CLI

```
notehub [mode] [options]
```

The optional `mode` keyword, which must come first, selects which options are
available and how the rest of the command line is read. Without one, the CLI runs in
its default mode.

| Mode | Purpose |
| --- | --- |
| *(omitted)* or `default` | Interact with Notehub: requests, uploads, environment variables, provisioning |
| `signin` | Sign in to Notehub in your browser |
| `signin-agent <agent-name>` | Sign in to Notehub from your agent |
| `netcat` | Send an HTTP request from stdin to Notehub, like netcat for HTTPS |
| `skills` | Read and manage the skills a project holds: show, list, get, set, rename, delete, backup, restore |

A mode named on its own performs its default action or reports required arguments.
`--help` describes the options of the mode being run:

```bash
notehub                 # the modes, the general options, and the saved settings
notehub --help          # every option available in the default mode
notehub skills --help   # every option available in skills mode
```

Value options accept either `--project app:123` or `--project=app:123`; boolean
options accept `--pretty` or `--pretty=false`.

In the default mode, place options before the request. Within a mode's command,
such as `notehub skills get`, options may appear before or after the command's
arguments. A standalone `--` ends option parsing; following arguments are treated
as positional values.

Running `notehub --hub example.com` alone saves the hub for future invocations.
With other options or a request, specifying `--hub` does not by itself save the setting.

### Sign in

```bash
notehub signin                         # same as notehub --signin
notehub signin-agent "My Agent"        # same as notehub --signin-agent "My Agent"
```

Both agent sign-in forms require a nonempty name. Quote names containing spaces.
The editable name in Notehub's API Access list starts as `Notehub CLI - My Agent`.
Normal sign-in uses the computer name up to its first dot: `rays-macbook.local`
becomes `Notehub CLI - rays-macbook`. Explicit agent names keep any dots.

With polling enabled, agent sign-in writes one JSON object per line to stdout:
the URL to give the user, progress every 10 seconds with `remaining_seconds`, and
a final result with `success` and `status`. Normal sign-in opens the browser.
With localhost authentication, both forms open the browser interactively.

### Netcat

`notehub netcat` works like netcat for HTTPS: it reads a complete HTTP/1.1 request from
stdin, sends it over TLS to the configured Notehub destination, and writes the complete
HTTP response, including its status line, headers, and body, to stdout. Diagnostics go to
stderr.

TLS is always enabled, and the server's certificate is verified. Notehub credentials from
`--signin` are used for the request.

```bash
notehub netcat < request.http > response.http
```

The request's `Authorization` header, if any, is replaced by your credentials, and its
`Host` by the hub's, so both may be omitted. A body without a `Content-Length` runs to the
end of the input. Stdout is always a complete HTTP response. The exit code is 0 when the
hub answered, whatever its status, and 1 when the response is `notehub`'s own: 401 when
you are not signed in, 400 when the request can't be read, and 502 when the hub can't be
reached.

### Skills

A project's skills are Markdown files, stored in the project, that tell an AI agent
what the product is and what its data means. To write them, point any AI agent at
https://notehub.md and ask it to train your project. The `skills` mode reads and
manages them:

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
`kind:` (or `kinds:` or `tags:`) line of its front matter. To change them, edit that
line and `set` the file again. A backup writes the kinds a skill is stored under into
its front matter when they aren't already there, so a restore keeps them.

`get` won't overwrite a local file that differs unless given `--force`, and neither
will `backup`. `set` stores only what differs, and never removes anything. `rename`
won't replace another skill without `--force`, and `delete all` requires it. `restore`
stores what the zip holds and then removes every other skill. `--dry-run` says what
`set`, `rename`, `delete`, `backup` or `restore` would do, without doing it.

## Building the CLIs

### Dependencies

- Install Go and the Go tools [(here)](https://golang.org/doc/install)

### Compiling the utilities

If you want to build the latest, follow the directions below.

```bash
cd notecard
go build .
```

```bash
cd notehub
go build .
```

## Additional Resources

To learn more about Blues Wireless, the Notecard and Notehub, see:

- [blues.com](https://blues.io)
- [notehub.io](https://notehub.io)
- [wireless.dev](https://wireless.dev)

## License

Copyright (c) 2017 Blues Inc. Released under the MIT license. See [LICENSE](LICENSE) for details.
