package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileOperation represents the type of file system operation
type FileOperation int

const (
	FileCreated FileOperation = iota
	FileModified
	FileDeleted
	FileRenamed
)

// String returns a string representation of the file operation
func (op FileOperation) String() string {
	switch op {
	case FileCreated:
		return "created"
	case FileModified:
		return "modified"
	case FileDeleted:
		return "deleted"
	case FileRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FileChangeEvent represents a file system change event
type FileChangeEvent struct {
	Path      string
	Operation FileOperation
	IsDir     bool
}

// FileChangeCallback is called when a file system change occurs
type FileChangeCallback func(event FileChangeEvent)

// FileWatcher monitors file system changes
type FileWatcher interface {
	WatchDirectory(path string, callback FileChangeCallback) error
	WatchFile(path string, callback FileChangeCallback) error
	Stop() error
	IsWatching(path string) bool
	GetWatchedPaths() []string
}

// FSNotifyWatcher implements FileWatcher using fsnotify
type FSNotifyWatcher struct {
	watcher   *fsnotify.Watcher
	callbacks map[string]FileChangeCallback
	watching  map[string]bool
	mutex     sync.RWMutex
	stopChan  chan struct{}
	stopped   bool
}

// NewFSNotifyWatcher creates a new file watcher using fsnotify
func NewFSNotifyWatcher() (*FSNotifyWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	fw := &FSNotifyWatcher{
		watcher:   watcher,
		callbacks: make(map[string]FileChangeCallback),
		watching:  make(map[string]bool),
		stopChan:  make(chan struct{}),
		stopped:   false,
	}

	// Start the event processing goroutine
	go fw.processEvents()

	return fw, nil
}

// WatchDirectory adds a directory to the watch list
func (fw *FSNotifyWatcher) WatchDirectory(path string, callback FileChangeCallback) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if fw.stopped {
		return fmt.Errorf("watcher is stopped")
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// Check if directory exists
	if info, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("directory %s does not exist: %w", absPath, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", absPath)
	}

	// Add to fsnotify watcher
	if err := fw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", absPath, err)
	}

	// Store callback and mark as watching
	fw.callbacks[absPath] = callback
	fw.watching[absPath] = true

	return nil
}

// WatchFile adds a single file to the watch list
func (fw *FSNotifyWatcher) WatchFile(path string, callback FileChangeCallback) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if fw.stopped {
		return fmt.Errorf("watcher is stopped")
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// Check if file exists
	if info, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file %s does not exist: %w", absPath, err)
	} else if info.IsDir() {
		return fmt.Errorf("%s is a directory, use WatchDirectory instead", absPath)
	}

	// Add to fsnotify watcher
	if err := fw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch file %s: %w", absPath, err)
	}

	// Store callback and mark as watching
	fw.callbacks[absPath] = callback
	fw.watching[absPath] = true

	return nil
}

// Stop stops the file watcher and cleans up resources
func (fw *FSNotifyWatcher) Stop() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if fw.stopped {
		return nil
	}

	fw.stopped = true
	close(fw.stopChan)

	// Close the fsnotify watcher
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close fsnotify watcher: %w", err)
	}

	// Clear callbacks and watching maps
	fw.callbacks = make(map[string]FileChangeCallback)
	fw.watching = make(map[string]bool)

	return nil
}

// IsWatching checks if a path is being watched
func (fw *FSNotifyWatcher) IsWatching(path string) bool {
	fw.mutex.RLock()
	defer fw.mutex.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return fw.watching[absPath]
}

// GetWatchedPaths returns a list of all watched paths
func (fw *FSNotifyWatcher) GetWatchedPaths() []string {
	fw.mutex.RLock()
	defer fw.mutex.RUnlock()

	paths := make([]string, 0, len(fw.watching))
	for path, watching := range fw.watching {
		if watching {
			paths = append(paths, path)
		}
	}

	return paths
}

// processEvents processes file system events from fsnotify
func (fw *FSNotifyWatcher) processEvents() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "File watcher error: %v\n", err)

		case <-fw.stopChan:
			return
		}
	}
}

// handleEvent processes a single fsnotify event
func (fw *FSNotifyWatcher) handleEvent(event fsnotify.Event) {
	fw.mutex.RLock()
	
	// Find the callback for this event
	var callback FileChangeCallback
	var found bool

	// Check exact path match first
	if cb, exists := fw.callbacks[event.Name]; exists {
		callback = cb
		found = true
	} else {
		// Check if this file is in a watched directory
		dir := filepath.Dir(event.Name)
		if cb, exists := fw.callbacks[dir]; exists {
			// Only handle .ics files in watched directories
			if strings.HasSuffix(strings.ToLower(event.Name), ".ics") {
				callback = cb
				found = true
			}
		}
	}
	
	fw.mutex.RUnlock()

	if !found {
		return
	}

	// Convert fsnotify event to our event type
	changeEvent := fw.convertEvent(event)
	
	// Call the callback
	callback(changeEvent)
}

// convertEvent converts an fsnotify.Event to our FileChangeEvent
func (fw *FSNotifyWatcher) convertEvent(event fsnotify.Event) FileChangeEvent {
	var operation FileOperation
	
	// Determine operation type
	if event.Has(fsnotify.Create) {
		operation = FileCreated
	} else if event.Has(fsnotify.Write) {
		operation = FileModified
	} else if event.Has(fsnotify.Remove) {
		operation = FileDeleted
	} else if event.Has(fsnotify.Rename) {
		operation = FileRenamed
	} else {
		operation = FileModified // Default to modified
	}

	// Check if it's a directory
	isDir := false
	if info, err := os.Stat(event.Name); err == nil {
		isDir = info.IsDir()
	}

	return FileChangeEvent{
		Path:      event.Name,
		Operation: operation,
		IsDir:     isDir,
	}
}

// CalDAVWatcher is a specialized watcher for CalDAV directories
type CalDAVWatcher struct {
	fileWatcher FileWatcher
	directories []string
	callback    FileChangeCallback
}

// NewCalDAVWatcher creates a new CalDAV-specific watcher
func NewCalDAVWatcher(callback FileChangeCallback) (*CalDAVWatcher, error) {
	fileWatcher, err := NewFSNotifyWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &CalDAVWatcher{
		fileWatcher: fileWatcher,
		directories: make([]string, 0),
		callback:    callback,
	}, nil
}

// AddDirectory adds a CalDAV directory to watch
func (cw *CalDAVWatcher) AddDirectory(path string) error {
	if err := cw.fileWatcher.WatchDirectory(path, cw.handleCalDAVEvent); err != nil {
		return fmt.Errorf("failed to watch CalDAV directory %s: %w", path, err)
	}

	cw.directories = append(cw.directories, path)
	return nil
}

// Stop stops the CalDAV watcher
func (cw *CalDAVWatcher) Stop() error {
	return cw.fileWatcher.Stop()
}

// GetWatchedDirectories returns the list of watched CalDAV directories
func (cw *CalDAVWatcher) GetWatchedDirectories() []string {
	return append([]string(nil), cw.directories...) // Return a copy
}

// handleCalDAVEvent handles file system events for CalDAV files
func (cw *CalDAVWatcher) handleCalDAVEvent(event FileChangeEvent) {
	// Only handle .ics files
	if !strings.HasSuffix(strings.ToLower(event.Path), ".ics") {
		return
	}

	// Forward the event to the main callback
	cw.callback(event)
}