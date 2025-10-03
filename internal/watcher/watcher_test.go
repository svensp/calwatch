package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFSNotifyWatcher_WatchDirectory(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calwatch_watcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create watcher
	watcher, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Set up callback to capture events
	var events []FileChangeEvent
	var eventsMutex sync.Mutex
	callback := func(event FileChangeEvent) {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		events = append(events, event)
	}

	// Start watching directory
	err = watcher.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Verify directory is being watched
	if !watcher.IsWatching(tempDir) {
		t.Error("Directory should be watched")
	}

	// Create a test file
	testFile := filepath.Join(tempDir, "test.ics")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.WriteString("BEGIN:VCALENDAR\nEND:VCALENDAR")
	file.Close()

	// Wait a bit for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// Check if we received the create event
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	if len(events) == 0 {
		t.Error("Expected at least one file event")
	}

	// Find the create event for our test file
	found := false
	for _, event := range events {
		if event.Path == testFile && event.Operation == FileCreated {
			found = true
			break
		}
	}

	if !found {
		t.Error("Did not receive create event for test file")
	}
}

func TestFSNotifyWatcher_WatchFile(t *testing.T) {
	// Create temporary directory and file for testing
	tempDir, err := os.MkdirTemp("", "calwatch_watcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.ics")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.WriteString("BEGIN:VCALENDAR\nEND:VCALENDAR")
	file.Close()

	// Create watcher
	watcher, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Set up callback to capture events
	var events []FileChangeEvent
	var eventsMutex sync.Mutex
	callback := func(event FileChangeEvent) {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		events = append(events, event)
	}

	// Start watching file
	err = watcher.WatchFile(testFile, callback)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// Verify file is being watched
	if !watcher.IsWatching(testFile) {
		t.Error("File should be watched")
	}

	// Modify the file
	file, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open test file for modification: %v", err)
	}
	file.WriteString("\n# Modified")
	file.Close()

	// Wait a bit for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// Check if we received the modify event
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	if len(events) == 0 {
		t.Error("Expected at least one file event")
	}

	// Find the modify event for our test file
	found := false
	for _, event := range events {
		if event.Path == testFile && event.Operation == FileModified {
			found = true
			break
		}
	}

	if !found {
		t.Error("Did not receive modify event for test file")
	}
}

func TestFSNotifyWatcher_NonexistentPath(t *testing.T) {
	watcher, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	callback := func(event FileChangeEvent) {}

	// Try to watch nonexistent directory
	err = watcher.WatchDirectory("/nonexistent/path", callback)
	if err == nil {
		t.Error("Expected error when watching nonexistent directory")
	}

	// Try to watch nonexistent file
	err = watcher.WatchFile("/nonexistent/file.ics", callback)
	if err == nil {
		t.Error("Expected error when watching nonexistent file")
	}
}

func TestFSNotifyWatcher_Stop(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calwatch_watcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	watcher, err := NewFSNotifyWatcher()
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	callback := func(event FileChangeEvent) {}

	// Start watching
	err = watcher.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Verify watching
	if !watcher.IsWatching(tempDir) {
		t.Error("Directory should be watched")
	}

	// Stop watcher
	err = watcher.Stop()
	if err != nil {
		t.Fatalf("Failed to stop watcher: %v", err)
	}

	// Verify not watching anymore
	if watcher.IsWatching(tempDir) {
		t.Error("Directory should not be watched after stop")
	}

	// Verify GetWatchedPaths returns empty
	paths := watcher.GetWatchedPaths()
	if len(paths) != 0 {
		t.Errorf("Expected 0 watched paths after stop, got %d", len(paths))
	}
}

func TestCalDAVWatcher(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "calwatch_caldav_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up callback to capture events
	var events []FileChangeEvent
	var eventsMutex sync.Mutex
	callback := func(event FileChangeEvent) {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		events = append(events, event)
	}

	// Create CalDAV watcher
	watcher, err := NewCalDAVWatcher(callback)
	if err != nil {
		t.Fatalf("Failed to create CalDAV watcher: %v", err)
	}
	defer watcher.Stop()

	// Add directory to watch
	err = watcher.AddDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to add directory to CalDAV watcher: %v", err)
	}

	// Verify directory is in watched list
	watchedDirs := watcher.GetWatchedDirectories()
	if len(watchedDirs) != 1 || watchedDirs[0] != tempDir {
		t.Errorf("Expected watched directories [%s], got %v", tempDir, watchedDirs)
	}

	// Create ICS file (should trigger event)
	icsFile := filepath.Join(tempDir, "calendar.ics")
	file, err := os.Create(icsFile)
	if err != nil {
		t.Fatalf("Failed to create ICS file: %v", err)
	}
	file.WriteString("BEGIN:VCALENDAR\nEND:VCALENDAR")
	file.Close()

	// Create non-ICS file (should not trigger event)
	txtFile := filepath.Join(tempDir, "notes.txt")
	file, err = os.Create(txtFile)
	if err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}
	file.WriteString("Some notes")
	file.Close()

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Check events
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	// Should only receive event for .ics file
	icsEventFound := false
	txtEventFound := false

	for _, event := range events {
		if event.Path == icsFile {
			icsEventFound = true
		}
		if event.Path == txtFile {
			txtEventFound = true
		}
	}

	if !icsEventFound {
		t.Error("Expected event for .ics file")
	}

	if txtEventFound {
		t.Error("Should not receive event for non-.ics file")
	}
}

func TestFileOperation_String(t *testing.T) {
	tests := []struct {
		op       FileOperation
		expected string
	}{
		{FileCreated, "created"},
		{FileModified, "modified"},
		{FileDeleted, "deleted"},
		{FileRenamed, "renamed"},
		{FileOperation(999), "unknown"},
	}

	for _, test := range tests {
		if got := test.op.String(); got != test.expected {
			t.Errorf("FileOperation(%d).String() = %s, want %s", test.op, got, test.expected)
		}
	}
}