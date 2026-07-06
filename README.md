# tmgr

A terminal UI for managing tmux sessions.

## Features

- List all tmux sessions
- Navigate with arrow keys (↑/↓) or vim keys (j/k)
- Attach to a session with Enter
- Visual indicator for attached sessions
- Rename sessions

## Installation

### Download binary

Download the latest release from the releases page.

### Build from source

Requires Go, [GoReleaser](https://goreleaser.com/), and [UPX](https://upx.github.io/).

```bash
./build-and-package.sh
sudo install -m 755 bin/tmgr /usr/local/bin/tmgr
```

## Usage

```bash
tmgr
```

### Keybindings

| Key         | Action                    |
|-------------|---------------------------|
| ↑ / k       | Move up                   |
| ↓ / j       | Move down                 |
| Enter / Space | Attach to session       |
| r           | Rename session             |
| q / Esc     | Quit                      |

## Building

```bash
./build-and-package.sh
```

This runs tests, vet, cross-compiles all targets, and compresses binaries
with UPX. The local binary is placed at `bin/tmgr` and distributable
archives are placed in `package/`.

For a quick local build without GoReleaser or UPX:

```bash
go build -o tmgr .
```

## License

MIT

---

Copyright (c) 2026-present Douglas Hoard

