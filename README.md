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
