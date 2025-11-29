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
├── claude-fzf.plugin.zsh # Zsh integration
├── claude-fzf.bash      # Bash integration
└── README.md
```

## License

MIT
