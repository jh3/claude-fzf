# claude-fzf

Fuzzy search and resume Claude Code sessions from any terminal.

## Features

- Fast startup (~5ms with cache, ~200ms cold)
- Search across all Claude Code sessions from all projects
- Preview session details (summary, messages, git branch)
- Resume any session with Enter
- Delete sessions with confirmation
- Tmux integration with automatic project session management
- Empty sessions (0 messages) hidden by default

## Requirements

- [Claude Code](https://claude.ai/code) CLI installed
- Go 1.21+ (for building from source)
- tmux (optional, for tmux integration)

## Installation

### From Source

```bash
git clone https://github.com/jh3/claude-fzf.git
cd claude-fzf
make install  # builds and installs to /usr/local/bin/
```

Or build without installing:

```bash
make build    # creates ./claude-fzf
```

## Usage

```bash
claude-fzf              # Interactive session picker
claude-fzf list         # List all sessions (for scripting)
claude-fzf clear-cache  # Clear the session cache
claude-fzf --help       # Show help

# Flags
claude-fzf -a           # Start with empty sessions visible
```

### Keybindings (in picker)

| Key | Action |
|-----|--------|
| `Enter` | Resume selected session |
| `Ctrl-D` | Delete selected session (with confirmation) |
| `Ctrl-A` | Toggle showing empty sessions |
| `Ctrl-C` / `Esc` | Cancel |
| Type | Filter sessions |

### Shell Keybinding (optional)

Add to your shell config:

**Zsh** (~/.zshrc):
```bash
bindkey -s '^g^c' 'claude-fzf\n'
```

**Bash** (~/.bashrc):
```bash
bind '"\C-g\C-c": "claude-fzf\n"'
```

## Tmux Integration

When running inside tmux, claude-fzf provides per-project session management.

Selecting a Claude session will:
1. Create a tmux session named after the project directory
2. Set up windows: `claude` (always) plus any configured windows
3. Resume the Claude session in the `claude` window

If a tmux session for that project already exists, it switches to it.

When running outside tmux, Claude resumes directly in your current terminal.

### Configuring Tmux Windows

You can customize the additional windows that are created alongside the `claude` window.

Config file location: `~/.config/claude-fzf/config.yaml`

```yaml
tmux:
  windows:
    - name: logs
    - name: edit
    - name: tests
      command: npm test -- --watch
    - name: server
      command: make run
```

Each window can optionally run a command on startup. Commands run silently (not typed out) and the shell stays alive after the command exits.

**Default windows** (when no config exists): `logs`, `edit`, `scratch`

See [config.example.yaml](config.example.yaml) for a full example.

### Tmux Keybinding (recommended)

Add to `~/.tmux.conf`:

```bash
bind-key g new-window "claude-fzf"
```

Then press `Ctrl-B g` to open the session picker from anywhere in tmux.

## How it works

Claude Code stores session data in `~/.claude/projects/`. Each project directory contains JSONL files with conversation history. This tool:

1. Scans all session files in parallel
2. Caches metadata (invalidated by file mtime)
3. Presents an interactive fuzzy finder
4. Runs `claude --resume <session-id>` on selection

## Project Structure

```
claude-fzf/
├── cmd/claude-fzf/main.go    # Entrypoint
├── internal/
│   ├── cache/cache.go        # Mtime-based caching
│   ├── config/config.go      # Configuration loading
│   ├── session/              # Session discovery & parsing
│   ├── tmux/tmux.go          # Tmux integration
│   └── ui/fzf.go             # Fuzzy finder UI
├── config.example.yaml       # Example configuration
├── go.mod
├── Makefile
└── README.md
```

## License

MIT
