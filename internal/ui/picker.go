package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jh3/claude-fzf/internal/session"
)

// Action represents what the user wants to do with the selected session
type Action int

const (
	ActionNone Action = iota
	ActionResume
	ActionDelete
	ActionNewProject
)

// Result holds the selected session and action
type Result struct {
	Session     *session.Session
	Action      Action
	ProjectPath string // for ActionNewProject
}

// ProjectGroup holds sessions grouped by project path
type ProjectGroup struct {
	ProjectPath string
	ProjectName string
	Sessions    []session.Session
	LatestMod   string // formatted date of most recent session
}

// pickerModel is the bubbletea model for the session picker
type pickerModel struct {
	// Data
	allSessions []session.Session
	projects    []ProjectGroup

	// View state
	mode             string // "projects", "sessions", "newproject"
	projectCursor    int
	sessionCursor    int
	selectedProject  *ProjectGroup
	filter           textinput.Model
	showEmpty        bool

	// Filtered views
	filteredProjects []ProjectGroup
	filteredSessions []session.Session

	// Layout
	width  int
	height int

	// Actions
	result        Result
	confirmDelete bool
	quitting      bool

	// New project mode
	projectsDir  string
	existingDirs []string
}

// Styles
var (
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("229"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	previewHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	confirmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func groupSessionsByProject(sessions []session.Session) []ProjectGroup {
	groups := make(map[string]*ProjectGroup)

	for _, s := range sessions {
		path := s.ProjectPath
		if path == "" {
			path = "(no project)"
		}

		if g, ok := groups[path]; ok {
			g.Sessions = append(g.Sessions, s)
		} else {
			name := filepath.Base(path)
			if name == "" || name == "." {
				name = "(no project)"
			}
			groups[path] = &ProjectGroup{
				ProjectPath: path,
				ProjectName: name,
				Sessions:    []session.Session{s},
			}
		}
	}

	// Convert to slice and sort each group's sessions by ModTime
	var result []ProjectGroup
	for _, g := range groups {
		sort.Slice(g.Sessions, func(i, j int) bool {
			return g.Sessions[i].ModTime.After(g.Sessions[j].ModTime)
		})
		g.LatestMod = g.Sessions[0].ModTime.Format("01/02 15:04")
		result = append(result, *g)
	}

	// Sort groups by most recent session
	sort.Slice(result, func(i, j int) bool {
		return result[i].Sessions[0].ModTime.After(result[j].Sessions[0].ModTime)
	})

	return result
}

func newPickerModel(sessions []session.Session, showEmpty bool, projectsDir string) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	m := pickerModel{
		allSessions: sessions,
		filter:      ti,
		showEmpty:   showEmpty,
		mode:        "projects",
		width:       80,
		height:      24,
		projectsDir: projectsDir,
	}
	m.rebuildProjects()
	m.applyProjectFilter()
	return m
}

func (m *pickerModel) rebuildProjects() {
	// Filter sessions by showEmpty first
	var filtered []session.Session
	for _, s := range m.allSessions {
		if !m.showEmpty && s.UserMsgCount == 0 && s.AsstMsgCount == 0 {
			continue
		}
		filtered = append(filtered, s)
	}
	m.projects = groupSessionsByProject(filtered)
}

func (m *pickerModel) applyProjectFilter() {
	query := strings.ToLower(m.filter.Value())
	m.filteredProjects = nil

	for _, p := range m.projects {
		if query != "" {
			searchText := strings.ToLower(p.ProjectPath + " " + p.ProjectName)
			// Also search session summaries and branches
			for _, s := range p.Sessions {
				searchText += " " + strings.ToLower(s.Summary+" "+s.GitBranch)
			}
			if !strings.Contains(searchText, query) {
				continue
			}
		}
		m.filteredProjects = append(m.filteredProjects, p)
	}

	if m.projectCursor >= len(m.filteredProjects) {
		m.projectCursor = max(0, len(m.filteredProjects)-1)
	}
}

func (m *pickerModel) applySessionFilter() {
	if m.selectedProject == nil {
		return
	}

	query := strings.ToLower(m.filter.Value())
	m.filteredSessions = nil

	for _, s := range m.selectedProject.Sessions {
		if query != "" {
			searchText := strings.ToLower(s.Summary + " " + s.GitBranch)
			if !strings.Contains(searchText, query) {
				continue
			}
		}
		m.filteredSessions = append(m.filteredSessions, s)
	}

	if m.sessionCursor >= len(m.filteredSessions) {
		m.sessionCursor = max(0, len(m.filteredSessions)-1)
	}
}

func (m pickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle delete confirmation mode
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				if m.mode == "sessions" && len(m.filteredSessions) > 0 {
					sess := m.filteredSessions[m.sessionCursor]
					os.Remove(sess.FilePath)

					// Remove from allSessions
					for i, s := range m.allSessions {
						if s.FilePath == sess.FilePath {
							m.allSessions = append(m.allSessions[:i], m.allSessions[i+1:]...)
							break
						}
					}

					// Rebuild and refilter
					m.rebuildProjects()

					// Update selected project reference
					for i := range m.filteredProjects {
						if m.filteredProjects[i].ProjectPath == m.selectedProject.ProjectPath {
							m.selectedProject = &m.filteredProjects[i]
							break
						}
					}

					// If project has no more sessions, go back to project view
					if m.selectedProject == nil || len(m.selectedProject.Sessions) == 0 {
						m.mode = "projects"
						m.selectedProject = nil
						m.applyProjectFilter()
					} else {
						m.applySessionFilter()
					}
				}
				m.confirmDelete = false
				return m, nil
			case "n", "N", "esc":
				m.confirmDelete = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.mode == "newproject" {
				m.mode = "projects"
				m.filter.SetValue("")
				m.filter.Placeholder = "Filter..."
				return m, nil
			}
			if m.mode == "sessions" {
				m.mode = "projects"
				m.selectedProject = nil
				m.filter.SetValue("")
				m.applyProjectFilter()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.mode == "newproject" {
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
			if m.mode == "projects" && len(m.filteredProjects) > 0 {
				// Quick resume: resume most recent session in project
				p := m.filteredProjects[m.projectCursor]
				m.result = Result{
					Session: &p.Sessions[0],
					Action:  ActionResume,
				}
				m.quitting = true
				return m, tea.Quit
			}
			if m.mode == "sessions" && len(m.filteredSessions) > 0 {
				m.result = Result{
					Session: &m.filteredSessions[m.sessionCursor],
					Action:  ActionResume,
				}
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil

		case "tab":
			if m.mode == "projects" && len(m.filteredProjects) > 0 {
				// Expand into project's sessions
				m.selectedProject = &m.filteredProjects[m.projectCursor]
				m.filteredSessions = m.selectedProject.Sessions
				m.sessionCursor = 0
				m.mode = "sessions"
				m.filter.SetValue("")
				return m, nil
			}
			return m, nil

		case "ctrl+n":
			m.mode = "newproject"
			m.filter.SetValue("")
			m.loadExistingDirs()
			if m.projectsDir != "" {
				m.filter.Placeholder = "Project name..."
			} else {
				m.filter.Placeholder = "Path (e.g. ~/projects/my-app)..."
				m.filter.SetValue("~/")
			}
			return m, nil

		case "ctrl+d":
			if m.mode == "sessions" && len(m.filteredSessions) > 0 {
				m.confirmDelete = true
			}
			return m, nil

		case "ctrl+a":
			m.showEmpty = !m.showEmpty
			m.rebuildProjects()
			if m.mode == "projects" {
				m.applyProjectFilter()
			} else if m.mode == "sessions" {
				// Update selected project reference
				for i := range m.projects {
					if m.projects[i].ProjectPath == m.selectedProject.ProjectPath {
						m.selectedProject = &m.projects[i]
						break
					}
				}
				m.applySessionFilter()
			}
			return m, nil

		case "up", "ctrl+p":
			if m.mode == "projects" && m.projectCursor > 0 {
				m.projectCursor--
			} else if m.mode == "sessions" && m.sessionCursor > 0 {
				m.sessionCursor--
			}
			return m, nil

		case "down":
			if m.mode == "projects" && m.projectCursor < len(m.filteredProjects)-1 {
				m.projectCursor++
			} else if m.mode == "sessions" && m.sessionCursor < len(m.filteredSessions)-1 {
				m.sessionCursor++
			}
			return m, nil

		case "pgup":
			if m.mode == "projects" {
				m.projectCursor = max(0, m.projectCursor-10)
			} else if m.mode == "sessions" {
				m.sessionCursor = max(0, m.sessionCursor-10)
			}
			return m, nil

		case "pgdown":
			if m.mode == "projects" {
				m.projectCursor = min(len(m.filteredProjects)-1, m.projectCursor+10)
			} else if m.mode == "sessions" {
				m.sessionCursor = min(len(m.filteredSessions)-1, m.sessionCursor+10)
			}
			return m, nil

		case "home", "ctrl+home":
			if m.mode == "projects" {
				m.projectCursor = 0
			} else if m.mode == "sessions" {
				m.sessionCursor = 0
			}
			return m, nil

		case "end", "ctrl+end":
			if m.mode == "projects" {
				m.projectCursor = max(0, len(m.filteredProjects)-1)
			} else if m.mode == "sessions" {
				m.sessionCursor = max(0, len(m.filteredSessions)-1)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filter.Width = min(40, msg.Width-20)
		return m, nil
	}

	// Handle text input for filtering
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.mode == "projects" {
		m.applyProjectFilter()
	} else if m.mode == "sessions" {
		m.applySessionFilter()
	}
	return m, cmd
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	switch m.mode {
	case "newproject":
		header := "New Project"
		if m.projectsDir != "" {
			header += fmt.Sprintf(" (in %s)", m.projectsDir)
		}
		b.WriteString(fmt.Sprintf("%s %s\n\n", header, m.filter.View()))
	case "sessions":
		emptyIndicator := ""
		if m.showEmpty {
			emptyIndicator = " [+empty]"
		}
		b.WriteString(fmt.Sprintf("← %s (%d sessions)%s %s\n\n",
			m.selectedProject.ProjectName,
			len(m.filteredSessions),
			emptyIndicator,
			m.filter.View()))
	default: // projects
		emptyIndicator := ""
		if m.showEmpty {
			emptyIndicator = " [+empty]"
		}
		b.WriteString(fmt.Sprintf("Projects %d%s %s\n\n",
			len(m.filteredProjects), emptyIndicator, m.filter.View()))
	}

	// Calculate layout
	listWidth := m.width / 2
	previewWidth := m.width - listWidth - 3
	listHeight := m.height - 6

	var listLines []string
	var previewLines []string

	switch m.mode {
	case "newproject":
		listLines, previewLines = m.renderNewProjectMode(listWidth, previewWidth, listHeight)
	case "sessions":
		listLines, previewLines = m.renderSessionsMode(listWidth, previewWidth, listHeight)
	default:
		listLines, previewLines = m.renderProjectsMode(listWidth, previewWidth, listHeight)
	}

	// Pad to full height
	emptyLine := strings.Repeat(" ", listWidth)
	for len(listLines) < listHeight {
		listLines = append(listLines, emptyLine)
	}
	for len(previewLines) < listHeight {
		previewLines = append(previewLines, "")
	}

	// Combine list and preview side by side
	for i := 0; i < listHeight; i++ {
		b.WriteString(fmt.Sprintf("%s │ %s\n", listLines[i], previewLines[i]))
	}

	// Footer
	b.WriteString("\n")
	if m.confirmDelete {
		b.WriteString(confirmStyle.Render("Delete this session? (y/n)"))
	} else {
		switch m.mode {
		case "newproject":
			b.WriteString(helpStyle.Render("enter: create • esc: cancel"))
		case "sessions":
			b.WriteString(helpStyle.Render("enter: resume • ctrl-d: delete • ctrl-a: toggle empty • esc: back"))
		default:
			b.WriteString(helpStyle.Render("enter: resume • tab: expand • ctrl-a: toggle empty • ctrl-n: new • esc: quit"))
		}
	}

	return b.String()
}

func (m *pickerModel) renderProjectsMode(listWidth, previewWidth, listHeight int) ([]string, []string) {
	var listLines []string
	contentWidth := listWidth - 2

	visibleStart := 0
	if m.projectCursor >= listHeight {
		visibleStart = m.projectCursor - listHeight + 1
	}

	for i := visibleStart; i < len(m.filteredProjects) && i < visibleStart+listHeight; i++ {
		p := m.filteredProjects[i]
		line := formatProjectLine(p, contentWidth)
		line = fixedWidth(line, contentWidth)

		if i == m.projectCursor {
			line = cursorStyle.Render("> ") + selectedStyle.Render(line)
		} else {
			line = "  " + line
		}
		listLines = append(listLines, line)
	}

	// Preview
	var previewLines []string
	if len(m.filteredProjects) > 0 && m.projectCursor < len(m.filteredProjects) {
		previewLines = formatProjectPreview(m.filteredProjects[m.projectCursor], previewWidth)
	}

	return listLines, previewLines
}

func (m *pickerModel) renderSessionsMode(listWidth, previewWidth, listHeight int) ([]string, []string) {
	var listLines []string
	contentWidth := listWidth - 2

	visibleStart := 0
	if m.sessionCursor >= listHeight {
		visibleStart = m.sessionCursor - listHeight + 1
	}

	for i := visibleStart; i < len(m.filteredSessions) && i < visibleStart+listHeight; i++ {
		s := m.filteredSessions[i]
		line := formatSessionLine(s, contentWidth)
		line = fixedWidth(line, contentWidth)

		if i == m.sessionCursor {
			line = cursorStyle.Render("> ") + selectedStyle.Render(line)
		} else {
			line = "  " + line
		}
		listLines = append(listLines, line)
	}

	// Preview
	var previewLines []string
	if len(m.filteredSessions) > 0 && m.sessionCursor < len(m.filteredSessions) {
		previewLines = formatSessionPreview(m.filteredSessions[m.sessionCursor], previewWidth)
	}

	return listLines, previewLines
}

func (m *pickerModel) renderNewProjectMode(listWidth, previewWidth, listHeight int) ([]string, []string) {
	var listLines []string
	contentWidth := listWidth - 2

	for i := 0; i < len(m.existingDirs) && i < listHeight; i++ {
		line := fixedWidth("  "+m.existingDirs[i], contentWidth+2)
		listLines = append(listLines, line)
	}

	previewLines := m.formatNewProjectPreview(previewWidth)
	return listLines, previewLines
}

func formatProjectLine(p ProjectGroup, maxWidth int) string {
	sessionCount := len(p.Sessions)
	countStr := fmt.Sprintf("%d", sessionCount)
	if sessionCount == 1 {
		countStr = "1"
	}

	// Format: "project-name        3   01/15 14:23"
	line := fmt.Sprintf("%-20s %3s   %s", truncate(p.ProjectName, 20), countStr, p.LatestMod)
	if len(line) > maxWidth {
		line = line[:maxWidth-1] + "…"
	}
	return line
}

func formatProjectPreview(p ProjectGroup, width int) []string {
	var lines []string

	lines = append(lines, previewHeader.Render("Project: ")+p.ProjectName)
	lines = append(lines, previewHeader.Render("Path: ")+p.ProjectPath)
	lines = append(lines, "")
	lines = append(lines, previewHeader.Render("Recent Sessions:"))

	// Show up to 5 recent sessions
	for i, s := range p.Sessions {
		if i >= 5 {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("  ... and %d more", len(p.Sessions)-5)))
			break
		}
		branch := s.GitBranch
		if branch == "" {
			branch = "-"
		}
		summary := s.Summary
		if summary == "" {
			summary = "(no summary)"
		}
		// Truncate summary to fit
		maxSummary := width - 25
		if len(summary) > maxSummary {
			summary = summary[:maxSummary-1] + "…"
		}
		line := fmt.Sprintf("  %s  %-12s  %s", s.ModTime.Format("01/02 15:04"), truncate(branch, 12), summary)
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("Enter: resume latest • Tab: see all sessions"))

	return lines
}

func formatSessionLine(s session.Session, maxWidth int) string {
	branch := s.GitBranch
	if branch == "" {
		branch = "-"
	}

	date := s.ModTime.Format("01/02 15:04")
	summary := s.Summary
	if summary == "" {
		summary = "(no summary)"
	}

	line := fmt.Sprintf("%s  %-14s  %s", date, truncate(branch, 14), summary)
	if len(line) > maxWidth {
		line = line[:maxWidth-1] + "…"
	}
	return line
}

func formatSessionPreview(s session.Session, width int) []string {
	var lines []string

	lines = append(lines, previewHeader.Render("Session: ")+s.ID)
	lines = append(lines, previewHeader.Render("Project: ")+s.ProjectPath)
	lines = append(lines, "")

	if s.Summary != "" && s.Summary != "(no summary)" {
		lines = append(lines, previewHeader.Render("Summary:"))
		wrapped := wordWrap(s.Summary, width)
		lines = append(lines, wrapped...)
		lines = append(lines, "")
	}

	if s.GitBranch != "" {
		lines = append(lines, previewHeader.Render("Branch: ")+s.GitBranch)
	}

	lines = append(lines, fmt.Sprintf("Messages: %d user / %d assistant", s.UserMsgCount, s.AsstMsgCount))
	lines = append(lines, dimStyle.Render("Modified: "+s.ModTime.Format("2006-01-02 15:04:05")))

	return lines
}

func (m *pickerModel) formatNewProjectPreview(width int) []string {
	var lines []string

	input := m.filter.Value()
	if input == "" {
		lines = append(lines, dimStyle.Render("Enter a project name..."))
		return lines
	}

	fullPath := m.expandPath(input)

	lines = append(lines, previewHeader.Render("Will create:"))
	lines = append(lines, "")
	lines = append(lines, "  "+fullPath)
	lines = append(lines, "")
	lines = append(lines, previewHeader.Render("Actions:"))
	lines = append(lines, "  • Create directory")
	lines = append(lines, "  • Initialize git repo")
	lines = append(lines, "  • Start Claude session")

	if _, err := os.Stat(fullPath); err == nil {
		lines = append(lines, "")
		lines = append(lines, confirmStyle.Render("Warning: Path already exists!"))
	}

	return lines
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func fixedWidth(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width-1]) + "…"
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

func wordWrap(s string, width int) []string {
	var lines []string
	words := strings.Fields(s)
	var line string

	for _, word := range words {
		if line == "" {
			line = word
		} else if len(line)+1+len(word) <= width {
			line += " " + word
		} else {
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (m *pickerModel) expandPath(input string) string {
	if m.projectsDir != "" {
		input = filepath.Join(m.projectsDir, input)
	}

	if strings.HasPrefix(input, "~/") {
		home, _ := os.UserHomeDir()
		input = filepath.Join(home, input[2:])
	}

	return input
}

func (m *pickerModel) loadExistingDirs() {
	m.existingDirs = nil

	dir := m.projectsDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = home
	}

	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			m.existingDirs = append(m.existingDirs, entry.Name())
		}
	}
}

// SelectSession runs the interactive picker and returns the result
func SelectSession(sessions []session.Session, showEmpty bool, projectsDir string) (Result, error) {
	if len(sessions) == 0 {
		return Result{}, fmt.Errorf("no sessions found")
	}

	m := newPickerModel(sessions, showEmpty, projectsDir)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	result := finalModel.(pickerModel).result
	return result, nil
}
