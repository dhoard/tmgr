# AGENTS.md

Instructions for any coding agent working in this repository.

## Identity

- **Project**: tmgr
- **Purpose**: Terminal UI for managing tmux sessions
- **Language**: Go 1.21
- **Module path**: `github.com/dhoard/tmgr`

## Build, Test, and Validation Commands

All commands are run from the project root (where `go.mod` lives).

```bash
# Run tests
go test ./...

# Run vet
go vet ./...

# Build binary
go build -o tmgr .

# Full release build: test + vet + cross-compile + package (requires GoReleaser + UPX)
./build-and-package.sh
```

No formatter (`go fmt`, `gofumpt`, `golangci-lint`) is currently configured as a
build gate. `go vet` is the only static analysis step.

## Source Layout

```
main.go              # single-file application: TUI model, tmux interaction, entry point
```

The project is a single Go source file in the project root — no subdirectory
layout, no internal packages, no separate command directory.

## Coding Conventions

- Every Go source file starts with the MIT copyright header block (14-line
  comment).
- Package naming follows Go conventions: lowercase, single word where possible.
- Exported types use PascalCase; unexported use camelCase.
- Error wrapping uses `fmt.Errorf("...: %w", err)`.
- TUI framework: `charmbracelet/bubbletea` (model-update-view pattern).
- Style framework: `charmbracelet/lipgloss`.
- External commands are executed via `os/exec.Command`.

### Copyright Header

Every `.go` file must start with the MIT license block:

```go
//
// Copyright (c) 2026-present Douglas Hoard
//
// Permission is hereby granted, free of charge, ... [14 lines total]
//
```

## Test Conventions

- Tests live alongside source in `*_test.go` files.
- Same-package testing is used (e.g., `package main` in `main_test.go`).
- No test framework dependency currently; stdlib `testing` package.
- The `main` package must have a compile smoke test in `main_test.go`.

## Key Dependencies

| Dependency | Role |
|-----------|------|
| `github.com/charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Terminal styling |

## Git Workflow

- Branch: `main`
- Commits should follow conventional commits where practical
- Signed-off commits preferred

## Constraints

- tmux must be installed and available at runtime
- Go 1.21 minimum (from `go.mod`)
- GoReleaser + UPX required for release packaging
- Single binary, no configuration files
