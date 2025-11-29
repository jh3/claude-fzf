# claude-fzf

Fuzzy search and resume Claude Code sessions from any terminal.

![claude-fzf demo](https://raw.githubusercontent.com/junegunn/i/master/fzf-preview.png)

## Features

- Search across all Claude Code sessions from all projects
- Preview session details (summary, messages, git branch, etc.)
- Resume any session with Enter
- Copy session ID to clipboard with Ctrl-Y
- Keybinding integration for zsh and bash

## Requirements

- [fzf](https://github.com/junegunn/fzf) - `brew install fzf`
- [jq](https://stedolan.github.io/jq/) - `brew install jq`
- [Claude Code](https://claude.ai/code) CLI installed

## Installation

```bash
git clone https://github.com/yourusername/claude-fzf.git ~/.claude-fzf
```

Or just copy the files somewhere in your PATH.

## Usage

### Direct invocation

```bash
# Open the fuzzy finder and resume selected session
claude-fzf

# Just print the session ID (don't resume)
claude-fzf --preview
```

### Keybinding (recommended)

Add to your shell config to enable `Ctrl-G Ctrl-C` keybinding:

**Zsh** (~/.zshrc):
```bash
source ~/.claude-fzf/claude-fzf.plugin.zsh
```

**Bash** (~/.bashrc):
```bash
source ~/.claude-fzf/claude-fzf.bash
```

Then press `Ctrl-G Ctrl-C` from any terminal to search sessions.

## FZF Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Resume the selected session |
| `Ctrl-Y` | Copy session ID to clipboard |
| `Ctrl-P` | Toggle preview window |
| `Ctrl-J/K` | Move up/down |
| `Ctrl-C` | Cancel |

## Tmux Integration

When running inside tmux, claude-fzf provides per-project session management:

When you select a Claude session, the tool will:
1. Create a tmux session named after the project directory (e.g., `claude-fzf`)
2. Set up 4 windows: `claude`, `logs`, `edit`, `scratch`
3. Resume the Claude session in the `claude` window

If a tmux session for that project already exists, it simply switches to it.

Use tmux's built-in `Ctrl-B s` to switch between sessions.

When running outside tmux, behavior is unchanged - Claude resumes in your current terminal.

### Tmux Keybinding (recommended)

Add this to your `~/.tmux.conf` to open claude-fzf from anywhere in tmux:

```bash
bind-key g new-window "/path/to/claude-fzf/claude-fzf"
```

Then reload tmux config (`Ctrl-B :source-file ~/.tmux.conf`) and press `Ctrl-B g` to open the session picker.

### Testing Tmux Integration

1. **Outside tmux**: Run `claude-fzf`, select a session - should resume in current terminal
2. **Inside tmux**: Run `claude-fzf` (or `Ctrl-B g`), select a session - should create/switch to project tmux session with 4 windows
3. **Verify windows**: Press `Ctrl-B w` to see the window list (`claude`, `logs`, `edit`, `scratch`)

## Customization

### Custom keybinding

Set `CLAUDE_FZF_KEY` before sourcing the plugin:

```bash
# Use Ctrl-X Ctrl-C instead
CLAUDE_FZF_KEY="^x^c"
source ~/.claude-fzf/claude-fzf.plugin.zsh
```

### Additional fzf options

```bash
# Add custom fzf options
export CLAUDE_FZF_OPTS="--height 100% --tmux center"
```

## How it works

Claude Code stores session data in `~/.claude/projects/`. Each project directory contains JSONL files with conversation history. This tool:

1. Scans all session files across all projects
2. Extracts metadata (summary, timestamps, messages)
3. Pipes formatted data into fzf for fuzzy searching
4. Runs `claude --resume <session-id>` on selection

## Files

```
claude-fzf/
├── claude-fzf           # Main script
├── claude-fzf-preview   # Preview helper for fzf
├── claude-fzf-tmux      # Tmux utility functions
├── claude-fzf.plugin.zsh # Zsh integration
├── claude-fzf.bash      # Bash integration
└── README.md
```

## License

MIT
