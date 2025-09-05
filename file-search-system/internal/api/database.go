package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/file-search/file-search-system/internal/database"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Database helper functions for API handlers

// getFiles retrieves files from the database based on request parameters
func (s *Server) getFiles(req *FileListRequest) ([]database.File, int64, error) {
	ctx := context.Background()
	
	// Build query
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT id, path, parent_path, filename, extension, file_type,
		       size_bytes, created_at, modified_at, last_indexed,
		       content_hash, indexing_status, error_message, metadata
		FROM files
	`)
	
	args := []interface{}{}
	argIndex := 1
	conditions := []string{}
	
	// Add WHERE conditions
	if req.Path != "" {
		conditions = append(conditions, fmt.Sprintf("path LIKE $%d", argIndex))
		args = append(args, req.Path+"%")
		argIndex++
	}
	
	if req.Status != "" {
		conditions = append(conditions, fmt.Sprintf("indexing_status = $%d", argIndex))
		args = append(args, req.Status)
		argIndex++
	}
	
	if len(req.FileTypes) > 0 {
		placeholders := make([]string, len(req.FileTypes))
		for i, fileType := range req.FileTypes {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, fileType)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("file_type IN (%s)", strings.Join(placeholders, ",")))
	}
	
	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}
	
	// Add ordering and pagination
	queryBuilder.WriteString(" ORDER BY modified_at DESC")
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1))
	args = append(args, req.Limit, req.Offset)
	
	// Execute query
	rows, err := s.db.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var files []database.File
	for rows.Next() {
		var file database.File
		err := rows.Scan(
			&file.ID, &file.Path, &file.ParentPath, &file.Filename,
			&file.Extension, &file.FileType, &file.SizeBytes,
			&file.CreatedAt, &file.ModifiedAt, &file.LastIndexed,
			&file.ContentHash, &file.IndexingStatus, &file.ErrorMessage,
			&file.Metadata,
		)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, file)
	}
	
	// Get total count
	countQuery := strings.Replace(queryBuilder.String(), 
		"SELECT id, path, parent_path, filename, extension, file_type, size_bytes, created_at, modified_at, last_indexed, content_hash, indexing_status, error_message, metadata FROM files",
		"SELECT COUNT(*) FROM files", 1)
	countQuery = strings.Split(countQuery, " ORDER BY")[0] // Remove ORDER BY and LIMIT
	
	var total int64
	err = s.db.QueryRow(ctx, countQuery, args[:len(args)-2]...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	return files, total, nil
}

// getFileByID retrieves a file by ID
func (s *Server) getFileByID(id int64) (*database.File, error) {
	ctx := context.Background()
	
	query := `
		SELECT id, path, parent_path, filename, extension, file_type,
		       size_bytes, created_at, modified_at, last_indexed,
		       content_hash, indexing_status, error_message, metadata
		FROM files
		WHERE id = $1
	`
	
	var file database.File
	err := s.db.QueryRow(ctx, query, id).Scan(
		&file.ID, &file.Path, &file.ParentPath, &file.Filename,
		&file.Extension, &file.FileType, &file.SizeBytes,
		&file.CreatedAt, &file.ModifiedAt, &file.LastIndexed,
		&file.ContentHash, &file.IndexingStatus, &file.ErrorMessage,
		&file.Metadata,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}
	
	return &file, nil
}

// getFileContent retrieves file content chunks
func (s *Server) getFileContent(fileID int64) (map[string]interface{}, error) {
	ctx := context.Background()
	
	// Get file info
	file, err := s.getFileByID(fileID)
	if err != nil {
		return nil, err
	}
	
	// Get chunks
	query := `
		SELECT id, chunk_index, content, start_line, char_start, char_end, chunk_type, metadata
		FROM chunks
		WHERE file_id = $1
		ORDER BY chunk_index
	`
	
	rows, err := s.db.Query(ctx, query, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var chunks []map[string]interface{}
	fullContent := strings.Builder{}
	
	for rows.Next() {
		var chunk database.Chunk
		err := rows.Scan(
			&chunk.ID, &chunk.ChunkIndex, &chunk.Content,
			&chunk.StartLine, &chunk.CharStart, &chunk.CharEnd,
			&chunk.ChunkType, &chunk.Metadata,
		)
		if err != nil {
			return nil, err
		}
		
		chunkData := map[string]interface{}{
			"id":          chunk.ID,
			"index":       chunk.ChunkIndex,
			"content":     chunk.Content,
			"start_line":  chunk.StartLine,
			"char_start":  chunk.CharStart,
			"char_end":    chunk.CharEnd,
			"chunk_type":  chunk.ChunkType,
			"metadata":    chunk.Metadata,
		}
		chunks = append(chunks, chunkData)
		
		fullContent.WriteString(chunk.Content)
		if chunk.ChunkIndex < len(chunks)-1 {
			fullContent.WriteString("\n")
		}
	}
	
	return map[string]interface{}{
		"file":         file,
		"chunks":       chunks,
		"full_content": fullContent.String(),
	}, nil
}

// reindexFile marks a file for reindexing
func (s *Server) reindexFile(fileID int64) error {
	ctx := context.Background()
	
	// Update file status
	query := `
		UPDATE files 
		SET indexing_status = 'pending', 
		    error_message = NULL,
		    last_indexed = NOW()
		WHERE id = $1
	`
	
	_, err := s.db.Exec(ctx, query, fileID)
	if err != nil {
		return err
	}
	
	// Add to file changes for processing
	file, err := s.getFileByID(fileID)
	if err != nil {
		return err
	}
	
	changeQuery := `
		INSERT INTO file_changes (file_path, change_type, detected_at)
		VALUES ($1, 'modified', NOW())
		ON CONFLICT (file_path) DO UPDATE SET
			detected_at = NOW(),
			processed = FALSE
	`
	
	_, err = s.db.Exec(ctx, changeQuery, file.Path)
	return err
}

// getIndexingStats retrieves indexing statistics
func (s *Server) getIndexingStats() (map[string]interface{}, error) {
	ctx := context.Background()
	
	query := `
		SELECT 
			total_files,
			indexed_files,
			failed_files,
			total_chunks,
			total_size_bytes,
			last_updated
		FROM indexing_stats
		WHERE id = 1
	`
	
	var stats database.IndexingStats
	err := s.db.QueryRow(ctx, query).Scan(
		&stats.TotalFiles,
		&stats.IndexedFiles,
		&stats.FailedFiles,
		&stats.TotalChunks,
		&stats.TotalSizeBytes,
		&stats.LastUpdated,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			// Database is empty (e.g., after reset), return zero stats
			stats = database.IndexingStats{
				TotalFiles:      0,
				IndexedFiles:    0,
				FailedFiles:     0,
				TotalChunks:     0,
				TotalSizeBytes:  0,
				LastUpdated:     time.Now(),
			}
		} else {
			return nil, err
		}
	}
	
	// Get pending files count
	pendingQuery := `SELECT COUNT(*) FROM files WHERE indexing_status = 'pending'`
	var pendingFiles int64
	s.db.QueryRow(ctx, pendingQuery).Scan(&pendingFiles)
	
	// Get processing files count
	processingQuery := `SELECT COUNT(*) FROM files WHERE indexing_status = 'processing'`
	var processingFiles int64
	s.db.QueryRow(ctx, processingQuery).Scan(&processingFiles)
	
	// Get recent activity
	recentQuery := `
		SELECT COUNT(*) 
		FROM files 
		WHERE last_indexed > NOW() - INTERVAL '1 hour'
	`
	var recentlyIndexed int64
	s.db.QueryRow(ctx, recentQuery).Scan(&recentlyIndexed)
	
	return map[string]interface{}{
		"total_files":       stats.TotalFiles,
		"indexed_files":     stats.IndexedFiles,
		"failed_files":      stats.FailedFiles,
		"pending_files":     pendingFiles,
		"processing_files":  processingFiles,
		"total_chunks":      stats.TotalChunks,
		"total_size_bytes":  stats.TotalSizeBytes,
		"recently_indexed":  recentlyIndexed,
		"last_updated":      stats.LastUpdated,
		"index_completion":  float64(stats.IndexedFiles) / float64(stats.TotalFiles) * 100,
	}, nil
}

// getSystemStatus retrieves comprehensive system status
func (s *Server) getSystemStatus() (*SystemStatus, error) {
	// Get resource usage
	resourceUsage, err := s.getResourceUsage()
	if err != nil {
		s.log.WithError(err).Warn("Failed to get resource usage")
		resourceUsage = ResourceUsage{}
	}
	
	// Get indexing stats
	stats, err := s.getIndexingStats()
	if err != nil {
		return nil, err
	}
	
	// Get service status
	indexingStatus := s.service.GetIndexingStatus()
	
	// Get cache stats
	cacheStats := s.searchEngine.GetCacheStats()
	
	status := &SystemStatus{
		Version:        "1.0.0",
		Uptime:         time.Since(s.service.GetStartTime()),
		IndexingActive: indexingStatus["active"].(bool),
		IndexingPaused: indexingStatus["paused"].(bool),
		TotalFiles:     stats["total_files"].(int64),
		IndexedFiles:   stats["indexed_files"].(int64),
		PendingFiles:   stats["pending_files"].(int64),
		FailedFiles:    stats["failed_files"].(int64),
		CacheSize:      cacheStats.Size,
		ResourceUsage:  resourceUsage,
	}
	
	return status, nil
}

// getResourceUsage retrieves system resource usage
func (s *Server) getResourceUsage() (ResourceUsage, error) {
	var usage ResourceUsage
	
	// Get CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		usage.CPUPercent = cpuPercent[0]
	}
	
	// Get memory usage
	memStat, err := mem.VirtualMemory()
	if err == nil {
		usage.MemoryPercent = memStat.UsedPercent
		usage.MemoryUsedMB = memStat.Used / (1024 * 1024)
		usage.MemoryTotalMB = memStat.Total / (1024 * 1024)
	}
	
	// Get disk usage
	diskStat, err := disk.Usage("/")
	if err == nil {
		usage.DiskUsedGB = float64(diskStat.Used) / (1024 * 1024 * 1024)
		usage.DiskTotalGB = float64(diskStat.Total) / (1024 * 1024 * 1024)
	}
	
	return usage, nil
}

// getMetrics retrieves system metrics
func (s *Server) getMetrics() (map[string]interface{}, error) {
	ctx := context.Background()
	
	// Database metrics
	dbMetrics := map[string]interface{}{}
	
	// Connection stats
	connQuery := `
		SELECT 
			sum(numbackends) as active_connections,
			sum(xact_commit) as transactions_committed,
			sum(xact_rollback) as transactions_rolled_back
		FROM pg_stat_database 
		WHERE datname = current_database()
	`
	
	var activeConn, txnCommit, txnRollback sql.NullInt64
	err := s.db.QueryRow(ctx, connQuery).Scan(&activeConn, &txnCommit, &txnRollback)
	if err == nil {
		dbMetrics["active_connections"] = activeConn.Int64
		dbMetrics["transactions_committed"] = txnCommit.Int64
		dbMetrics["transactions_rolled_back"] = txnRollback.Int64
	}
	
	// Table sizes
	sizeQuery := `
		SELECT 
			schemaname,
			tablename,
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
		FROM pg_tables 
		WHERE schemaname = 'public'
		ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
	`
	
	rows, err := s.db.Query(ctx, sizeQuery)
	if err == nil {
		defer rows.Close()
		
		tableSizes := []map[string]interface{}{}
		for rows.Next() {
			var schema, table, size string
			if err := rows.Scan(&schema, &table, &size); err == nil {
				tableSizes = append(tableSizes, map[string]interface{}{
					"table": table,
					"size":  size,
				})
			}
		}
		dbMetrics["table_sizes"] = tableSizes
	}
	
	// Search metrics
	searchMetrics := s.searchEngine.GetCacheStats()
	
	// System metrics
	systemMetrics, _ := s.getResourceUsage()
	
	return map[string]interface{}{
		"database": dbMetrics,
		"search":   searchMetrics,
		"system":   systemMetrics,
		"timestamp": time.Now(),
	}, nil
}

// getSearchSuggestions generates search suggestions
func (s *Server) getSearchSuggestions(query string, limit int) ([]string, error) {
	ctx := context.Background()
	
	// Get suggestions from file names
	filenameQuery := `
		SELECT DISTINCT filename
		FROM files
		WHERE filename ILIKE $1
		AND indexing_status = 'completed'
		ORDER BY filename
		LIMIT $2
	`
	
	rows, err := s.db.Query(ctx, filenameQuery, "%"+query+"%", limit/2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var suggestions []string
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err == nil {
			suggestions = append(suggestions, filename)
		}
	}
	
	// Get suggestions from content (if we have room)
	if len(suggestions) < limit {
		remaining := limit - len(suggestions)
		contentQuery := `
			SELECT DISTINCT word
			FROM ts_stat('SELECT tsv_content FROM text_search WHERE tsv_content @@ plainto_tsquery($1)')
			ORDER BY nentry DESC
			LIMIT $2
		`
		
		rows, err := s.db.Query(ctx, contentQuery, query, remaining)
		if err == nil {
			defer rows.Close()
			
			for rows.Next() {
				var word string
				if err := rows.Scan(&word); err == nil {
					suggestions = append(suggestions, word)
				}
			}
		}
	}
	
	return suggestions, nil
}

// updateConfig updates system configuration
func (s *Server) updateConfig(updates map[string]interface{}) error {
	// Validate and apply configuration updates
	// This would typically involve validating the updates and applying them
	// to the running system. For now, just log the updates.
	
	updateData, _ := json.Marshal(updates)
	s.log.WithField("updates", string(updateData)).Info("Configuration update requested")
	
	// In a real implementation, you would:
	// 1. Validate the updates
	// 2. Apply them to the running configuration
	// 3. Persist them if necessary
	// 4. Notify other components of the changes
	
	return nil
}