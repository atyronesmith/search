package search

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// LLMEnhancer provides intelligent query enhancement using LLM
type LLMEnhancer struct {
	ollamaClient    *OllamaClient
	royalProcessor  *RoyalSearchProcessor
	enabled         bool
}

// NewLLMEnhancer creates a new LLM enhancer
func NewLLMEnhancer(ollamaURL string) *LLMEnhancer {
	enhancer := &LLMEnhancer{
		ollamaClient:   NewOllamaClient(ollamaURL),
		royalProcessor: NewRoyalSearchProcessor(ollamaURL),
		enabled:        true,
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

// QueryClassification represents the classification of a query
type QueryClassification struct {
	NeedsLLM     bool     `json:"needs_llm"`
	QueryType    string   `json:"query_type"`    // "simple", "complex", "analytical", "temporal"
	Intent       string   `json:"intent"`        // "search", "count", "analysis", "filter"
	Confidence   float64  `json:"confidence"`
	ComplexTerms []string `json:"complex_terms"` // Terms that suggest complex search
	Reasoning    string   `json:"reasoning"`
}

// EnhancedQuery represents an LLM-enhanced query
type EnhancedQuery struct {
	Original        string            `json:"original"`
	Enhanced        string            `json:"enhanced"`
	SearchTerms     []string          `json:"search_terms"`
	VectorTerms     []string          `json:"vector_terms"`      // For vector similarity search
	PgTsQuery       string            `json:"pg_tsquery"`         // PostgreSQL full-text search query
	ContentFilters  []ContentFilter   `json:"content_filters"`
	MetadataFilters []MetadataFilter  `json:"metadata_filters"`
	Intent          string            `json:"intent"`
	RequiresCount   bool              `json:"requires_count"`
	SearchStrategy  string            `json:"search_strategy"`    // Description of search approach
}

// ContentFilter represents semantic content filtering
type ContentFilter struct {
	Type        string  `json:"type"`        // "contains", "pattern", "semantic"
	Description string  `json:"description"` // Human description
	Pattern     string  `json:"pattern"`     // Regex pattern if applicable
	Keywords    []string `json:"keywords"`   // Keywords for semantic search
	Confidence  float64 `json:"confidence"`
}

// MetadataFilter represents file metadata filtering
type MetadataFilter struct {
	Field       string      `json:"field"`         // "type", "size", "created_date", "modified_date", "name"
	Operator    string      `json:"operator"`      // "equals", "contains", "greater", "less", "between"
	Value       interface{} `json:"value"`
	StartDate   *time.Time  `json:"start_date,omitempty"`
	EndDate     *time.Time  `json:"end_date,omitempty"`
	DateField   string      `json:"date_field,omitempty"` // "created_at", "modified_at" - specifies which date column to filter on
}

// ClassifyQuery determines if a query needs LLM enhancement
func (e *LLMEnhancer) ClassifyQuery(query string) (*QueryClassification, error) {
	if !e.enabled {
		return &QueryClassification{
			NeedsLLM:   false,
			QueryType:  "simple",
			Intent:     "search",
			Confidence: 1.0,
			Reasoning:  "LLM enhancement disabled",
		}, nil
	}

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
		enhanced, err := e.llmClassify(query)
		if err != nil {
			// Fall back to quick classification if LLM fails
			return classification, nil
		}
		return enhanced, nil
	}

	return classification, nil
}

// quickClassify performs fast pattern-based classification
func (e *LLMEnhancer) quickClassify(query string) *QueryClassification {
	query = strings.ToLower(strings.TrimSpace(query))
	
	// Simple keyword searches - no LLM needed
	simplePatterns := []string{
		`^[a-zA-Z0-9\s\-_\.]+$`, // Just alphanumeric and basic chars
		`^"[^"]*"$`,              // Simple quoted phrase
		`^\w+:\w+$`,              // Simple filter like type:pdf
	}
	
	for _, pattern := range simplePatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched && !e.hasComplexTerms(query) {
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
	complexTerms := e.findComplexTerms(query)
	
	// Analytical queries
	analyticalPatterns := []string{
		`\b(find|show|list|count|how many)\b.*\b(that|with|containing|have)\b`,
		`\b(all files|documents|files)\b.*\b(contain|have|with)\b`,
		`\b(social security|ssn|credit card|financial|legal|medical)\b`,
		`\b(table|chart|graph|figure|image)\b.*\b(with|containing|about)\b`,
		`\b(correspondence|email|letter|communication)\b.*\b(with|from|to)\b`,
		`\b(last week|tuesday|yesterday|recent|modified on)\b`,
		`\b(look like|similar to|type of|kind of)\b`,
	}

	for _, pattern := range analyticalPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			return &QueryClassification{
				NeedsLLM:     true,
				QueryType:    "analytical",
				Intent:       e.detectIntent(query),
				Confidence:   0.8,
				ComplexTerms: complexTerms,
				Reasoning:    "Complex analytical query detected",
			}
		}
	}

	// Natural language questions
	questionWords := []string{"what", "where", "when", "why", "how", "who", "which"}
	for _, word := range questionWords {
		if strings.HasPrefix(query, word+" ") || strings.Contains(query, "?") {
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
			Intent:       e.detectIntent(query),
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
		NeedsLLM:   true,
		QueryType:  "analytical",
		Intent:     e.detectIntent(query),
		Confidence: 0.8,
		ComplexTerms: e.findComplexTerms(query),
		Reasoning:  "Simplified classification to avoid JSON parsing issues",
	}, nil
}

// EnhanceQuery transforms a complex query into structured search parameters
func (e *LLMEnhancer) EnhanceQuery(query string, classification *QueryClassification) (*EnhancedQuery, error) {
	if !classification.NeedsLLM {
		// For simple queries, just return basic enhancement
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
	royalTerms, err := e.royalProcessor.GenerateSearchTerms(query, searchContext)
	if err != nil {
		fmt.Printf("DEBUG: Royal processor failed: %v, falling back to legacy\n", err)
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

	response, err := e.ollamaClient.Generate(ctx, "phi3:mini", prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM enhancement failed: %w", err)
	}

	// Parse the JSON response
	cleanResponse := strings.TrimSpace(response)
	
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

	// File type detection
	fileTypePatterns := map[string][]string{
		"pdf":   {"pdf", "pdfs", "pdf file", "pdf document"},
		"docx":  {"docx", "word", "word document", "microsoft word"},
		"xlsx":  {"xlsx", "excel", "spreadsheet", "excel file"},
		"txt":   {"txt", "text file", "plain text"},
		"md":    {"markdown", "md file", ".md"},
		"py":    {"python", "py file", ".py", "python script"},
		"go":    {"golang", "go file", ".go", "go code"},
		"js":    {"javascript", "js file", ".js", "javascript code"},
		"json":  {"json", "json file", ".json"},
		"yaml":  {"yaml", "yml", ".yaml", ".yml"},
		"csv":   {"csv", "csv file", "comma separated"},
	}

	for fileType, patterns := range fileTypePatterns {
		for _, pattern := range patterns {
			if strings.Contains(queryLower, pattern) {
				filters = append(filters, MetadataFilter{
					Field:    "type",
					Operator: "equals",
					Value:    fileType,
				})
				break
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
		{"large file", "size", "greater", 10 * 1024 * 1024},     // > 10MB
		{"small file", "size", "less", 1 * 1024 * 1024},         // < 1MB
		{"huge file", "size", "greater", 100 * 1024 * 1024},     // > 100MB
		{"tiny file", "size", "less", 100 * 1024},               // < 100KB
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