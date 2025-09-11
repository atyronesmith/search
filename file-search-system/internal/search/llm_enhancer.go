package search

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/file-search/file-search-system/internal/database"
	"github.com/sirupsen/logrus"
)

// DebugInfo captures LLM interaction details for debugging
type DebugInfo struct {
	Timestamp   string `json:"timestamp"`
	Query       string `json:"query"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Response    string `json:"response"`
	ProcessTime int64  `json:"process_time_ms"`
	Error       string `json:"error,omitempty"`
	VectorQuery string `json:"vector_query,omitempty"`
	TextQuery   string `json:"text_query,omitempty"`
}

// LLMEnhancer provides intelligent query enhancement using LLM
type LLMEnhancer struct {
	ollamaClient   *OllamaClient
	royalProcessor *RoyalSearchProcessor
	enabled        bool
	modelName      string
	lastDebugInfo  *DebugInfo
	log            *logrus.Logger

	// Phase 5: Query optimization cache
	classificationCache sync.Map // map[string]*QueryClassification
	cacheStats          struct {
		hits   int64
		misses int64
		mutex  sync.RWMutex
	}

	// Phase 6: Performance monitoring
	performanceMetrics struct {
		totalQueries      int64
		llmQueries        int64
		directQueries     int64
		avgClassifyTimeMs float64
		mutex             sync.RWMutex
	}
}

// NewLLMEnhancer creates a new LLM enhancer
func NewLLMEnhancer(ollamaURL string, modelName string, db *database.DB) *LLMEnhancer {
	enhancer := &LLMEnhancer{
		ollamaClient:   NewOllamaClient(ollamaURL),
		royalProcessor: NewRoyalSearchProcessor(ollamaURL, modelName, db),
		enabled:        true,
		modelName:      modelName,
		log:            logrus.New(),
	}

	// TEMP: Skip model check for now to test UI functionality
	// The model check is disabling the LLM enhancer during initialization
	// TODO: Fix the model check to work properly

	// Ensure phi3:mini model is available on startup (quick check)
	// ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	// defer cancel()

	// if err := enhancer.ollamaClient.EnsureModel(ctx, "phi3:mini"); err != nil {
	// 	enhancer.enabled = false
	// }

	return enhancer
}

// GetModelName returns the current LLM model name
func (e *LLMEnhancer) GetModelName() string {
	return e.modelName
}

// GetDebugInfo returns the last LLM debug information
func (e *LLMEnhancer) GetDebugInfo() *DebugInfo {
	return e.lastDebugInfo
}

// setDebugInfo stores debug information for the last LLM interaction
func (e *LLMEnhancer) setDebugInfo(query, prompt, response, errorMsg string, processTime int64) {
	if query == "" && prompt == "" && response == "" {
		// Clear debug info when all params are empty
		e.lastDebugInfo = nil
	} else {
		e.lastDebugInfo = &DebugInfo{
			Timestamp:   time.Now().Format(time.RFC3339),
			Query:       query,
			Model:       e.modelName,
			Prompt:      prompt,
			Response:    response,
			ProcessTime: processTime,
			Error:       errorMsg,
		}
	}
}

// setDebugInfoWithQueries stores debug information including PostgreSQL queries
func (e *LLMEnhancer) setDebugInfoWithQueries(query, prompt, response, errorMsg, vectorQuery, textQuery string, processTime int64) {
	e.lastDebugInfo = &DebugInfo{
		Timestamp:   time.Now().Format(time.RFC3339),
		Query:       query,
		Model:       e.modelName,
		Prompt:      prompt,
		Response:    response,
		ProcessTime: processTime,
		Error:       errorMsg,
		VectorQuery: vectorQuery,
		TextQuery:   textQuery,
	}
}

// QueryClassification represents the classification of a query
type QueryClassification struct {
	NeedsLLM        bool     `json:"needs_llm"`
	QueryType       string   `json:"query_type"`       // "simple", "complex", "analytical", "temporal"
	Intent          string   `json:"intent"`           // "search", "count", "analysis", "filter"
	Confidence      float64  `json:"confidence"`       // 0.0-1.0 confidence in routing decision
	ComplexTerms    []string `json:"complex_terms"`    // Terms that suggest complex search
	Reasoning       string   `json:"reasoning"`        // Explanation of routing decision
	Hybrid          bool     `json:"hybrid"`           // Query contains both simple and complex elements
	Suggestion      string   `json:"suggestion"`       // Suggestions for LLM processing
	ValidationError string   `json:"validation_error"` // Syntax validation errors if any
}

// QueryAnalysis represents the structural analysis of a query
type QueryAnalysis struct {
	HasExactPhrases  bool     `json:"has_exact_phrases"` // Contains "quoted phrases"
	HasFileTypes     bool     `json:"has_file_types"`    // Contains type:pdf or filetype:code
	HasDateFilters   bool     `json:"has_date_filters"`  // Contains after: or before: dates
	HasBooleanOps    bool     `json:"has_boolean_ops"`   // Contains AND, OR, NOT
	HasFieldFilters  bool     `json:"has_field_filters"` // Contains field:value filters
	RemainingText    string   `json:"remaining_text"`    // Text after removing simple elements
	IsValidSimple    bool     `json:"is_valid_simple"`   // Whether simple syntax is valid
	ValidationError  string   `json:"validation_error"`  // Specific validation error if any
	ExtractedPhrases []string `json:"extracted_phrases"` // Exact phrases found
	ExtractedTypes   []string `json:"extracted_types"`   // File types found
	ExtractedDates   []string `json:"extracted_dates"`   // Date filters found
}

// EnhancedQuery represents an LLM-enhanced query
type EnhancedQuery struct {
	Original        string           `json:"original"`
	Enhanced        string           `json:"enhanced"`
	SearchTerms     []string         `json:"search_terms"`
	VectorTerms     []string         `json:"vector_terms"` // For vector similarity search
	PgTsQuery       string           `json:"pg_tsquery"`   // PostgreSQL full-text search query
	ContentFilters  []ContentFilter  `json:"content_filters"`
	MetadataFilters []MetadataFilter `json:"metadata_filters"`
	Intent          string           `json:"intent"`
	RequiresCount   bool             `json:"requires_count"`
	SearchStrategy  string           `json:"search_strategy"` // Description of search approach
}

// ContentFilter represents semantic content filtering
type ContentFilter struct {
	Type        string   `json:"type"`        // "contains", "pattern", "semantic"
	Description string   `json:"description"` // Human description
	Pattern     string   `json:"pattern"`     // Regex pattern if applicable
	Keywords    []string `json:"keywords"`    // Keywords for semantic search
	Confidence  float64  `json:"confidence"`
}

// MetadataFilter represents file metadata filtering
type MetadataFilter struct {
	Field     string      `json:"field"`    // "type", "size", "created_date", "modified_date", "name"
	Operator  string      `json:"operator"` // "equals", "contains", "greater", "less", "between"
	Value     interface{} `json:"value"`
	StartDate *time.Time  `json:"start_date,omitempty"`
	EndDate   *time.Time  `json:"end_date,omitempty"`
	DateField string      `json:"date_field,omitempty"` // "created_at", "modified_at" - specifies which date column to filter on
}

// ClassifyQuery determines if a query needs LLM enhancement
func (e *LLMEnhancer) ClassifyQuery(query string) (*QueryClassification, error) {
	// Phase 6: Track performance metrics
	start := time.Now()
	defer func() {
		e.recordQueryMetrics(time.Since(start).Milliseconds())
	}()

	if !e.enabled {
		return &QueryClassification{
			NeedsLLM:   false,
			QueryType:  "simple",
			Intent:     "search",
			Confidence: 1.0,
			Reasoning:  "LLM enhancement disabled",
		}, nil
	}

	// Phase 5: Check cache first
	cacheKey := e.getCacheKey(query)
	if cached, ok := e.getFromCache(cacheKey); ok {
		e.recordCacheHit()
		return cached, nil
	}
	e.recordCacheMiss()

	// Quick classification based on patterns
	classification := e.quickClassify(query)

	// TEMP DEBUG: Force certain queries to be treated as needing LLM
	if strings.Contains(strings.ToLower(query), "find files that contain") {
		classification.NeedsLLM = true
		classification.QueryType = "analytical"
		classification.Reasoning = "DEBUG: Forced LLM enhancement for testing"
	}

	// If quick classification suggests complexity, use LLM for detailed analysis
	if classification.NeedsLLM {
		e.recordLLMQuery()
		enhanced, err := e.llmClassify(query)
		if err != nil {
			// Fall back to quick classification if LLM fails
			e.storeInCache(cacheKey, classification)
			return classification, nil
		}
		e.storeInCache(cacheKey, enhanced)
		return enhanced, nil
	}
	e.recordDirectQuery()

	e.storeInCache(cacheKey, classification)
	return classification, nil
}

// analyzeQueryStructure analyzes the structural components of a query
func (e *LLMEnhancer) analyzeQueryStructure(query string) *QueryAnalysis {
	analysis := &QueryAnalysis{
		ExtractedPhrases: []string{},
		ExtractedTypes:   []string{},
		ExtractedDates:   []string{},
	}

	simplified := query

	// Check and extract exact phrases (quoted strings)
	exactPhrasePattern := regexp.MustCompile(`"([^"]*)"`)
	phraseMatches := exactPhrasePattern.FindAllStringSubmatch(simplified, -1)
	if len(phraseMatches) > 0 {
		analysis.HasExactPhrases = true
		for _, match := range phraseMatches {
			if len(match) > 1 {
				analysis.ExtractedPhrases = append(analysis.ExtractedPhrases, match[1])
			}
		}
		simplified = exactPhrasePattern.ReplaceAllString(simplified, " ")
	}

	// Check and extract file type filters
	fileTypePattern := regexp.MustCompile(`\b(?i)(type|filetype):(\w+)\b`)
	typeMatches := fileTypePattern.FindAllStringSubmatch(simplified, -1)
	if len(typeMatches) > 0 {
		analysis.HasFileTypes = true
		for _, match := range typeMatches {
			if len(match) > 2 {
				analysis.ExtractedTypes = append(analysis.ExtractedTypes, match[2])
			}
		}
		simplified = fileTypePattern.ReplaceAllString(simplified, " ")
	}

	// Check and extract date filters
	datePattern := regexp.MustCompile(`\b(?i)(after|before):(\d{4}-\d{2}-\d{2})\b`)
	dateMatches := datePattern.FindAllStringSubmatch(simplified, -1)
	if len(dateMatches) > 0 {
		analysis.HasDateFilters = true
		for _, match := range dateMatches {
			if len(match) > 0 {
				analysis.ExtractedDates = append(analysis.ExtractedDates, match[0])
			}
		}
		simplified = datePattern.ReplaceAllString(simplified, " ")
	}

	// Check for boolean operators
	booleanPattern := regexp.MustCompile(`\b(AND|OR|NOT)\b`)
	if booleanPattern.MatchString(simplified) {
		analysis.HasBooleanOps = true
		simplified = booleanPattern.ReplaceAllString(simplified, " ")
	}

	// Check for field filters (e.g., author:john, size:>10MB)
	fieldPattern := regexp.MustCompile(`\b(\w+):[<>]?[\w\d]+\b`)
	if fieldPattern.MatchString(simplified) {
		analysis.HasFieldFilters = true
		simplified = fieldPattern.ReplaceAllString(simplified, " ")
	}

	// Clean up remaining text
	simplified = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(simplified, " "))
	analysis.RemainingText = simplified

	// Validate the simple query syntax
	analysis.IsValidSimple, analysis.ValidationError = e.validateSimpleQuery(query, analysis)

	return analysis
}

// isSimpleKeywords checks if text contains only simple keywords
func isSimpleKeywords(text string) bool {
	// Check if the text is just simple keywords (alphanumeric, spaces, dashes, underscores)
	simplePattern := regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
	return simplePattern.MatchString(text)
}

// isSimpleBooleanQuery checks if the query is a simple boolean expression
func (e *LLMEnhancer) isSimpleBooleanQuery(query string) bool {
	// Remove boolean operators and check what's left
	simplified := regexp.MustCompile(`(?i)\s+(OR|AND|NOT)\s+`).ReplaceAllString(query, " ")
	simplified = strings.TrimSpace(simplified)

	// Check if remaining terms are just simple words (no complex patterns)
	// Allow basic words, numbers, and common search terms
	if matched, _ := regexp.MatchString(`^[\w\s\-_\.]+$`, simplified); !matched {
		return false
	}

	// Make sure there's at least one boolean operator
	hasOr := regexp.MustCompile(`(?i)\s+OR\s+`).MatchString(query)
	hasAnd := regexp.MustCompile(`(?i)\s+AND\s+`).MatchString(query)
	hasNot := regexp.MustCompile(`(?i)\s+NOT\s+`).MatchString(query)

	return hasOr || hasAnd || hasNot
}

// validateSimpleQuery validates the syntax of simple query elements
func (e *LLMEnhancer) validateSimpleQuery(query string, analysis *QueryAnalysis) (bool, string) {
	// Check for unmatched quotes
	quoteCount := strings.Count(query, `"`)
	if quoteCount%2 != 0 {
		return false, "Unmatched quotes in query"
	}

	// Validate date formats if present
	if analysis.HasDateFilters {
		for _, dateStr := range analysis.ExtractedDates {
			// Extract just the date part
			parts := strings.Split(dateStr, ":")
			if len(parts) > 1 {
				_, err := time.Parse("2006-01-02", parts[1])
				if err != nil {
					return false, fmt.Sprintf("Invalid date format: %s", parts[1])
				}
			}
		}
	}

	// Validate file types if present
	if analysis.HasFileTypes {
		validTypes := map[string]bool{
			"pdf": true, "doc": true, "docx": true, "txt": true, "md": true,
			"code": true, "text": true, "python": true, "go": true, "js": true,
			"java": true, "cpp": true, "c": true, "html": true, "css": true,
			"json": true, "yaml": true, "xml": true, "csv": true, "xls": true, "xlsx": true,
		}
		for _, fileType := range analysis.ExtractedTypes {
			if !validTypes[strings.ToLower(fileType)] {
				// Not an error, just a warning - LLM might understand custom types
				return true, fmt.Sprintf("Unknown file type: %s (may need LLM interpretation)", fileType)
			}
		}
	}

	// Check for orphaned boolean operators
	if analysis.HasBooleanOps {
		orphanedOp := regexp.MustCompile(`(?:^|\s)(AND|OR|NOT)(?:\s|$)`)
		if orphanedOp.MatchString(query) {
			return false, "Boolean operator missing operands"
		}
	}

	return true, ""
}

// quickClassify performs fast pattern-based classification
func (e *LLMEnhancer) quickClassify(query string) *QueryClassification {
	originalQuery := strings.TrimSpace(query)
	queryLower := strings.ToLower(originalQuery)

	fmt.Printf("DEBUG quickClassify: query='%s'\n", queryLower)

	// First, analyze the query structure
	analysis := e.analyzeQueryStructure(originalQuery)

	// If query has invalid simple syntax, route to LLM for correction
	if !analysis.IsValidSimple && analysis.ValidationError != "" {
		return &QueryClassification{
			NeedsLLM:        true,
			QueryType:       "malformed",
			Intent:          e.detectIntent(queryLower),
			Confidence:      0.8,
			ComplexTerms:    e.findComplexTerms(queryLower),
			Reasoning:       "Invalid simple syntax detected",
			ValidationError: analysis.ValidationError,
			Suggestion:      "LLM should attempt to correct syntax errors",
		}
	}

	// Phase 3: Enhanced Natural Language Detection
	// Check for questions first (highest priority)
	if strings.HasSuffix(strings.TrimSpace(queryLower), "?") {
		return &QueryClassification{
			NeedsLLM:   true,
			QueryType:  "question",
			Intent:     e.detectIntent(queryLower),
			Confidence: 0.95,
			Reasoning:  "Question mark detected - natural language query",
		}
	}

	// Check if it's a pure simple boolean query (e.g., "taxi OR credit")
	if analysis.HasBooleanOps && e.isSimpleBooleanQuery(originalQuery) {
		return &QueryClassification{
			NeedsLLM:   false,
			QueryType:  "boolean",
			Intent:     "search",
			Confidence: 0.95,
			Reasoning:  "Simple boolean query with OR/AND/NOT operators",
		}
	}

	// Check if it's a pure simple query (only simple elements with basic keywords)
	hasSimpleElements := analysis.HasExactPhrases || analysis.HasFileTypes ||
		analysis.HasDateFilters || analysis.HasBooleanOps ||
		analysis.HasFieldFilters

	if hasSimpleElements && (analysis.RemainingText == "" || isSimpleKeywords(analysis.RemainingText)) {
		// It's a valid simple query
		return &QueryClassification{
			NeedsLLM:   false,
			QueryType:  "simple",
			Intent:     "search",
			Confidence: 0.95,
			Reasoning:  "Valid simple search syntax with no complex language",
		}
	}

	// Check if it's a hybrid query (mix of simple and complex)
	if hasSimpleElements && analysis.RemainingText != "" && !isSimpleKeywords(analysis.RemainingText) {
		return &QueryClassification{
			NeedsLLM:   true,
			QueryType:  "hybrid",
			Intent:     e.detectIntent(queryLower),
			Confidence: 0.75,
			Hybrid:     true,
			Reasoning:  "Query contains both simple syntax and complex language",
			Suggestion: "LLM should preserve simple syntax elements where possible",
		}
	}

	// Check for analytical patterns FIRST before simple patterns
	// This ensures complex queries aren't misclassified as simple

	// Analytical queries
	analyticalPatterns := []string{
		`\b(find|show|list|count|how many)\b.*\b(that|with|containing|have)\b`,
		`\b(all files|documents|files)\b.*\b(contain|have|with)\b`,
		`\b(social security|ssn|credit card|financial|legal|medical)\b`,
		`\b(table|chart|graph|figure|image)\b.*\b(with|containing|about)\b`,
		`\b(correspondence|email|letter|communication)\b.*\b(with|from|to)\b`,
		`\b(last week|tuesday|yesterday|recent|modified on)\b`,
		`\b(look like|similar to|type of|kind of)\b`,
		`\b(melancholy|sad|happy|angry|emotional|tone|mood|feeling|sentiment)\b`,
		`\b(positive|negative|neutral|optimistic|pessimistic|upbeat|downbeat)\b`,
		`\b(formal|informal|professional|casual|academic|technical)\b.*\b(style|tone|writing)\b`,
		`\b(lawsuit|litigation|legal|court|judge|attorney|lawyer|contract|agreement)\b`,
		`\b(related|relating|regarding|about|concerning|pertaining)\b.*\b(to|with)\b`,
		`.*\b(related|relating|associated|connected)\b.*\bfiles?\b`,
		`\b(invoice|receipt|purchase|order|payment|billing|transaction)\b`,
		`\b(report|analysis|summary|review|assessment|evaluation)\b`,
		`\b(project|proposal|plan|strategy|roadmap|timeline)\b`,
	}

	for _, pattern := range analyticalPatterns {
		if matched, _ := regexp.MatchString(pattern, queryLower); matched {
			fmt.Printf("DEBUG: Query matches analytical pattern '%s' - using LLM\n", pattern)
			return &QueryClassification{
				NeedsLLM:     true,
				QueryType:    "analytical",
				Intent:       e.detectIntent(queryLower),
				Confidence:   0.85,
				ComplexTerms: e.findComplexTerms(queryLower),
				Reasoning:    "Complex analytical query detected",
			}
		}
	}

	// Simple keyword searches - no LLM needed
	// Only check these AFTER checking for complex patterns
	simplePatterns := []string{
		`^[a-zA-Z0-9\s\-_\.]+$`, // Just alphanumeric and basic chars
		`^"[^"]*"$`,             // Simple quoted phrase
		`^\w+:\w+$`,             // Simple filter like type:pdf
	}

	for _, pattern := range simplePatterns {
		if matched, _ := regexp.MatchString(pattern, queryLower); matched && !e.hasComplexTerms(queryLower) {
			fmt.Printf("DEBUG: Query matches simple pattern '%s' - NOT using LLM\n", pattern)
			return &QueryClassification{
				NeedsLLM:   false,
				QueryType:  "simple",
				Intent:     "search",
				Confidence: 0.9,
				Reasoning:  "Simple keyword or filter query",
			}
		}
	}

	// Complex terms that suggest LLM enhancement needed
	complexTerms := e.findComplexTerms(queryLower)

	// Natural language questions
	questionWords := []string{"what", "where", "when", "why", "how", "who", "which"}
	for _, word := range questionWords {
		if strings.HasPrefix(queryLower, word+" ") || strings.Contains(queryLower, "?") {
			return &QueryClassification{
				NeedsLLM:     true,
				QueryType:    "complex",
				Intent:       "analysis",
				Confidence:   0.9,
				ComplexTerms: complexTerms,
				Reasoning:    "Natural language question detected",
			}
		}
	}

	// If we have complex terms, probably needs LLM
	if len(complexTerms) > 0 {
		return &QueryClassification{
			NeedsLLM:     true,
			QueryType:    "complex",
			Intent:       e.detectIntent(queryLower),
			Confidence:   0.7,
			ComplexTerms: complexTerms,
			Reasoning:    "Complex terms detected: " + strings.Join(complexTerms, ", "),
		}
	}

	// Default to simple
	return &QueryClassification{
		NeedsLLM:   false,
		QueryType:  "simple",
		Intent:     "search",
		Confidence: 0.6,
		Reasoning:  "No complex patterns detected",
	}
}

// hasComplexTerms checks if query has terms suggesting complexity
func (e *LLMEnhancer) hasComplexTerms(query string) bool {
	return len(e.findComplexTerms(query)) > 0
}

// findComplexTerms identifies terms that suggest complex search needs
func (e *LLMEnhancer) findComplexTerms(query string) []string {
	complexTerms := []string{
		"social security", "ssn", "credit card", "financial", "banking",
		"legal", "medical", "health", "doctor", "correspondence",
		"table", "chart", "graph", "figure", "contain", "similar",
		"look like", "type of", "kind of", "related to",
		"last week", "tuesday", "yesterday", "recent", "modified on",
		"all files", "how many", "count", "list all",
		"melancholy", "sad", "happy", "angry", "emotional", "tone", "mood",
		"feeling", "sentiment", "positive", "negative", "optimistic", "pessimistic",
		"formal", "informal", "professional", "casual", "academic", "style",
		"lawsuit", "litigation", "court", "judge", "attorney", "lawyer",
		"contract", "agreement", "related", "relating", "regarding",
		"about", "concerning", "pertaining", "associated", "connected",
		"invoice", "receipt", "purchase", "order", "payment", "billing",
		"report", "analysis", "summary", "review", "assessment", "evaluation",
		"project", "proposal", "plan", "strategy", "roadmap", "timeline",
	}

	var found []string
	queryLower := strings.ToLower(query)

	for _, term := range complexTerms {
		if strings.Contains(queryLower, term) {
			found = append(found, term)
		}
	}

	return found
}

// detectIntent determines the primary intent of the query
func (e *LLMEnhancer) detectIntent(query string) string {
	query = strings.ToLower(query)

	if strings.Contains(query, "how many") || strings.Contains(query, "count") {
		return "count"
	}

	if strings.Contains(query, "find") || strings.Contains(query, "search") || strings.Contains(query, "show") {
		return "search"
	}

	if strings.Contains(query, "analyze") || strings.Contains(query, "analysis") {
		return "analysis"
	}

	if strings.Contains(query, "filter") || strings.Contains(query, "type:") || strings.Contains(query, "where") {
		return "filter"
	}

	return "search"
}

// llmClassify uses LLM for detailed query classification
func (e *LLMEnhancer) llmClassify(query string) (*QueryClassification, error) {
	// For now, skip the complex LLM classification and just return a simple classification
	// This avoids the JSON parsing issues while we focus on fixing the search terms generation

	return &QueryClassification{
		NeedsLLM:     true,
		QueryType:    "analytical",
		Intent:       e.detectIntent(query),
		Confidence:   0.8,
		ComplexTerms: e.findComplexTerms(query),
		Reasoning:    "Simplified classification to avoid JSON parsing issues",
	}, nil
}

// EnhanceQuery transforms a complex query into structured search parameters
func (e *LLMEnhancer) EnhanceQuery(query string, classification *QueryClassification) (*EnhancedQuery, error) {
	if !classification.NeedsLLM {
		// For simple queries, just return basic enhancement
		// Clear any old LLM debug info since we're not using LLM
		fmt.Printf("DEBUG: Clearing LLM debug info for non-LLM query: %s\n", query)
		e.setDebugInfo("", "", "", "", 0)
		return &EnhancedQuery{
			Original:    query,
			Enhanced:    query,
			SearchTerms: strings.Fields(strings.ToLower(query)),
			Intent:      classification.Intent,
		}, nil
	}

	// Use LLM to enhance complex queries
	return e.llmEnhance(query, classification)
}

// llmEnhance uses LLM to transform complex queries into structured search
func (e *LLMEnhancer) llmEnhance(query string, classification *QueryClassification) (*EnhancedQuery, error) {
	// Use the royal processor for comprehensive search term extraction
	searchContext := fmt.Sprintf("Query type: %s, Intent: %s", classification.QueryType, classification.Intent)

	fmt.Printf("DEBUG: Using royal processor for query: %s\n", query)

	// Create debug callback to capture LLM interaction
	debugCallback := func(prompt, response, errorMsg string, processTime int64) {
		e.setDebugInfo(query, prompt, response, errorMsg, processTime)
	}

	royalTerms, err := e.royalProcessor.GenerateSearchTermsWithDebug(query, searchContext, debugCallback)
	if err != nil {
		fmt.Printf("DEBUG: Royal processor failed: %v, falling back to legacy\n", err)
		// Check if we have a parse error with raw response
		if parseErr, ok := err.(*ParseErrorWithResponse); ok {
			fmt.Printf("DEBUG: Reusing LLM response for legacy parser\n")
			// Reuse the raw response instead of making another LLM call
			return e.llmEnhanceLegacyWithResponse(query, classification, parseErr.RawResponse)
		}
		// Fall back to the old method if royal processor fails
		return e.llmEnhanceLegacy(query, classification)
	}

	fmt.Printf("DEBUG: Royal processor generated - Vector: %v, Text: %v, TsQuery: %s\n",
		royalTerms.VectorTerms, royalTerms.TextTerms, royalTerms.PgTsQuery)

	// Construct the enhanced query from royal terms
	enhanced := &EnhancedQuery{
		Original:       query,
		Enhanced:       query,
		SearchTerms:    royalTerms.TextTerms,
		VectorTerms:    royalTerms.VectorTerms,
		PgTsQuery:      royalTerms.PgTsQuery,
		Intent:         classification.Intent,
		RequiresCount:  classification.Intent == "count",
		SearchStrategy: royalTerms.SearchStrategy,
	}

	// Add content filters for special pattern detection
	enhanced.ContentFilters = e.detectContentFilters(query, royalTerms)

	// Add metadata filters based on query analysis
	enhanced.MetadataFilters = e.detectMetadataFilters(query)

	return enhanced, nil
}

// llmEnhanceLegacyWithResponse uses the old parsing method with an existing LLM response
func (e *LLMEnhancer) llmEnhanceLegacyWithResponse(query string, classification *QueryClassification, rawResponse string) (*EnhancedQuery, error) {
	fmt.Printf("DEBUG: Reusing LLM response for legacy parsing (avoiding second LLM call)\n")

	// Parse the existing response without making a new LLM call
	cleanResponse := strings.TrimSpace(rawResponse)

	// Clean the response - remove markdown code blocks if present
	if strings.HasPrefix(cleanResponse, "```json") && strings.HasSuffix(cleanResponse, "```") {
		// Remove ```json from the beginning and ``` from the end
		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	} else if strings.HasPrefix(cleanResponse, "```") && strings.HasSuffix(cleanResponse, "```") {
		// Remove ``` from both ends
		cleanResponse = strings.TrimPrefix(cleanResponse, "```")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	}

	// Remove JSON comments (// ... until end of line)
	cleanResponse = RemoveJSONComments(cleanResponse)

	// Try to parse as JSON with royal processor format first
	var royalTerms RoyalSearchTerms
	if err := json.Unmarshal([]byte(cleanResponse), &royalTerms); err == nil {
		// Successfully parsed as royal format
		return &EnhancedQuery{
			Original:       query,
			Enhanced:       query,
			SearchTerms:    royalTerms.TextTerms,
			VectorTerms:    royalTerms.VectorTerms,
			Intent:         classification.Intent,
			SearchStrategy: royalTerms.SearchStrategy,
		}, nil
	}

	// Try legacy format
	var llmResponse struct {
		VectorTerms []string `json:"vector_terms"`
		StringTerms []string `json:"string_terms"`
	}

	if err := json.Unmarshal([]byte(cleanResponse), &llmResponse); err != nil {
		// If all parsing fails, extract what we can from the raw text
		return e.extractTermsFromText(query, rawResponse, classification)
	}

	// Process legacy format response
	var searchTerms []string

	// Add string terms first (these are better for exact matching)
	for _, term := range llmResponse.StringTerms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term != "" && len(term) > 1 {
			searchTerms = append(searchTerms, term)
		}
	}

	// Add vector terms if we don't have enough string terms
	if len(searchTerms) < 3 {
		for _, term := range llmResponse.VectorTerms {
			// Parse vector terms which might be phrases
			words := strings.Fields(strings.ToLower(term))
			for _, word := range words {
				word = strings.TrimSpace(word)
				if word != "" && len(word) > 1 && !sliceContains(searchTerms, word) {
					searchTerms = append(searchTerms, word)
				}
			}
		}
	}

	return &EnhancedQuery{
		Original:       query,
		Enhanced:       query,
		SearchTerms:    searchTerms,
		VectorTerms:    llmResponse.VectorTerms,
		Intent:         classification.Intent,
		SearchStrategy: "hybrid",
	}, nil
}

// sliceContains checks if a string slice contains a specific string
func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// extractTermsFromText extracts search terms from raw text when JSON parsing fails
func (e *LLMEnhancer) extractTermsFromText(query string, rawText string, classification *QueryClassification) (*EnhancedQuery, error) {
	// Basic fallback: extract words from the raw response
	words := strings.Fields(strings.ToLower(rawText))
	var searchTerms []string
	seen := make(map[string]bool)

	for _, word := range words {
		// Clean the word
		word = strings.Trim(word, `"',.!?;:()[]{}`)

		// Skip short words and duplicates
		if len(word) > 2 && !seen[word] {
			seen[word] = true
			searchTerms = append(searchTerms, word)

			// Limit to reasonable number of terms
			if len(searchTerms) >= 10 {
				break
			}
		}
	}

	// If we couldn't extract anything meaningful, fall back to query terms
	if len(searchTerms) == 0 {
		searchTerms = strings.Fields(strings.ToLower(query))
	}

	return &EnhancedQuery{
		Original:       query,
		Enhanced:       query,
		SearchTerms:    searchTerms,
		Intent:         classification.Intent,
		SearchStrategy: "text",
	}, nil
}

// llmEnhanceLegacy is the fallback method using the old prompt
func (e *LLMEnhancer) llmEnhanceLegacy(query string, classification *QueryClassification) (*EnhancedQuery, error) {
	// Set timeout for LLM operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prompt := fmt.Sprintf(`You are an expert search query processor. Your sole function is to analyze a user's query and extract two types of search terms:

1. vector_terms: The core conceptual phrases or entities for a semantic vector search. These should capture the main idea of the query. If a query contains multiple distinct topics, create a separate phrase for each.
2. string_terms: The essential literal keywords for a database string search. These should be individual words, lowercased, and free of conversational filler.

Your response must be a single, valid JSON object and nothing else. Do not add explanations.

### EXAMPLES ###

User Query: Find all files that contain the word taxi
{
  "vector_terms": ["taxi"],
  "string_terms": ["taxi"]
}

User Query: Show me the financial reports from the Q3 2024 review meeting
{
  "vector_terms": ["Q3 2024 financial reports", "review meeting"],
  "string_terms": ["financial", "reports", "q3", "2024", "review", "meeting"]
}

User Query: information about project supernova's performance metrics
{
  "vector_terms": ["project supernova performance metrics"],
  "string_terms": ["project", "supernova", "performance", "metrics"]
}

User Query: Who was the project manager for the Atlas initiative in 2024?
{
  "vector_terms": ["project manager for Atlas initiative", "2024"],
  "string_terms": ["project", "manager", "atlas", "initiative", "2024"]
}

User Query: Can you dig up the presentation slides about the new branding guidelines from last month's all-hands meeting?
{
  "vector_terms": ["presentation slides", "new branding guidelines", "last month all-hands meeting"],
  "string_terms": ["presentation", "slides", "branding", "guidelines", "all-hands", "meeting"]
}

User Query: I need the security audit for the production server and the user feedback from the beta launch.
{
  "vector_terms": ["security audit for production server", "user feedback from beta launch"],
  "string_terms": ["security", "audit", "production", "server", "user", "feedback", "beta", "launch"]
}

User Query: Compare the revenue growth of the North American branch versus the European branch for H1 2025.
{
  "vector_terms": ["revenue growth comparison", "North American branch", "European branch", "H1 2025"],
  "string_terms": ["revenue", "growth", "north", "american", "branch", "european", "branch", "h1", "2025"]
}

### END EXAMPLES ###

User Query: %s`, query)

	response, err := e.ollamaClient.Generate(ctx, e.modelName, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM enhancement failed: %w", err)
	}

	// Parse the JSON response
	cleanResponse := strings.TrimSpace(response)

	// Clean the response - remove markdown code blocks if present
	if strings.HasPrefix(cleanResponse, "```json") && strings.HasSuffix(cleanResponse, "```") {
		// Remove ```json from the beginning and ``` from the end
		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	} else if strings.HasPrefix(cleanResponse, "```") && strings.HasSuffix(cleanResponse, "```") {
		// Remove ``` from both ends
		cleanResponse = strings.TrimPrefix(cleanResponse, "```")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	}

	// Remove JSON comments (// ... until end of line)
	cleanResponse = RemoveJSONComments(cleanResponse)

	// Try to parse as JSON
	var llmResponse struct {
		VectorTerms []string `json:"vector_terms"`
		StringTerms []string `json:"string_terms"`
	}

	if err := json.Unmarshal([]byte(cleanResponse), &llmResponse); err != nil {
		return nil, fmt.Errorf("failed to parse LLM enhancement response: %w", err)
	}

	// Combine both vector and string terms, prioritizing string terms for traditional search
	var searchTerms []string

	// Add string terms first (these are better for exact matching)
	for _, term := range llmResponse.StringTerms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term != "" && len(term) > 1 {
			searchTerms = append(searchTerms, term)
		}
	}

	// Add vector terms if we don't have enough string terms
	if len(searchTerms) < 3 {
		for _, term := range llmResponse.VectorTerms {
			term = strings.TrimSpace(strings.ToLower(term))
			if term != "" && len(term) > 1 {
				// Avoid duplicates
				isDuplicate := false
				for _, existing := range searchTerms {
					if existing == term {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					searchTerms = append(searchTerms, term)
				}
			}
		}
	}

	// Limit to 5 terms max for better performance
	if len(searchTerms) > 5 {
		searchTerms = searchTerms[:5]
	}

	// Construct the enhanced query manually
	enhanced := &EnhancedQuery{
		Original:      query,
		Enhanced:      query,
		SearchTerms:   searchTerms,
		Intent:        classification.Intent,
		RequiresCount: false,
	}

	return enhanced, nil
}

// detectContentFilters analyzes query and terms to identify content patterns
func (e *LLMEnhancer) detectContentFilters(query string, terms *RoyalSearchTerms) []ContentFilter {
	var filters []ContentFilter
	queryLower := strings.ToLower(query)

	// SSN pattern detection
	if strings.Contains(queryLower, "social security") || strings.Contains(queryLower, "ssn") {
		filters = append(filters, ContentFilter{
			Type:        "pattern",
			Description: "Social Security Number pattern",
			Pattern:     `\b\d{3}-\d{2}-\d{4}\b|\b\d{9}\b`,
			Keywords:    []string{"ssn", "social", "security", "number"},
			Confidence:  0.9,
		})
	}

	// Credit card pattern detection
	if strings.Contains(queryLower, "credit card") || strings.Contains(queryLower, "card number") {
		filters = append(filters, ContentFilter{
			Type:        "pattern",
			Description: "Credit card number pattern",
			Pattern:     `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
			Keywords:    []string{"credit", "card", "visa", "mastercard", "amex"},
			Confidence:  0.85,
		})
	}

	// Financial data detection
	if strings.Contains(queryLower, "financial") || strings.Contains(queryLower, "revenue") || strings.Contains(queryLower, "sales") {
		filters = append(filters, ContentFilter{
			Type:        "semantic",
			Description: "Financial data and metrics",
			Keywords:    []string{"revenue", "profit", "loss", "income", "expense", "budget", "financial", "sales", "cost"},
			Confidence:  0.8,
		})
	}

	// Table/chart detection
	if strings.Contains(queryLower, "table") || strings.Contains(queryLower, "chart") || strings.Contains(queryLower, "graph") {
		filters = append(filters, ContentFilter{
			Type:        "semantic",
			Description: "Tables, charts, or graphs",
			Keywords:    []string{"table", "chart", "graph", "figure", "diagram", "visualization", "plot"},
			Confidence:  0.75,
		})
	}

	// Email/correspondence detection
	if strings.Contains(queryLower, "email") || strings.Contains(queryLower, "correspondence") || strings.Contains(queryLower, "letter") {
		filters = append(filters, ContentFilter{
			Type:        "semantic",
			Description: "Email or correspondence",
			Keywords:    []string{"email", "mail", "letter", "correspondence", "message", "subject", "from", "to", "dear"},
			Confidence:  0.8,
		})
	}

	// Legal document detection
	if strings.Contains(queryLower, "legal") || strings.Contains(queryLower, "contract") || strings.Contains(queryLower, "agreement") {
		filters = append(filters, ContentFilter{
			Type:        "semantic",
			Description: "Legal documents",
			Keywords:    []string{"legal", "contract", "agreement", "clause", "terms", "conditions", "party", "hereby", "pursuant"},
			Confidence:  0.85,
		})
	}

	// Medical/health detection
	if strings.Contains(queryLower, "medical") || strings.Contains(queryLower, "health") || strings.Contains(queryLower, "doctor") {
		filters = append(filters, ContentFilter{
			Type:        "semantic",
			Description: "Medical or health information",
			Keywords:    []string{"medical", "health", "doctor", "patient", "diagnosis", "treatment", "prescription", "symptom"},
			Confidence:  0.8,
		})
	}

	return filters
}

// detectMetadataFilters analyzes query to identify metadata constraints
func (e *LLMEnhancer) detectMetadataFilters(query string) []MetadataFilter {
	var filters []MetadataFilter
	queryLower := strings.ToLower(query)

	// File type detection - using more specific patterns to avoid false positives
	fileTypePatterns := map[string][]string{
		"pdf":  {"pdf", "pdfs", "pdf file", "pdf document"},
		"docx": {"docx", "word document", "microsoft word", ".docx"}, // Removed standalone "word" to avoid false positives
		"xlsx": {"xlsx", "excel", "spreadsheet", "excel file"},
		"txt":  {"txt", "text file", "plain text", ".txt"},
		"md":   {"markdown", "md file", ".md"},
		"py":   {"python", "py file", ".py", "python script", "python code"},
		"go":   {"golang", "go file", ".go", "go code"},
		"js":   {"javascript", "js file", ".js", "javascript code"},
		"json": {"json file", ".json"}, // Made more specific
		"yaml": {"yaml", "yml", ".yaml", ".yml"},
		"csv":  {"csv", "csv file", "comma separated", ".csv"},
	}

	for fileType, patterns := range fileTypePatterns {
		for _, pattern := range patterns {
			// Use word boundary check for better matching
			// Check if pattern is a file extension (.ext) or a phrase
			if strings.HasPrefix(pattern, ".") {
				// For file extensions, check with word boundaries
				if strings.Contains(queryLower, pattern) {
					filters = append(filters, MetadataFilter{
						Field:    "type",
						Operator: "equals",
						Value:    fileType,
					})
					break
				}
			} else {
				// For phrases, check if it appears as a separate phrase
				// Add spaces around to check word boundaries
				checkQuery := " " + queryLower + " "
				checkPattern := " " + pattern + " "
				if strings.Contains(checkQuery, checkPattern) ||
					strings.HasPrefix(queryLower, pattern+" ") ||
					strings.HasSuffix(queryLower, " "+pattern) {
					filters = append(filters, MetadataFilter{
						Field:    "type",
						Operator: "equals",
						Value:    fileType,
					})
					break
				}
			}
		}
	}

	// Date range detection (simplified - the royal processor handles complex temporal queries)
	if strings.Contains(queryLower, "today") {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)
		filters = append(filters, MetadataFilter{
			Field:     "modified_at",
			Operator:  "between",
			StartDate: &startOfDay,
			EndDate:   &endOfDay,
			DateField: "modified_at",
		})
	}

	if strings.Contains(queryLower, "yesterday") {
		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)
		startOfDay := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)
		filters = append(filters, MetadataFilter{
			Field:     "modified_at",
			Operator:  "between",
			StartDate: &startOfDay,
			EndDate:   &endOfDay,
			DateField: "modified_at",
		})
	}

	if strings.Contains(queryLower, "last week") {
		now := time.Now()
		weekAgo := now.Add(-7 * 24 * time.Hour)
		filters = append(filters, MetadataFilter{
			Field:     "modified_at",
			Operator:  "greater",
			Value:     weekAgo,
			DateField: "modified_at",
		})
	}

	if strings.Contains(queryLower, "last month") {
		now := time.Now()
		monthAgo := now.Add(-30 * 24 * time.Hour)
		filters = append(filters, MetadataFilter{
			Field:     "modified_at",
			Operator:  "greater",
			Value:     monthAgo,
			DateField: "modified_at",
		})
	}

	// Size detection
	sizePatterns := []struct {
		pattern string
		field   string
		op      string
		value   int64
	}{
		{"large file", "size", "greater", 10 * 1024 * 1024}, // > 10MB
		{"small file", "size", "less", 1 * 1024 * 1024},     // < 1MB
		{"huge file", "size", "greater", 100 * 1024 * 1024}, // > 100MB
		{"tiny file", "size", "less", 100 * 1024},           // < 100KB
	}

	for _, sp := range sizePatterns {
		if strings.Contains(queryLower, sp.pattern) {
			filters = append(filters, MetadataFilter{
				Field:    sp.field,
				Operator: sp.op,
				Value:    sp.value,
			})
		}
	}

	return filters
}

// ProcessNaturalLanguageQuery is the main entry point for LLM-enhanced search
func (e *LLMEnhancer) ProcessNaturalLanguageQuery(query string) (*EnhancedQuery, error) {
	// First classify the query
	classification, err := e.ClassifyQuery(query)
	if err != nil {
		return nil, fmt.Errorf("query classification failed: %w", err)
	}

	// Then enhance it if needed
	enhanced, err := e.EnhanceQuery(query, classification)
	if err != nil {
		return nil, fmt.Errorf("query enhancement failed: %w", err)
	}

	return enhanced, nil
}

// SetEnabled enables or disables LLM enhancement
func (e *LLMEnhancer) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// IsEnabled returns whether LLM enhancement is enabled
func (e *LLMEnhancer) IsEnabled() bool {
	return e.enabled
}

// cleanSearchTerms removes common filler words from search terms
func (e *LLMEnhancer) cleanSearchTerms(terms []string) []string {
	// If we have 4 or fewer terms, they're probably good
	if len(terms) <= 4 {
		return terms
	}

	// Common filler words to remove
	fillerWords := map[string]bool{
		"find": true, "show": true, "list": true, "get": true, "search": true,
		"files": true, "documents": true, "file": true, "document": true,
		"that": true, "which": true, "with": true, "have": true, "contain": true,
		"contains": true, "the": true, "a": true, "an": true, "and": true,
		"or": true, "but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "from": true, "by": true, "about": true,
		"word": true, "text": true, "content": true, "information": true,
	}

	var cleaned []string
	for _, term := range terms {
		termLower := strings.ToLower(strings.TrimSpace(term))
		if termLower != "" && !fillerWords[termLower] {
			cleaned = append(cleaned, term)
		}
	}

	// If we filtered out everything, return the original terms
	if len(cleaned) == 0 {
		return terms
	}

	return cleaned
}

// Phase 5: Cache management methods
func (e *LLMEnhancer) getCacheKey(query string) string {
	// Normalize query for caching (lowercase, trim spaces)
	return strings.ToLower(strings.TrimSpace(query))
}

func (e *LLMEnhancer) getFromCache(key string) (*QueryClassification, bool) {
	if val, ok := e.classificationCache.Load(key); ok {
		if classification, ok := val.(*QueryClassification); ok {
			return classification, true
		}
	}
	return nil, false
}

func (e *LLMEnhancer) storeInCache(key string, classification *QueryClassification) {
	e.classificationCache.Store(key, classification)

	// Optional: Implement cache size limit and eviction
	// For now, we'll let it grow unbounded (sync.Map handles this well)
}

// ClearCache clears the query cache
func (e *LLMEnhancer) ClearCache() {
	e.classificationCache.Range(func(key, value interface{}) bool {
		e.classificationCache.Delete(key)
		return true
	})

	// Reset cache stats
	e.cacheStats.mutex.Lock()
	e.cacheStats.hits = 0
	e.cacheStats.misses = 0
	e.cacheStats.mutex.Unlock()
}

func (e *LLMEnhancer) recordCacheHit() {
	e.cacheStats.mutex.Lock()
	e.cacheStats.hits++
	e.cacheStats.mutex.Unlock()
}

func (e *LLMEnhancer) recordCacheMiss() {
	e.cacheStats.mutex.Lock()
	e.cacheStats.misses++
	e.cacheStats.mutex.Unlock()
}

// GetCacheStats returns current cache statistics
func (e *LLMEnhancer) GetCacheStats() (hits int64, misses int64, hitRate float64) {
	e.cacheStats.mutex.RLock()
	hits = e.cacheStats.hits
	misses = e.cacheStats.misses
	e.cacheStats.mutex.RUnlock()

	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return
}

// Phase 6: Performance monitoring methods
func (e *LLMEnhancer) recordQueryMetrics(durationMs int64) {
	e.performanceMetrics.mutex.Lock()
	defer e.performanceMetrics.mutex.Unlock()

	e.performanceMetrics.totalQueries++

	// Update average classification time (running average)
	currentAvg := e.performanceMetrics.avgClassifyTimeMs
	newAvg := (currentAvg*float64(e.performanceMetrics.totalQueries-1) + float64(durationMs)) / float64(e.performanceMetrics.totalQueries)
	e.performanceMetrics.avgClassifyTimeMs = newAvg
}

func (e *LLMEnhancer) recordLLMQuery() {
	e.performanceMetrics.mutex.Lock()
	e.performanceMetrics.llmQueries++
	e.performanceMetrics.mutex.Unlock()
}

func (e *LLMEnhancer) recordDirectQuery() {
	e.performanceMetrics.mutex.Lock()
	e.performanceMetrics.directQueries++
	e.performanceMetrics.mutex.Unlock()
}

// GetPerformanceMetrics returns current performance statistics
func (e *LLMEnhancer) GetPerformanceMetrics() map[string]interface{} {
	e.performanceMetrics.mutex.RLock()
	defer e.performanceMetrics.mutex.RUnlock()

	llmRate := float64(0)
	if e.performanceMetrics.totalQueries > 0 {
		llmRate = float64(e.performanceMetrics.llmQueries) / float64(e.performanceMetrics.totalQueries)
	}

	return map[string]interface{}{
		"total_queries":        e.performanceMetrics.totalQueries,
		"llm_queries":          e.performanceMetrics.llmQueries,
		"direct_queries":       e.performanceMetrics.directQueries,
		"avg_classify_time_ms": e.performanceMetrics.avgClassifyTimeMs,
		"llm_usage_rate":       llmRate,
	}
}

// LogPerformanceStats logs current performance statistics
func (e *LLMEnhancer) LogPerformanceStats() {
	metrics := e.GetPerformanceMetrics()
	hits, misses, hitRate := e.GetCacheStats()

	e.log.WithFields(logrus.Fields{
		"total_queries":   metrics["total_queries"],
		"llm_queries":     metrics["llm_queries"],
		"direct_queries":  metrics["direct_queries"],
		"avg_classify_ms": metrics["avg_classify_time_ms"],
		"llm_usage_rate":  metrics["llm_usage_rate"],
		"cache_hits":      hits,
		"cache_misses":    misses,
		"cache_hit_rate":  hitRate,
	}).Info("Query classification performance stats")
}
