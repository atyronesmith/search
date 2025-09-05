package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
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
	db        *database.DB
	embedder  *embeddings.OllamaClient
	config    *Config
	log       *logrus.Logger
	cache     *SearchCache
	processor *QueryProcessor
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
		VectorWeight:   0.6,
		BM25Weight:     0.3,
		MetadataWeight: 0.1,
		DefaultLimit:   20,
		CacheTTL:       15 * time.Minute,
		MinScore:       0.1,
	}
}

// SearchRequest represents a search query
type SearchRequest struct {
	Query       string                 `json:"query"`
	Limit       int                    `json:"limit"`
	Offset      int                    `json:"offset"`
	FileTypes   []string               `json:"file_types,omitempty"`
	Extensions  []string               `json:"extensions,omitempty"`
	Paths       []string               `json:"paths,omitempty"`
	DateFrom    *time.Time             `json:"date_from,omitempty"`
	DateTo      *time.Time             `json:"date_to,omitempty"`
	MinSize     int64                  `json:"min_size,omitempty"`
	MaxSize     int64                  `json:"max_size,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	SearchType  string                 `json:"search_type"` // "hybrid", "vector", "text"
}

// SearchResponse represents search results
type SearchResponse struct {
	Query       string          `json:"query"`
	Results     []SearchResult  `json:"results"`
	TotalCount  int             `json:"total_count"`
	SearchTime  time.Duration   `json:"search_time"`
	Cached      bool            `json:"cached"`
}

// SearchResult represents a single search result
type SearchResult struct {
	FileID         int64                  `json:"file_id"`
	ChunkID        int64                  `json:"chunk_id"`
	FilePath       string                 `json:"file_path"`
	Filename       string                 `json:"filename"`
	FileType       string                 `json:"file_type"`
	Content        string                 `json:"content"`
	Score          float64                `json:"score"`
	VectorScore    float64                `json:"vector_score"`
	TextScore      float64                `json:"text_score"`
	MetadataScore  float64                `json:"metadata_score"`
	Highlights     []string               `json:"highlights"`
	StartLine      *int                   `json:"start_line,omitempty"`
	EndLine        *int                   `json:"end_line,omitempty"`
	CharStart      int                    `json:"char_start"`
	CharEnd        int                    `json:"char_end"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// NewEngine creates a new search engine
func NewEngine(db *database.DB, embedder *embeddings.OllamaClient, config *Config, log *logrus.Logger) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Engine{
		db:        db,
		embedder:  embedder,
		config:    config,
		log:       log,
		cache:     NewSearchCache(config.CacheTTL),
		processor: NewQueryProcessor(),
	}
}

// Search performs a hybrid search
func (e *Engine) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	startTime := time.Now()
	
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
	
	// Log processed query details at debug level
	e.log.WithFields(logrus.Fields{
		"original_query": processedQuery.Original,
		"cleaned_query":  processedQuery.Cleaned,
		"phrases":        processedQuery.Phrases,
		"query_type":     processedQuery.QueryType,
	}).Debug("Query processing results")
	
	// Update request with processed information
	if processedQuery != nil {
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
	if searchType == "" {
		searchType = "hybrid"
	}
	
	var results []SearchResult
	
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
		results = []SearchResult{}
	}
	
	response := &SearchResponse{
		Query:      req.Query,
		Results:    results,
		TotalCount: totalCount,
		SearchTime: time.Since(startTime),
		Cached:     false,
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
func (e *Engine) hybridSearch(ctx context.Context, req *SearchRequest, processedQuery *ProcessedQuery) ([]SearchResult, error) {
	// Get both result sets concurrently
	vectorChan := make(chan []SearchResult, 1)
	textChan := make(chan []SearchResult, 1)
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
	var vectorResults, textResults []SearchResult
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
	
	// Combine and rank results
	return e.combineResults(vectorResults, textResults, req)
}

// vectorSearch performs vector similarity search
func (e *Engine) vectorSearch(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	// Validate query is not empty
	if strings.TrimSpace(req.Query) == "" {
		return []SearchResult{}, nil // Return empty results for empty queries
	}
	
	// Generate query embedding
	embedding, err := e.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	
	// Handle empty embedding response
	if len(embedding) == 0 {
		return []SearchResult{}, nil // Return empty results for empty embeddings
	}
	
	// Convert float64 embedding to float32 for pgvector
	float32Embedding := make([]float32, len(embedding))
	for i, v := range embedding {
		float32Embedding[i] = float32(v)
	}
	vec := pgvector.NewVector(float32Embedding)
	
	// Build query
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
		WHERE f.indexing_status = 'completed'
		ORDER BY c.embedding <=> $1
		LIMIT $2
	`
	
	limit := req.Limit * 3 // Get more results for filtering
	if limit <= 0 {
		limit = e.config.DefaultLimit * 3
	}
	
	rows, err := e.db.Query(ctx, query, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search query failed: %w", err)
	}
	defer rows.Close()
	
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
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
			json.Unmarshal(chunkMetadata, &result.Metadata)
		}
		
		// Normalize vector score
		result.VectorScore = e.normalizeVectorScore(result.VectorScore)
		result.Score = result.VectorScore // Initial score
		
		results = append(results, result)
	}
	
	return results, nil
}

// textSearch performs BM25 full-text search
func (e *Engine) textSearch(ctx context.Context, req *SearchRequest, processedQuery *ProcessedQuery) ([]SearchResult, error) {
	// Prepare query for full-text search
	tsQuery := e.prepareTextSearchQuery(req.Query, processedQuery)
	
	query := `
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
			ts_rank(ts.tsv_content, plainto_tsquery('english', $1)) as rank,
			ts_headline('english', ts.content, plainto_tsquery('english', $1), 
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20') as headline
		FROM text_search ts
		JOIN chunks c ON ts.chunk_id = c.id
		JOIN files f ON ts.file_id = f.id
		WHERE ts.tsv_content @@ plainto_tsquery('english', $1)
			AND f.indexing_status = 'completed'
		ORDER BY rank DESC
		LIMIT $2
	`
	
	limit := req.Limit * 3
	if limit <= 0 {
		limit = e.config.DefaultLimit * 3
	}
	
	rows, err := e.db.Query(ctx, query, tsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("text search query failed: %w", err)
	}
	defer rows.Close()
	
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
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
		
		// Parse metadata
		result.Metadata = make(map[string]interface{})
		if len(chunkMetadata) > 0 {
			json.Unmarshal(chunkMetadata, &result.Metadata)
		}
		
		// Extract highlights from headline
		result.Highlights = e.extractHighlights(headline)
		
		// Normalize text score
		result.TextScore = e.normalizeTextScore(result.TextScore)
		result.Score = result.TextScore
		
		results = append(results, result)
	}
	
	return results, nil
}

// combineResults merges and ranks results from vector and text search
func (e *Engine) combineResults(vectorResults, textResults []SearchResult, req *SearchRequest) ([]SearchResult, error) {
	// Create a map to combine results by chunk ID
	resultMap := make(map[int64]*SearchResult)
	
	// Add vector results
	for _, vr := range vectorResults {
		result := vr
		result.Score = vr.VectorScore * e.config.VectorWeight
		resultMap[vr.ChunkID] = &result
	}
	
	// Merge text results
	for _, tr := range textResults {
		if existing, ok := resultMap[tr.ChunkID]; ok {
			// Combine scores
			existing.TextScore = tr.TextScore
			existing.Score = (existing.VectorScore * e.config.VectorWeight) + 
			                 (tr.TextScore * e.config.BM25Weight)
			existing.Highlights = tr.Highlights
		} else {
			// Add new result
			result := tr
			result.Score = tr.TextScore * e.config.BM25Weight
			resultMap[tr.ChunkID] = &result
		}
	}
	
	// Calculate metadata scores and finalize
	var results []SearchResult
	for _, result := range resultMap {
		// Calculate metadata score
		metadataScore := e.calculateMetadataScore(result, req)
		result.MetadataScore = metadataScore
		
		// Final score calculation
		if result.VectorScore > 0 && result.TextScore > 0 {
			// Both scores available - full hybrid
			result.Score = (result.VectorScore * e.config.VectorWeight) +
			              (result.TextScore * e.config.BM25Weight) +
			              (metadataScore * e.config.MetadataWeight)
		} else if result.VectorScore > 0 {
			// Only vector score
			result.Score = (result.VectorScore * (e.config.VectorWeight + e.config.BM25Weight)) +
			              (metadataScore * e.config.MetadataWeight)
		} else {
			// Only text score
			result.Score = (result.TextScore * (e.config.BM25Weight + e.config.VectorWeight)) +
			              (metadataScore * e.config.MetadataWeight)
		}
		
		// Filter by minimum score
		if result.Score >= e.config.MinScore {
			results = append(results, *result)
		}
	}
	
	return results, nil
}

// calculateMetadataScore calculates score based on metadata factors
func (e *Engine) calculateMetadataScore(result *SearchResult, req *SearchRequest) float64 {
	score := 0.5 // Base score
	
	// File type relevance
	if req.FileTypes != nil && len(req.FileTypes) > 0 {
		for _, ft := range req.FileTypes {
			if result.FileType == ft {
				score += 0.2
				break
			}
		}
	}
	
	// Path relevance
	if req.Paths != nil && len(req.Paths) > 0 {
		for _, path := range req.Paths {
			if strings.Contains(result.FilePath, path) {
				score += 0.15
				break
			}
		}
	}
	
	// Extension match
	if req.Extensions != nil && len(req.Extensions) > 0 {
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

// applyFilters applies additional filters to results
func (e *Engine) applyFilters(results []SearchResult, req *SearchRequest) []SearchResult {
	if req.FileTypes == nil && req.Extensions == nil && req.Paths == nil &&
	   req.DateFrom == nil && req.DateTo == nil &&
	   req.MinSize == 0 && req.MaxSize == 0 {
		return results
	}
	
	var filtered []SearchResult
	for _, result := range results {
		// File type filter
		if req.FileTypes != nil && len(req.FileTypes) > 0 {
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
		if req.Extensions != nil && len(req.Extensions) > 0 {
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
		if req.Paths != nil && len(req.Paths) > 0 {
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
	
	return filtered
}

// normalizeVectorScore normalizes vector similarity score to 0-1 range
func (e *Engine) normalizeVectorScore(score float64) float64 {
	// Cosine similarity is already in [-1, 1], convert to [0, 1]
	normalized := (score + 1.0) / 2.0
	
	// Apply sigmoid for better distribution
	return 1.0 / (1.0 + math.Exp(-10*(normalized-0.5)))
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
	
	// Handle boolean operators
	query = strings.ReplaceAll(query, " AND ", " & ")
	query = strings.ReplaceAll(query, " OR ", " | ")
	query = strings.ReplaceAll(query, " NOT ", " ! ")
	
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
func (e *Engine) validateRequest(req *SearchRequest) error {
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
func (e *Engine) generateCacheKey(req *SearchRequest) string {
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

// ClearCache clears the search cache
func (e *Engine) ClearCache() {
	e.cache.Clear()
}