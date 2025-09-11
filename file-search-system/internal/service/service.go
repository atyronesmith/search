package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/file-search/file-search-system/internal/config"
	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/internal/embeddings"
	"github.com/file-search/file-search-system/internal/indexing"
	// "github.com/file-search/file-search-system/internal/nlp"
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
	extractor *extractor.Manager
	chunker   *chunker.Manager
	// nlpClient *nlp.Client
	
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
	
	// Worker synchronization
	workersWG        sync.WaitGroup  // Tracks active worker goroutines
	processingCount  int32           // atomic - number of files currently being processed
	workChan         chan *database.File // Channel for distributing work to workers
	workersDone      chan struct{}   // Signal channel to stop workers
	workerCtx        context.Context    // Context for worker operations
	workerCancel     context.CancelFunc // Cancel function for worker context
	
	// Statistics
	stats     *Stats
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

// Stats holds service statistics
type Stats struct {
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
	engine := search.NewEngine(db, embedder, searchConfig, log, cfg.OllamaHost, cfg.LLMModel)
	
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
	
	// Initialize monitor - will be configured from database when started
	monitor, err := indexing.NewMonitor(db, []string{}, []string{}, log)
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
	extractorManager := extractor.NewManager()
	
	// Unstructured.io configuration for comprehensive document processing
	unstructuredConfig := extractor.UnstructuredConfig{
		VenvPath: "/Users/asmith/dev/search/file-search-system/unstructured-venv",
		Timeout:  300 * time.Second, // Increased to 5 minutes for large files
	}
	
	// Priority order matters: UnstructuredExtractor handles most document formats
	extractorManager.AddExtractor(extractor.NewUnstructuredExtractor(unstructuredConfig, log))
	
	// Enhanced PDF extractor as fallback for PDFs
	// DISABLED: Using UnstructuredExtractor for PDFs to get royal metadata
	// doclingConfig := &extractor.DoclingConfig{
	// 	ServiceURL: cfg.DoclingServiceURL,
	// 	Timeout:    30 * time.Second,
	// 	Enabled:    false, // Disable Docling in favor of Unstructured
	// }
	// extractorManager.AddExtractor(extractor.NewEnhancedPDFExtractor(extractor.DefaultConfig(), doclingConfig))
	
	// TextExtractor handles remaining text files
	// TEMPORARILY DISABLED: Let UnstructuredExtractor handle text files for metadata
	// extractorManager.AddExtractor(extractor.NewTextExtractor(extractor.DefaultConfig()))
	
	// CodeExtractor handles programming language files  
	// NOTE: Only for files not supported by UnstructuredExtractor
	extractorManager.AddExtractor(extractor.NewCodeExtractor(extractor.DefaultConfig()))
	
	// Initialize chunker manager  
	chunkerManager := chunker.NewManager(chunker.DefaultConfig())
	
	// Initialize NLP client
	// nlpServiceURL := os.Getenv("NLP_SERVICE_URL")
	// if nlpServiceURL == "" {
	// 	nlpServiceURL = "http://localhost:8081"
	// }
	// nlpClient := nlp.NewClient(nlpServiceURL)
	
	s := &Service{
		config:    cfg,
		db:        db,
		embedder:  embedder,
		scanner:   scanner,
		monitor:   monitor,
		engine:    engine,
		extractor: extractorManager,
		chunker:   chunkerManager,
		// nlpClient: nlpClient,
		
		ctx:    ctx,
		cancel: cancel,
		
		stats: &Stats{
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
	
	// Do NOT auto-enable indexing - let the user control when to start
	atomic.StoreInt32(&s.indexingActive, 0)
	atomic.StoreInt32(&s.indexingPaused, 0)
	s.log.Info("Indexing is idle, waiting for user to start")
	
	// Start file monitoring
	if err := s.StartFileMonitoring(); err != nil {
		s.log.WithError(err).Error("Failed to start file monitoring")
		// Don't fail service startup if monitoring fails
	}
	
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
	
	// Stop file monitoring
	if err := s.StopFileMonitoring(); err != nil {
		s.log.WithError(err).Error("Error stopping file monitoring")
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
	
	// Create a cancellable context for workers
	s.workerCtx, s.workerCancel = context.WithCancel(s.ctx)
	
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
	
	// First, stop accepting new work - this prevents dispatcher from adding more files
	atomic.StoreInt32(&s.indexingActive, 0)
	atomic.StoreInt32(&s.indexingPaused, 0)
	
	// Give the dispatcher a moment to see the flag change (it runs on 1 second ticker)
	time.Sleep(1100 * time.Millisecond)
	
	// Now drain the work channel to prevent workers from picking up queued files
	if s.workChan != nil {
		s.log.Info("Draining work queue...")
		drained := 0
		drainLoop:
		for {
			select {
			case file := <-s.workChan:
				if file != nil {
					// Reset file status back to pending since we're not processing it
					ctx := context.Background()
					s.updateFileStatus(ctx, file.ID, "pending", "")
					drained++
				}
			default:
				break drainLoop
			}
		}
		if drained > 0 {
			s.log.WithField("count", drained).Info("Drained pending files from work queue")
		}
	}
	
	// Wait for processing count to reach zero
	s.log.Info("Waiting for in-flight processing to complete...")
	startWait := time.Now()
	for {
		count := atomic.LoadInt32(&s.processingCount)
		if count == 0 {
			break
		}
		
		// Add timeout to prevent infinite wait - reduced since we're cancelling context
		if time.Since(startWait) > 15*time.Second {
			s.log.WithField("processing", count).Error("Timeout waiting for processing to complete")
			break
		}
		
		s.log.WithField("processing", count).Info("Files still processing")
		time.Sleep(1 * time.Second)
	}
	
	// Now signal workers to stop
	if s.workersDone != nil {
		close(s.workersDone)
		s.workersDone = nil
	}
	
	// Wait for all workers to exit
	s.log.Info("Waiting for workers to exit...")
	s.workersWG.Wait()
	
	// Final check
	processingCount := atomic.LoadInt32(&s.processingCount)
	if processingCount > 0 {
		s.log.WithField("count", processingCount).Error("Processing count not zero after workers stopped - this is a bug")
	}
	
	s.log.Info("All workers have finished, indexing stopped")
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

// GetProcessingCount returns the number of files currently being processed
func (s *Service) GetProcessingCount() int32 {
	return atomic.LoadInt32(&s.processingCount)
}

// ReindexFailed requeues all failed files for reprocessing
func (s *Service) ReindexFailed() error {
	s.log.Info("Starting reindex of failed files")
	
	// Query for all failed files
	failedFiles, err := s.db.GetFailedFiles(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get failed files: %v", err)
	}
	
	if len(failedFiles) == 0 {
		s.log.Info("No failed files found to reindex")
		return nil
	}
	
	s.log.WithField("count", len(failedFiles)).Info("Found failed files to reindex")
	
	// Reset status of failed files to pending
	for _, filePath := range failedFiles {
		if err := s.db.ResetFileStatus(context.Background(), filePath); err != nil {
			s.log.WithError(err).WithField("file", filePath).Warn("Failed to reset file status")
			continue
		}
	}
	
	s.emitSystemEvent("reindex_failed_started", fmt.Sprintf("Started reindexing %d failed files", len(failedFiles)), "info")
	return nil
}

// GetStartTime returns the service start time
func (s *Service) GetStartTime() time.Time {
	return s.startTime
}

// GetIndexingStatus returns the current indexing status
func (s *Service) GetIndexingStatus() map[string]interface{} {
	return map[string]interface{}{
		"active":     atomic.LoadInt32(&s.indexingActive) == 1,
		"paused":     atomic.LoadInt32(&s.indexingPaused) == 1,
		"scanning":   atomic.LoadInt32(&s.scanningActive) == 1,
		"processing": atomic.LoadInt32(&s.processingCount),
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
func (s *Service) GetStats() *Stats {
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
		"file_changes",
		"document_elements",
		"search_cache",
		"indexing_stats",
	}
	
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := s.db.Exec(ctx, query); err != nil {
			s.log.WithError(err).WithField("table", table).Error("Failed to reset table")
			return fmt.Errorf("failed to reset table %s: %w", table, err)
		}
		s.log.WithField("table", table).Info("Reset table")
	}
	
	// Reclaim disk space with VACUUM FULL
	s.log.Info("Reclaiming disk space after database reset")
	if _, err := s.db.Exec(ctx, "VACUUM FULL"); err != nil {
		s.log.WithError(err).Warn("Failed to reclaim disk space, but reset was successful")
		// Don't fail the reset operation if VACUUM FULL fails
	} else {
		s.log.Info("Database disk space reclaimed successfully")
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

// runIndexingLoop runs the main indexing processing loop with parallel workers
func (s *Service) runIndexingLoop() {
	defer s.wg.Done()
	
	s.log.WithField("workers", s.config.IndexWorkers).Info("Starting indexing loop dispatcher")
	
	// Create work channel for distributing file processing tasks
	s.workChan = make(chan *database.File, s.config.IndexWorkers*2) // Buffer for smooth processing
	s.workersDone = make(chan struct{})
	
	// Initialize worker context with main context as default
	s.workerCtx = s.ctx
	s.workerCancel = func() {} // No-op by default
	
	// Don't start workers here - they will be started when indexing is enabled
	workersStarted := false
	
	// Main dispatcher loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer close(s.workChan) // Close channel when done
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Start workers if indexing becomes active and they're not started
			if atomic.LoadInt32(&s.indexingActive) == 1 && !workersStarted {
				s.log.Info("Starting indexing workers")
				for i := 0; i < s.config.IndexWorkers; i++ {
					s.workersWG.Add(1)
					go s.runIndexingWorker(i, s.workChan)
				}
				workersStarted = true
			}
			
			// Stop workers if indexing becomes inactive and they're started
			if atomic.LoadInt32(&s.indexingActive) == 0 && workersStarted {
				s.log.Info("Stopping indexing workers via dispatcher")
				if s.workersDone != nil {
					close(s.workersDone)
					s.workersDone = make(chan struct{}) // Create new channel for next time
				}
				// Don't wait here - it will block the dispatcher
				// Workers will exit on their own when they see workersDone closed
				workersStarted = false
			}
			
			// Only dispatch work if indexing is active and not paused
			if atomic.LoadInt32(&s.indexingActive) == 1 && 
			   atomic.LoadInt32(&s.indexingPaused) == 0 {
				
				// Apply rate limiting
				if s.rateLimiter.AllowIndexing() {
					s.dispatchNextFile(s.workChan)
				}
			}
		}
	}
}

// runIndexingWorker processes files from the work channel
func (s *Service) runIndexingWorker(workerID int, workChan <-chan *database.File) {
	defer s.workersWG.Done()
	
	s.log.WithField("worker_id", workerID).Info("Starting indexing worker")
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.workersDone:
			s.log.WithField("worker_id", workerID).Info("Worker received stop signal")
			return
		case file, ok := <-workChan:
			if !ok {
				s.log.WithField("worker_id", workerID).Info("Work channel closed, stopping worker")
				return
			}
			
			if file != nil {
				// Check if we should still process (indexing might have been stopped)
				if atomic.LoadInt32(&s.indexingActive) == 0 {
					// Reset file status back to pending since we're not processing it
					ctx := context.Background()
					s.updateFileStatus(ctx, file.ID, "pending", "")
					s.log.WithField("worker_id", workerID).Debug("Skipping file - indexing stopped")
					continue
				}
				
				// Increment processing count
				atomic.AddInt32(&s.processingCount, 1)
				s.processFile(workerID, file)
				// Decrement processing count
				atomic.AddInt32(&s.processingCount, -1)
			}
		}
	}
}

// dispatchNextFile gets the next pending file and sends it to workers
func (s *Service) dispatchNextFile(workChan chan<- *database.File) {
	ctx := context.Background()
	
	// Check if we should stop dispatching
	if atomic.LoadInt32(&s.indexingActive) == 0 {
		return
	}
	
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
	
	// Send to worker (non-blocking)
	select {
	case workChan <- file:
		// File dispatched successfully
	default:
		// All workers are busy, skip this iteration
		// Reset file status back to pending
		s.updateFileStatus(ctx, file.ID, "pending", "")
	}
}

// processFile processes a single file (called by workers)
func (s *Service) processFile(workerID int, file *database.File) {
	// Use the worker context which can be cancelled
	ctx := s.workerCtx
	if ctx == nil {
		// Fallback if worker context not set
		ctx = s.ctx
	}
	start := time.Now()
	
	s.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"file_id":   file.ID,
		"path":      file.Path,
	}).Info("Processing file")
	
	// Process the file with improved error handling
	if err := s.processFileComplete(ctx, file); err != nil {
		// Categorize errors for better handling
		errorCategory := s.categorizeProcessingError(err)
		
		s.log.WithError(err).WithFields(logrus.Fields{
			"worker_id":      workerID,
			"file_id":        file.ID,
			"path":           file.Path,
			"error_category": errorCategory,
		}).Warn("Failed to process file")
		
		// For certain error types, mark as skipped instead of error
		if s.shouldSkipFile(err, errorCategory) {
			s.log.WithFields(logrus.Fields{
				"worker_id": workerID,
				"file_id":   file.ID,
				"path":      file.Path,
				"reason":    err.Error(),
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
		"worker_id": workerID,
		"file_id":   file.ID,
		"path":      file.Path,
		"time_ms":   processingTime,
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
		ORDER BY last_indexed ASC NULLS FIRST, id ASC
		LIMIT 1
	`
	
	var file database.File
	var lastIndexed sql.NullTime
	err := s.db.QueryRow(ctx, query).Scan(
		&file.ID, &file.Path, &file.ParentPath, &file.Filename,
		&file.Extension, &file.FileType, &file.SizeBytes,
		&file.CreatedAt, &file.ModifiedAt, &lastIndexed,
		&file.ContentHash, &file.IndexingStatus, &file.ErrorMessage,
		&file.Metadata,
	)
	
	// Convert NullTime to *time.Time
	if lastIndexed.Valid {
		t := lastIndexed.Time
		file.LastIndexed = &t
	} else {
		file.LastIndexed = nil
	}
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no pending files")
		}
		return nil, err
	}
	
	return &file, nil
}

// storeRoyalMetadata stores the royal metadata from Unstructured extraction
func (s *Service) storeRoyalMetadata(ctx context.Context, fileID int64, metadata map[string]interface{}) error {
	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Update the royal_metadata column
	query := `UPDATE files SET royal_metadata = $1 WHERE id = $2`
	_, err = s.db.Exec(ctx, query, metadataJSON, fileID)
	if err != nil {
		return fmt.Errorf("failed to update royal metadata: %w", err)
	}
	
	return nil
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
	// Check if context is cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Get the extractor that will be used for this file
	var extractorName string
	if extractor := s.extractor.GetExtractorForFile(file.Path); extractor != nil {
		extractorName = extractor.GetName()
		s.log.WithFields(logrus.Fields{
			"file_id":   file.ID,
			"path":      file.Path,
			"extractor": extractorName,
		}).Debug("Using extractor for file")
	}
	
	// Extract content from file
	extractedContent, err := s.extractor.Extract(ctx, file.Path)
	if err != nil {
		return fmt.Errorf("failed to extract content: %w", err)
	}
	
	// Update the extraction_method in the database
	if extractorName != "" {
		updateQuery := `UPDATE files SET extraction_method = $1 WHERE id = $2`
		if _, err := s.db.Exec(ctx, updateQuery, extractorName, file.ID); err != nil {
			s.log.WithError(err).WithField("file_id", file.ID).Warn("Failed to update extraction method")
		}
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
		// Check context before processing each chunk
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
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
	
	// Store royal metadata if available
	s.log.WithFields(logrus.Fields{
		"file_id":      file.ID,
		"has_metadata": extractedContent.Metadata != nil,
		"metadata_len": len(extractedContent.Metadata),
	}).Info("DEBUG: Checking metadata for storage")
	
	if extractedContent.Metadata != nil && len(extractedContent.Metadata) > 0 {
		s.log.WithFields(logrus.Fields{
			"file_id":        file.ID,
			"metadata_count": len(extractedContent.Metadata),
		}).Info("DEBUG: Storing royal metadata")
		
		if err := s.storeRoyalMetadata(ctx, file.ID, extractedContent.Metadata); err != nil {
			// Log but don't fail - metadata is supplementary
			s.log.WithError(err).WithField("file_id", file.ID).Warn("Failed to store royal metadata")
		} else {
			s.log.WithFields(logrus.Fields{
				"file_id":        file.ID,
				"metadata_count": len(extractedContent.Metadata),
			}).Info("Royal metadata stored successfully")
		}
	}
	
	// Process file with NLP (entity extraction and document classification)
	// This is done after chunks are processed to avoid blocking the main pipeline
	// TODO: Re-enable when NLP package is implemented
	// if err := s.ProcessFileWithNLP(ctx, file.ID, extractedContent.Text); err != nil {
	// 	// Log but don't fail - NLP is supplementary
	// 	s.log.WithError(err).WithField("file_id", file.ID).Warn("NLP processing failed but file indexing succeeded")
	// }
	
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
	// Check if context is cancelled before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
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
	
	// Check context again before database operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
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

// StartFileMonitoring starts the file monitoring with paths from database config
func (s *Service) StartFileMonitoring() error {
	// Get current database configuration
	dbConfigService := config.NewDBConfigService(s.db)
	dbConfig, err := dbConfigService.GetConfig(context.Background())
	if err != nil {
		s.log.WithError(err).Error("Failed to get database config for monitoring")
		return err
	}

	// Stop existing monitoring if running
	s.StopFileMonitoring()

	// Update monitor with database paths
	if err := s.monitor.UpdatePaths(dbConfig.WatchPaths, dbConfig.WatchIgnorePatterns); err != nil {
		s.log.WithError(err).Error("Failed to update monitor paths")
		return err
	}

	// Start monitoring
	if err := s.monitor.Start(s.ctx); err != nil {
		s.log.WithError(err).Error("Failed to start file monitoring")
		return err
	}

	atomic.StoreInt32(&s.monitoringActive, 1)
	s.log.WithField("paths", dbConfig.WatchPaths).Info("File monitoring started with database configuration")

	// Start processing file changes
	s.wg.Add(1)
	go s.processFileChanges()

	return nil
}

// StopFileMonitoring stops the file monitoring
func (s *Service) StopFileMonitoring() error {
	if atomic.LoadInt32(&s.monitoringActive) == 0 {
		return nil // Already stopped
	}

	atomic.StoreInt32(&s.monitoringActive, 0)
	if err := s.monitor.Stop(); err != nil {
		s.log.WithError(err).Error("Failed to stop file monitor")
		return err
	}

	s.log.Info("File monitoring stopped")
	return nil
}

// RestartFileMonitoring restarts file monitoring with updated database configuration
func (s *Service) RestartFileMonitoring() error {
	s.log.Info("Restarting file monitoring with updated configuration")
	return s.StartFileMonitoring()
}

// processFileChanges processes file system changes detected by the monitor
func (s *Service) processFileChanges() {
	defer s.wg.Done()
	
	s.log.Info("Starting file changes processor")
	changes := s.monitor.GetChangesChan()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case change, ok := <-changes:
			if !ok {
				s.log.Info("File changes channel closed")
				return
			}
			
			s.log.WithFields(logrus.Fields{
				"path":        change.Path,
				"change_type": change.ChangeType,
				"timestamp":   change.Timestamp,
			}).Debug("Processing file change")
			
			// Handle different change types
			switch change.ChangeType {
			case database.ChangeTypeCreated, database.ChangeTypeModified:
				if err := s.handleFileAddedOrModified(change.Path); err != nil {
					s.log.WithError(err).WithField("path", change.Path).Error("Failed to handle file change")
				}
			case database.ChangeTypeDeleted:
				if err := s.handleFileDeleted(change.Path); err != nil {
					s.log.WithError(err).WithField("path", change.Path).Error("Failed to handle file deletion")
				}
			case database.ChangeTypeRenamed:
				if err := s.handleFileRenamed(change.OldPath, change.Path); err != nil {
					s.log.WithError(err).WithFields(logrus.Fields{
						"old_path": change.OldPath,
						"new_path": change.Path,
					}).Error("Failed to handle file rename")
				}
			}
		}
	}
}

// handleFileAddedOrModified handles when a file is created or modified
func (s *Service) handleFileAddedOrModified(path string) error {
	ctx := context.Background()
	
	// Check if file already exists in database
	var fileID int64
	query := `SELECT id FROM files WHERE path = $1`
	err := s.db.QueryRow(ctx, query, path).Scan(&fileID)
	
	if err != nil {
		// File doesn't exist, add it directly to database for indexing
		if err := s.addFileForMonitoring(path); err != nil {
			return fmt.Errorf("failed to add new file: %w", err)
		}
		s.log.WithField("path", path).Info("Added new file for indexing")
	} else {
		// File exists, mark for re-indexing
		updateQuery := `UPDATE files SET indexing_status = 'pending', last_indexed = NULL WHERE id = $1`
		if _, err := s.db.Exec(ctx, updateQuery, fileID); err != nil {
			return fmt.Errorf("failed to mark file for re-indexing: %w", err)
		}
		s.log.WithField("path", path).Info("Marked modified file for re-indexing")
	}
	
	return nil
}

// handleFileDeleted handles when a file is deleted
func (s *Service) handleFileDeleted(path string) error {
	ctx := context.Background()
	
	// Remove from database (cascading deletes will handle chunks)
	query := `DELETE FROM files WHERE path = $1`
	result, err := s.db.Exec(ctx, query, path)
	if err != nil {
		return fmt.Errorf("failed to delete file from database: %w", err)
	}
	
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		s.log.WithField("path", path).Info("Removed deleted file from database")
	}
	
	return nil
}

// handleFileRenamed handles when a file is renamed
func (s *Service) handleFileRenamed(oldPath, newPath string) error {
	ctx := context.Background()
	
	// Update path in database
	query := `UPDATE files SET path = $1, parent_path = $2 WHERE path = $3`
	newParentPath := filepath.Dir(newPath)
	
	result, err := s.db.Exec(ctx, query, newPath, newParentPath, oldPath)
	if err != nil {
		return fmt.Errorf("failed to update renamed file path: %w", err)
	}
	
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		s.log.WithFields(logrus.Fields{
			"old_path": oldPath,
			"new_path": newPath,
		}).Info("Updated renamed file path in database")
	}
	
	return nil
}

// addFileForMonitoring adds a new file to the database for indexing (used by monitoring system)
func (s *Service) addFileForMonitoring(path string) error {
	ctx := context.Background()
	
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Calculate basic properties
	parentPath := filepath.Dir(path)
	filename := filepath.Base(path)
	extension := strings.ToLower(filepath.Ext(path))
	
	// Determine file type
	var fileType string
	switch extension {
	case ".pdf", ".doc", ".docx", ".rtf":
		fileType = "document"
	case ".txt", ".md", ".csv":
		fileType = "text"
	case ".xls", ".xlsx":
		fileType = "spreadsheet"
	case ".py", ".js", ".go", ".java", ".c", ".cpp", ".rs", ".ts", ".jsx", ".tsx":
		fileType = "code"
	default:
		fileType = "text" // Default fallback
	}
	
	// Insert file into database
	query := `
		INSERT INTO files (path, parent_path, filename, extension, file_type, size_bytes, 
			               created_at, modified_at, indexing_status, last_indexed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', NULL)
	`
	
	_, err = s.db.Exec(ctx, query,
		path,
		parentPath,
		filename,
		extension,
		fileType,
		info.Size(),
		info.ModTime(),
		info.ModTime(),
	)
	
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}
	
	return nil
}
