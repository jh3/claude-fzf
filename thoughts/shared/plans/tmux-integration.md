# Tmux Integration Implementation Plan

## Overview

Add optional tmux integration to claude-fzf that manages tmux sessions on a per-project basis. When selecting a Claude session, the tool will create or switch to a dedicated tmux session for that project with a standard 4-window layout.

## Current State

- `claude-fzf` finds and lists all Claude sessions across projects
- On selection, it `cd`s to the project directory and runs `claude --resume <id>`
- No tmux awareness - runs in whatever terminal context it's invoked from

## Desired End State

When running inside tmux:
1. Selecting a Claude session creates/switches to a tmux session named after the project
2. The tmux session has 4 windows: `claude`, `logs`, `edit`, `scratch`
3. The selected Claude session resumes in the `claude` window
4. A new keybinding (`Ctrl-G Ctrl-S`) allows fast switching between existing project sessions

When running outside tmux:
- Behavior unchanged (resume claude in current terminal)

### Verification
- Run `claude-fzf` inside tmux, select a session → should switch to/create project tmux session
- Run `claude-fzf` outside tmux → should work as before
- Press `Ctrl-G Ctrl-S` → should show fzf picker of existing tmux sessions

## What We're NOT Doing

- Per-project window/pane configuration (too complex, save for v2)
- Custom commands in logs/edit/scratch windows (just empty shells)
- Persisting tmux sessions across reboots (user manages this)
- Managing non-Claude tmux sessions (only sessions we create)

---

## Phase 1: Core Tmux Detection and Session Management

### Overview
Add functions to detect tmux context and manage tmux sessions.

### Changes Required

#### 1. New file: `claude-fzf-tmux`
**File**: `claude-fzf-tmux`

Helper script with tmux utility functions:

```bash
#!/usr/bin/env bash
#
# claude-fzf-tmux - Tmux integration helpers for claude-fzf
#

set -euo pipefail

# Check if we're running inside tmux
is_in_tmux() {
    [[ -n "${TMUX:-}" ]]
}

# Get tmux session name from project path
# /Users/jh3/Development/claude-fzf -> claude-fzf
project_to_session_name() {
    local project_path="$1"
    basename "$project_path"
}

# Check if a tmux session exists
session_exists() {
    local session_name="$1"
    tmux has-session -t "=$session_name" 2>/dev/null
}

# List all tmux sessions (just names)
list_sessions() {
    tmux list-sessions -F "#{session_name}" 2>/dev/null || true
}

# Create a new tmux session with standard 4-window layout
# Does NOT attach - caller decides whether to switch
create_project_session() {
    local session_name="$1"
    local project_path="$2"

    # Create session with first window named "claude"
    tmux new-session -d -s "$session_name" -n "claude" -c "$project_path"

    # Create remaining windows
    tmux new-window -t "$session_name" -n "logs" -c "$project_path"
    tmux new-window -t "$session_name" -n "edit" -c "$project_path"
    tmux new-window -t "$session_name" -n "scratch" -c "$project_path"

    # Select the claude window
    tmux select-window -t "$session_name:claude"
}

# Switch to a session (from within tmux)
switch_to_session() {
    local session_name="$1"
    tmux switch-client -t "=$session_name"
}

# Run a command in a specific window of a session
run_in_window() {
    local session_name="$1"
    local window_name="$2"
    local command="$3"

    tmux send-keys -t "$session_name:$window_name" "$command" Enter
}

# Get the current tmux session name
current_session() {
    tmux display-message -p "#{session_name}" 2>/dev/null || echo ""
}

# Export functions for use by other scripts
export -f is_in_tmux project_to_session_name session_exists list_sessions
export -f create_project_session switch_to_session run_in_window current_session
```

### Success Criteria

#### Automated Verification:
- [ ] Script is executable: `test -x claude-fzf-tmux`
- [ ] Shellcheck passes: `shellcheck claude-fzf-tmux`
- [ ] Functions are defined: `bash -c 'source claude-fzf-tmux && type is_in_tmux'`

#### Manual Verification:
- [ ] `source claude-fzf-tmux && is_in_tmux && echo yes || echo no` returns correct result
- [ ] `project_to_session_name /Users/jh3/Development/claude-fzf` returns `claude-fzf`

---

## Phase 2: Update Main Script for Tmux Awareness

### Overview
Modify `claude-fzf` to use tmux session management when running inside tmux.

### Changes Required

#### 1. Update `claude-fzf`
**File**: `claude-fzf`

Add tmux integration after the session selection:

```bash
# Near the top, after SCRIPT_DIR
source "${SCRIPT_DIR}/claude-fzf-tmux"

# ... existing code ...

# Replace the current "resume" logic at the end of main() with:

    if $preview_only; then
        # Output both path and session ID for shell integration
        echo "${project_path}|${session_id}"
    else
        # Check if we're in tmux and should use session management
        if is_in_tmux; then
            local session_name
            session_name=$(project_to_session_name "$project_path")

            if session_exists "$session_name"; then
                # Session exists - switch to it and resume claude in the claude window
                switch_to_session "$session_name"
                run_in_window "$session_name" "claude" "claude --resume $session_id"
            else
                # Create new session with 4-window layout
                create_project_session "$session_name" "$project_path"
                switch_to_session "$session_name"
                run_in_window "$session_name" "claude" "claude --resume $session_id"
            fi
        else
            # Not in tmux - use original behavior
            if [[ -d "$project_path" ]]; then
                cd "$project_path"
                exec claude --resume "$session_id"
            else
                echo "Warning: Project directory not found: $project_path"
                echo "Attempting to resume anyway..."
                exec claude --resume "$session_id"
            fi
        fi
    fi
```

### Success Criteria

#### Automated Verification:
- [ ] Shellcheck passes: `shellcheck claude-fzf`
- [ ] Script runs without errors: `claude-fzf --help`

#### Manual Verification:
- [ ] Outside tmux: `claude-fzf` works as before (resumes in current terminal)
- [ ] Inside tmux: selecting a session creates/switches to project tmux session
- [ ] Selecting same project again switches back to existing session
- [ ] Claude resumes correctly in the `claude` window

---

## Phase 3: Session Switcher (Ctrl-G Ctrl-S)

### Overview
Add a fast session switcher that shows existing tmux sessions and switches to the selected one.

### Changes Required

#### 1. New script: `claude-fzf-sessions`
**File**: `claude-fzf-sessions`

Standalone session switcher:

```bash
#!/usr/bin/env bash
#
# claude-fzf-sessions - Quick tmux session switcher
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/claude-fzf-tmux"

main() {
    if ! is_in_tmux; then
        echo "Error: Not running inside tmux"
        exit 1
    fi

    local current
    current=$(current_session)

    local sessions
    sessions=$(list_sessions)

    if [[ -z "$sessions" ]]; then
        echo "No tmux sessions found"
        exit 0
    fi

    local selected
    selected=$(echo "$sessions" | fzf \
        --height='40%' \
        --layout=reverse \
        --border \
        --prompt='Tmux Sessions > ' \
        --header="Current: $current | Enter: Switch" \
        --preview='tmux list-windows -t {} -F "  #{window_index}: #{window_name}"' \
        --preview-window='right:30%' \
    ) || true

    if [[ -n "$selected" && "$selected" != "$current" ]]; then
        switch_to_session "$selected"
    fi
}

main "$@"
```

#### 2. Update `claude-fzf.plugin.zsh`
**File**: `claude-fzf.plugin.zsh`

Add the session switcher keybinding:

```bash
# Add after existing widget

# Session switcher widget
claude-fzf-sessions-widget() {
    "${CLAUDE_FZF_DIR}/claude-fzf-sessions"
    zle reset-prompt
}

zle -N claude-fzf-sessions-widget
bindkey "^g^s" claude-fzf-sessions-widget
```

#### 3. Update `claude-fzf.bash`
**File**: `claude-fzf.bash`

Add the session switcher keybinding:

```bash
# Add after existing widget

__claude_fzf_sessions_widget() {
    "${CLAUDE_FZF_DIR}/claude-fzf-sessions"
}

bind -x '"\C-g\C-s": __claude_fzf_sessions_widget'
```

### Success Criteria

#### Automated Verification:
- [ ] Scripts are executable: `test -x claude-fzf-sessions`
- [ ] Shellcheck passes: `shellcheck claude-fzf-sessions`

#### Manual Verification:
- [ ] `Ctrl-G Ctrl-S` opens session picker
- [ ] Shows all tmux sessions with window preview
- [ ] Selecting a session switches to it
- [ ] Pressing Escape/Ctrl-C cancels without switching

---

## Phase 4: Update Documentation

### Overview
Update README with tmux integration documentation.

### Changes Required

#### 1. Update README.md

Add new section after "Keybinding":

```markdown
## Tmux Integration

When running inside tmux, claude-fzf provides enhanced session management:

### Per-Project Tmux Sessions

When you select a Claude session, the tool will:
1. Create a tmux session named after the project directory (e.g., `claude-fzf`)
2. Set up 4 windows: `claude`, `logs`, `edit`, `scratch`
3. Resume the Claude session in the `claude` window

If a tmux session for that project already exists, it simply switches to it.

### Session Switching

Use `Ctrl-G Ctrl-S` to quickly switch between existing tmux sessions:

| Key | Action |
|-----|--------|
| `Ctrl-G Ctrl-S` | Open tmux session switcher |
| `Enter` | Switch to selected session |
| `Ctrl-C` | Cancel |

### Outside Tmux

When running outside tmux, behavior is unchanged - Claude resumes in your current terminal.
```

### Success Criteria

#### Automated Verification:
- [ ] README contains "Tmux Integration" section

#### Manual Verification:
- [ ] Documentation is clear and accurate

---

## File Structure (After Implementation)

```
claude-fzf/
├── claude-fzf              # Main script (updated)
├── claude-fzf-preview      # Preview helper (unchanged)
├── claude-fzf-tmux         # NEW: Tmux utility functions
├── claude-fzf-sessions     # NEW: Session switcher
├── claude-fzf.plugin.zsh   # Zsh integration (updated)
├── claude-fzf.bash         # Bash integration (updated)
├── README.md               # Documentation (updated)
└── docs/
    └── plan-tmux-integration.md  # This plan
```

---

## Testing Strategy

### Manual Testing Steps

1. **Outside tmux**:
   - Run `claude-fzf`, select a session
   - Verify claude resumes in current terminal (unchanged behavior)

2. **Inside tmux, new project**:
   - Run `claude-fzf`, select a session from a project with no tmux session
   - Verify new tmux session is created with correct name
   - Verify 4 windows exist: claude, logs, edit, scratch
   - Verify claude is running in the claude window

3. **Inside tmux, existing project**:
   - Run `claude-fzf`, select a session from a project that already has a tmux session
   - Verify it switches to existing session (doesn't create duplicate)
   - Verify claude resumes in the claude window

4. **Session switcher**:
   - Press `Ctrl-G Ctrl-S`
   - Verify all sessions are shown
   - Select a different session, verify switch works
   - Cancel with Ctrl-C, verify nothing changes

---

## Future Enhancements (v2+)

- Per-project window configuration via `.claude-fzf.yaml`
- Custom pane layouts for logs window
- Session persistence/restore hints
- Integration with tmuxinator/tmuxp
- Indicator in fzf showing which projects have active tmux sessions
