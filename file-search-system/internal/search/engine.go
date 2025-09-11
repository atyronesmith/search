package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/internal/embeddings"
	"github.com/pgvector/pgvector-go"
	"github.com/sirupsen/logrus"
)

// Engine represents the hybrid search engine
type Engine struct {
	db          *database.DB
	embedder    *embeddings.OllamaClient
	config      *Config
	log         *logrus.Logger
	cache       *Cache
	processor   *QueryProcessor
	llmEnhancer *LLMEnhancer
	// Track last executed queries for debug purposes
	lastVectorQuery string
	lastTextQuery   string
	// Store debug info for non-LLM queries
	lastDebugInfo *DebugInfo
}

// Config holds search engine configuration
type Config struct {
	VectorWeight   float64       `json:"vector_weight"`
	BM25Weight     float64       `json:"bm25_weight"`
	MetadataWeight float64       `json:"metadata_weight"`
	DefaultLimit   int           `json:"default_limit"`
	CacheTTL       time.Duration `json:"cache_ttl"`
	MinScore       float64       `json:"min_score"`
}

// DefaultConfig returns default search configuration
func DefaultConfig() *Config {
	return &Config{
		VectorWeight:   0.4,
		BM25Weight:     0.5,
		MetadataWeight: 0.1,
		DefaultLimit:   20,
		CacheTTL:       15 * time.Minute,
		MinScore:       0.2,
	}
}

// substituteSQLParams replaces PostgreSQL placeholders ($1, $2, etc.) with actual values for debug display
func substituteSQLParams(query string, args []interface{}) string {
	result := query
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		var valueStr string
		switch v := arg.(type) {
		case string:
			valueStr = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
		case []byte:
			valueStr = fmt.Sprintf("'\\x%s'", hex.EncodeToString(v))
		case []float32:
			valueStr = fmt.Sprintf("'[%v]'", v)
		case nil:
			valueStr = "NULL"
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	return result
}

// Request represents a search query
type Request struct {
	Query          string                 `json:"query"`
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
	FileTypes      []string               `json:"file_types,omitempty"`
	Extensions     []string               `json:"extensions,omitempty"`
	Paths          []string               `json:"paths,omitempty"`
	DateFrom       *time.Time             `json:"date_from,omitempty"`
	DateTo         *time.Time             `json:"date_to,omitempty"`
	MinSize        int64                  `json:"min_size,omitempty"`
	MaxSize        int64                  `json:"max_size,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	SearchType     string                 `json:"search_type"`               // "hybrid", "vector", "text"
	ContentFilters []ContentFilter        `json:"content_filters,omitempty"` // LLM-derived content filters
	RequiresCount  bool                   `json:"requires_count,omitempty"`  // Whether this is a counting query
}

// Response represents search results
type Response struct {
	Query         string         `json:"query"`
	EnhancedQuery *EnhancedQuery `json:"enhanced_query,omitempty"` // LLM-enhanced query details
	Results       []Result       `json:"results"`
	TotalCount    int            `json:"total_count"`
	SearchTime    time.Duration  `json:"search_time"`
	Cached        bool           `json:"cached"`
	UsedLLM       bool           `json:"used_llm"` // Whether LLM enhancement was used
}

// Result represents a single search result
type Result struct {
	FileID        int64                  `json:"file_id"`
	ChunkID       int64                  `json:"chunk_id"`
	FilePath      string                 `json:"file_path"`
	Filename      string                 `json:"filename"`
	FileType      string                 `json:"file_type"`
	Content       string                 `json:"content"`
	Score         float64                `json:"score"`
	VectorScore   float64                `json:"vector_score"`
	TextScore     float64                `json:"text_score"`
	MetadataScore float64                `json:"metadata_score"`
	Highlights    []string               `json:"highlights"`
	StartLine     *int                   `json:"start_line,omitempty"`
	EndLine       *int                   `json:"end_line,omitempty"`
	CharStart     int                    `json:"char_start"`
	CharEnd       int                    `json:"char_end"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewEngine creates a new search engine
func NewEngine(db *database.DB, embedder *embeddings.OllamaClient, config *Config, log *logrus.Logger, ollamaHost string, llmModel string) *Engine {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize LLM enhancer with configurable model
	llmEnhancer := NewLLMEnhancer(ollamaHost, llmModel, db)

	return &Engine{
		db:          db,
		embedder:    embedder,
		config:      config,
		log:         log,
		cache:       NewCache(config.CacheTTL),
		processor:   NewQueryProcessor(),
		llmEnhancer: llmEnhancer,
	}
}

// Search performs a hybrid search
func (e *Engine) Search(ctx context.Context, req *Request) (*Response, error) {
	startTime := time.Now()

	// Clear previous query tracking and debug info
	e.lastVectorQuery = ""
	e.lastTextQuery = ""
	e.lastDebugInfo = nil

	// Clear LLM enhancer's debug info as well
	if e.llmEnhancer != nil {
		e.llmEnhancer.setDebugInfo("", "", "", "", 0)
	}

	e.log.WithField("query", req.Query).Info("DEBUG: Starting search with cleared debug info")

	// Log incoming request at debug level
	e.log.WithFields(logrus.Fields{
		"query":       req.Query,
		"search_type": req.SearchType,
		"limit":       req.Limit,
		"offset":      req.Offset,
	}).Debug("Incoming search request")

	// Validate request
	if err := e.validateRequest(req); err != nil {
		return nil, err
	}

	// First, try LLM enhancement for complex queries
	var enhancedQuery *EnhancedQuery
	if e.llmEnhancer != nil && e.llmEnhancer.IsEnabled() {
		enhanced, err := e.llmEnhancer.ProcessNaturalLanguageQuery(req.Query)
		if err != nil {
			e.log.WithError(err).Debug("LLM enhancement failed, falling back to traditional processing")
			// Clear any old LLM debug info when LLM fails
			if e.llmEnhancer != nil {
				e.llmEnhancer.setDebugInfo("", "", "", "", 0)
			}
		} else {
			enhancedQuery = enhanced
			e.log.WithFields(logrus.Fields{
				"original": enhanced.Original,
				"enhanced": enhanced.Enhanced,
				"intent":   enhanced.Intent,
			}).Debug("Query enhanced with LLM")
		}
	} else {
		// Clear any old LLM debug info when LLM is not used
		if e.llmEnhancer != nil {
			e.llmEnhancer.setDebugInfo("", "", "", "", 0)
		}
	}

	// Process query to extract phrases and determine optimal search type
	processedQuery, err := e.processor.ProcessQuery(req.Query)
	if err != nil {
		e.log.WithError(err).Error("Failed to process query")
		// Continue with original query if processing fails
		processedQuery = &ProcessedQuery{
			Original: req.Query,
			Cleaned:  req.Query,
		}
	}

	// Apply LLM enhancements if available
	if enhancedQuery != nil {
		e.applyLLMEnhancements(req, enhancedQuery, processedQuery)
	}

	// Update request with processed information
	if processedQuery != nil {
		// Log processed query details at debug level
		e.log.WithFields(logrus.Fields{
			"original_query": processedQuery.Original,
			"cleaned_query":  processedQuery.Cleaned,
			"phrases":        processedQuery.Phrases,
			"query_type":     processedQuery.QueryType,
		}).Debug("Query processing results")
		// Apply extracted filters to the request
		if len(processedQuery.FileTypes) > 0 {
			req.FileTypes = append(req.FileTypes, processedQuery.FileTypes...)
			e.log.WithField("file_types", processedQuery.FileTypes).Debug("Applied file type filters")
		}

		// Apply date range filters
		if processedQuery.DateRange != nil {
			req.DateFrom = &processedQuery.DateRange.From
			req.DateTo = &processedQuery.DateRange.To
			e.log.WithField("date_range", processedQuery.DateRange).Debug("Applied date range filters")
		}

		// Apply size range filters
		if processedQuery.SizeRange != nil {
			req.MinSize = processedQuery.SizeRange.Min
			req.MaxSize = processedQuery.SizeRange.Max
			e.log.WithField("size_range", processedQuery.SizeRange).Debug("Applied size range filters")
		}

		// Use cleaned query (with filters removed) for actual search
		if processedQuery.Cleaned != "" {
			req.Query = processedQuery.Cleaned
		} else if len(processedQuery.Phrases) > 0 {
			// If we have phrases, use them as the search query
			req.Query = strings.Join(processedQuery.Phrases, " ")
			e.log.WithField("phrases", processedQuery.Phrases).Debug("Using phrases as search query")
		} else if len(processedQuery.FileTypes) > 0 || processedQuery.DateRange != nil || processedQuery.SizeRange != nil {
			// If query becomes empty but we have filters, use a generic search to find all content
			// Use a common word that should match many files
			req.Query = "the"
			e.log.Debug("Empty query with filters, using generic word search")
		}

		// For phrase queries, always prefer text search for exact matching
		if len(processedQuery.Phrases) > 0 {
			req.SearchType = "text"
			e.log.WithField("phrases", processedQuery.Phrases).Debug("Set search type to text for phrase search")
		}
	}

	// Check cache (use original query for cache key consistency)
	cacheKey := e.generateCacheKey(req)
	if cached := e.cache.Get(cacheKey); cached != nil {
		e.log.Debug("Returning cached results")
		cached.Cached = true
		return cached, nil
	}

	// Determine search type
	searchType := req.SearchType
	e.log.WithFields(logrus.Fields{
		"requested_search_type": req.SearchType,
		"query":                 req.Query,
	}).Info("DEBUG: Checking search type")

	// For non-LLM queries, always use text-only search
	// Check if we actually used LLM (have debug info with prompt)
	usedLLM := e.llmEnhancer != nil && e.llmEnhancer.GetDebugInfo() != nil && e.llmEnhancer.GetDebugInfo().Prompt != ""

	// If no search type specified or it's hybrid, determine based on query
	if searchType == "" || searchType == "hybrid" {
		if usedLLM && enhancedQuery != nil && enhancedQuery.VectorTerms != nil && len(enhancedQuery.VectorTerms) > 0 {
			searchType = "hybrid"
			e.log.WithFields(logrus.Fields{
				"query":        req.Query,
				"vector_terms": enhancedQuery.VectorTerms,
			}).Info("Using hybrid search with LLM vector terms")
		} else {
			searchType = "text"
			e.log.WithField("query", req.Query).Info("Using text-only search for simple query")
		}
	}

	var results []Result

	switch searchType {
	case "vector":
		results, err = e.vectorSearch(ctx, req)
	case "text":
		results, err = e.textSearch(ctx, req, processedQuery)
	case "hybrid":
		results, err = e.hybridSearch(ctx, req, processedQuery)
	default:
		return nil, fmt.Errorf("invalid search type: %s", searchType)
	}

	if err != nil {
		return nil, err
	}

	// Store debug info based on whether LLM was used
	if e.llmEnhancer != nil && e.llmEnhancer.GetDebugInfo() != nil {
		// LLM was used - copy the debug info and add SQL queries
		llmDebug := e.llmEnhancer.GetDebugInfo()
		e.lastDebugInfo = &DebugInfo{
			Timestamp:   llmDebug.Timestamp,
			Query:       llmDebug.Query,
			Model:       llmDebug.Model,
			Prompt:      llmDebug.Prompt,
			Response:    llmDebug.Response,
			ProcessTime: llmDebug.ProcessTime,
			Error:       llmDebug.Error,
			VectorQuery: e.lastVectorQuery,
			TextQuery:   e.lastTextQuery,
		}
	} else {
		// LLM was not used - always store non-LLM debug info
		e.lastDebugInfo = &DebugInfo{
			Timestamp:   time.Now().Format(time.RFC3339),
			Query:       req.Query,
			Model:       "none",
			Prompt:      "", // No prompt for non-LLM queries
			Response:    "", // No response for non-LLM queries
			ProcessTime: 0,
			VectorQuery: e.lastVectorQuery, // Will be empty for text-only searches
			TextQuery:   e.lastTextQuery,   // Will be empty for vector-only searches
		}
	}

	// Apply content filters from LLM enhancement
	if len(req.ContentFilters) > 0 {
		results, err = e.applyContentFilters(ctx, results, req.ContentFilters)
		if err != nil {
			e.log.WithError(err).Debug("Content filtering failed, continuing with unfiltered results")
		}
	}

	// Note: Exact term filtering for single-word queries is now handled at the database level
	// in the vector search function using WHERE clauses for better performance

	// Apply metadata filters
	results = e.applyFilters(results, req)

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply pagination
	totalCount := len(results)
	if req.Offset > 0 && req.Offset < len(results) {
		results = results[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(results) {
		results = results[:req.Limit]
	}

	// Generate highlights
	for i := range results {
		results[i].Highlights = e.generateHighlights(results[i].Content, req.Query)
	}

	// Ensure results is never null
	if results == nil {
		results = []Result{}
	}

	// Check again if LLM was actually used (for used_llm flag)
	usedLLMFlag := e.lastDebugInfo != nil && e.lastDebugInfo.Model != "none" && e.lastDebugInfo.Prompt != ""

	response := &Response{
		Query:         req.Query,
		EnhancedQuery: enhancedQuery,
		Results:       results,
		TotalCount:    totalCount,
		SearchTime:    time.Since(startTime),
		Cached:        false,
		UsedLLM:       usedLLMFlag,
	}

	// Cache results
	e.cache.Set(cacheKey, response)

	e.log.WithFields(logrus.Fields{
		"query":       req.Query,
		"results":     len(results),
		"search_time": response.SearchTime,
		"search_type": searchType,
	}).Info("Search completed")

	return response, nil
}

// hybridSearch performs combined vector and text search
func (e *Engine) hybridSearch(ctx context.Context, req *Request, processedQuery *ProcessedQuery) ([]Result, error) {
	// Get both result sets concurrently
	vectorChan := make(chan []Result, 1)
	textChan := make(chan []Result, 1)
	errChan := make(chan error, 2)

	// Vector search
	go func() {
		results, err := e.vectorSearch(ctx, req)
		if err != nil {
			errChan <- err
			return
		}
		vectorChan <- results
	}()

	// Text search
	go func() {
		results, err := e.textSearch(ctx, req, processedQuery)
		if err != nil {
			errChan <- err
			return
		}
		textChan <- results
	}()

	// Wait for both results
	var vectorResults, textResults []Result
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			return nil, err
		case v := <-vectorChan:
			vectorResults = v
		case t := <-textChan:
			textResults = t
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	e.log.WithFields(logrus.Fields{
		"vector_results": len(vectorResults),
		"text_results":   len(textResults),
	}).Debug("Hybrid search combining results")

	// Combine and rank results
	return e.combineResults(vectorResults, textResults, req)
}

// vectorSearch performs vector similarity search
func (e *Engine) vectorSearch(ctx context.Context, req *Request) ([]Result, error) {
	// Validate query is not empty
	if strings.TrimSpace(req.Query) == "" {
		return []Result{}, nil // Return empty results for empty queries
	}

	// Generate query embedding
	embedding, err := e.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Handle empty embedding response
	if len(embedding) == 0 {
		return []Result{}, nil // Return empty results for empty embeddings
	}

	// Convert float64 embedding to float32 for pgvector
	float32Embedding := make([]float32, len(embedding))
	for i, v := range embedding {
		float32Embedding[i] = float32(v)
	}
	vec := pgvector.NewVector(float32Embedding)

	// Build query with date filtering and optional exact term filtering
	whereClause, dateArgs := e.buildDateFilter(req)

	// Add exact term filtering for single-word queries to improve precision
	exactTermFilter := ""
	if isSimpleWordQuery(req.Query) {
		exactTermFilter = " AND c.content ILIKE $" + fmt.Sprintf("%d", len(dateArgs)+2)
		dateArgs = append(dateArgs, "%"+strings.TrimSpace(req.Query)+"%")
	}

	query := `
		SELECT 
			c.id as chunk_id,
			c.file_id,
			c.content,
			c.char_start,
			c.char_end,
			c.start_line,
			c.metadata as chunk_metadata,
			f.path,
			f.filename,
			f.file_type,
			f.metadata as file_metadata,
			1 - (c.embedding <=> $1) as similarity
		FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE f.indexing_status = 'completed'` + whereClause + exactTermFilter + `
		ORDER BY c.embedding <=> $1
		LIMIT $` + fmt.Sprintf("%d", len(dateArgs)+2) + `
	`

	limit := req.Limit * 3 // Get more results for filtering
	if limit <= 0 {
		limit = e.config.DefaultLimit * 3
	}

	// Build query arguments: embedding, date args, limit
	args := []interface{}{vec}
	args = append(args, dateArgs...)
	args = append(args, limit)

	// Store the query with substituted parameters for debugging
	e.lastVectorQuery = substituteSQLParams(query, args)

	rows, err := e.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search query failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			e.log.WithError(err).Error("Failed to close vector search rows")
		}
	}()

	var results []Result
	for rows.Next() {
		var result Result
		var chunkMetadata, fileMetadata json.RawMessage

		err := rows.Scan(
			&result.ChunkID,
			&result.FileID,
			&result.Content,
			&result.CharStart,
			&result.CharEnd,
			&result.StartLine,
			&chunkMetadata,
			&result.FilePath,
			&result.Filename,
			&result.FileType,
			&fileMetadata,
			&result.VectorScore,
		)
		if err != nil {
			e.log.WithError(err).Error("Failed to scan vector search result")
			continue
		}

		// Parse metadata
		result.Metadata = make(map[string]interface{})
		if len(chunkMetadata) > 0 {
			if err := json.Unmarshal(chunkMetadata, &result.Metadata); err != nil {
				e.log.WithError(err).Error("Failed to unmarshal chunk metadata")
				result.Metadata = make(map[string]interface{})
			}
		}

		// Normalize vector score
		result.VectorScore = e.normalizeVectorScore(result.VectorScore)
		result.Score = result.VectorScore // Initial score

		results = append(results, result)
	}

	return results, nil
}

// textSearch performs BM25 full-text search
func (e *Engine) textSearch(ctx context.Context, req *Request, processedQuery *ProcessedQuery) ([]Result, error) {
	// Prepare query for full-text search
	tsQuery := e.prepareTextSearchQuery(req.Query, processedQuery)

	// Log the prepared query for debugging
	e.log.WithFields(logrus.Fields{
		"original_query": req.Query,
		"prepared_query": tsQuery,
	}).Debug("Prepared text search query")

	// Build query with date filtering
	whereClause, dateArgs := e.buildDateFilterForTextSearch(req)

	// Determine if we should use to_tsquery or plainto_tsquery
	// Use to_tsquery if we have a properly formatted query with operators
	queryFunction := "plainto_tsquery"

	// Check if tsQuery contains PostgreSQL operators (result of conversion)
	if strings.Contains(tsQuery, "&") || strings.Contains(tsQuery, "|") || strings.Contains(tsQuery, "!") {
		queryFunction = "to_tsquery"
		e.log.WithField("query_function", "to_tsquery").Debug("Using to_tsquery for boolean operators")
	} else {
		e.log.WithField("query_function", "plainto_tsquery").Debug("Using plainto_tsquery for simple terms")
	}

	query := fmt.Sprintf(`
		SELECT 
			ts.chunk_id,
			ts.file_id,
			ts.content,
			c.char_start,
			c.char_end,
			c.start_line,
			c.metadata as chunk_metadata,
			f.path,
			f.filename,
			f.file_type,
			f.metadata as file_metadata,
			ts_rank(ts.tsv_content, %s('english', $1)) as rank,
			ts_headline('english', ts.content, %s('english', $1), 
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20') as headline
		FROM text_search ts
		JOIN chunks c ON ts.chunk_id = c.id
		JOIN files f ON ts.file_id = f.id
		WHERE ts.tsv_content @@ %s('english', $1)
			AND f.indexing_status = 'completed'%s
		ORDER BY rank DESC
		LIMIT $%d
	`, queryFunction, queryFunction, queryFunction, whereClause, len(dateArgs)+2)

	limit := req.Limit * 3
	if limit <= 0 {
		limit = e.config.DefaultLimit * 3
	}

	// Build query arguments: tsQuery, date args, limit
	args := []interface{}{tsQuery}
	args = append(args, dateArgs...)
	args = append(args, limit)

	// Store the query with substituted parameters for debugging
	e.lastTextQuery = substituteSQLParams(query, args)

	rows, err := e.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("text search query failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			e.log.WithError(err).Error("Failed to close text search rows")
		}
	}()

	var results []Result
	resultCount := 0
	for rows.Next() {
		var result Result
		var chunkMetadata, fileMetadata json.RawMessage
		var headline string

		err := rows.Scan(
			&result.ChunkID,
			&result.FileID,
			&result.Content,
			&result.CharStart,
			&result.CharEnd,
			&result.StartLine,
			&chunkMetadata,
			&result.FilePath,
			&result.Filename,
			&result.FileType,
			&fileMetadata,
			&result.TextScore,
			&headline,
		)
		if err != nil {
			e.log.WithError(err).Error("Failed to scan text search result")
			continue
		}

		resultCount++

		// Parse metadata
		result.Metadata = make(map[string]interface{})
		if len(chunkMetadata) > 0 {
			if err := json.Unmarshal(chunkMetadata, &result.Metadata); err != nil {
				e.log.WithError(err).Error("Failed to unmarshal chunk metadata")
				result.Metadata = make(map[string]interface{})
			}
		}

		// Extract highlights from headline
		result.Highlights = e.extractHighlights(headline)

		// Normalize text score
		result.TextScore = e.normalizeTextScore(result.TextScore)
		result.Score = result.TextScore

		results = append(results, result)
	}

	e.log.WithFields(logrus.Fields{
		"query":          tsQuery,
		"result_count":   resultCount,
		"returned_count": len(results),
	}).Info("DEBUG: Text search completed")

	// Log first few results for debugging
	for i, r := range results {
		if i < 3 {
			e.log.WithFields(logrus.Fields{
				"index":      i,
				"chunk_id":   r.ChunkID,
				"file_path":  r.FilePath,
				"text_score": r.TextScore,
			}).Debug("Text search result detail")
		}
	}

	return results, nil
}

// combineResults merges and ranks results from vector and text search
func (e *Engine) combineResults(vectorResults, textResults []Result, req *Request) ([]Result, error) {
	// Log incoming results for debugging
	e.log.WithFields(logrus.Fields{
		"vector_count": len(vectorResults),
		"text_count":   len(textResults),
		"query":        req.Query,
	}).Info("DEBUG: combineResults called")

	// Log details of text results if present
	if len(textResults) > 0 {
		for i, tr := range textResults {
			if i < 3 { // Log first 3 for brevity
				e.log.WithFields(logrus.Fields{
					"index":      i,
					"chunk_id":   tr.ChunkID,
					"file_path":  tr.FilePath,
					"text_score": tr.TextScore,
				}).Debug("Text result in combineResults")
			}
		}
	}

	// Use Reciprocal Rank Fusion (RRF) for better hybrid search
	// RRF formula: score = 1 / (k + rank) where k = 60 (common choice)
	const rrfK = 60.0

	// Pre-allocate maps with estimated capacity to reduce allocations
	vectorLen := len(vectorResults)
	textLen := len(textResults)
	estimatedSize := vectorLen + textLen
	
	// Create rank maps for RRF calculation
	vectorRanks := make(map[int64]int, vectorLen)
	textRanks := make(map[int64]int, textLen)

	// Assign ranks to vector results (1-based)
	for i, vr := range vectorResults {
		vectorRanks[vr.ChunkID] = i + 1
	}

	// Assign ranks to text results (1-based)
	for i, tr := range textResults {
		textRanks[tr.ChunkID] = i + 1
	}

	// Create a map to combine results by chunk ID
	resultMap := make(map[int64]*Result, estimatedSize)

	// Process all unique chunk IDs from both result sets
	allChunkIDs := make(map[int64]bool, estimatedSize)
	for _, vr := range vectorResults {
		allChunkIDs[vr.ChunkID] = true
	}
	for _, tr := range textResults {
		allChunkIDs[tr.ChunkID] = true
	}

	// Calculate RRF scores for each chunk
	for chunkID := range allChunkIDs {
		var result *Result
		var vectorRRF, textRRF float64

		// Get vector result if exists
		for _, vr := range vectorResults {
			if vr.ChunkID == chunkID {
				result = &vr
				if rank, ok := vectorRanks[chunkID]; ok {
					vectorRRF = 1.0 / (rrfK + float64(rank))
				}
				break
			}
		}

		// Get text result if exists (and merge with vector result)
		for _, tr := range textResults {
			if tr.ChunkID == chunkID {
				if result == nil {
					result = &tr
				} else {
					// Merge text data into existing result
					result.TextScore = tr.TextScore
					result.Highlights = tr.Highlights
				}
				if rank, ok := textRanks[chunkID]; ok {
					textRRF = 1.0 / (rrfK + float64(rank))
				}
				break
			}
		}

		if result != nil {
			// Calculate combined RRF score
			result.Score = vectorRRF + textRRF

			e.log.WithFields(logrus.Fields{
				"chunk_id":       chunkID,
				"vector_rrf":     vectorRRF,
				"text_rrf":       textRRF,
				"combined_score": result.Score,
				"has_vector":     vectorRRF > 0,
				"has_text":       textRRF > 0,
			}).Debug("RRF score calculation")

			resultMap[chunkID] = result
		}
	}

	// Calculate metadata scores and finalize results
	var results []Result
	for _, result := range resultMap {
		// Calculate metadata score
		metadataScore := e.calculateMetadataScore(result, req)
		result.MetadataScore = metadataScore

		// Add metadata score to RRF score (small weight to preserve RRF dominance)
		result.Score += metadataScore * e.config.MetadataWeight

		// Individual scores are kept for debugging and UI display
		// Vector and text scores are already stored in result.VectorScore and result.TextScore

		// Apply minimum score threshold (adjusted for RRF scale)
		// RRF scores are typically much smaller than weighted scores
		minRRFScore := 0.01 // Roughly equivalent to being in top 100 results

		e.log.WithFields(logrus.Fields{
			"chunk_id":       result.ChunkID,
			"score":          result.Score,
			"text_score":     result.TextScore,
			"vector_score":   result.VectorScore,
			"metadata_score": result.MetadataScore,
			"min_threshold":  minRRFScore,
			"passed":         result.Score >= minRRFScore,
		}).Debug("Result score check")

		if result.Score >= minRRFScore {
			results = append(results, *result)
		}
	}

	// Log final results
	e.log.WithFields(logrus.Fields{
		"input_vector_count": len(vectorResults),
		"input_text_count":   len(textResults),
		"output_count":       len(results),
		"min_threshold":      0.01, // minRRFScore value
	}).Info("DEBUG: combineResults returning")

	return results, nil
}

// calculateMetadataScore calculates score based on metadata factors
func (e *Engine) calculateMetadataScore(result *Result, req *Request) float64 {
	score := 0.5 // Base score

	// File type relevance
	if len(req.FileTypes) > 0 {
		for _, ft := range req.FileTypes {
			if result.FileType == ft {
				score += 0.2
				break
			}
		}
	}

	// Path relevance
	if len(req.Paths) > 0 {
		for _, path := range req.Paths {
			if strings.Contains(result.FilePath, path) {
				score += 0.15
				break
			}
		}
	}

	// Extension match
	if len(req.Extensions) > 0 {
		for _, ext := range req.Extensions {
			if strings.HasSuffix(result.Filename, ext) {
				score += 0.15
				break
			}
		}
	}

	// Normalize to 0-1 range
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// buildDateFilter constructs SQL WHERE clause and arguments for date filtering
func (e *Engine) buildDateFilter(req *Request) (string, []interface{}) {
	var whereClause string
	var args []interface{}

	if req.DateFrom == nil && req.DateTo == nil {
		return "", args
	}

	// Determine which date field to filter on
	dateField := "f.modified_at" // default
	if req.Metadata != nil {
		if field, ok := req.Metadata["date_field"].(string); ok && field != "" {
			switch field {
			case "created_at":
				dateField = "f.created_at"
			case "modified_at":
				dateField = "f.modified_at"
			}
		}
	}

	argIndex := 2 // Start at 2 because $1 is the embedding vector

	if req.DateFrom != nil {
		whereClause += fmt.Sprintf(" AND %s >= $%d", dateField, argIndex)
		args = append(args, *req.DateFrom)
		argIndex++
	}

	if req.DateTo != nil {
		whereClause += fmt.Sprintf(" AND %s <= $%d", dateField, argIndex)
		args = append(args, *req.DateTo)
		// argIndex not incremented as it's not used again
	}

	e.log.WithFields(logrus.Fields{
		"date_field":   dateField,
		"date_from":    req.DateFrom,
		"date_to":      req.DateTo,
		"where_clause": whereClause,
	}).Debug("Built date filter")

	return whereClause, args
}

// buildDateFilterForTextSearch constructs SQL WHERE clause and arguments for date filtering in text search
func (e *Engine) buildDateFilterForTextSearch(req *Request) (string, []interface{}) {
	var whereClause string
	var args []interface{}

	if req.DateFrom == nil && req.DateTo == nil {
		return "", args
	}

	// Determine which date field to filter on
	dateField := "f.modified_at" // default
	if req.Metadata != nil {
		if field, ok := req.Metadata["date_field"].(string); ok && field != "" {
			switch field {
			case "created_at":
				dateField = "f.created_at"
			case "modified_at":
				dateField = "f.modified_at"
			}
		}
	}

	argIndex := 2 // Start at 2 because $1 is the tsQuery

	if req.DateFrom != nil {
		whereClause += fmt.Sprintf(" AND %s >= $%d", dateField, argIndex)
		args = append(args, *req.DateFrom)
		argIndex++
	}

	if req.DateTo != nil {
		whereClause += fmt.Sprintf(" AND %s <= $%d", dateField, argIndex)
		args = append(args, *req.DateTo)
		// argIndex not incremented as it's not used again
	}

	e.log.WithFields(logrus.Fields{
		"date_field":   dateField,
		"date_from":    req.DateFrom,
		"date_to":      req.DateTo,
		"where_clause": whereClause,
	}).Debug("Built date filter for text search")

	return whereClause, args
}

// applyFilters applies additional filters to results
func (e *Engine) applyFilters(results []Result, req *Request) []Result {
	// Log what filters are being applied
	e.log.WithFields(logrus.Fields{
		"input_count":    len(results),
		"has_file_types": len(req.FileTypes) > 0,
		"file_types":     req.FileTypes,
		"has_extensions": len(req.Extensions) > 0,
		"has_paths":      len(req.Paths) > 0,
		"has_date_from":  req.DateFrom != nil,
		"has_date_to":    req.DateTo != nil,
		"min_size":       req.MinSize,
		"max_size":       req.MaxSize,
	}).Info("DEBUG: applyFilters called")

	if req.FileTypes == nil && req.Extensions == nil && req.Paths == nil &&
		req.DateFrom == nil && req.DateTo == nil &&
		req.MinSize == 0 && req.MaxSize == 0 {
		return results
	}

	var filtered []Result
	for _, result := range results {
		// File type filter
		if len(req.FileTypes) > 0 {
			found := false
			for _, ft := range req.FileTypes {
				// Check exact match first
				if result.FileType == ft {
					found = true
					break
				}
				// Also check file extension match for common cases
				if strings.HasSuffix(strings.ToLower(result.Filename), "."+strings.ToLower(ft)) {
					found = true
					break
				}
				// Check if file type contains the filter (e.g., "yaml" matches "code" type with .yaml extension)
				if result.FileType == "code" && strings.HasSuffix(strings.ToLower(result.Filename), "."+strings.ToLower(ft)) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Extension filter
		if len(req.Extensions) > 0 {
			found := false
			for _, ext := range req.Extensions {
				if strings.HasSuffix(result.Filename, ext) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Path filter
		if len(req.Paths) > 0 {
			found := false
			for _, path := range req.Paths {
				if strings.Contains(result.FilePath, path) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, result)
	}

	e.log.WithFields(logrus.Fields{
		"input_count":  len(results),
		"output_count": len(filtered),
	}).Info("DEBUG: applyFilters returning")

	return filtered
}

// applyContentFilters applies LLM-derived content filters to search results
func (e *Engine) applyContentFilters(ctx context.Context, results []Result, filters []ContentFilter) ([]Result, error) {
	if len(filters) == 0 {
		return results, nil
	}

	var filtered []Result

	for _, result := range results {
		passesFilters := true

		for _, filter := range filters {
			passes, err := e.evaluateContentFilter(ctx, result, filter)
			if err != nil {
				e.log.WithError(err).WithFields(logrus.Fields{
					"file_id":     result.FileID,
					"filter_type": filter.Type,
				}).Debug("Content filter evaluation failed")
				// On error, assume it passes to avoid false negatives
				continue
			}

			if !passes {
				passesFilters = false
				break
			}
		}

		if passesFilters {
			filtered = append(filtered, result)
		}
	}

	e.log.WithFields(logrus.Fields{
		"original_count": len(results),
		"filtered_count": len(filtered),
		"filter_count":   len(filters),
	}).Debug("Applied content filters")

	return filtered, nil
}

// evaluateContentFilter evaluates a single content filter against a search result
func (e *Engine) evaluateContentFilter(ctx context.Context, result Result, filter ContentFilter) (bool, error) {
	content := strings.ToLower(result.Content)

	switch filter.Type {
	case "contains":
		// Check if content contains all specified keywords
		for _, keyword := range filter.Keywords {
			if !strings.Contains(content, strings.ToLower(keyword)) {
				return false, nil
			}
		}
		return true, nil

	case "pattern":
		// Use regex pattern matching
		if filter.Pattern == "" {
			return true, nil
		}

		// Handle special patterns
		switch strings.ToLower(filter.Description) {
		case "social security number", "ssn":
			// SSN patterns: XXX-XX-XXXX, XXXXXXXXX
			patterns := []string{
				`\b\d{3}-\d{2}-\d{4}\b`,
				`\b\d{9}\b`,
				`\b\d{3}\s\d{2}\s\d{4}\b`,
			}
			for _, pattern := range patterns {
				if matched, _ := regexp.MatchString(pattern, result.Content); matched {
					return true, nil
				}
			}
			return false, nil

		case "credit card", "credit card number":
			// Credit card patterns: XXXX-XXXX-XXXX-XXXX, XXXXXXXXXXXXXXXX
			patterns := []string{
				`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
				`\b\d{16}\b`,
			}
			for _, pattern := range patterns {
				if matched, _ := regexp.MatchString(pattern, result.Content); matched {
					return true, nil
				}
			}
			return false, nil

		case "table", "financial table":
			// Look for table-like patterns in text
			patterns := []string{
				`\|\s*[^|]+\s*\|`, // Markdown table syntax
				`\t.*\t.*\t`,      // Tab-separated values
				`\$[\d,]+\.?\d*`,  // Dollar amounts
				`\b(total|sum|amount|balance|price|cost|revenue|profit)\s*:?\s*\$?[\d,]+\.?\d*`,
			}
			for _, pattern := range patterns {
				if matched, _ := regexp.MatchString(`(?i)`+pattern, result.Content); matched {
					return true, nil
				}
			}
			return false, nil
		}

		// Default regex pattern matching
		matched, err := regexp.MatchString(filter.Pattern, result.Content)
		return matched, err

	case "semantic":
		// Use vector similarity for semantic matching
		if e.embedder == nil {
			e.log.Debug("Embedder not available for semantic filtering")
			return true, nil // Assume passes if we can't evaluate
		}

		// Generate query embedding from filter keywords
		queryText := strings.Join(filter.Keywords, " ")
		if filter.Description != "" {
			queryText = filter.Description + " " + queryText
		}

		queryEmbedding, err := e.embedder.Embed(ctx, queryText)
		if err != nil {
			return false, fmt.Errorf("failed to generate query embedding: %w", err)
		}

		// Get chunk embedding from database (this would need to be implemented)
		// For now, use a simple keyword check as fallback
		for _, keyword := range filter.Keywords {
			if strings.Contains(content, strings.ToLower(keyword)) {
				return true, nil
			}
		}

		// If no simple keyword match, compute similarity (placeholder logic)
		_ = queryEmbedding // Use when implementing actual vector similarity

		// For now, return true if any keywords match
		return len(filter.Keywords) == 0, nil

	default:
		e.log.WithField("filter_type", filter.Type).Debug("Unknown content filter type")
		return true, nil
	}
}

// isSimpleWordQuery checks if the query is a simple single word that would benefit from exact filtering
func isSimpleWordQuery(query string) bool {
	trimmed := strings.TrimSpace(query)

	// Skip if empty, quoted, or contains special characters
	if trimmed == "" || strings.Contains(trimmed, `"`) || strings.Contains(trimmed, ":") {
		return false
	}

	// Check if it's a single word (no spaces)
	words := strings.Fields(trimmed)
	if len(words) != 1 {
		return false
	}

	word := words[0]

	// Must be at least 3 characters and only contain word characters
	if len(word) < 3 {
		return false
	}

	// Simple check for word characters (letters, numbers, basic punctuation)
	for _, r := range word {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}

	return true
}

// normalizeVectorScore normalizes vector similarity score to 0-1 range
func (e *Engine) normalizeVectorScore(score float64) float64 {
	// Cosine similarity is already in [-1, 1], convert to [0, 1]
	normalized := (score + 1.0) / 2.0

	// Apply more conservative scoring for better precision
	// Only scores above 0.75 similarity get high ranking
	if normalized < 0.75 {
		return normalized * 0.5 // Reduce low similarity scores
	}

	// Apply sigmoid for better distribution of high-quality matches
	return 1.0 / (1.0 + math.Exp(-15*(normalized-0.8)))
}

// normalizeTextScore normalizes BM25 score to 0-1 range
func (e *Engine) normalizeTextScore(score float64) float64 {
	// BM25 scores are typically in [0, inf), use logarithmic scaling
	if score <= 0 {
		return 0
	}

	// Apply logarithmic scaling with cutoff
	normalized := math.Log1p(score) / math.Log1p(10.0)
	if normalized > 1.0 {
		normalized = 1.0
	}

	return normalized
}

// prepareTextSearchQuery prepares the query string for PostgreSQL text search
func (e *Engine) prepareTextSearchQuery(query string, processedQuery *ProcessedQuery) string {
	// If we have an enhanced query with a PostgreSQL tsquery, use it
	if processedQuery != nil && processedQuery.EnhancedQuery != nil && processedQuery.EnhancedQuery.PgTsQuery != "" {
		// Validate and clean the tsquery
		tsQuery := processedQuery.EnhancedQuery.PgTsQuery

		e.log.WithFields(logrus.Fields{
			"original_query": query,
			"pg_tsquery":     tsQuery,
		}).Info("DEBUG: Using LLM-provided PgTsQuery")

		// Ensure proper formatting for PostgreSQL
		tsQuery = strings.TrimSpace(tsQuery)

		// Check if this is a query for finding files with specific words
		// These queries often get misinterpreted as needing all words
		lowerQuery := strings.ToLower(query)
		if strings.Contains(lowerQuery, "find") && strings.Contains(lowerQuery, "files") &&
			strings.Contains(lowerQuery, "word") {
			// Extract the actual word mentioned after "word" in the query
			words := strings.Fields(lowerQuery)
			for i, word := range words {
				if word == "word" && i+1 < len(words) {
					searchWord := words[i+1]
					// Clean the word of common punctuation
					searchWord = strings.Trim(searchWord, ".,!?;:")
					if searchWord != "" {
						e.log.WithField("extracted_word", searchWord).Info("Extracted word from 'find files with word X' query")
						return searchWord
					}
				}
			}

			// Fallback: use the first search term if extraction failed
			if len(processedQuery.EnhancedQuery.SearchTerms) > 0 {
				term := strings.ToLower(strings.TrimSpace(processedQuery.EnhancedQuery.SearchTerms[0]))
				if term != "" && term != "files" && term != "word" && term != "find" {
					e.log.WithField("extracted_term", term).Info("Using first search term as fallback")
					return term
				}
			}
		}

		// If it contains operators, it's ready for to_tsquery
		if strings.Contains(tsQuery, "&") || strings.Contains(tsQuery, "|") || strings.Contains(tsQuery, "!") {
			// Special handling: if the query contains common meta words with AND operators,
			// extract just the meaningful search terms
			if strings.Contains(tsQuery, "files") && strings.Contains(tsQuery, "&") {
				// Split by & and extract meaningful terms
				parts := strings.Split(tsQuery, "&")
				var meaningfulTerms []string
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part != "files" && part != "word" && part != "find" && part != "all" && part != "" {
						meaningfulTerms = append(meaningfulTerms, part)
					}
				}
				if len(meaningfulTerms) > 0 {
					// Use OR operator for better recall
					return strings.Join(meaningfulTerms, " | ")
				}
			}
			return tsQuery
		}

		// Otherwise, treat as simple terms
		return strings.Join(strings.Fields(tsQuery), " ")
	}

	// If we have phrases, construct phrase query
	if processedQuery != nil && len(processedQuery.Phrases) > 0 {
		var parts []string

		// Add each phrase with proper PostgreSQL phrase query syntax
		for _, phrase := range processedQuery.Phrases {
			// Use PostgreSQL phrase query syntax: <phrase> for exact phrase match
			escapedPhrase := strings.ReplaceAll(phrase, "'", "''") // Escape single quotes
			parts = append(parts, fmt.Sprintf("'%s'", escapedPhrase))
		}

		// Add non-phrase terms if any exist
		if processedQuery.Cleaned != "" {
			// Remove quotes from cleaned query and prepare remaining terms
			remainingQuery := strings.ReplaceAll(processedQuery.Cleaned, "\"", "")
			remainingQuery = strings.TrimSpace(remainingQuery)

			if remainingQuery != "" {
				// Remove special characters that might break tsquery
				remainingQuery = strings.ReplaceAll(remainingQuery, "'", "")
				remainingQuery = strings.ReplaceAll(remainingQuery, "\\", "")

				// Handle boolean operators
				remainingQuery = strings.ReplaceAll(remainingQuery, " AND ", " & ")
				remainingQuery = strings.ReplaceAll(remainingQuery, " OR ", " | ")
				remainingQuery = strings.ReplaceAll(remainingQuery, " NOT ", " ! ")

				if remainingQuery != "" {
					parts = append(parts, remainingQuery)
				}
			}
		}

		// Combine all parts with AND
		if len(parts) > 1 {
			return strings.Join(parts, " & ")
		} else if len(parts) == 1 {
			return parts[0]
		}
	}

	// Fall back to original logic for non-phrase queries
	// Remove special characters that might break tsquery
	query = strings.ReplaceAll(query, "'", "")
	query = strings.ReplaceAll(query, "\"", "") // Still remove quotes for non-phrase queries
	query = strings.ReplaceAll(query, "\\", "")

	// Handle boolean operators - case insensitive
	// Replace common variations
	query = regexp.MustCompile(`(?i)\s+OR\s+`).ReplaceAllString(query, " | ")
	query = regexp.MustCompile(`(?i)\s+AND\s+`).ReplaceAllString(query, " & ")
	query = regexp.MustCompile(`(?i)\s+NOT\s+`).ReplaceAllString(query, " ! ")

	// Log the formatted query for debugging
	e.log.WithFields(logrus.Fields{
		"formatted": query,
	}).Debug("Formatted text search query")

	return query
}

// generateHighlights generates text highlights for search results
func (e *Engine) generateHighlights(content, query string) []string {
	var highlights []string

	// Split query into terms
	terms := strings.Fields(strings.ToLower(query))

	// Find sentences containing query terms
	sentences := strings.Split(content, ".")
	for _, sentence := range sentences {
		sentenceLower := strings.ToLower(sentence)
		for _, term := range terms {
			if strings.Contains(sentenceLower, term) {
				highlight := strings.TrimSpace(sentence)
				if highlight != "" {
					highlights = append(highlights, highlight)
					break
				}
			}
		}

		if len(highlights) >= 3 {
			break
		}
	}

	return highlights
}

// extractHighlights extracts highlights from PostgreSQL headline
func (e *Engine) extractHighlights(headline string) []string {
	var highlights []string

	// Extract marked sections
	parts := strings.Split(headline, "<mark>")
	for i := 1; i < len(parts); i++ {
		endIdx := strings.Index(parts[i], "</mark>")
		if endIdx > 0 {
			highlight := parts[i][:endIdx]
			highlights = append(highlights, highlight)
		}
	}

	return highlights
}

// validateRequest validates search request parameters
func (e *Engine) validateRequest(req *Request) error {
	if req.Query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	if req.Limit <= 0 {
		req.Limit = e.config.DefaultLimit
	}

	if req.Limit > 100 {
		req.Limit = 100
	}

	if req.Offset < 0 {
		req.Offset = 0
	}

	return nil
}

// generateCacheKey generates a cache key for the search request
func (e *Engine) generateCacheKey(req *Request) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// GetCacheStats returns cache statistics
func (e *Engine) GetCacheStats() CacheStats {
	return e.cache.GetStats()
}

// GetSearchHistory returns recent search history (placeholder implementation)
func (e *Engine) GetSearchHistory(limit int) []map[string]interface{} {
	// This would typically come from a database table or cache
	// For now, return empty slice
	return []map[string]interface{}{}
}

// applyLLMEnhancements applies LLM-derived enhancements to search request
func (e *Engine) applyLLMEnhancements(req *Request, enhanced *EnhancedQuery, processed *ProcessedQuery) {
	// Store the enhanced query in the processed query for use in search functions
	processed.EnhancedQuery = enhanced

	// Use the enhanced query terms if available
	if enhanced.Enhanced != "" {
		req.Query = enhanced.Enhanced
	}

	// If we have enhanced search terms, update the processed query terms
	if len(enhanced.SearchTerms) > 0 {
		processed.Terms = enhanced.SearchTerms
	}

	// If we have vector terms for better semantic search
	if len(enhanced.VectorTerms) > 0 {
		// These will be used by the vector search function
		if req.Metadata == nil {
			req.Metadata = make(map[string]interface{})
		}
		req.Metadata["vector_terms"] = enhanced.VectorTerms
	}

	// Apply metadata filters from LLM analysis
	for _, filter := range enhanced.MetadataFilters {
		switch filter.Field {
		case "type":
			if fileType, ok := filter.Value.(string); ok {
				req.FileTypes = append(req.FileTypes, fileType)
				e.log.WithField("file_type", fileType).Debug("Applied LLM file type filter")
			}
		case "created_date", "modified_date":
			// Handle new date field specifications with date_field support
			if filter.StartDate != nil {
				req.DateFrom = filter.StartDate
			}
			if filter.EndDate != nil {
				req.DateTo = filter.EndDate
			}
			// Store which date field to filter on (created_at vs modified_at)
			if filter.DateField != "" {
				if req.Metadata == nil {
					req.Metadata = make(map[string]interface{})
				}
				req.Metadata["date_field"] = filter.DateField
			} else {
				// Default behavior based on field name
				if req.Metadata == nil {
					req.Metadata = make(map[string]interface{})
				}
				if filter.Field == "created_date" {
					req.Metadata["date_field"] = "created_at"
				} else {
					req.Metadata["date_field"] = "modified_at"
				}
			}
			e.log.WithFields(logrus.Fields{
				"date_from":    req.DateFrom,
				"date_to":      req.DateTo,
				"date_field":   req.Metadata["date_field"],
				"filter_field": filter.Field,
			}).Debug("Applied LLM date filter")
		case "date":
			// Legacy date handling - defaults to modified_at
			if filter.StartDate != nil {
				req.DateFrom = filter.StartDate
			}
			if filter.EndDate != nil {
				req.DateTo = filter.EndDate
			}
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["date_field"] = "modified_at" // Default to modified_at for backward compatibility
			e.log.WithFields(logrus.Fields{
				"date_from":  req.DateFrom,
				"date_to":    req.DateTo,
				"date_field": "modified_at",
			}).Debug("Applied LLM legacy date filter")
		case "size":
			switch filter.Operator {
			case "greater":
				if size, ok := filter.Value.(float64); ok {
					req.MinSize = int64(size)
				}
			case "less":
				if size, ok := filter.Value.(float64); ok {
					req.MaxSize = int64(size)
				}
			}
			e.log.WithFields(logrus.Fields{
				"min_size": req.MinSize,
				"max_size": req.MaxSize,
			}).Debug("Applied LLM size filter")
		}
	}

	// Apply content filters to the search request
	if len(enhanced.ContentFilters) > 0 {
		req.ContentFilters = enhanced.ContentFilters
	}

	// Set requires count flag
	req.RequiresCount = enhanced.RequiresCount

	// Apply search type based on content filters
	for _, filter := range enhanced.ContentFilters {
		switch filter.Type {
		case "semantic":
			// Use hybrid search for semantic matching
			req.SearchType = "hybrid"
			e.log.WithField("filter_type", "semantic").Debug("Set search type to hybrid for semantic matching")
		case "pattern":
			// Use text search for pattern matching
			req.SearchType = "text"
			e.log.WithField("pattern", filter.Pattern).Debug("Set search type to text for pattern matching")
		case "contains":
			// Use text search for exact text matching
			req.SearchType = "text"
			e.log.WithField("keywords", filter.Keywords).Debug("Set search type to text for content matching")
		}
	}

	// Override search type if not already set
	if req.SearchType == "" {
		if enhanced.Intent == "count" {
			req.SearchType = "text" // Use text search for counting for better performance
		} else {
			req.SearchType = "hybrid" // Default to hybrid for enhanced queries
		}
	}

	e.log.WithFields(logrus.Fields{
		"intent":            enhanced.Intent,
		"content_filters":   len(enhanced.ContentFilters),
		"metadata_filters":  len(enhanced.MetadataFilters),
		"final_search_type": req.SearchType,
	}).Debug("Applied LLM enhancements to search request")
}

// ClearCache clears the search cache
func (e *Engine) ClearCache() {
	e.cache.Clear()
}

// GetLLMEnhancer returns the LLM enhancer instance
func (e *Engine) GetLLMEnhancer() *LLMEnhancer {
	return e.llmEnhancer
}

// GetLLMModelName returns the current LLM model name
func (e *Engine) GetLLMModelName() string {
	if e.llmEnhancer != nil {
		return e.llmEnhancer.GetModelName()
	}
	return "unknown"
}

// GetLLMDebugInfo returns the last LLM debug information with SQL queries
func (e *Engine) GetLLMDebugInfo() *DebugInfo {
	fmt.Printf("DEBUG GetLLMDebugInfo: llmEnhancer=%v, llmDebugInfo=%v, lastDebugInfo=%v\n",
		e.llmEnhancer != nil,
		e.llmEnhancer != nil && e.llmEnhancer.GetDebugInfo() != nil,
		e.lastDebugInfo != nil)

	// Prioritize engine's lastDebugInfo (most recent query, LLM or non-LLM)
	if e.lastDebugInfo != nil {
		fmt.Printf("DEBUG: Returning engine's lastDebugInfo with query: %s\n", e.lastDebugInfo.Query)
		return e.lastDebugInfo
	}

	// Fall back to LLM enhancer's debug info if no engine debug info
	if e.llmEnhancer != nil {
		debugInfo := e.llmEnhancer.GetDebugInfo()
		if debugInfo != nil {
			fmt.Printf("DEBUG: Returning LLM debug info with query: %s\n", debugInfo.Query)
			// Make a copy and add the SQL queries
			enhanced := *debugInfo
			enhanced.VectorQuery = e.lastVectorQuery
			enhanced.TextQuery = e.lastTextQuery
			return &enhanced
		}
	}

	fmt.Printf("DEBUG: No debug info available\n")
	return nil
}

// ReloadPromptTemplate reloads the LLM prompt template from the database
func (e *Engine) ReloadPromptTemplate() {
	if e.llmEnhancer != nil && e.llmEnhancer.royalProcessor != nil {
		e.llmEnhancer.royalProcessor.ReloadPromptTemplate()
	}
}
