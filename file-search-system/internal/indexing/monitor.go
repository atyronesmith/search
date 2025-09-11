package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/file-search/file-search-system/internal/database"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Monitor watches for file system changes
type Monitor struct {
	watcher        *fsnotify.Watcher
	db             *database.DB
	watchPaths     []string
	ignorePatterns []string
	log            *logrus.Logger
	changesChan    chan FileChange
}

// FileChange represents a detected file system change
type FileChange struct {
	Path       string
	ChangeType database.ChangeType
	OldPath    string // For rename operations
	Timestamp  time.Time
}

// NewMonitor creates a new file system monitor
func NewMonitor(db *database.DB, watchPaths []string, ignorePatterns []string, log *logrus.Logger) (*Monitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Monitor{
		watcher:        watcher,
		db:             db,
		watchPaths:     watchPaths,
		ignorePatterns: ignorePatterns,
		log:            log,
		changesChan:    make(chan FileChange, 1000),
	}, nil
}

// Start begins monitoring the configured paths
func (m *Monitor) Start(ctx context.Context) error {
	// Add all watch paths
	for _, path := range m.watchPaths {
		if err := m.addWatchRecursive(path); err != nil {
			m.log.WithError(err).WithField("path", path).Error("Failed to add watch")
		}
	}

	// Start event processing
	go m.processEvents(ctx)

	return nil
}

// Stop stops the monitor
func (m *Monitor) Stop() error {
	close(m.changesChan)
	return m.watcher.Close()
}

// UpdatePaths updates the watch paths and ignore patterns
func (m *Monitor) UpdatePaths(watchPaths []string, ignorePatterns []string) error {
	// Close existing watcher
	if m.watcher != nil {
		m.watcher.Close()
	}

	// Create new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Update configuration
	m.watcher = watcher
	m.watchPaths = watchPaths
	m.ignorePatterns = ignorePatterns
	m.changesChan = make(chan FileChange, 1000)

	m.log.WithFields(logrus.Fields{
		"watch_paths":     watchPaths,
		"ignore_patterns": ignorePatterns,
	}).Info("Updated monitor configuration")

	return nil
}

// GetChangesChan returns the channel for receiving file changes
func (m *Monitor) GetChangesChan() <-chan FileChange {
	return m.changesChan
}

// addWatchRecursive adds a watch recursively to a directory
func (m *Monitor) addWatchRecursive(path string) error {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home := getHomeDir()
		path = filepath.Join(home, path[2:])
	}

	// Add watch to the root path
	if err := m.watcher.Add(path); err != nil {
		return err
	}

	// Walk subdirectories
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() && p != path {
			// Check if directory should be ignored
			if m.shouldIgnore(p) {
				return filepath.SkipDir
			}
			// Add watch to subdirectory
			if err := m.watcher.Add(p); err != nil {
				m.log.WithError(err).WithField("path", p).Debug("Failed to add watch")
			}
		}

		return nil
	})
}

// processEvents processes file system events
func (m *Monitor) processEvents(ctx context.Context) {
	renameOldPath := ""
	renameTimer := time.NewTimer(100 * time.Millisecond)
	renameTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// Ignore if path matches ignore patterns
			if m.shouldIgnore(event.Name) {
				continue
			}

			m.log.WithFields(logrus.Fields{
				"path":      event.Name,
				"operation": event.Op.String(),
			}).Debug("File system event")

			// Handle different event types
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				m.handleCreate(event.Name)

			case event.Op&fsnotify.Write == fsnotify.Write:
				m.handleModify(event.Name)

			case event.Op&fsnotify.Remove == fsnotify.Remove:
				m.handleDelete(event.Name)

			case event.Op&fsnotify.Rename == fsnotify.Rename:
				// Rename events come in pairs (old name, then new name)
				if renameOldPath == "" {
					renameOldPath = event.Name
					renameTimer.Reset(100 * time.Millisecond)
				} else {
					renameTimer.Stop()
					m.handleRename(renameOldPath, event.Name)
					renameOldPath = ""
				}
			}

		case <-renameTimer.C:
			// Timeout waiting for rename pair, treat as delete
			if renameOldPath != "" {
				m.handleDelete(renameOldPath)
				renameOldPath = ""
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.log.WithError(err).Error("Watcher error")
		}
	}
}

// shouldIgnore checks if a path should be ignored
func (m *Monitor) shouldIgnore(path string) bool {
	basename := filepath.Base(path)

	for _, pattern := range m.ignorePatterns {
		// Check basename patterns
		if matched, _ := filepath.Match(pattern, basename); matched {
			return true
		}
		// Check path patterns
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for hidden files
	if strings.HasPrefix(basename, ".") {
		return true
	}

	// Check for temporary files
	if strings.HasSuffix(basename, "~") || strings.HasSuffix(basename, ".tmp") {
		return true
	}

	return false
}

// handleCreate handles file creation events
func (m *Monitor) handleCreate(path string) {
	// Check if it's a directory and add watch
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if err := m.addWatchRecursive(path); err != nil {
			m.log.WithError(err).WithField("path", path).Error("Failed to add watch to new directory")
		}
		return
	}

	// Check if it's a supported file
	if !m.isSupportedFile(path) {
		return
	}

	change := FileChange{
		Path:       path,
		ChangeType: database.ChangeTypeCreated,
		Timestamp:  time.Now(),
	}

	select {
	case m.changesChan <- change:
	default:
		m.log.Warn("Changes channel full, dropping event")
	}

	// Store in database
	m.storeChange(change)
}

// handleModify handles file modification events
func (m *Monitor) handleModify(path string) {
	if !m.isSupportedFile(path) {
		return
	}

	change := FileChange{
		Path:       path,
		ChangeType: database.ChangeTypeModified,
		Timestamp:  time.Now(),
	}

	select {
	case m.changesChan <- change:
	default:
		m.log.Warn("Changes channel full, dropping event")
	}

	m.storeChange(change)
}

// handleDelete handles file deletion events
func (m *Monitor) handleDelete(path string) {
	change := FileChange{
		Path:       path,
		ChangeType: database.ChangeTypeDeleted,
		Timestamp:  time.Now(),
	}

	select {
	case m.changesChan <- change:
	default:
		m.log.Warn("Changes channel full, dropping event")
	}

	m.storeChange(change)
}

// handleRename handles file rename events
func (m *Monitor) handleRename(oldPath, newPath string) {
	change := FileChange{
		Path:       newPath,
		OldPath:    oldPath,
		ChangeType: database.ChangeTypeRenamed,
		Timestamp:  time.Now(),
	}

	select {
	case m.changesChan <- change:
	default:
		m.log.Warn("Changes channel full, dropping event")
	}

	m.storeChange(change)
}

// isSupportedFile checks if a file has a supported extension
func (m *Monitor) isSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supportedExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true,
		".xls": true, ".xlsx": true, ".csv": true,
		".txt": true, ".md": true, ".rtf": true,
		".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".java": true,
		".cpp": true, ".c": true, ".go": true,
		".rs": true, ".json": true, ".yaml": true,
		".yml": true,
	}
	return supportedExts[ext]
}

// storeChange stores a file change in the database
func (m *Monitor) storeChange(change FileChange) {
	ctx := context.Background()

	query := `
		INSERT INTO file_changes (file_path, change_type, old_path, detected_at)
		VALUES ($1, $2, $3, $4)
	`

	var oldPath *string
	if change.OldPath != "" {
		oldPath = &change.OldPath
	}

	if _, err := m.db.Exec(ctx, query, change.Path, string(change.ChangeType), oldPath, change.Timestamp); err != nil {
		m.log.WithError(err).WithField("path", change.Path).Error("Failed to store file change")
	}
}

// getHomeDir returns the user's home directory
func getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	return ""
}
