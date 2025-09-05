package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/file-search/file-search-system/internal/search"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

// Search handlers

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := s.parseJSON(r, &req); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	// Validate request
	if req.Query == "" {
		s.sendError(w, http.StatusBadRequest, "query is required")
		return
	}
	
	// Generate query ID for tracking
	queryID := uuid.New().String()
	
	// Send initial search update
	s.SendSearchUpdate(queryID, "started", 0, 0)
	
	// Convert to search engine request
	searchReq := &search.SearchRequest{
		Query:      req.Query,
		Limit:      req.Limit,
		Offset:     req.Offset,
		FileTypes:  req.FileTypes,
		Extensions: req.Extensions,
		Paths:      req.Paths,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		SearchType: req.SearchType,
	}
	
	// Set defaults
	if searchReq.Limit <= 0 {
		searchReq.Limit = 20
	}
	if searchReq.SearchType == "" {
		searchReq.SearchType = "hybrid"
	}
	
	// Perform search
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	
	results, err := s.searchEngine.Search(ctx, searchReq)
	if err != nil {
		s.log.WithError(err).Error("Search failed")
		s.SendSearchUpdate(queryID, "error", 0, 0)
		s.sendError(w, http.StatusInternalServerError, "search failed")
		return
	}
	
	// Send completion update
	s.SendSearchUpdate(queryID, "completed", len(results.Results), results.SearchTime.Milliseconds())
	
	// Return results
	s.sendSuccess(w, map[string]interface{}{
		"query_id": queryID,
		"results":  results,
	})
}

func (s *Server) handleSearchSuggest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	
	// Generate search suggestions based on indexed content
	suggestions, err := s.getSearchSuggestions(query, limit)
	if err != nil {
		s.log.WithError(err).Error("Failed to get search suggestions")
		s.sendError(w, http.StatusInternalServerError, "failed to get suggestions")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"suggestions": suggestions,
	})
}

func (s *Server) handleSearchHistory(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	
	// Get search history from cache
	history := s.searchEngine.GetSearchHistory(limit)
	
	s.sendSuccess(w, map[string]interface{}{
		"history": history,
	})
}

// File handlers

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	var req FileListRequest
	
	// Parse query parameters
	req.Path = r.URL.Query().Get("path")
	req.Status = r.URL.Query().Get("status")
	
	if types := r.URL.Query().Get("file_types"); types != "" {
		req.FileTypes = []string{types}
	}
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			req.Limit = parsed
		}
	}
	
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			req.Offset = parsed
		}
	}
	
	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 50
	}
	
	// Get files from database
	files, total, err := s.getFiles(&req)
	if err != nil {
		s.log.WithError(err).Error("Failed to list files")
		s.sendError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"files": files,
		"total": total,
		"limit": req.Limit,
		"offset": req.Offset,
	})
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	
	file, err := s.getFileByID(id)
	if err != nil {
		s.log.WithError(err).Error("Failed to get file")
		s.sendError(w, http.StatusNotFound, "file not found")
		return
	}
	
	s.sendSuccess(w, file)
}

func (s *Server) handleGetFileContent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	
	content, err := s.getFileContent(id)
	if err != nil {
		s.log.WithError(err).Error("Failed to get file content")
		s.sendError(w, http.StatusNotFound, "file content not found")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"content": content,
	})
}

func (s *Server) handleReindexFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	
	if err := s.reindexFile(id); err != nil {
		s.log.WithError(err).Error("Failed to reindex file")
		s.sendError(w, http.StatusInternalServerError, "failed to reindex file")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "file queued for reindexing",
	})
}

// Indexing control handlers

func (s *Server) handleStartIndexing(w http.ResponseWriter, r *http.Request) {
	var req IndexingControlRequest
	if err := s.parseJSON(r, &req); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	if err := s.service.StartIndexing(req.Paths, req.Recursive); err != nil {
		s.log.WithError(err).Error("Failed to start indexing")
		s.sendError(w, http.StatusInternalServerError, "failed to start indexing")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "indexing started",
	})
}

func (s *Server) handleStopIndexing(w http.ResponseWriter, r *http.Request) {
	if err := s.service.StopIndexing(); err != nil {
		s.log.WithError(err).Error("Failed to stop indexing")
		s.sendError(w, http.StatusInternalServerError, "failed to stop indexing")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "indexing stopped",
	})
}

func (s *Server) handlePauseIndexing(w http.ResponseWriter, r *http.Request) {
	if err := s.service.PauseIndexing(); err != nil {
		s.log.WithError(err).Error("Failed to pause indexing")
		s.sendError(w, http.StatusInternalServerError, "failed to pause indexing")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "indexing paused",
	})
}

func (s *Server) handleResumeIndexing(w http.ResponseWriter, r *http.Request) {
	if err := s.service.ResumeIndexing(); err != nil {
		s.log.WithError(err).Error("Failed to resume indexing")
		s.sendError(w, http.StatusInternalServerError, "failed to resume indexing")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "indexing resumed",
	})
}

func (s *Server) handleIndexingStatus(w http.ResponseWriter, r *http.Request) {
	status := s.service.GetIndexingStatus()
	
	s.sendSuccess(w, status)
}

func (s *Server) handleIndexingStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.getIndexingStats()
	if err != nil {
		s.log.WithError(err).Error("Failed to get indexing stats")
		s.sendError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	
	s.sendSuccess(w, stats)
}

func (s *Server) handleScanDirectory(w http.ResponseWriter, r *http.Request) {
	var req IndexingControlRequest
	if err := s.parseJSON(r, &req); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	if len(req.Paths) == 0 {
		s.sendError(w, http.StatusBadRequest, "paths are required")
		return
	}
	
	// Start directory scan
	go func() {
		for _, path := range req.Paths {
			if err := s.service.ScanDirectory(path, req.Recursive); err != nil {
				s.log.WithError(err).WithField("path", path).Error("Failed to scan directory")
			}
		}
	}()
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "directory scan started",
		"paths":   req.Paths,
	})
}

// System handlers

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.getSystemStatus()
	if err != nil {
		s.log.WithError(err).Error("Failed to get system status")
		s.sendError(w, http.StatusInternalServerError, "failed to get system status")
		return
	}
	
	s.sendSuccess(w, status)
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	if err := s.db.Ping(ctx); err != nil {
		s.sendError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	
	// Check search engine
	if s.searchEngine == nil {
		s.sendError(w, http.StatusServiceUnavailable, "search engine unavailable")
		return
	}
	
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.getMetrics()
	if err != nil {
		s.log.WithError(err).Error("Failed to get metrics")
		s.sendError(w, http.StatusInternalServerError, "failed to get metrics")
		return
	}
	
	s.sendSuccess(w, metrics)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return sanitized config (remove sensitive values)
	config := map[string]interface{}{
		"api_host":           s.config.APIHost,
		"api_port":           s.config.APIPort,
		"search_weights": map[string]float64{
			"vector":   s.config.SearchVectorWeight,
			"bm25":     s.config.SearchBM25Weight,
			"metadata": s.config.SearchMetadataWeight,
		},
		"indexing": map[string]interface{}{
			"batch_size":      s.config.IndexBatchSize,
			"chunk_size":      s.config.IndexChunkSize,
			"chunk_overlap":   s.config.IndexChunkOverlap,
			"max_file_size":   s.config.IndexMaxFileSizeMB,
		},
		"watch_paths":        s.config.WatchPaths,
		"ignore_patterns":    s.config.WatchIgnorePatterns,
		"cpu_threshold":      s.config.CPUThreshold,
		"memory_threshold":   s.config.MemoryThreshold,
		"log_level":          s.config.LogLevel,
	}
	
	s.sendSuccess(w, config)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := s.parseJSON(r, &updates); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	// Apply configuration updates (implement validation)
	if err := s.updateConfig(updates); err != nil {
		s.log.WithError(err).Error("Failed to update config")
		s.sendError(w, http.StatusBadRequest, "invalid configuration")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "configuration updated",
	})
}

func (s *Server) handleDatabaseReset(w http.ResponseWriter, r *http.Request) {
	if err := s.service.ResetDatabase(); err != nil {
		s.log.WithError(err).Error("Failed to reset database")
		s.sendError(w, http.StatusInternalServerError, "failed to reset database")
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "database reset successfully",
	})
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	s.searchEngine.ClearCache()
	
	s.sendSuccess(w, map[string]interface{}{
		"message": "search cache cleared successfully",
	})
}

// WebSocket handler

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	
	// Add client to map
	s.wsMutex.Lock()
	s.wsClients[conn] = true
	s.wsMutex.Unlock()
	
	s.log.WithField("remote_addr", r.RemoteAddr).Info("New WebSocket connection")
	
	// Send initial status
	status, err := s.getSystemStatus()
	if err == nil {
		s.broadcastWSMessage("system_status", status)
	}
	
	// Handle client messages and cleanup
	go s.handleWSClient(conn)
}

func (s *Server) handleWSClient(conn *websocket.Conn) {
	defer func() {
		// Remove client from map
		s.wsMutex.Lock()
		delete(s.wsClients, conn)
		s.wsMutex.Unlock()
		
		conn.Close()
	}()
	
	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	
	// Read messages (mostly for ping/pong)
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.log.WithError(err).Error("Unexpected WebSocket close")
			}
			break
		}
		
		// Handle ping/pong
		if messageType == websocket.PingMessage {
			conn.WriteMessage(websocket.PongMessage, message)
		}
		
		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}