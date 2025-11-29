package session

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const claudeProjectsDir = ".claude/projects"

// SessionCache is an interface for caching sessions
type SessionCache interface {
	Get(path string, mtime time.Time) (Session, bool)
	Set(path string, mtime time.Time, sess Session)
	Prune(validPaths map[string]bool)
}

// Scanner finds and parses Claude sessions
type Scanner struct {
	baseDir string
}

// NewScanner creates a scanner for the default Claude projects directory
func NewScanner() *Scanner {
	home, _ := os.UserHomeDir()
	return &Scanner{baseDir: filepath.Join(home, claudeProjectsDir)}
}

// ScanAll finds all sessions with parallel processing (no caching)
func (s *Scanner) ScanAll() ([]Session, error) {
	files, err := s.findSessionFiles()
	if err != nil {
		return nil, err
	}

	return s.parseFilesParallel(files, nil), nil
}

// ScanAllCached finds sessions using cache for unchanged files
func (s *Scanner) ScanAllCached(cache SessionCache) ([]Session, error) {
	files, err := s.findSessionFilesWithInfo()
	if err != nil {
		return nil, err
	}

	validPaths := make(map[string]bool, len(files))
	for _, f := range files {
		validPaths[f.path] = true
	}

	sessions := s.parseFilesWithCache(files, cache)
	cache.Prune(validPaths)
	return sessions, nil
}

type fileInfo struct {
	path    string
	modTime time.Time
}

func (s *Scanner) findSessionFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") || strings.HasPrefix(d.Name(), "agent-") {
			return nil
		}
		files = append(files, path)
		return nil
	})

	return files, err
}

func (s *Scanner) findSessionFilesWithInfo() ([]fileInfo, error) {
	var files []fileInfo

	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") || strings.HasPrefix(d.Name(), "agent-") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, fileInfo{path: path, modTime: info.ModTime()})
		return nil
	})

	return files, err
}

func (s *Scanner) parseFilesParallel(files []string, _ SessionCache) []Session {
	var wg sync.WaitGroup
	results := make(chan Session, len(files))

	for _, f := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			if sess, err := ParseFile(path); err == nil {
				results <- sess
			}
		}(f)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return collectResults(results)
}

func (s *Scanner) parseFilesWithCache(files []fileInfo, cache SessionCache) []Session {
	var wg sync.WaitGroup
	results := make(chan Session, len(files))

	for _, f := range files {
		// Check cache first
		if cached, ok := cache.Get(f.path, f.modTime); ok {
			results <- cached
			continue
		}

		// Parse in parallel
		wg.Add(1)
		go func(fi fileInfo) {
			defer wg.Done()
			sess, err := ParseFile(fi.path)
			if err != nil {
				return
			}
			cache.Set(fi.path, fi.modTime, sess)
			results <- sess
		}(f)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return collectResults(results)
}

func collectResults(results chan Session) []Session {
	var sessions []Session
	for sess := range results {
		sessions = append(sessions, sess)
	}
	return sessions
}
