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
	// Map frontend column names to database column names
	columnMap := map[string]string{
		"filename":        "filename",
		"file_type":       "file_type",
		"indexing_status": "indexing_status",
		"size_bytes":      "size_bytes",
		"modified_at":     "modified_at",
	}

	// Determine sort column (default to filename if not specified or invalid)
	sortColumn := "filename"
	if req.SortBy != "" {
		if col, ok := columnMap[req.SortBy]; ok {
			sortColumn = col
		}
	}

	// Determine sort direction (default to ASC for filename, DESC for others)
	sortDir := "ASC"
	if req.SortDir != "" {
		if strings.ToLower(req.SortDir) == "desc" {
			sortDir = "DESC"
		}
	} else if sortColumn == "modified_at" {
		// Default to DESC for modified_at only if no direction specified
		sortDir = "DESC"
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", sortColumn, sortDir))
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

	// Get total count - build separate count query
	countQueryBuilder := strings.Builder{}
	countQueryBuilder.WriteString("SELECT COUNT(*) FROM files")

	if len(conditions) > 0 {
		countQueryBuilder.WriteString(" WHERE ")
		countQueryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	var total int64
	err = s.db.QueryRow(ctx, countQueryBuilder.String(), args[:len(args)-2]...).Scan(&total)
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
	err = s.db.QueryRow(ctx, pendingQuery).Scan(&pendingFiles)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get pending files count")
		pendingFiles = 0
	}

	// Get processing files count
	processingQuery := `SELECT COUNT(*) FROM files WHERE indexing_status = 'processing'`
	var processingFiles int64
	err = s.db.QueryRow(ctx, processingQuery).Scan(&processingFiles)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get processing files count")
		processingFiles = 0
	}

	// Get actual file counts from database (more accurate than indexing_stats table)
	actualCountsQuery := `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN indexing_status = 'completed' THEN 1 END) as completed,
			COUNT(CASE WHEN indexing_status = 'failed' THEN 1 END) as failed,
			COUNT(CASE WHEN indexing_status = 'skipped' THEN 1 END) as skipped
		FROM files`
	var actualTotal, actualCompleted, actualFailed, actualSkipped int64
	err = s.db.QueryRow(ctx, actualCountsQuery).Scan(&actualTotal, &actualCompleted, &actualFailed, &actualSkipped)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get actual file counts, using indexing_stats")
		actualTotal = stats.TotalFiles
		actualCompleted = stats.IndexedFiles
		actualFailed = stats.FailedFiles
		actualSkipped = 0
	}

	// Use actual counts instead of potentially stale indexing_stats
	stats.TotalFiles = actualTotal
	stats.IndexedFiles = actualCompleted
	stats.FailedFiles = actualFailed // Keep failed separate

	// Get recent activity
	recentQuery := `
		SELECT COUNT(*)
		FROM files
		WHERE last_indexed > NOW() - INTERVAL '1 hour'
	`
	var recentlyIndexed int64
	err = s.db.QueryRow(ctx, recentQuery).Scan(&recentlyIndexed)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get recently indexed files count")
		recentlyIndexed = 0
	}

	// Get file type breakdown for successfully indexed files
	fileTypeQuery := `
		SELECT
			LOWER(extension) as ext,
			COUNT(*) as count
		FROM files
		WHERE indexing_status = 'completed' AND extension IS NOT NULL
		GROUP BY LOWER(extension)
		ORDER BY count DESC
		LIMIT 20
	`

	fileTypeBreakdown := []map[string]interface{}{}
	ftRows, err := s.db.Query(ctx, fileTypeQuery)
	if err == nil {
		defer ftRows.Close()
		for ftRows.Next() {
			var ext string
			var count int64
			if err := ftRows.Scan(&ext, &count); err == nil {
				// Map extensions to readable file types
				fileType := getFileTypeFromExtension(ext)
				fileTypeBreakdown = append(fileTypeBreakdown, map[string]interface{}{
					"extension": ext,
					"type":      fileType,
					"count":     count,
				})
			}
		}
	}

	// Get database disk usage
	dbSizeQuery := `
		SELECT
			pg_size_pretty(pg_database_size(current_database())) as total_db_size,
			pg_size_pretty(pg_total_relation_size('files')) as files_table_size,
			pg_size_pretty(pg_total_relation_size('chunks')) as chunks_table_size,
			pg_size_pretty(pg_total_relation_size('text_search')) as text_search_table_size,
			pg_database_size(current_database()) as total_db_size_bytes,
			pg_total_relation_size('files') as files_table_size_bytes,
			pg_total_relation_size('chunks') as chunks_table_size_bytes,
			pg_total_relation_size('text_search') as text_search_table_size_bytes
	`

	var totalDBSize, filesTableSize, chunksTableSize, textSearchTableSize string
	var totalDBSizeBytes, filesTableSizeBytes, chunksTableSizeBytes, textSearchTableSizeBytes int64
	err = s.db.QueryRow(ctx, dbSizeQuery).Scan(
		&totalDBSize, &filesTableSize, &chunksTableSize, &textSearchTableSize,
		&totalDBSizeBytes, &filesTableSizeBytes, &chunksTableSizeBytes, &textSearchTableSizeBytes,
	)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get database size information")
		// Set default values on error
		totalDBSize = "N/A"
		filesTableSize = "N/A"
		chunksTableSize = "N/A"
		textSearchTableSize = "N/A"
		totalDBSizeBytes = 0
		filesTableSizeBytes = 0
		chunksTableSizeBytes = 0
		textSearchTableSizeBytes = 0
	}

	return map[string]interface{}{
		"total_files":       stats.TotalFiles,
		"indexed_files":     stats.IndexedFiles,
		"failed_files":      actualFailed,     // Explicitly use actualFailed
		"skipped_files":     actualSkipped,    // Add separate skipped count
		"pending_files":     pendingFiles,
		"processing_files":  processingFiles,
		"total_chunks":      stats.TotalChunks,
		"total_size_bytes":  stats.TotalSizeBytes,
		"recently_indexed":  recentlyIndexed,
		"last_updated":      stats.LastUpdated,
		"index_completion":  float64(stats.IndexedFiles) / float64(stats.TotalFiles) * 100,
		"database_size": map[string]interface{}{
			"total_db_size":           totalDBSize,
			"files_table_size":        filesTableSize,
			"chunks_table_size":       chunksTableSize,
			"text_search_table_size":  textSearchTableSize,
			"total_db_size_bytes":     totalDBSizeBytes,
			"files_table_size_bytes":  filesTableSizeBytes,
			"chunks_table_size_bytes": chunksTableSizeBytes,
			"text_search_table_size_bytes": textSearchTableSizeBytes,
		},
		"file_type_breakdown": fileTypeBreakdown, // Add file type statistics
	}, nil
}

// getFileTypeFromExtension maps file extensions to readable file type categories
func getFileTypeFromExtension(ext string) string {
	// Remove leading dot if present
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ToLower(ext)

	switch ext {
	// Documents
	case "pdf":
		return "PDF Document"
	case "doc", "docx":
		return "Word Document"
	case "xls", "xlsx":
		return "Spreadsheet"
	case "ppt", "pptx":
		return "Presentation"
	case "txt":
		return "Text File"
	case "rtf":
		return "Rich Text"
	case "md", "markdown":
		return "Markdown"

	// Code files
	case "go":
		return "Go Source"
	case "js", "javascript":
		return "JavaScript"
	case "ts", "tsx":
		return "TypeScript"
	case "py":
		return "Python"
	case "java":
		return "Java"
	case "c", "cpp", "cc", "h", "hpp":
		return "C/C++"
	case "cs":
		return "C#"
	case "rb":
		return "Ruby"
	case "php":
		return "PHP"
	case "swift":
		return "Swift"
	case "rs":
		return "Rust"
	case "kt":
		return "Kotlin"

	// Web files
	case "html", "htm":
		return "HTML"
	case "css", "scss", "sass":
		return "Stylesheet"
	case "json":
		return "JSON"
	case "xml":
		return "XML"
	case "yaml", "yml":
		return "YAML"

	// Data files
	case "csv":
		return "CSV Data"
	case "sql":
		return "SQL Script"

	// Image files (if we index metadata)
	case "jpg", "jpeg", "png", "gif", "bmp", "svg":
		return "Image"

	default:
		// Capitalize first letter and add "File"
		if len(ext) > 0 {
			return strings.ToUpper(string(ext[0])) + strings.ToLower(ext[1:]) + " File"
		}
		return "Other"
	}
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

	// Extract database size from stats
	var databaseSize int64 = 0
	var databaseSizeInfo map[string]interface{}
	if dbSizeInfo, ok := stats["database_size"].(map[string]interface{}); ok {
		databaseSizeInfo = dbSizeInfo
		if totalSizeBytes, ok := dbSizeInfo["total_db_size_bytes"].(int64); ok {
			databaseSize = totalSizeBytes
		}
	}

	// Extract file type breakdown from stats
	var fileTypeBreakdown []map[string]interface{}
	if ftBreakdown, ok := stats["file_type_breakdown"].([]map[string]interface{}); ok {
		fileTypeBreakdown = ftBreakdown
	}

	// Extract skipped files count
	var skippedFiles int64 = 0
	if skipped, ok := stats["skipped_files"].(int64); ok {
		skippedFiles = skipped
	}

	status := &SystemStatus{
		Version:           "1.0.0",
		Uptime:            time.Since(s.service.GetStartTime()),
		IndexingActive:    indexingStatus["active"].(bool),
		IndexingPaused:    indexingStatus["paused"].(bool),
		TotalFiles:        stats["total_files"].(int64),
		IndexedFiles:      stats["indexed_files"].(int64),
		PendingFiles:      stats["pending_files"].(int64),
		FailedFiles:       stats["failed_files"].(int64),
		SkippedFiles:      skippedFiles,
		DatabaseSize:      databaseSize,
		DatabaseSizeInfo:  databaseSizeInfo,
		FileTypeBreakdown: fileTypeBreakdown,
		CacheSize:         cacheStats.Size,
		ResourceUsage:     resourceUsage,
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