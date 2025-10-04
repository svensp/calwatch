package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// DaemonState represents the persistent state of the CalWatch daemon
type DaemonState struct {
	LastAlertTick time.Time `json:"last_alert_tick"`
	Version       string    `json:"version"`
	// Future: could add more persistent state like alert states per event
}

// StateManager handles persistent state operations
type StateManager interface {
	GetLastAlertTick() time.Time
	SetLastAlertTick(tick time.Time) error
	Load() error
	Save() error
}

// XDGStateManager implements StateManager using XDG state directory
type XDGStateManager struct {
	state    DaemonState
	filePath string
	mutex    sync.RWMutex
}

// NewXDGStateManager creates a new state manager using XDG state directory
func NewXDGStateManager() (*XDGStateManager, error) {
	// Use XDG state directory for persistent daemon state
	stateFilePath, err := xdg.StateFile("calwatch/state.json")
	if err != nil {
		return nil, fmt.Errorf("failed to get XDG state file path: %w", err)
	}

	return &XDGStateManager{
		state: DaemonState{
			Version: "0.1.0", // Current version for state format compatibility
		},
		filePath: stateFilePath,
	}, nil
}

// GetLastAlertTick returns the last recorded alert tick time
func (s *XDGStateManager) GetLastAlertTick() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.state.LastAlertTick
}

// SetLastAlertTick updates the last alert tick time and saves to disk
func (s *XDGStateManager) SetLastAlertTick(tick time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.state.LastAlertTick = tick
	return s.saveLocked()
}

// Load reads the state from disk
func (s *XDGStateManager) Load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Check if state file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// State file doesn't exist, use default state with current time
		s.state.LastAlertTick = time.Now()
		return s.saveLocked() // Create the file with default state
	}
	
	// Read the state file
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}
	
	// Parse JSON
	var loadedState DaemonState
	if err := json.Unmarshal(data, &loadedState); err != nil {
		// State file is corrupted, use current time as fallback
		s.state.LastAlertTick = time.Now()
		return s.saveLocked() // Overwrite corrupted file
	}
	
	// Validate loaded state
	if loadedState.LastAlertTick.IsZero() {
		// Invalid timestamp, use current time
		loadedState.LastAlertTick = time.Now()
	}
	
	s.state = loadedState
	return nil
}

// Save writes the current state to disk
func (s *XDGStateManager) Save() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	return s.saveLocked()
}

// saveLocked performs the actual save operation (must be called with lock held)
func (s *XDGStateManager) saveLocked() error {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	
	// Marshal to JSON with pretty formatting for easier debugging
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	// Atomic write: write to temporary file first, then rename
	tempFile := s.filePath + ".tmp"
	
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}
	
	if err := os.Rename(tempFile, s.filePath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename state file: %w", err)
	}
	
	return nil
}

// GetStateFilePath returns the path to the state file (useful for debugging)
func (s *XDGStateManager) GetStateFilePath() string {
	return s.filePath
}

// IsFirstRun checks if this is the first time the daemon is running (no state file exists)
func (s *XDGStateManager) IsFirstRun() bool {
	_, err := os.Stat(s.filePath)
	return os.IsNotExist(err)
}