package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"calwatch/internal/alerts"
	"calwatch/internal/config"
	"calwatch/internal/notifications"
	"calwatch/internal/parser"
	"calwatch/internal/storage"
	"calwatch/internal/watcher"
)

// CalWatch represents the main application
type CalWatch struct {
	config             *config.Config
	eventStorage       storage.EventStorage
	stateManager       storage.StateManager
	parser             parser.CalDAVParser
	watcher            *watcher.CalDAVWatcher
	alertManager       *alerts.AlertManager
	notificationManager *notifications.NotificationManager
	alertScheduler     alerts.AlertScheduler
	
	// Synchronization
	stopChan   chan struct{}
	wg         sync.WaitGroup
	isRunning  bool
}

// NewCalWatch creates a new CalWatch instance
func NewCalWatch() *CalWatch {
	return &CalWatch{
		stopChan: make(chan struct{}),
	}
}

// Initialize sets up all components
func (cw *CalWatch) Initialize() error {
	fmt.Fprintf(os.Stderr, "Initializing CalWatch...\n")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	cw.config = cfg

	fmt.Fprintf(os.Stderr, "Loaded configuration with %d directories\n", len(cfg.Directories))

	// Initialize state manager
	stateManager, err := storage.NewXDGStateManager()
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	cw.stateManager = stateManager

	// Load existing state
	if err := cw.stateManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load state, starting fresh: %v\n", err)
	}

	// Initialize event storage
	cw.eventStorage = storage.NewMemoryEventStorage()

	// Initialize parser
	cw.parser = parser.NewGocalParser()

	// Initialize notification manager
	cw.notificationManager = notifications.NewNotificationManager(cfg.Notification)

	// Initialize alert scheduler and manager
	scheduler := alerts.NewMinuteBasedScheduler()
	scheduler.SetEventStorage(cw.eventStorage)
	scheduler.SetDirectoryConfigs(cfg.Directories)
	scheduler.SetStateManager(cw.stateManager)
	cw.alertScheduler = scheduler
	cw.alertManager = alerts.NewAlertManager(scheduler)

	// Initialize file watcher
	cw.watcher, err = watcher.NewCalDAVWatcher(cw.handleFileChange)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add directories to watcher
	for _, dirConfig := range cfg.Directories {
		fmt.Fprintf(os.Stderr, "Watching directory: %s\n", dirConfig.Directory)
		if err := cw.watcher.AddDirectory(dirConfig.Directory); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to watch directory %s: %v\n", dirConfig.Directory, err)
		}
	}

	return nil
}

// Start starts the CalWatch daemon
func (cw *CalWatch) Start() error {
	if cw.isRunning {
		return fmt.Errorf("CalWatch is already running")
	}

	fmt.Fprintf(os.Stderr, "Starting CalWatch daemon...\n")

	// Perform initial scan of all directories
	if err := cw.performInitialScan(); err != nil {
		return fmt.Errorf("initial scan failed: %w", err)
	}

	// Check for wake-up and process missed events if enabled
	if err := cw.handleWakeupDetection(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: wake-up detection failed: %v\n", err)
	}

	// Start alert manager
	if err := cw.alertManager.Start(); err != nil {
		return fmt.Errorf("failed to start alert manager: %w", err)
	}

	// Start alert processing goroutine
	cw.wg.Add(1)
	go cw.processAlerts()

	cw.isRunning = true

	fmt.Fprintf(os.Stderr, "CalWatch daemon started successfully\n")
	fmt.Fprintf(os.Stderr, "Watching %d directories for calendar changes\n", len(cw.config.Directories))

	return nil
}

// Stop stops the CalWatch daemon
func (cw *CalWatch) Stop() error {
	if !cw.isRunning {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Stopping CalWatch daemon...\n")

	// Save current state before stopping
	if cw.stateManager != nil {
		if err := cw.stateManager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
		}
	}

	// Signal all goroutines to stop
	close(cw.stopChan)

	// Stop alert manager
	if err := cw.alertManager.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping alert manager: %v\n", err)
	}

	// Stop file watcher
	if err := cw.watcher.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping file watcher: %v\n", err)
	}

	// Wait for all goroutines to finish
	cw.wg.Wait()

	cw.isRunning = false

	fmt.Fprintf(os.Stderr, "CalWatch daemon stopped\n")

	return nil
}

// performInitialScan scans all configured directories for existing ICS files
func (cw *CalWatch) performInitialScan() error {
	fmt.Fprintf(os.Stderr, "Performing initial scan of calendar directories...\n")

	totalEvents := 0

	for _, dirConfig := range cw.config.Directories {
		fmt.Fprintf(os.Stderr, "Scanning directory: %s\n", dirConfig.Directory)

		events, err := cw.parser.ParseDirectory(dirConfig.Directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse directory %s: %v\n", dirConfig.Directory, err)
			continue
		}

		// Add events to storage
		for _, event := range events {
			if err := cw.eventStorage.UpsertEvent(event); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to store event %s: %v\n", event.GetUID(), err)
			}
		}

		fmt.Fprintf(os.Stderr, "Loaded %d events from %s\n", len(events), dirConfig.Directory)
		totalEvents += len(events)
	}

	// Regenerate daily index for today
	today := time.Now().Truncate(24 * time.Hour)
	if err := cw.eventStorage.RegenerateIndex(today); err != nil {
		return fmt.Errorf("failed to regenerate daily index: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Initial scan complete: loaded %d total events\n", totalEvents)

	return nil
}

// handleWakeupDetection checks for system wake-up and processes missed events
func (cw *CalWatch) handleWakeupDetection() error {
	if !cw.config.WakeupHandling.Enable {
		return nil
	}

	// Check if we've been asleep/shutdown
	wasAsleep, sleepDuration := cw.alertScheduler.DetectWakeup()
	if !wasAsleep {
		fmt.Fprintf(os.Stderr, "No wake-up detected, continuing normal operation\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Wake-up detected! System was inactive for %v\n", sleepDuration)

	// Get the last tick time and current time
	lastTick := cw.stateManager.GetLastAlertTick()
	currentTime := time.Now()

	fmt.Fprintf(os.Stderr, "Processing missed events from %v to %v\n", 
		lastTick.Format("2006-01-02 15:04:05"), 
		currentTime.Format("2006-01-02 15:04:05"))

	// Process missed events
	missedAlerts := cw.alertScheduler.CheckMissedAlerts(lastTick, currentTime, cw.config.WakeupHandling)

	if len(missedAlerts) == 0 {
		fmt.Fprintf(os.Stderr, "No missed events found\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Found %d missed events, sending notifications...\n", len(missedAlerts))

	// Send missed event notifications
	for _, alertRequest := range missedAlerts {
		fmt.Fprintf(os.Stderr, "Sending missed alert for event: %s (was due %s ago)\n", 
			alertRequest.Event.GetSummary(), alertRequest.AlertOffset.String())

		// For now, use regular notification method
		// TODO: Extend NotificationManager to support context-aware notifications for missed events
		if err := cw.notificationManager.SendNotification(alertRequest); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send missed notification for event %s: %v\n", 
				alertRequest.Event.GetSummary(), err)
		}
	}

	fmt.Fprintf(os.Stderr, "Missed event processing complete\n")
	return nil
}

// handleFileChange handles file system change events
func (cw *CalWatch) handleFileChange(event watcher.FileChangeEvent) {
	fmt.Fprintf(os.Stderr, "File change detected: %s (%s)\n", event.Path, event.Operation.String())

	switch event.Operation {
	case watcher.FileCreated, watcher.FileModified:
		// Parse the changed file
		events, err := cw.parser.ParseFile(event.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", event.Path, err)
			return
		}

		// Update events in storage
		for _, event := range events {
			if err := cw.eventStorage.UpsertEvent(event); err != nil {
				fmt.Fprintf(os.Stderr, "Error storing event %s: %v\n", event.GetUID(), err)
			}
		}

		fmt.Fprintf(os.Stderr, "Updated %d events from %s\n", len(events), event.Path)

	case watcher.FileDeleted:
		// For deleted files, we can't easily determine which events to remove
		// In a full implementation, we might track file-to-event mappings
		fmt.Fprintf(os.Stderr, "File deleted: %s (event cleanup not implemented)\n", event.Path)

	case watcher.FileRenamed:
		// Handle rename similar to delete + create
		fmt.Fprintf(os.Stderr, "File renamed: %s\n", event.Path)
	}

	// Regenerate daily index after changes
	today := time.Now().Truncate(24 * time.Hour)
	if err := cw.eventStorage.RegenerateIndex(today); err != nil {
		fmt.Fprintf(os.Stderr, "Error regenerating daily index: %v\n", err)
	}
}

// processAlerts handles alert notifications
func (cw *CalWatch) processAlerts() {
	defer cw.wg.Done()

	alertChan := cw.alertManager.GetAlertChannel()

	for {
		select {
		case alertRequests, ok := <-alertChan:
			if !ok {
				// Channel closed, exit
				return
			}

			// Process each alert request
			for _, request := range alertRequests {
				fmt.Fprintf(os.Stderr, "Sending alert for event: %s (in %s)\n", 
					request.Event.GetSummary(), request.AlertOffset.String())

				if err := cw.notificationManager.SendNotification(request); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to send notification for event %s: %v\n", 
						request.Event.GetSummary(), err)
				}
			}

		case <-cw.stopChan:
			return
		}
	}
}

// PrintStatus prints current daemon status
func (cw *CalWatch) PrintStatus() {
	if !cw.isRunning {
		fmt.Println("CalWatch daemon is not running")
		return
	}

	watchedDirs := cw.watcher.GetWatchedDirectories()
	totalEvents := len(cw.eventStorage.GetAllEvents())

	fmt.Printf("CalWatch Status:\n")
	fmt.Printf("  Status: Running\n")
	fmt.Printf("  Watched directories: %d\n", len(watchedDirs))
	fmt.Printf("  Total events: %d\n", totalEvents)
	
	for _, dir := range watchedDirs {
		fmt.Printf("    - %s\n", dir)
	}

	// Get today's events
	today := time.Now().Truncate(24 * time.Hour)
	todaysEvents := cw.eventStorage.GetEventsForDay(today)
	fmt.Printf("  Today's events: %d\n", len(todaysEvents))

	// Get upcoming events
	upcoming := cw.eventStorage.GetUpcomingEvents(time.Now(), 24*time.Hour)
	fmt.Printf("  Upcoming (24h): %d\n", len(upcoming))
}

// setupSignalHandling sets up graceful shutdown on SIGINT/SIGTERM
func (cw *CalWatch) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "Received signal %v, shutting down...\n", sig)
		cw.Stop()
	}()
}

func main() {
	// Parse command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "status":
			// TODO: Implement status checking (connect to running daemon)
			fmt.Println("Status checking not implemented yet")
			return
		case "stop":
			// TODO: Implement daemon stopping (send signal to running daemon)
			fmt.Println("Daemon stopping not implemented yet")
			return
		case "init":
			// Create default configuration and templates
			fmt.Println("Creating default configuration...")
			
			configPath, err := config.WriteDefaultConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating default config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Created default configuration at: %s\n", configPath)

			if err := notifications.CreateDefaultTemplates(); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating default templates: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Created default notification templates")
			return
		case "help", "-h", "--help":
			fmt.Println("CalWatch - CalDAV Directory Watcher Daemon")
			fmt.Println("")
			fmt.Println("Usage:")
			fmt.Println("  calwatch          Start the daemon")
			fmt.Println("  calwatch init     Create default configuration and templates")
			fmt.Println("  calwatch status   Show daemon status")
			fmt.Println("  calwatch stop     Stop the daemon")
			fmt.Println("  calwatch help     Show this help")
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "Use 'calwatch help' for usage information\n")
			os.Exit(1)
		}
	}

	// Create and initialize CalWatch
	app := NewCalWatch()

	if err := app.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	app.setupSignalHandling()

	// Start the daemon
	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start CalWatch: %v\n", err)
		os.Exit(1)
	}

	// Print initial status
	app.PrintStatus()

	// Wait for shutdown signal
	select {
	case <-app.stopChan:
		// Daemon was stopped
	}

	fmt.Fprintf(os.Stderr, "CalWatch exiting\n")
}