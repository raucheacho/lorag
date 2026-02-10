package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/store"
)

// Watcher watches directories for file changes
type Watcher struct {
	store      *store.BoltStore
	indexer    *indexer.Indexer
	fsWatcher  *fsnotify.Watcher
	watches    map[string]*Watch
	mu         sync.RWMutex
	events     chan Event
	done       chan struct{}
}

// Watch represents a watched directory
type Watch struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Recursive bool      `json:"recursive"`
	Patterns  []string  `json:"patterns"`
	CreatedAt time.Time `json:"created_at"`
}

// Event represents a file system event
type Event struct {
	Type     string    `json:"type"`
	Path     string    `json:"path"`
	WatchID  string    `json:"watch_id"`
	Time     time.Time `json:"time"`
}

// NewWatcher creates a new file watcher
func NewWatcher(st *store.BoltStore, idx *indexer.Indexer) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	
	return &Watcher{
		store:     st,
		indexer:   idx,
		fsWatcher: fsWatcher,
		watches:   make(map[string]*Watch),
		events:    make(chan Event, 100),
		done:      make(chan struct{}),
	}, nil
}

// Start starts the watcher
func (w *Watcher) Start(ctx context.Context) error {
	// Load existing watches
	if err := w.loadWatches(); err != nil {
		return err
	}
	
	// Start event processor
	go w.processEvents(ctx)
	
	// Start fsnotify handler
	go w.handleFsEvents(ctx)
	
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// AddWatch adds a new directory watch
func (w *Watcher) AddWatch(path string, recursive bool, patterns []string) (*Watch, error) {
	// Validate path
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}
	
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	
	// Check if already watching
	w.mu.RLock()
	for _, watch := range w.watches {
		if watch.Path == absPath {
			w.mu.RUnlock()
			return nil, fmt.Errorf("already watching: %s", absPath)
		}
	}
	w.mu.RUnlock()
	
	// Create watch
	watch := &Watch{
		ID:        generateWatchID(),
		Path:      absPath,
		Recursive: recursive,
		Patterns:  patterns,
		CreatedAt: time.Now(),
	}
	
	// Add to filesystem watcher
	if err := w.addToFsWatcher(watch); err != nil {
		return nil, err
	}
	
	// Save to store
	w.mu.Lock()
	w.watches[watch.ID] = watch
	w.mu.Unlock()
	
	if err := w.saveWatch(watch); err != nil {
		// Remove from fs watcher on error
		w.removeFromFsWatcher(watch)
		return nil, err
	}
	
	// Trigger initial indexing
	go w.indexPath(absPath, recursive)
	
	return watch, nil
}

// RemoveWatch removes a watch
func (w *Watcher) RemoveWatch(watchID string) error {
	w.mu.Lock()
	watch, ok := w.watches[watchID]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("watch not found: %s", watchID)
	}
	delete(w.watches, watchID)
	w.mu.Unlock()
	
	// Remove from filesystem watcher
	w.removeFromFsWatcher(watch)
	
	// Remove from store
	return w.deleteWatch(watchID)
}

// ListWatches returns all active watches
func (w *Watcher) ListWatches() []*Watch {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	watches := make([]*Watch, 0, len(w.watches))
	for _, w := range w.watches {
		watches = append(watches, w)
	}
	return watches
}

// GetEvents returns the events channel
func (w *Watcher) GetEvents() <-chan Event {
	return w.events
}

// addToFsWatcher adds paths to fsnotify watcher
func (w *Watcher) addToFsWatcher(watch *Watch) error {
	if watch.Recursive {
		return filepath.Walk(watch.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return w.fsWatcher.Add(path)
			}
			return nil
		})
	}
	return w.fsWatcher.Add(watch.Path)
}

// removeFromFsWatcher removes paths from fsnotify watcher
func (w *Watcher) removeFromFsWatcher(watch *Watch) error {
	if watch.Recursive {
		return filepath.Walk(watch.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return w.fsWatcher.Remove(path)
			}
			return nil
		})
	}
	return w.fsWatcher.Remove(watch.Path)
}

// handleFsEvents handles fsnotify events
func (w *Watcher) handleFsEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("watcher error: %v\n", err)
		}
	}
}

// handleEvent processes a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Determine event type
	var eventType string
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = "create"
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = "modify"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = "remove"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = "rename"
	default:
		return
	}
	
	// Find matching watch
	w.mu.RLock()
	var watchID string
	for id, watch := range w.watches {
		if strings.HasPrefix(event.Name, watch.Path) {
			// Check patterns if specified
			if len(watch.Patterns) > 0 && !event.Op.Has(fsnotify.Remove) {
				if !w.matchesPatterns(event.Name, watch.Patterns) {
					continue
				}
			}
			watchID = id
			break
		}
	}
	w.mu.RUnlock()
	
	if watchID == "" {
		return
	}
	
	// Send event
	select {
	case w.events <- Event{
		Type:    eventType,
		Path:    event.Name,
		WatchID: watchID,
		Time:    time.Now(),
	}:
	default:
		// Channel full, drop event
	}
}

// processEvents processes events from the channel
func (w *Watcher) processEvents(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	pending := make(map[string]Event)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event := <-w.events:
			pending[event.Path] = event
		case <-ticker.C:
			for _, event := range pending {
				w.processPendingEvent(ctx, event)
			}
			pending = make(map[string]Event)
		}
	}
}

// processPendingEvent processes a pending event
func (w *Watcher) processPendingEvent(ctx context.Context, event Event) {
	switch event.Type {
	case "create", "modify":
		if w.isSupportedFile(event.Path) {
			if _, err := w.indexer.IndexFile(ctx, event.Path); err != nil {
				fmt.Printf("failed to index %s: %v\n", event.Path, err)
			}
		}
	case "remove":
		// Find and delete document
		doc, err := w.store.GetDocumentByPath(event.Path)
		if err == nil {
			if err := w.indexer.DeleteDocument(ctx, doc.ID); err != nil {
				fmt.Printf("failed to delete %s: %v\n", event.Path, err)
			}
		}
	}
}

// indexPath indexes all files in a path
func (w *Watcher) indexPath(path string, recursive bool) {
	ctx := context.Background()
	_, err := w.indexer.IndexDirectory(ctx, path, recursive)
	if err != nil {
		fmt.Printf("failed to index %s: %v\n", path, err)
	}
}

// matchesPatterns checks if a path matches any pattern
func (w *Watcher) matchesPatterns(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// isSupportedFile checks if file type is supported
func (w *Watcher) isSupportedFile(path string) bool {
	// Check if it's a regular file
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	supported := []string{
		".txt", ".md", ".markdown", ".pdf",
		".docx", ".doc",
		".go", ".py", ".js", ".ts", ".jsx", ".tsx",
		".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php",
		".html", ".htm", ".css",
		".json", ".yaml", ".yml", ".xml",
	}
	
	for _, s := range supported {
		if ext == s {
			return true
		}
	}
	return false
}

// loadWatches loads watches from store
func (w *Watcher) loadWatches() error {
	// TODO: Implement loading from store
	return nil
}

// saveWatch saves a watch to store
func (w *Watcher) saveWatch(watch *Watch) error {
	// TODO: Implement saving to store
	return nil
}

// deleteWatch removes a watch from store
func (w *Watcher) deleteWatch(watchID string) error {
	// TODO: Implement deleting from store
	return nil
}

// generateWatchID generates a unique watch ID
func generateWatchID() string {
	return fmt.Sprintf("watch_%d", time.Now().UnixNano())
}
