package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/file-search/file-search-system/internal/config"
	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/internal/embeddings"
	"github.com/file-search/file-search-system/internal/indexing"
	"github.com/file-search/file-search-system/internal/search"
	"github.com/file-search/file-search-system/pkg/chunker"
	"github.com/file-search/file-search-system/pkg/extractor"
	"github.com/pgvector/pgvector-go"
	"github.com/sirupsen/logrus"
)

// Service represents the main background service that orchestrates all components
type Service struct {
	config    *config.Config
	db        *database.DB
	embedder  *embeddings.OllamaClient
	scanner   *indexing.Scanner
	monitor   *indexing.Monitor
	engine    *search.Engine
	extractor *extractor.ExtractorManager
	chunker   *chunker.ChunkerManager
	
	// Service state
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	
	// Component states
	indexingActive   int32  // atomic
	indexingPaused   int32  // atomic
	monitoringActive int32  // atomic
	scanningActive   int32  // atomic
	
	// Statistics
	stats     *ServiceStats
	statsLock sync.RWMutex
	
	// Resource monitoring
	resourceMonitor *ResourceMonitor
	rateLimiter     *RateLimiter
	
	// Event channels
	indexingEvents chan IndexingEvent
	systemEvents   chan SystemEvent
	
	// Lifecycle
	startTime time.Time
	log       *logrus.Logger
}

// ServiceStats holds service statistics
type ServiceStats struct {
	StartTime        time.Time     `json:"start_time"`
	Uptime           time.Duration `json:"uptime"`
	IndexingActive   bool          `json:"indexing_active"`
	IndexingPaused   bool          `json:"indexing_paused"`
	MonitoringActive bool          `json:"monitoring_active"`
	
	// File statistics
	TotalFiles      int64 `json:"total_files"`
	IndexedFiles    int64 `json:"indexed_files"`
	PendingFiles    int64 `json:"pending_files"`
	FailedFiles     int64 `json:"failed_files"`
	ProcessingRate  int64 `json:"processing_rate"` // files per minute
	
	// Search statistics
	SearchCacheSize   int     `json:"search_cache_size"`
	ActiveSearches    int     `json:"active_searches"`
	SearchQPS         float64 `json:"search_qps"`
	AvgSearchTime     float64 `json:"avg_search_time_ms"`
	
	// Resource usage
	ResourceUsage ResourceUsage `json:"resource_usage"`
	
	// Database statistics
	DatabaseSize   int64   `json:"database_size_bytes"`
	ChunkCount     int64   `json:"chunk_count"`
	EmbeddingCount int64   `json:"embedding_count"`
	
	// Error statistics
	RecentErrors   []ErrorInfo `json:"recent_errors"`
	ErrorRate      float64     `json:"error_rate"`
}

// IndexingEvent represents an indexing event
type IndexingEvent struct {
	Type        string    `json:"type"`
	FilePath    string    `json:"file_path"`
	Status      string    `json:"status"`
	Error       error     `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	ProcessTime int64     `json:"process_time_ms"`
}

// SystemEvent represents a system event
type SystemEvent struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Severity  string                 `json:"severity"`
	Timestamp time.Time              `json:"timestamp"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Message   string    `json:"message"`
	Component string    `json:"component"`
	Count     int       `json:"count"`
	LastSeen  time.Time `json:"last_seen"`
}

// NewService creates a new background service
func NewService(cfg *config.Config, db *database.DB, log *logrus.Logger) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize embedder
	ollamaConfig := &embeddings.OllamaConfig{
		Host:    cfg.OllamaHost,
		Model:   cfg.OllamaModel,
		Timeout: cfg.OllamaTimeout,
	}
	embedder := embeddings.NewOllamaClient(ollamaConfig, log)
	
	// Initialize search engine
	searchConfig := &search.Config{
		VectorWeight:   cfg.SearchVectorWeight,
		BM25Weight:     cfg.SearchBM25Weight,
		MetadataWeight: cfg.SearchMetadataWeight,
		DefaultLimit:   20,
		CacheTTL:       15 * time.Minute,
		MinScore:       0.1,
	}
	engine := search.NewEngine(db, embedder, searchConfig, log)
	
	// Initialize scanner
	scannerConfig := &indexing.ScannerConfig{
		WatchPaths:     cfg.WatchPaths,
		MaxFileSizeMB:  cfg.IndexMaxFileSizeMB,
		IgnorePatterns: cfg.WatchIgnorePatterns,
		SupportedTypes: []string{
			".pdf", ".doc", ".docx",                                    // Documents
			".xls", ".xlsx", ".csv",                                    // Spreadsheets  
			".txt", ".md", ".rtf",                                      // Text files
			".py", ".js", ".ts", ".jsx", ".tsx", ".java",              // Code files
			".cpp", ".c", ".go", ".rs", ".json", ".yaml", ".yml",      // More code files
			".h", ".hpp", ".css", ".html",                             // Additional formats
		},
	}
	scanner := indexing.NewScanner(db, scannerConfig, log)
	
	// Initialize monitor
	monitor, err := indexing.NewMonitor(db, cfg.WatchPaths, cfg.WatchIgnorePatterns, log)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create file monitor: %w", err)
	}
	
	// Initialize resource monitor
	resourceMonitor := NewResourceMonitor(&ResourceConfig{
		CPUThreshold:    cfg.CPUThreshold,
		MemoryThreshold: cfg.MemoryThreshold,
		CheckInterval:   5 * time.Second,
		HistorySize:     60, // Keep 5 minutes of history
	}, log)
	
	// Initialize rate limiter
	rateLimiter := NewRateLimiter(&RateLimiterConfig{
		IndexingRate:   60,  // files per minute
		EmbeddingRate:  120, // embeddings per minute
		SearchRate:     300, // searches per minute
		BurstSize:      10,
	})
	
	// Initialize extractor manager
	extractorManager := extractor.NewExtractorManager()
	extractorManager.AddExtractor(extractor.NewTextExtractor(extractor.DefaultConfig()))
	extractorManager.AddExtractor(extractor.NewCodeExtractor(extractor.DefaultConfig()))
	extractorManager.AddExtractor(extractor.NewPDFExtractor(extractor.DefaultConfig()))
	
	// Initialize chunker manager  
	chunkerManager := chunker.NewChunkerManager(chunker.DefaultConfig())
	
	s := &Service{
		config:    cfg,
		db:        db,
		embedder:  embedder,
		scanner:   scanner,
		monitor:   monitor,
		engine:    engine,
		extractor: extractorManager,
		chunker:   chunkerManager,
		
		ctx:    ctx,
		cancel: cancel,
		
		stats: &ServiceStats{
			StartTime: time.Now(),
		},
		
		resourceMonitor: resourceMonitor,
		rateLimiter:     rateLimiter,
		
		indexingEvents: make(chan IndexingEvent, 1000),
		systemEvents:   make(chan SystemEvent, 100),
		
		startTime: time.Now(),
		log:       log,
	}
	
	return s, nil
}

// Start starts the background service
func (s *Service) Start() error {
	s.log.Info("Starting background service")
	
	// Start resource monitoring
	s.wg.Add(1)
	go s.runResourceMonitor()
	
	// Start event processor
	s.wg.Add(1)
	go s.runEventProcessor()
	
	// Start statistics updater
	s.wg.Add(1)
	go s.runStatsUpdater()
	
	// Start indexing loop
	s.wg.Add(1)
	go s.runIndexingLoop()
	
	// Emit startup event
	s.emitSystemEvent("service_started", "Background service started successfully", "info")
	
	s.log.Info("Background service started successfully")
	return nil
}

// Stop stops the background service gracefully
func (s *Service) Stop() error {
	s.log.Info("Stopping background service")
	
	// Stop all operations
	s.cancel()
	
	// Stop monitoring
	if err := s.StopIndexing(); err != nil {
		s.log.WithError(err).Error("Error stopping indexing")
	}
	
	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	// Wait with timeout
	select {
	case <-done:
		s.log.Info("Background service stopped successfully")
	case <-time.After(30 * time.Second):
		s.log.Warn("Background service stop timeout, forcing shutdown")
	}
	
	// Emit shutdown event
	s.emitSystemEvent("service_stopped", "Background service stopped", "info")
	
	return nil
}

// StartIndexing starts the indexing process
func (s *Service) StartIndexing(paths []string, recursive bool) error {
	if atomic.LoadInt32(&s.indexingActive) == 1 {
		return fmt.Errorf("indexing is already active")
	}
	
	s.log.WithFields(logrus.Fields{
		"paths":     paths,
		"recursive": recursive,
	}).Info("Starting indexing")
	
	// Set indexing as active
	atomic.StoreInt32(&s.indexingActive, 1)
	atomic.StoreInt32(&s.indexingPaused, 0)
	
	// Start scanner for specified paths
	go func() {
		atomic.StoreInt32(&s.scanningActive, 1)
		defer atomic.StoreInt32(&s.scanningActive, 0)
		
		for _, path := range paths {
			if err := s.scanner.ScanDirectory(s.ctx, path); err != nil {
				s.log.WithError(err).WithField("path", path).Error("Error scanning path")
				s.emitSystemEvent("scan_error", fmt.Sprintf("Error scanning path %s: %v", path, err), "error")
			}
		}
	}()
	
	s.emitSystemEvent("indexing_started", "Indexing process started", "info")
	return nil
}

// StopIndexing stops the indexing process
func (s *Service) StopIndexing() error {
	s.log.Info("Stopping indexing")
	
	atomic.StoreInt32(&s.indexingActive, 0)
	atomic.StoreInt32(&s.indexingPaused, 0)
	
	s.emitSystemEvent("indexing_stopped", "Indexing process stopped", "info")
	return nil
}

// PauseIndexing pauses the indexing process
func (s *Service) PauseIndexing() error {
	if atomic.LoadInt32(&s.indexingActive) == 0 {
		return fmt.Errorf("indexing is not active")
	}
	
	s.log.Info("Pausing indexing")
	atomic.StoreInt32(&s.indexingPaused, 1)
	
	s.emitSystemEvent("indexing_paused", "Indexing process paused", "info")
	return nil
}

// ResumeIndexing resumes the indexing process
func (s *Service) ResumeIndexing() error {
	if atomic.LoadInt32(&s.indexingActive) == 0 {
		return fmt.Errorf("indexing is not active")
	}
	
	s.log.Info("Resuming indexing")
	atomic.StoreInt32(&s.indexingPaused, 0)
	
	s.emitSystemEvent("indexing_resumed", "Indexing process resumed", "info")
	return nil
}

// GetStartTime returns the service start time
func (s *Service) GetStartTime() time.Time {
	return s.startTime
}

// GetIndexingStatus returns the current indexing status
func (s *Service) GetIndexingStatus() map[string]interface{} {
	return map[string]interface{}{
		"active":   atomic.LoadInt32(&s.indexingActive) == 1,
		"paused":   atomic.LoadInt32(&s.indexingPaused) == 1,
		"scanning": atomic.LoadInt32(&s.scanningActive) == 1,
	}
}

// categorizeProcessingError categorizes errors for better handling
func (s *Service) categorizeProcessingError(err error) string {
	errorStr := err.Error()
	
	switch {
	case strings.Contains(errorStr, "empty text provided for embedding"):
		return "empty_content"
	case strings.Contains(errorStr, "file contains invalid UTF-8"):
		return "encoding_error"
	case strings.Contains(errorStr, "invalid byte sequence for encoding"):
		return "encoding_error"
	case strings.Contains(errorStr, "string is too long for tsvector"):
		return "content_too_large"
	case strings.Contains(errorStr, "no extractor available"):
		return "unsupported_format"
	case strings.Contains(errorStr, "file too large"):
		return "file_too_large"
	case strings.Contains(errorStr, "permission denied"):
		return "permission_error"
	case strings.Contains(errorStr, "no such file or directory"):
		return "file_not_found"
	default:
		return "processing_error"
	}
}

// shouldSkipFile determines if a file should be skipped based on error type
func (s *Service) shouldSkipFile(err error, category string) bool {
	// These error types are expected and files should be skipped rather than marked as failed
	skipCategories := map[string]bool{
		"empty_content":      true,  // Empty files are valid, just skip them
		"encoding_error":     true,  // Binary files with encoding issues
		"content_too_large":  true,  // These should now be handled by chunking, skip if still failing
		"unsupported_format": true,  // Files we can't process
		"file_too_large":     true,  // Files exceeding size limits
		"permission_error":   true,  // Files we can't access
		"file_not_found":     true,  // Files that disappeared during processing
	}
	
	return skipCategories[category]
}

// ScanDirectory scans a specific directory
func (s *Service) ScanDirectory(path string, recursive bool) error {
	s.log.WithFields(logrus.Fields{
		"path":      path,
		"recursive": recursive,
	}).Info("Scanning directory")
	
	go func() {
		atomic.StoreInt32(&s.scanningActive, 1)
		defer atomic.StoreInt32(&s.scanningActive, 0)
		
		if err := s.scanner.ScanDirectory(s.ctx, path); err != nil {
			s.log.WithError(err).WithField("path", path).Error("Error scanning directory")
			s.emitSystemEvent("scan_error", fmt.Sprintf("Error scanning directory %s: %v", path, err), "error")
		}
	}()
	
	return nil
}

// GetStats returns current service statistics
func (s *Service) GetStats() *ServiceStats {
	s.statsLock.RLock()
	defer s.statsLock.RUnlock()
	
	// Create a copy of stats
	stats := *s.stats
	stats.Uptime = time.Since(s.startTime)
	
	return &stats
}

// GetSearchEngine returns the search engine instance
func (s *Service) GetSearchEngine() *search.Engine {
	return s.engine
}

// ResetDatabase resets the database to a blank state
func (s *Service) ResetDatabase() error {
	s.log.Info("Resetting database")
	
	// Stop indexing first
	if err := s.StopIndexing(); err != nil {
		s.log.WithError(err).Error("Failed to stop indexing before reset")
		return fmt.Errorf("failed to stop indexing: %w", err)
	}
	
	ctx := context.Background()
	
	// Reset all tables in correct order (respecting foreign key constraints)
	tables := []string{
		"text_search",
		"chunks", 
		"files",
		"indexing_stats",
	}
	
	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s", table)
		if _, err := s.db.Exec(ctx, query); err != nil {
			s.log.WithError(err).WithField("table", table).Error("Failed to reset table")
			return fmt.Errorf("failed to reset table %s: %w", table, err)
		}
		s.log.WithField("table", table).Info("Reset table")
	}
	
	s.emitSystemEvent("database_reset", "Database reset successfully", "info")
	return nil
}

// runResourceMonitor runs the resource monitoring loop
func (s *Service) runResourceMonitor() {
	defer s.wg.Done()
	
	s.log.Info("Starting resource monitor")
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			usage := s.resourceMonitor.GetCurrentUsage()
			
			// Update stats
			s.statsLock.Lock()
			s.stats.ResourceUsage = usage
			s.statsLock.Unlock()
			
			// Check if we need to pause indexing due to high resource usage
			if s.resourceMonitor.ShouldPauseIndexing(usage) && 
			   atomic.LoadInt32(&s.indexingActive) == 1 && 
			   atomic.LoadInt32(&s.indexingPaused) == 0 {
				
				s.log.WithFields(logrus.Fields{
					"cpu_percent":    usage.CPUPercent,
					"memory_percent": usage.MemoryPercent,
				}).Warn("High resource usage detected, pausing indexing")
				
				s.PauseIndexing()
				s.emitSystemEvent("auto_pause", "Indexing auto-paused due to high resource usage", "warning")
			} else if !s.resourceMonitor.ShouldPauseIndexing(usage) && 
			          atomic.LoadInt32(&s.indexingActive) == 1 && 
			          atomic.LoadInt32(&s.indexingPaused) == 1 {
				
				s.log.Info("Resource usage normalized, resuming indexing")
				s.ResumeIndexing()
				s.emitSystemEvent("auto_resume", "Indexing auto-resumed, resource usage normalized", "info")
			}
		}
	}
}

// runEventProcessor processes indexing and system events
func (s *Service) runEventProcessor() {
	defer s.wg.Done()
	
	s.log.Info("Starting event processor")
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.indexingEvents:
			s.processIndexingEvent(event)
		case event := <-s.systemEvents:
			s.processSystemEvent(event)
		}
	}
}

// runStatsUpdater periodically updates service statistics
func (s *Service) runStatsUpdater() {
	defer s.wg.Done()
	
	s.log.Info("Starting stats updater")
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateStats()
		}
	}
}

// runIndexingLoop runs the main indexing processing loop
func (s *Service) runIndexingLoop() {
	defer s.wg.Done()
	
	s.log.Info("Starting indexing loop")
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Only process if indexing is active and not paused
			if atomic.LoadInt32(&s.indexingActive) == 1 && 
			   atomic.LoadInt32(&s.indexingPaused) == 0 {
				
				// Apply rate limiting
				if s.rateLimiter.AllowIndexing() {
					s.processNextFile()
				}
			}
		}
	}
}

// processNextFile processes the next file in the indexing queue
func (s *Service) processNextFile() {
	ctx := context.Background()
	start := time.Now()
	
	// Get next pending file
	file, err := s.getNextPendingFile(ctx)
	if err != nil {
		if err.Error() != "no pending files" {
			s.log.WithError(err).Error("Failed to get next pending file")
		}
		return
	}
	if file == nil {
		return
	}
	
	// Mark file as processing
	if err := s.updateFileStatus(ctx, file.ID, "processing", ""); err != nil {
		s.log.WithError(err).WithField("file_id", file.ID).Error("Failed to mark file as processing")
		return
	}
	
	s.log.WithFields(logrus.Fields{
		"file_id": file.ID,
		"path":    file.Path,
	}).Info("Processing file")
	
	// Process the file with improved error handling
	if err := s.processFileComplete(ctx, file); err != nil {
		// Categorize errors for better handling
		errorCategory := s.categorizeProcessingError(err)
		
		s.log.WithError(err).WithFields(logrus.Fields{
			"file_id":        file.ID,
			"path":           file.Path,
			"error_category": errorCategory,
		}).Warn("Failed to process file")
		
		// For certain error types, mark as skipped instead of error
		if s.shouldSkipFile(err, errorCategory) {
			s.log.WithFields(logrus.Fields{
				"file_id": file.ID,
				"path":    file.Path,
				"reason":  err.Error(),
			}).Info("Skipping file due to expected condition")
			
			s.updateFileStatus(ctx, file.ID, "skipped", err.Error())
			s.emitIndexingEvent("file_processed", file.Path, "skipped", err, time.Since(start).Milliseconds())
			return
		}
		
		// Mark as error for unexpected failures
		s.updateFileStatus(ctx, file.ID, "error", err.Error())
		s.emitIndexingEvent("file_processed", file.Path, "failed", err, time.Since(start).Milliseconds())
		return
	}
	
	// Mark as completed
	if err := s.updateFileStatus(ctx, file.ID, "completed", ""); err != nil {
		s.log.WithError(err).WithField("file_id", file.ID).Error("Failed to mark file as completed")
		return
	}
	
	processingTime := time.Since(start).Milliseconds()
	s.log.WithFields(logrus.Fields{
		"file_id": file.ID,
		"path":    file.Path,
		"time_ms": processingTime,
	}).Info("File processed successfully")
	
	// Update indexing statistics
	s.updateIndexingStats(ctx)
	
	s.emitIndexingEvent("file_processed", file.Path, "completed", nil, processingTime)
}

// emitIndexingEvent emits an indexing event
func (s *Service) emitIndexingEvent(eventType, filePath, status string, err error, processTime int64) {
	event := IndexingEvent{
		Type:        eventType,
		FilePath:    filePath,
		Status:      status,
		Error:       err,
		Timestamp:   time.Now(),
		ProcessTime: processTime,
	}
	
	select {
	case s.indexingEvents <- event:
	default:
		// Channel is full, drop event
		s.log.Warn("Indexing event channel full, dropping event")
	}
}

// emitSystemEvent emits a system event
func (s *Service) emitSystemEvent(eventType, message, severity string) {
	event := SystemEvent{
		Type:      eventType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}
	
	select {
	case s.systemEvents <- event:
	default:
		// Channel is full, drop event
		s.log.Warn("System event channel full, dropping event")
	}
}

// processIndexingEvent processes an indexing event
func (s *Service) processIndexingEvent(event IndexingEvent) {
	s.log.WithFields(logrus.Fields{
		"type":      event.Type,
		"file_path": event.FilePath,
		"status":    event.Status,
	}).Debug("Processing indexing event")
	
	// Update statistics based on event
	s.statsLock.Lock()
	switch event.Status {
	case "completed":
		s.stats.IndexedFiles++
	case "failed":
		s.stats.FailedFiles++
		if event.Error != nil {
			s.addError("indexing", event.Error.Error())
		}
	}
	s.statsLock.Unlock()
}

// processSystemEvent processes a system event
func (s *Service) processSystemEvent(event SystemEvent) {
	logLevel := logrus.InfoLevel
	switch event.Severity {
	case "error":
		logLevel = logrus.ErrorLevel
	case "warning":
		logLevel = logrus.WarnLevel
	case "debug":
		logLevel = logrus.DebugLevel
	}
	
	s.log.WithFields(logrus.Fields{
		"event_type": event.Type,
		"message":    event.Message,
	}).Log(logLevel, "System event")
}

// updateStats updates service statistics
func (s *Service) updateStats() {
	s.statsLock.Lock()
	defer s.statsLock.Unlock()
	
	s.stats.IndexingActive = atomic.LoadInt32(&s.indexingActive) == 1
	s.stats.IndexingPaused = atomic.LoadInt32(&s.indexingPaused) == 1
	s.stats.MonitoringActive = atomic.LoadInt32(&s.monitoringActive) == 1
	
	// Update search cache size
	if s.engine != nil {
		// This would get actual cache size from search engine
		s.stats.SearchCacheSize = 0
	}
	
	// Calculate processing rate (simplified)
	s.stats.ProcessingRate = s.stats.IndexedFiles // files per minute (simplified)
}

// addError adds an error to the recent errors list
func (s *Service) addError(component, message string) {
	// Find existing error or create new one
	found := false
	for i := range s.stats.RecentErrors {
		if s.stats.RecentErrors[i].Component == component && 
		   s.stats.RecentErrors[i].Message == message {
			s.stats.RecentErrors[i].Count++
			s.stats.RecentErrors[i].LastSeen = time.Now()
			found = true
			break
		}
	}
	
	if !found {
		error := ErrorInfo{
			Message:   message,
			Component: component,
			Count:     1,
			LastSeen:  time.Now(),
		}
		s.stats.RecentErrors = append(s.stats.RecentErrors, error)
		
		// Keep only last 10 errors
		if len(s.stats.RecentErrors) > 10 {
			s.stats.RecentErrors = s.stats.RecentErrors[1:]
		}
	}
}

// getNextPendingFile gets the next file that needs to be processed
func (s *Service) getNextPendingFile(ctx context.Context) (*database.File, error) {
	query := `
		SELECT id, path, parent_path, filename, extension, file_type,
		       size_bytes, created_at, modified_at, last_indexed,
		       content_hash, indexing_status, error_message, metadata
		FROM files
		WHERE indexing_status = 'pending'
		ORDER BY last_indexed ASC, id ASC
		LIMIT 1
	`
	
	var file database.File
	err := s.db.QueryRow(ctx, query).Scan(
		&file.ID, &file.Path, &file.ParentPath, &file.Filename,
		&file.Extension, &file.FileType, &file.SizeBytes,
		&file.CreatedAt, &file.ModifiedAt, &file.LastIndexed,
		&file.ContentHash, &file.IndexingStatus, &file.ErrorMessage,
		&file.Metadata,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no pending files")
		}
		return nil, err
	}
	
	return &file, nil
}

// updateFileStatus updates the status of a file
func (s *Service) updateFileStatus(ctx context.Context, fileID int64, status, errorMessage string) error {
	var query string
	var args []interface{}
	
	if errorMessage != "" {
		query = `
			UPDATE files 
			SET indexing_status = $1, error_message = $2, last_indexed = NOW()
			WHERE id = $3
		`
		args = []interface{}{status, errorMessage, fileID}
	} else {
		query = `
			UPDATE files 
			SET indexing_status = $1, error_message = NULL, last_indexed = NOW()
			WHERE id = $2
		`
		args = []interface{}{status, fileID}
	}
	
	_, err := s.db.Exec(ctx, query, args...)
	return err
}

// processFileComplete handles the complete processing of a single file
func (s *Service) processFileComplete(ctx context.Context, file *database.File) error {
	// Extract content from file
	extractedContent, err := s.extractor.Extract(ctx, file.Path)
	if err != nil {
		return fmt.Errorf("failed to extract content: %w", err)
	}
	
	// Early validation: Skip files with no meaningful content
	if extractedContent == nil || strings.TrimSpace(extractedContent.Text) == "" {
		s.log.WithFields(logrus.Fields{
			"file_id": file.ID,
			"path":    file.Path,
		}).Debug("Skipping file with no extractable content")
		return nil // Successfully "process" empty files by skipping them
	}
	
	// Determine file type for chunking
	fileType := "text"
	if file.FileType != nil {
		fileType = *file.FileType
	}
	
	// Chunk the content
	chunks, err := s.chunker.ChunkContent(extractedContent, fileType)
	if err != nil {
		return fmt.Errorf("failed to chunk content: %w", err)
	}
	
	// Skip files that produce no valid chunks
	if len(chunks) == 0 {
		s.log.WithFields(logrus.Fields{
			"file_id": file.ID,
			"path":    file.Path,
		}).Debug("Skipping file with no valid chunks")
		return nil
	}
	
	// Clear existing chunks for this file (in case of reindexing)
	if err := s.clearFileChunks(ctx, file.ID); err != nil {
		return fmt.Errorf("failed to clear existing chunks: %w", err)
	}
	
	// Process each chunk with improved error handling
	processedCount := 0
	for _, chunk := range chunks {
		if err := s.processChunk(ctx, file.ID, &chunk); err != nil {
			// Log the error but continue with other chunks instead of failing the entire file
			s.log.WithError(err).WithFields(logrus.Fields{
				"file_id":     file.ID,
				"chunk_index": chunk.Index,
				"path":        file.Path,
			}).Warn("Failed to process individual chunk, continuing with others")
			continue
		}
		processedCount++
	}
	
	// Only fail if no chunks were successfully processed
	if processedCount == 0 {
		return fmt.Errorf("no chunks could be processed successfully")
	}
	
	s.log.WithFields(logrus.Fields{
		"file_id":         file.ID,
		"total_chunks":    len(chunks),
		"processed_chunks": processedCount,
	}).Debug("File processing completed")
	
	return nil
}

// clearFileChunks removes existing chunks for a file
func (s *Service) clearFileChunks(ctx context.Context, fileID int64) error {
	// Delete from text_search first (foreign key constraint)
	_, err := s.db.Exec(ctx, `DELETE FROM text_search WHERE file_id = $1`, fileID)
	if err != nil {
		return err
	}
	
	// Delete chunks
	_, err = s.db.Exec(ctx, `DELETE FROM chunks WHERE file_id = $1`, fileID)
	return err
}

// processChunk processes a single chunk (embedding + storage)
func (s *Service) processChunk(ctx context.Context, fileID int64, chunk *chunker.Chunk) error {
	// Skip chunks with empty or whitespace-only content
	trimmedContent := strings.TrimSpace(chunk.Content)
	if trimmedContent == "" {
		s.log.WithFields(logrus.Fields{
			"file_id":     fileID,
			"chunk_index": chunk.Index,
		}).Debug("Skipping empty chunk")
		return nil // Skip empty chunks entirely
	}
	
	// Generate embedding
	embedding, err := s.embedder.Embed(ctx, chunk.Content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}
	
	// Handle case where embedder returns empty embedding for empty content
	if len(embedding) == 0 {
		s.log.WithFields(logrus.Fields{
			"file_id":     fileID,
			"chunk_index": chunk.Index,
		}).Debug("Skipping chunk with empty embedding")
		return nil // Skip chunks that produce empty embeddings
	}
	
	// Convert to pgvector format (float64 to float32)
	embedding32 := make([]float32, len(embedding))
	for i, v := range embedding {
		embedding32[i] = float32(v)
	}
	pgEmbedding := pgvector.NewVector(embedding32)
	
	// Prepare metadata
	metadata, _ := json.Marshal(chunk.Metadata)
	
	// Insert chunk
	chunkQuery := `
		INSERT INTO chunks (
			file_id, chunk_index, content, embedding, start_line, 
			char_start, char_end, chunk_type, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	
	var chunkID int64
	err = s.db.QueryRow(ctx, chunkQuery,
		fileID, chunk.Index, chunk.Content, pgEmbedding,
		chunk.StartLine, chunk.StartChar, chunk.EndChar,
		chunk.Type, metadata,
	).Scan(&chunkID)
	
	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}
	
	// Insert into text_search for full-text search
	textSearchQuery := `
		INSERT INTO text_search (file_id, chunk_id, content)
		VALUES ($1, $2, $3)
	`
	
	_, err = s.db.Exec(ctx, textSearchQuery, fileID, chunkID, chunk.Content)
	if err != nil {
		return fmt.Errorf("failed to insert text search entry: %w", err)
	}
	
	return nil
}
// updateIndexingStats updates the indexing statistics in the database
func (s *Service) updateIndexingStats(ctx context.Context) {
	// Calculate stats from the files table
	statsQuery := `
		INSERT INTO indexing_stats (id, total_files, indexed_files, failed_files, total_chunks, total_size_bytes, last_updated)
		SELECT 
			1,
			COUNT(*) as total_files,
			COUNT(CASE WHEN indexing_status = 'completed' THEN 1 END) as indexed_files,
			COUNT(CASE WHEN indexing_status = 'error' THEN 1 END) as failed_files,
			(SELECT COUNT(*) FROM chunks) as total_chunks,
			COALESCE(SUM(size_bytes), 0) as total_size_bytes,
			NOW() as last_updated
		FROM files
		ON CONFLICT (id) DO UPDATE SET
			total_files = EXCLUDED.total_files,
			indexed_files = EXCLUDED.indexed_files,
			failed_files = EXCLUDED.failed_files,
			total_chunks = EXCLUDED.total_chunks,
			total_size_bytes = EXCLUDED.total_size_bytes,
			last_updated = EXCLUDED.last_updated
	`
	
	if _, err := s.db.Exec(ctx, statsQuery); err != nil {
		s.log.WithError(err).Warn("Failed to update indexing stats")
	}
}
