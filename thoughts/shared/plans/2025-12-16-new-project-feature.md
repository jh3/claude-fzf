# New Project Feature Implementation Plan

## Overview

Add the ability to create new projects from the claude-fzf picker. User presses `Ctrl-N`, types a path (or just project name if `projects_dir` is configured), and the system creates the directory, initializes git, creates a tmux session (if in tmux), and starts a fresh Claude session.

## Current State Analysis

- Picker supports `ActionResume` and `ActionDelete` actions
- Config only has `tmux.windows` setting
- `CreateProjectSession` in tmux.go already supports creating sessions with a shell command
- Filter input is a `textinput.Model` that can be repurposed for path entry

## Desired End State

1. User can press `Ctrl-N` in picker to enter "new project" mode
2. Filter input becomes path input (placeholder changes, value cleared)
3. If `projects_dir` is set in config, user types just the project name
4. If not set, user types full path (defaulting to `~/` prefix)
5. Enter creates directory, git init, tmux session, and runs `claude`
6. Works both inside and outside tmux

### Verification:
- `Ctrl-N` switches picker to path entry mode with appropriate placeholder
- Entering a name/path creates the directory structure
- Git repo is initialized in the new directory
- Tmux session is created with configured windows (if in tmux)
- `claude` starts in the new directory

## What We're NOT Doing

- Template support (e.g., create from boilerplate)
- Integration with `gh repo create` or other git hosting
- Nested project creation validation
- Project naming conventions/validation beyond basic path rules

## Implementation Approach

Modify the picker to support a "new project mode" where the filter input is repurposed for path entry. Add a new `ActionNewProject` that returns the path. Handle this action in main.go to create the directory, init git, and launch claude.

---

## Phase 1: Config Changes

### Overview
Add optional `projects_dir` setting to config.

### Changes Required:

#### 1. Config struct
**File**: `internal/config/config.go`
**Changes**: Add `ProjectsDir` field to Config struct

```go
// Config holds all configuration options
type Config struct {
	ProjectsDir string `yaml:"projects_dir,omitempty"`
	Tmux        Tmux   `yaml:"tmux"`
}
```

#### 2. Update config.example.yaml
**File**: `config.example.yaml`
**Changes**: Document the new setting

```yaml
# Optional: base directory for new projects
# If set, Ctrl-N in picker prompts for project name only
# If not set, prompts for full path (starting from ~)
# projects_dir: ~/projects

tmux:
  windows:
    - name: logs
    - name: edit
    - name: scratch
```

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `go build ./...`
- [x] Config with `projects_dir` loads correctly

#### Manual Verification:
- [x] Config without `projects_dir` uses empty string (falsy)

---

## Phase 2: Picker UI Changes

### Overview
Add `Ctrl-N` keybinding and "new project mode" to picker.

### Changes Required:

#### 1. Add new action type
**File**: `internal/ui/picker.go`
**Changes**: Add `ActionNewProject` constant and update Result struct

```go
const (
	ActionNone Action = iota
	ActionResume
	ActionDelete
	ActionNewProject  // NEW
)

// Result holds the selected session and action
type Result struct {
	Session     *session.Session
	Action      Action
	ProjectPath string  // NEW: for ActionNewProject
}
```

#### 2. Add new project mode state
**File**: `internal/ui/picker.go`
**Changes**: Add fields to pickerModel

```go
type pickerModel struct {
	// ... existing fields ...
	newProjectMode bool    // NEW: are we in new project path entry mode?
	projectsDir    string  // NEW: base directory from config (may be empty)
}
```

#### 3. Update constructor
**File**: `internal/ui/picker.go`
**Changes**: Accept projectsDir parameter

```go
func newPickerModel(sessions []session.Session, showEmpty bool, projectsDir string) pickerModel {
	// ... existing code ...
	m := pickerModel{
		// ... existing fields ...
		projectsDir: projectsDir,  // NEW
	}
	// ...
}
```

#### 4. Update SelectSession function
**File**: `internal/ui/picker.go`
**Changes**: Accept and pass projectsDir

```go
func SelectSession(sessions []session.Session, showEmpty bool, projectsDir string) (Result, error) {
	m := newPickerModel(sessions, showEmpty, projectsDir)
	// ... rest unchanged ...
}
```

#### 5. Add Ctrl-N handler
**File**: `internal/ui/picker.go`
**Changes**: Add case in Update method's switch block (around line 136)

```go
case "ctrl+n":
	m.newProjectMode = true
	m.filter.SetValue("")
	if m.projectsDir != "" {
		m.filter.Placeholder = "Project name..."
	} else {
		m.filter.Placeholder = "Path (e.g. ~/projects/my-app)..."
		m.filter.SetValue("~/")
	}
	return m, nil
```

#### 6. Handle Enter in new project mode
**File**: `internal/ui/picker.go`
**Changes**: Modify the "enter" case to check newProjectMode first

```go
case "enter":
	if m.newProjectMode {
		path := m.filter.Value()
		if path != "" {
			m.result = Result{
				Action:      ActionNewProject,
				ProjectPath: m.expandPath(path),
			}
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	// ... existing ActionResume logic ...
```

#### 7. Handle Escape in new project mode
**File**: `internal/ui/picker.go`
**Changes**: Modify escape handler to exit new project mode first

```go
case "ctrl+c", "esc":
	if m.newProjectMode {
		m.newProjectMode = false
		m.filter.SetValue("")
		m.filter.Placeholder = "Filter..."
		return m, nil
	}
	m.quitting = true
	return m, tea.Quit
```

#### 8. Add path expansion helper
**File**: `internal/ui/picker.go`
**Changes**: Add method to expand ~ and handle projectsDir

```go
func (m *pickerModel) expandPath(input string) string {
	// If projectsDir is set, prepend it
	if m.projectsDir != "" {
		input = filepath.Join(m.projectsDir, input)
	}

	// Expand ~
	if strings.HasPrefix(input, "~/") {
		home, _ := os.UserHomeDir()
		input = filepath.Join(home, input[2:])
	}

	return input
}
```

#### 9. Update View for new project mode
**File**: `internal/ui/picker.go`
**Changes**: Show different header when in new project mode

```go
// In View() method, update header section:
if m.newProjectMode {
	header := "New Project"
	if m.projectsDir != "" {
		header += fmt.Sprintf(" (in %s)", m.projectsDir)
	}
	b.WriteString(fmt.Sprintf("%s %s\n\n", header, m.filter.View()))
} else {
	// existing header code
	b.WriteString(fmt.Sprintf("Sessions %d/%d%s %s\n\n", ...))
}
```

#### 10. Update help text
**File**: `internal/ui/picker.go`
**Changes**: Add Ctrl-N to help line (around line 284)

```go
// Update helpText to include Ctrl-N
helpText := "↑/↓ navigate • enter select • ctrl-d delete • ctrl-a toggle empty • ctrl-n new • esc quit"
```

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `go build ./...`
- [x] No linting errors

#### Manual Verification:
- [ ] `Ctrl-N` switches to new project mode
- [ ] Placeholder shows "Project name..." when projectsDir is set
- [ ] Placeholder shows "Path..." when projectsDir is not set
- [ ] `Esc` returns to normal picker mode
- [ ] `Enter` with valid path returns ActionNewProject
- [ ] Help text shows ctrl-n keybinding

---

## Phase 3: Main Orchestration

### Overview
Handle `ActionNewProject` in main.go to create directory, init git, and launch claude.

### Changes Required:

#### 1. Update runInteractive to pass projectsDir
**File**: `cmd/claude-fzf/main.go`
**Changes**: Pass config's ProjectsDir to SelectSession

```go
func runInteractive(showAll bool) {
	cfg = config.Load()
	sessions := loadAllSessions()

	result, err := ui.SelectSession(sessions, showAll, cfg.ProjectsDir)
	// ... rest unchanged ...
}
```

#### 2. Handle ActionNewProject
**File**: `cmd/claude-fzf/main.go`
**Changes**: Add handling after the picker returns

```go
func runInteractive(showAll bool) {
	cfg = config.Load()
	sessions := loadAllSessions()

	result, err := ui.SelectSession(sessions, showAll, cfg.ProjectsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch result.Action {
	case ui.ActionNewProject:
		createNewProject(result.ProjectPath)
	case ui.ActionResume:
		if result.Session != nil {
			resumeSession(result.Session)
		}
	}
	// ActionNone and ActionDelete don't need handling here
}
```

#### 3. Add createNewProject function
**File**: `cmd/claude-fzf/main.go`
**Changes**: Add new function

```go
func createNewProject(projectPath string) {
	// Create directory
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize git repo
	gitInit := exec.Command("git", "init")
	gitInit.Dir = projectPath
	if err := gitInit.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing git: %v\n", err)
		os.Exit(1)
	}

	// Launch claude
	if tmux.IsInsideTmux() {
		createProjectInTmux(projectPath)
	} else {
		createProjectDirectly(projectPath)
	}
}

func createProjectDirectly(projectPath string) {
	os.Chdir(projectPath)
	cmd := exec.Command("claude")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func createProjectInTmux(projectPath string) {
	mgr, err := tmux.New()
	if err != nil {
		createProjectDirectly(projectPath)
		return
	}

	sessionName := tmux.ProjectToSessionName(projectPath)

	// Check if we can repurpose the current session
	if disposable, _ := mgr.IsDisposableSession(); disposable {
		if err := mgr.RepurposeCurrentSession(sessionName, projectPath, cfg.Tmux.Windows); err != nil {
			fmt.Fprintf(os.Stderr, "Error repurposing session: %v\n", err)
			os.Exit(1)
		}
		// Respawn claude window with fresh claude (no --resume)
		wrappedCmd := fmt.Sprintf("cd %q && claude; exec $SHELL", projectPath)
		if err := mgr.RespawnWindow(sessionName, "claude", wrappedCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error respawning window: %v\n", err)
			os.Exit(1)
		}
		if err := mgr.SelectWindow(sessionName, "claude"); err != nil {
			fmt.Fprintf(os.Stderr, "Error selecting window: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Create a new tmux session
	if err := mgr.CreateProjectSession(sessionName, projectPath, "", cfg.Tmux.Windows); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating tmux session: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.SwitchToSession(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "Error switching to session: %v\n", err)
		os.Exit(1)
	}

	// Run claude in the claude window
	if err := mgr.RespawnWindow(sessionName, "claude", "claude"); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting claude: %v\n", err)
		os.Exit(1)
	}
}
```

#### 4. Update help text
**File**: `cmd/claude-fzf/main.go`
**Changes**: Add Ctrl-N to keybindings section

```go
Keybindings (in picker):
  Enter         Resume selected session
  Ctrl-D        Delete selected session (with confirmation)
  Ctrl-A        Toggle showing empty sessions
  Ctrl-N        Create new project
  Ctrl-C/Esc    Cancel
```

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `go build ./...`
- [x] `go test ./...` passes

#### Manual Verification:
- [ ] `Ctrl-N` → enter path → creates directory
- [ ] Git repo initialized in new directory
- [ ] Tmux session created with configured windows (when in tmux)
- [ ] Claude starts in new directory
- [ ] Works without tmux (just mkdir, git init, claude)

---

## Phase 4: Polish

### Overview
Edge case handling and UX improvements.

### Changes Required:

#### 1. Validate path doesn't already exist
**File**: `cmd/claude-fzf/main.go`
**Changes**: Check before creating

```go
func createNewProject(projectPath string) {
	// Check if path already exists
	if _, err := os.Stat(projectPath); err == nil {
		fmt.Fprintf(os.Stderr, "Error: %s already exists\n", projectPath)
		os.Exit(1)
	}
	// ... rest of function
}
```

#### 2. Handle empty input gracefully
Already handled in Phase 2 - Enter with empty path does nothing.

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `go build ./...`

#### Manual Verification:
- [ ] Creating project at existing path shows error
- [ ] Empty path entry doesn't crash

---

## Testing Strategy

### Manual Testing Steps:
1. Run `claude-fzf` without config, press `Ctrl-N`, verify placeholder says "Path..."
2. Type `~/test-project-1`, press Enter, verify directory created with git
3. Add `projects_dir: ~/projects` to config, restart
4. Press `Ctrl-N`, verify placeholder says "Project name..."
5. Type `test-project-2`, verify created at `~/projects/test-project-2`
6. Test inside tmux: verify session created with windows
7. Test outside tmux: verify claude starts directly
8. Test `Esc` exits new project mode without action
9. Test creating at existing path shows error

## References

- `internal/ui/picker.go` - Main picker implementation
- `internal/config/config.go` - Config loading
- `cmd/claude-fzf/main.go` - Orchestration
- `internal/tmux/tmux.go` - Tmux session management
