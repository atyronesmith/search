
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/file-search/file-search-system/internal/config"
	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/internal/search"
	"github.com/file-search/file-search-system/internal/service"
)

// Server represents the API server
type Server struct {
	config      *config.Config
	db          *database.DB
	dbConfig    *config.DBConfigService
	searchEngine *search.Engine
	service     *service.Service
	router      *mux.Router
	httpServer  *http.Server
	wsUpgrader  websocket.Upgrader
	wsClients   map[*websocket.Conn]bool
	wsMutex     sync.RWMutex
	log         *logrus.Logger
	rateLimiter *RateLimiter
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, db *database.DB, svc *service.Service, log *logrus.Logger) *Server {
	s := &Server{
		config:       cfg,
		db:           db,
		dbConfig:     config.NewDBConfigService(db),
		service:      svc,
		searchEngine: svc.GetSearchEngine(), // Get search engine from service
		log:          log,
		wsClients:    make(map[*websocket.Conn]bool),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development, restrict in production
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		rateLimiter: NewRateLimiter(100, time.Minute), // 100 requests per minute
	}

	s.setupRoutes()
	return s
}

// Start starts the API server
func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.log.WithField("address", addr).Info("Starting API server")
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the API server
func (s *Server) Stop() error {
	s.log.Info("Stopping API server")

	// Close all WebSocket connections
	s.wsMutex.Lock()
	for conn := range s.wsClients {
		conn.Close()
	}
	s.wsMutex.Unlock()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	s.router = mux.NewRouter()

	// Apply middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
	s.router.Use(s.recoveryMiddleware)
	s.router.Use(s.rateLimitMiddleware)

	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Search endpoints
	api.HandleFunc("/search", s.handleSearch).Methods("POST", "OPTIONS")
	api.HandleFunc("/search/suggest", s.handleSearchSuggest).Methods("GET", "OPTIONS")
	api.HandleFunc("/search/history", s.handleSearchHistory).Methods("GET", "OPTIONS")

	// File endpoints
	api.HandleFunc("/files", s.handleListFiles).Methods("GET", "OPTIONS")
	api.HandleFunc("/files/{id}", s.handleGetFile).Methods("GET", "OPTIONS")
	api.HandleFunc("/files/{id}/content", s.handleGetFileContent).Methods("GET", "OPTIONS")
	api.HandleFunc("/files/{id}/reindex", s.handleReindexFile).Methods("POST", "OPTIONS")

	// Indexing control endpoints
	api.HandleFunc("/indexing/start", s.handleStartIndexing).Methods("POST", "OPTIONS")
	api.HandleFunc("/indexing/stop", s.handleStopIndexing).Methods("POST", "OPTIONS")
	api.HandleFunc("/indexing/pause", s.handlePauseIndexing).Methods("POST", "OPTIONS")
	api.HandleFunc("/indexing/resume", s.handleResumeIndexing).Methods("POST", "OPTIONS")
	api.HandleFunc("/indexing/status", s.handleIndexingStatus).Methods("GET", "OPTIONS")

	// File monitoring control endpoints
	api.HandleFunc("/monitoring/start", s.handleStartMonitoring).Methods("POST", "OPTIONS")
	api.HandleFunc("/monitoring/stop", s.handleStopMonitoring).Methods("POST", "OPTIONS")
	api.HandleFunc("/monitoring/restart", s.handleRestartMonitoring).Methods("POST", "OPTIONS")
	api.HandleFunc("/monitoring/status", s.handleMonitoringStatus).Methods("GET", "OPTIONS")

	// Database reset endpoint
	api.HandleFunc("/database/reset", s.handleDatabaseReset).Methods("POST", "OPTIONS")
	
	// Cache management endpoints
	api.HandleFunc("/cache/clear", s.handleClearCache).Methods("POST", "OPTIONS")

	// System endpoints
	api.HandleFunc("/status", s.handleSystemStatus).Methods("GET", "OPTIONS")
	api.HandleFunc("/health", s.handleHealthCheck).Methods("GET", "OPTIONS")
	api.HandleFunc("/metrics", s.handleMetrics).Methods("GET", "OPTIONS")
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET", "OPTIONS")
	api.HandleFunc("/config", s.handleUpdateConfig).Methods("PUT", "OPTIONS")
	
	// Ollama endpoints
	api.HandleFunc("/ollama/models", s.handleGetOllamaModels).Methods("GET", "OPTIONS")

	// WebSocket endpoint
	api.HandleFunc("/ws", s.handleWebSocket)

	// Static file serving (for frontend)
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/static/")))
}

// Response types

// Response types

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type SearchRequest struct {
	Query      string                 `json:"query"`
	Limit      int                    `json:"limit,omitempty"`
	Offset     int                    `json:"offset,omitempty"`
	FileTypes  []string               `json:"file_types,omitempty"`
	Extensions []string               `json:"extensions,omitempty"`
	Paths      []string               `json:"paths,omitempty"`
	DateFrom   *time.Time             `json:"date_from,omitempty"`
	DateTo     *time.Time             `json:"date_to,omitempty"`
	SearchType string                 `json:"search_type,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
}

type FileListRequest struct {
	Path      string   `json:"path,omitempty"`
	FileTypes []string `json:"file_types,omitempty"`
	Status    string   `json:"status,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
}

type IndexingControlRequest struct {
	Paths     []string `json:"paths,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`
	Force     bool     `json:"force,omitempty"`
}

type SystemStatus struct {
	Version          string                 `json:"version"`
	Uptime           time.Duration          `json:"uptime"`
	IndexingActive   bool                   `json:"indexing_active"`
	IndexingPaused   bool                   `json:"indexing_paused"`
	TotalFiles       int64                  `json:"total_files"`
	IndexedFiles     int64                  `json:"indexed_files"`
	PendingFiles     int64                  `json:"pending_files"`
	FailedFiles      int64                  `json:"failed_files"`
	DatabaseSize     int64                  `json:"database_size"`
	DatabaseSizeInfo map[string]interface{} `json:"database_size_info"`
	CacheSize        int                    `json:"cache_size"`
	ActiveSearches   int                    `json:"active_searches"`
	ResourceUsage    ResourceUsage          `json:"resource_usage"`
}

type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsedMB  uint64  `json:"memory_used_mb"`
	MemoryTotalMB uint64  `json:"memory_total_mb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
}

// Helper functions

func (s *Server) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.log.WithError(err).Error("Failed to encode JSON response")
	}
}

func (s *Server) sendError(w http.ResponseWriter, status int, message string) {
	s.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

func (s *Server) sendSuccess(w http.ResponseWriter, data interface{}) {
	s.sendJSON(w, http.StatusOK, SuccessResponse{
		Success: true,
		Data:    data,
	})
}

func (s *Server) parseJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// WebSocket message types

type WSMessage struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type WSIndexingUpdate struct {
	Status       string  `json:"status"`
	CurrentFile  string  `json:"current_file"`
	Progress     float64 `json:"progress"`
	FilesIndexed int64   `json:"files_indexed"`
	TotalFiles   int64   `json:"total_files"`
	ErrorCount   int     `json:"error_count"`
}

type WSSearchUpdate struct {
	QueryID     string `json:"query_id"`
	Status      string `json:"status"`
	ResultCount int    `json:"result_count"`
	SearchTime  int64  `json:"search_time_ms"`
}

// broadcastWSMessage sends a message to all connected WebSocket clients
func (s *Server) broadcastWSMessage(msgType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.log.WithError(err).Error("Failed to marshal WebSocket message")
		return
	}

	msg := WSMessage{
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      jsonData,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		s.log.WithError(err).Error("Failed to marshal WebSocket message wrapper")
		return
	}

	s.wsMutex.RLock()
	defer s.wsMutex.RUnlock()

	for conn := range s.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			s.log.WithError(err).Debug("Failed to send WebSocket message")
			// Client will be cleaned up on next read/write
		}
	}
}

// SendIndexingUpdate sends an indexing status update to WebSocket clients
func (s *Server) SendIndexingUpdate(status, currentFile string, progress float64, filesIndexed, totalFiles int64, errorCount int) {
	update := WSIndexingUpdate{
		Status:       status,
		CurrentFile:  currentFile,
		Progress:     progress,
		FilesIndexed: filesIndexed,
		TotalFiles:   totalFiles,
		ErrorCount:   errorCount,
	}

	s.broadcastWSMessage("indexing_update", update)
}

// SendSearchUpdate sends a search status update to WebSocket clients
func (s *Server) SendSearchUpdate(queryID, status string, resultCount int, searchTimeMs int64) {
	update := WSSearchUpdate{
		QueryID:     queryID,
		Status:      status,
		ResultCount: resultCount,
		SearchTime:  searchTimeMs,
	}

	s.broadcastWSMessage("search_update", update)
}