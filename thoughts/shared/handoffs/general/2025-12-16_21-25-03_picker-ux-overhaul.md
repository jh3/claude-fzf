---
date: 2025-12-17T02:25:03+0000
researcher: Claude
git_commit: 3a9859712127f5f64b07e30efac13a263a133444
branch: main
repository: claude-fzf
topic: "Session Picker UX Overhaul and Tmux Bug Fixes"
tags: [implementation, ui, tmux, picker, sessions]
status: complete
last_updated: 2025-12-16
last_updated_by: Claude
type: implementation_strategy
---

# Handoff: Picker UX Overhaul and Tmux Bug Fixes

## Task(s)
1. **Project-grouped session picker** (completed) - Refactored the flat session list into a two-level navigation:
   - Projects view: Shows unique projects with session count and latest date
   - Sessions view: Shows individual sessions within a project when expanded
   - Enter = quick resume latest session, Tab = expand to see all sessions

2. **Tmux session creation bug fix** (completed) - Fixed issue where sessions were created with only 1 window instead of 4 (claude + logs + edit + scratch). Root cause was gotmux library not working reliably for detached sessions.

3. **Shell wrapper fix** (completed) - Added `; exec $SHELL` wrapper to respawn commands so panes stay alive if Claude exits unexpectedly.

## Critical References
- `CLAUDE.md` - Project overview and architecture
- `internal/ui/picker.go` - Main picker implementation with ProjectGroup type
- `internal/tmux/tmux.go` - Tmux operations using direct exec.Command

## Recent changes
- `internal/ui/picker.go` - Complete rewrite to support project grouping with two-mode navigation
- `internal/tmux/tmux.go:15-23` - Added `runTmux()` helper using exec.Command instead of gotmux
- `internal/tmux/tmux.go:157-183` - Rewrote `CreateProjectSession` with direct tmux commands
- `internal/tmux/tmux.go:192-231` - Added `EnsureSessionWindows` to fix partially created sessions
- `cmd/claude-fzf/main.go:155-161` - Added call to `EnsureSessionWindows` when session exists

## Learnings
1. **gotmux library unreliable for detached sessions** - When creating tmux sessions from within tmux, gotmux's `NewSession` and `NewWindow` methods fail silently or return errors. Using direct `exec.Command("tmux", ...)` is more reliable.

2. **Window index conflicts** - Using `new-window -a -t session:` (with `-a` flag and trailing colon) appends windows reliably without index conflicts.

3. **Bubbletea alt screen hides stderr** - Debug output to stderr isn't visible during TUI operation; need to write to a log file for debugging.

4. **Picker cursor/selection pattern** - When implementing multi-mode navigation, maintain separate cursors for each mode (`projectCursor`, `sessionCursor`) and update `filteredProjects`/`filteredSessions` based on current mode.

## Artifacts
- `internal/ui/picker.go` - Refactored picker with ProjectGroup type and two-mode navigation
- `internal/tmux/tmux.go` - Updated tmux operations with direct exec.Command
- `cmd/claude-fzf/main.go` - Updated resumeInTmux with EnsureSessionWindows call

## Action Items & Next Steps
All tasks completed. Potential future improvements:
- Consider adding keyboard shortcut hints in preview pane
- Could add session deletion from projects view (currently only in sessions view)
- Could show more session metadata in the projects preview

## Other Notes
- The picker uses string modes ("projects", "sessions", "newproject") rather than an enum
- Sessions are sorted by ModTime descending (newest first) within each project group
- Projects are sorted by their most recent session's ModTime
- The `EnsureSessionWindows` function auto-heals partially created sessions by adding missing windows
