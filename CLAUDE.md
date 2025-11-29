# CLAUDE.md

This file provides guidance for Claude Code when working on this project.

## Project Overview

`claude-fzf` is a fuzzy finder for Claude Code sessions. It scans `~/.claude/projects/` for session files, presents an interactive picker, and resumes selected sessions. It integrates with tmux to create per-project terminal layouts.

## Project Structure

```
claude-fzf/
├── cmd/claude-fzf/main.go     # CLI entrypoint, argument parsing, orchestration
├── internal/
│   ├── cache/cache.go         # Gob-encoded session cache (~/.cache/claude-fzf/)
│   ├── config/config.go       # YAML config loading (~/.config/claude-fzf/)
│   ├── session/
│   │   ├── scanner.go         # Discovers .jsonl files in ~/.claude/projects/
│   │   ├── parser.go          # Parses JSONL session files
│   │   └── session.go         # Session struct definition
│   ├── tmux/tmux.go           # Tmux session/window management via gotmux
│   └── ui/picker.go           # Bubbletea-based interactive picker
├── config.example.yaml        # Example configuration file
├── go.mod
├── Makefile
└── README.md
```

## Building

```bash
# Build the binary
make build

# Or directly with go
go build -o claude-fzf ./cmd/claude-fzf

# Install to /usr/local/bin
make install
```

## Testing

```bash
# Run all tests
make test

# Or directly with go
go test ./...

# Run with verbose output
go test -v ./...
```

## Key Dependencies

- `github.com/charmbracelet/bubbletea` - Terminal UI framework
- `github.com/charmbracelet/bubbles` - UI components (text input)
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/GianlucaP106/gotmux` - Tmux control
- `gopkg.in/yaml.v3` - Config file parsing

## Architecture Notes

### Session Discovery
- Sessions are JSONL files in `~/.claude/projects/<encoded-path>/`
- Files are scanned in parallel, results cached by mtime
- Cache stored as gob-encoded file in `~/.cache/claude-fzf/sessions.cache`

### Picker UI
- Built with bubbletea for custom keybinding support
- Supports: Enter (resume), Ctrl-D (delete), Ctrl-A (toggle empty), filtering
- Deletes happen in-place without leaving the picker

### Tmux Integration
- Creates project-named tmux sessions with configurable windows
- Repurposes "disposable" sessions (numeric name, ≤2 windows) to avoid orphans
- Windows can have startup commands (run silently via respawn-pane)

## Common Tasks

### Adding a new keybinding
Edit `internal/ui/picker.go`, find the `Update` method, add a case in the `switch msg.String()` block.

### Adding a new tmux command
Edit `internal/tmux/tmux.go`, add a method to the `Manager` struct using `m.tmux.Command()`.

### Changing default config
Edit `internal/config/config.go`, modify `DefaultConfig()`.
