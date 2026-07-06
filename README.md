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

```bash
go build -o tmgr .
sudo install -m 755 tmgr /usr/local/bin/tmgr
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
go build -o tmgr .
```

## Packaging

```bash
./build-and-package.sh
```

## License

MIT
