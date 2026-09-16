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

## Using the Notehub CLI

```
notehub [mode] [options]
```

The optional leading `mode` keyword determines which options are available and how the
rest of the command line is interpreted. With no mode keyword the CLI runs in its
default mode, which is how it has always worked, so existing command lines and scripts
continue to work unchanged. Naming the default mode explicitly is identical to not
naming a mode at all.

| Mode | Purpose |
| --- | --- |
| *(omitted)* or `default` | Interact with Notehub: requests, uploads, environment variables, provisioning |
| `skills` | Build and manage Notehub skills |

A mode keyword, when used, must be the first argument on the command line.

Naming a mode with nothing else performs that mode's own default action, which is not
necessarily output meant for a person to read. Help is always displayed by `-help`,
which describes the options belonging to the mode being run:

```bash
notehub                 # the modes, the general options, and the saved settings
notehub -help           # every option available in the default mode
notehub skills -help    # every option available in skills mode
```

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
