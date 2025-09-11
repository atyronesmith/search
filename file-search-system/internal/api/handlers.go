package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/file-search/file-search-system/internal/search"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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

	// Log the incoming search type
	s.log.WithFields(logrus.Fields{
		"query":                req.Query,
		"incoming_search_type": req.SearchType,
	}).Info("DEBUG: API received search request")

	// If search type is empty or "hybrid", let the engine decide based on query complexity
	searchType := req.SearchType
	if searchType == "" || searchType == "hybrid" {
		searchType = "" // Let engine determine
	}

	// Convert to search engine request
	searchReq := &search.Request{
		Query:      req.Query,
		Limit:      req.Limit,
		Offset:     req.Offset,
		FileTypes:  req.FileTypes,
		Extensions: req.Extensions,
		Paths:      req.Paths,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		SearchType: searchType,
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

	// Parse sorting parameters
	req.SortBy = r.URL.Query().Get("sort_by")
	req.SortDir = r.URL.Query().Get("sort_dir")

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 50
	}

	// Default sort by filename if not specified
	if req.SortBy == "" {
		req.SortBy = "filename"
	}

	// Default sort direction to ascending if not specified
	if req.SortDir == "" {
		req.SortDir = "asc"
	}

	// Get files from database
	files, total, err := s.getFiles(&req)
	if err != nil {
		s.log.WithError(err).Error("Failed to list files")
		s.sendError(w, http.StatusInternalServerError, "failed to list files")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"files":  files,
		"total":  total,
		"limit":  req.Limit,
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

	// If no paths provided, use configured watch paths from database
	paths := req.Paths
	if len(paths) == 0 {
		// Try to get watch paths from database config first
		dbConfig, err := s.dbConfig.GetConfig(r.Context())
		if err != nil {
			s.log.WithError(err).Warn("Failed to get database config, falling back to environment config")
			paths = s.config.WatchPaths
		} else {
			paths = dbConfig.WatchPaths
		}
		s.log.WithField("paths", paths).Info("Using configured watch paths for indexing")
	}

	if err := s.service.StartIndexing(paths, req.Recursive); err != nil {
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

func (s *Server) handleReindexFailed(w http.ResponseWriter, r *http.Request) {
	if err := s.service.ReindexFailed(); err != nil {
		s.log.WithError(err).Error("Failed to reindex failed files")
		s.sendError(w, http.StatusInternalServerError, "failed to reindex failed files")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"message": "reindexing failed files started",
	})
}

func (s *Server) handleIndexingStatus(w http.ResponseWriter, r *http.Request) {
	status := s.service.GetIndexingStatus()

	s.sendSuccess(w, status)
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
	// Use database configuration service if available
	if s.dbConfig != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		config, err := s.dbConfig.GetConfigMap(ctx)
		if err != nil {
			s.log.WithError(err).Error("Failed to get config from database")
			s.sendError(w, http.StatusInternalServerError, "failed to get configuration")
			return
		}

		s.sendSuccess(w, config)
		return
	}

	// Fallback to file-based config (legacy)
	config := map[string]interface{}{
		"api_host": s.config.APIHost,
		"api_port": s.config.APIPort,
		"search_weights": map[string]float64{
			"vector":   s.config.SearchVectorWeight,
			"bm25":     s.config.SearchBM25Weight,
			"metadata": s.config.SearchMetadataWeight,
		},
		"indexing": map[string]interface{}{
			"batch_size":    s.config.IndexBatchSize,
			"chunk_size":    s.config.IndexChunkSize,
			"chunk_overlap": s.config.IndexChunkOverlap,
			"max_file_size": s.config.IndexMaxFileSizeMB,
		},
		"watch_paths":      s.config.WatchPaths,
		"ignore_patterns":  s.config.WatchIgnorePatterns,
		"cpu_threshold":    s.config.CPUThreshold,
		"memory_threshold": s.config.MemoryThreshold,
		"log_level":        s.config.LogLevel,
	}

	s.sendSuccess(w, config)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := s.parseJSON(r, &updates); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Use database configuration service if available
	if s.dbConfig != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if err := s.dbConfig.UpdateConfig(ctx, updates); err != nil {
			s.log.WithError(err).Error("Failed to update config in database")
			s.sendError(w, http.StatusInternalServerError, "failed to update configuration")
			return
		}

		s.log.WithField("updates", updates).Info("Configuration updated in database")

		// Check if watch_paths or ignore_patterns were updated
		if _, hasWatchPaths := updates["watch_paths"]; hasWatchPaths {
			s.log.Info("Watch paths updated, restarting file monitoring")
			if err := s.service.RestartFileMonitoring(); err != nil {
				s.log.WithError(err).Error("Failed to restart file monitoring after config update")
				// Don't fail the config update if monitoring restart fails
			}
		}

		s.sendSuccess(w, map[string]interface{}{
			"message": "configuration updated successfully",
		})
		return
	}

	// Fallback to legacy file-based config update
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

	// Also clear LLM enhancer cache if available
	if s.searchEngine.GetLLMEnhancer() != nil {
		s.searchEngine.GetLLMEnhancer().ClearCache()
	}

	s.sendSuccess(w, map[string]interface{}{
		"message": "search cache cleared successfully",
	})
}

// handleQueryPerformanceStats returns query classification performance statistics
func (s *Server) handleQueryPerformanceStats(w http.ResponseWriter, r *http.Request) {
	if s.searchEngine.GetLLMEnhancer() == nil {
		s.sendError(w, http.StatusServiceUnavailable, "LLM enhancer not available")
		return
	}

	llmEnhancer := s.searchEngine.GetLLMEnhancer()

	// Get performance metrics
	perfMetrics := llmEnhancer.GetPerformanceMetrics()

	// Get cache statistics
	cacheHits, cacheMisses, cacheHitRate := llmEnhancer.GetCacheStats()

	// Combine all statistics
	stats := map[string]interface{}{
		"performance": perfMetrics,
		"cache": map[string]interface{}{
			"hits":     cacheHits,
			"misses":   cacheMisses,
			"hit_rate": cacheHitRate,
		},
	}

	// Log the stats for debugging
	llmEnhancer.LogPerformanceStats()

	s.sendSuccess(w, stats)
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

		if err := conn.Close(); err != nil {
			s.log.WithError(err).Error("Failed to close WebSocket connection")
		}
	}()

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		s.log.WithError(err).Error("Failed to set WebSocket read deadline")
		return
	}

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
			if err := conn.WriteMessage(websocket.PongMessage, message); err != nil {
				s.log.WithError(err).Error("Failed to write PongMessage to WebSocket")
				break
			}
		}

		// Reset read deadline
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			s.log.WithError(err).Error("Failed to reset WebSocket read deadline")
			break
		}
	}
}

// Ollama handlers

func (s *Server) handleGetOllamaModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	models, err := s.dbConfig.GetOllamaModels(ctx)
	if err != nil {
		s.log.WithError(err).Error("Failed to get Ollama models")
		s.sendError(w, http.StatusInternalServerError, "failed to get Ollama models")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"models": models,
	})
}

func (s *Server) handleGetCurrentLLMModel(w http.ResponseWriter, r *http.Request) {
	modelName := s.searchEngine.GetLLMModelName()
	s.sendSuccess(w, map[string]interface{}{
		"model": modelName,
	})
}

func (s *Server) handleGetLLMDebugInfo(w http.ResponseWriter, r *http.Request) {
	debugInfo := s.searchEngine.GetLLMDebugInfo()
	s.sendSuccess(w, map[string]interface{}{
		"debug_info": debugInfo,
	})
}

// Prompt management handlers

func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var promptValue string
	query := `SELECT config_value FROM system_config WHERE config_key = 'llm_prompt_template'`
	err := s.db.QueryRow(ctx, query).Scan(&promptValue)
	if err != nil {
		s.log.WithError(err).Error("Failed to fetch prompt template from database")
		s.sendError(w, http.StatusInternalServerError, "failed to fetch prompt template")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"prompt": promptValue,
	})
}

func (s *Server) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Prompt string `json:"prompt"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if request.Prompt == "" {
		s.sendError(w, http.StatusBadRequest, "prompt cannot be empty")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE system_config SET config_value = $1, updated_at = CURRENT_TIMESTAMP
	          WHERE config_key = 'llm_prompt_template'`
	result, err := s.db.Exec(ctx, query, request.Prompt)
	if err != nil {
		s.log.WithError(err).Error("Failed to update prompt template in database")
		s.sendError(w, http.StatusInternalServerError, "failed to update prompt template")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.sendError(w, http.StatusNotFound, "prompt template not found in database")
		return
	}

	// Reload the cached prompt template in the search engine
	if s.searchEngine != nil {
		s.searchEngine.ReloadPromptTemplate()
		s.log.Info("Prompt template cache reloaded successfully")
	}

	s.log.Info("Prompt template updated successfully")
	s.sendSuccess(w, map[string]interface{}{
		"message": "Prompt template updated successfully",
		"prompt":  request.Prompt,
	})
}

// Monitoring handlers

func (s *Server) handleStartMonitoring(w http.ResponseWriter, r *http.Request) {
	if err := s.service.StartFileMonitoring(); err != nil {
		s.log.WithError(err).Error("Failed to start file monitoring")
		s.sendError(w, http.StatusInternalServerError, "failed to start file monitoring")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"message": "file monitoring started",
	})
}

func (s *Server) handleStopMonitoring(w http.ResponseWriter, r *http.Request) {
	if err := s.service.StopFileMonitoring(); err != nil {
		s.log.WithError(err).Error("Failed to stop file monitoring")
		s.sendError(w, http.StatusInternalServerError, "failed to stop file monitoring")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"message": "file monitoring stopped",
	})
}

func (s *Server) handleRestartMonitoring(w http.ResponseWriter, r *http.Request) {
	if err := s.service.RestartFileMonitoring(); err != nil {
		s.log.WithError(err).Error("Failed to restart file monitoring")
		s.sendError(w, http.StatusInternalServerError, "failed to restart file monitoring")
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"message": "file monitoring restarted",
	})
}

func (s *Server) handleMonitoringStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.service.GetStats()

	s.sendSuccess(w, map[string]interface{}{
		"active": stats.MonitoringActive,
	})
}
