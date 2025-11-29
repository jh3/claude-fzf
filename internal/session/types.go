package session

import "time"

// Session represents a Claude Code session
type Session struct {
	ID           string
	ProjectPath  string
	Summary      string
	ModTime      time.Time
	FilePath     string
	GitBranch    string
	UserMsgCount int
	AsstMsgCount int
}
