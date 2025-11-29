package cache

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jh3/claude-fzf/internal/session"
)

const cacheFileName = "sessions.cache"

// Entry stores cached session data with its file mtime
type Entry struct {
	ModTime time.Time
	Session session.Session
}

// Cache manages session metadata caching
type Cache struct {
	path    string
	entries map[string]Entry
	mu      sync.RWMutex
}

// New creates or loads a cache
func New() *Cache {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cache", "claude-fzf")
	os.MkdirAll(cacheDir, 0755)

	c := &Cache{
		path:    filepath.Join(cacheDir, cacheFileName),
		entries: make(map[string]Entry),
	}
	c.load()
	return c
}

func (c *Cache) load() {
	f, err := os.Open(c.path)
	if err != nil {
		return
	}
	defer f.Close()
	gob.NewDecoder(f).Decode(&c.entries)
}

// Save persists the cache to disk
func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(c.entries)
}

// Get retrieves a cached session if mtime matches
func (c *Cache) Get(path string, mtime time.Time) (session.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[path]
	if !ok {
		return session.Session{}, false
	}
	if !entry.ModTime.Equal(mtime) {
		return session.Session{}, false
	}
	return entry.Session, true
}

// Set stores a session in the cache
func (c *Cache) Set(path string, mtime time.Time, sess session.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[path] = Entry{ModTime: mtime, Session: sess}
}

// Prune removes entries for files that no longer exist
func (c *Cache) Prune(validPaths map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for path := range c.entries {
		if !validPaths[path] {
			delete(c.entries, path)
		}
	}
}

// Clear removes the cache file
func (c *Cache) Clear() error {
	return os.Remove(c.path)
}
