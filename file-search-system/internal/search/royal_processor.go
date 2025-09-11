package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	
	"github.com/file-search/file-search-system/internal/database"
)

// RoyalSearchTerms represents the comprehensive search term structure
type RoyalSearchTerms struct {
	VectorTerms        []string                   `json:"vector_terms"`
	TextTerms          []string                   `json:"text_terms"`
	PgTsQuery          string                     `json:"pg_tsquery"`
	SearchStrategy     string                     `json:"search_strategy"`
	MetadataFilters    map[string]interface{}     `json:"metadata_filters,omitempty"`
	MetadataBoostFields []string                   `json:"metadata_boost_fields,omitempty"`
}

// ParseErrorWithResponse contains both the parsing error and the raw LLM response
type ParseErrorWithResponse struct {
	Err         error
	RawResponse string
}

// Error implements the error interface
func (e *ParseErrorWithResponse) Error() string {
	return e.Err.Error()
}

// RoyalSearchProcessor handles advanced search term extraction using LLM
type RoyalSearchProcessor struct {
	ollamaClient    *OllamaClient
	model           string
	enabled         bool
	corpusMetadata  *CorpusMetadata
	db              *database.DB
	promptTemplate  string
}

// CorpusMetadata represents analyzed metadata patterns from the document corpus
type CorpusMetadata struct {
	DocumentTypes   []string  `json:"document_types"`
	Departments     []string  `json:"departments"`
	Projects        []string  `json:"projects"`
	Languages       []string  `json:"languages"`
	TimeRange       struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"time_range"`
	Categories      []string  `json:"categories"`
	TotalFiles      int       `json:"total_files"`
	LastAnalyzed    string    `json:"last_analyzed"`
}

// NewRoyalSearchProcessor creates a new royal search processor
func NewRoyalSearchProcessor(ollamaURL string, modelName string, db *database.DB) *RoyalSearchProcessor {
	processor := &RoyalSearchProcessor{
		ollamaClient: NewOllamaClient(ollamaURL),
		model:        modelName,
		enabled:      true,
		db:           db,
	}
	
	// Initialize with default corpus metadata
	processor.corpusMetadata = &CorpusMetadata{
		DocumentTypes: []string{"pdf", "docx", "xlsx", "csv", "txt"},
		Departments:   []string{"engineering", "finance", "hr", "legal", "marketing"},
		Projects:      []string{},
		Languages:     []string{"en"},
		Categories:    []string{"NarrativeText", "Title", "Table", "ListItem"},
		TotalFiles:    0,
		LastAnalyzed:  "",
	}
	
	// Load prompt template from database
	processor.loadPromptTemplate()
	
	return processor
}

// loadPromptTemplate loads the LLM prompt template from the database
func (r *RoyalSearchProcessor) loadPromptTemplate() {
	if r.db == nil {
		r.promptTemplate = r.getDefaultPromptTemplate()
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	query := `SELECT config_value FROM system_config WHERE config_key = 'llm_prompt_template'`
	err := r.db.QueryRow(ctx, query).Scan(&r.promptTemplate)
	if err != nil {
		fmt.Printf("DEBUG: Failed to load prompt template from database: %v, using default\n", err)
		r.promptTemplate = r.getDefaultPromptTemplate()
		return
	}
	
	fmt.Printf("DEBUG: Loaded prompt template from database (length: %d)\n", len(r.promptTemplate))
}

// getDefaultPromptTemplate returns the default hardcoded prompt template as fallback
func (r *RoyalSearchProcessor) getDefaultPromptTemplate() string {
	return `You are an expert search term extraction system with access to document metadata for optimized hybrid search.

TASK: Generate search terms considering both the query and document metadata patterns in the corpus. For emotional or abstract queries, generate related terms, synonyms, and conceptually similar words.

USER QUERY: {{USER_QUERY}}

AVAILABLE METADATA CONTEXT:
- Document Types in Corpus: {{DOC_TYPES}}
- Time Range of Documents: {{TIME_RANGE}}
- Common Categories: {{CATEGORIES}}
- Departments/Projects: {{DEPARTMENTS}}
- Total Files: {{TOTAL_FILES}}

SEARCH CONTEXT: Query type: analytical, Intent: search

METADATA-AWARE EXTRACTION RULES:
1. If query implies time-sensitivity, emphasize temporal search terms
2. If query mentions document types (report, email, memo), include type-specific terms  
3. For departmental queries, include relevant organizational terms
4. Consider document hierarchy (section headers, chapters) for navigation queries
5. Weight terms based on metadata frequency in corpus

ENHANCED OUTPUT STRUCTURE:
{
  "vector_terms": [
    "semantic phrases matching query intent",
    "additional 3-5 contextual phrases"
  ],
  "text_terms": [
    "keywords from query", 
    "additional 6-8 relevant terms"
  ],
  "metadata_filters": {
    "file_types": ["relevant file extensions"],
    "date_range": {
      "start": "ISO date or null",
      "end": "ISO date or null"
    },
    "categories": ["relevant Unstructured categories"],
    "departments": [],
    "document_types": ["report", "email", "memo", "etc."],
    "confidence_threshold": 0.7
  },
  "pg_tsquery": "simple PostgreSQL tsquery using only &, |, ! operators with single words",
  "search_strategy": "explanation of approach",
  "metadata_boost_fields": ["fields to prioritize in ranking"]
}

IMPORTANT:
- Return ONLY the JSON object, no additional text or comments
- Ensure all JSON is properly formatted and valid - NO COMMENTS IN JSON
- Generate 5 vector_terms and 8-10 text_terms
- The pg_tsquery MUST use ONLY simple PostgreSQL tsquery operators: & (AND), | (OR), ! (NOT)
- DO NOT use date ranges, complex syntax, or invalid operators in pg_tsquery
- Use only single words or simple phrases in pg_tsquery
- EXAMPLE VALID: "taxi | cab", "taxi & transport", "!spam"
- EXAMPLE INVALID: "AND (2020-01-01 TO 2025-12-31)", "&:all", "-&-", complex phrases
- Adapt term complexity based on query sophistication
- JSON must be parseable - do not include any explanatory text or comments within the JSON structure
- NEVER use // or /* */ comments in the JSON output
- For empty arrays, use [] without any comments or explanations


formatted syntactically correct JSON OUTPUT:`
}

// GenerateSearchTerms generates comprehensive search terms for hybrid search
func (r *RoyalSearchProcessor) GenerateSearchTerms(query string, searchContext string) (*RoyalSearchTerms, error) {
	return r.GenerateSearchTermsWithDebug(query, searchContext, nil)
}

// GenerateSearchTermsWithDebug generates search terms and captures debug information
func (r *RoyalSearchProcessor) GenerateSearchTermsWithDebug(query string, searchContext string, debugCallback func(string, string, string, int64)) (*RoyalSearchTerms, error) {
	if !r.enabled {
		return r.fallbackTerms(query), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	prompt := r.buildPrompt(query, searchContext)
	
	fmt.Printf("DEBUG: Calling LLM with model=%s, query='%s'\n", r.model, query)
	response, err := r.ollamaClient.Generate(ctx, r.model, prompt)
	processTime := time.Since(startTime).Milliseconds()
	fmt.Printf("DEBUG: LLM call completed in %dms, err=%v\n", processTime, err)
	
	// Call debug callback if provided
	if debugCallback != nil {
		errorMsg := ""
		if err != nil {
			errorMsg = err.Error()
		}
		debugCallback(prompt, response, errorMsg, processTime)
	}
	
	if err != nil {
		fmt.Printf("DEBUG: Royal processor Ollama error: %v\n", err)
		// Fallback to basic extraction on error
		return r.fallbackTerms(query), nil
	}
	
	fmt.Printf("DEBUG: Royal processor raw response: %s\n", response)
	fmt.Printf("DEBUG: Royal processor raw response length: %d\n", len(response))

	// Clean the response - remove markdown code blocks if present
	cleanedResponse := strings.TrimSpace(response)
	if strings.HasPrefix(cleanedResponse, "```json") && strings.HasSuffix(cleanedResponse, "```") {
		// Remove ```json from the beginning and ``` from the end
		cleanedResponse = strings.TrimPrefix(cleanedResponse, "```json")
		cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
		cleanedResponse = strings.TrimSpace(cleanedResponse)
	} else if strings.HasPrefix(cleanedResponse, "```") && strings.HasSuffix(cleanedResponse, "```") {
		// Remove ``` from both ends
		cleanedResponse = strings.TrimPrefix(cleanedResponse, "```")
		cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
		cleanedResponse = strings.TrimSpace(cleanedResponse)
	}

	// Remove JSON comments (// ... until end of line)
	cleanedResponse = RemoveJSONComments(cleanedResponse)

	// Parse the JSON response
	var terms RoyalSearchTerms
	if err := json.Unmarshal([]byte(cleanedResponse), &terms); err != nil {
		// Return error with raw response so it can be reused
		return nil, &ParseErrorWithResponse{
			Err:         fmt.Errorf("JSON parsing failed: %w", err),
			RawResponse: response,
		}
	}

	// Validate and clean the terms
	terms = r.validateAndClean(terms, query)
	
	return &terms, nil
}

// buildPrompt constructs the comprehensive prompt for search term extraction using template substitution
func (r *RoyalSearchProcessor) buildPrompt(query, searchContext string) string {
	if searchContext == "" {
		searchContext = "General file search"
	}

	// Prepare template variables
	docTypes := strings.Join(r.corpusMetadata.DocumentTypes, ", ")
	departments := strings.Join(r.corpusMetadata.Departments, ", ")
	categories := strings.Join(r.corpusMetadata.Categories, ", ")
	timeRange := r.corpusMetadata.TimeRange.Start + " to " + r.corpusMetadata.TimeRange.End
	if timeRange == " to " {
		timeRange = "2020-01-01 to 2025-12-31"
	}
	totalFiles := fmt.Sprintf("%d", r.corpusMetadata.TotalFiles)

	// Use the loaded prompt template with variable substitution
	prompt := r.promptTemplate
	if prompt == "" {
		prompt = r.getDefaultPromptTemplate()
	}
	
	// Perform template variable substitution
	prompt = strings.ReplaceAll(prompt, "{{USER_QUERY}}", query)
	prompt = strings.ReplaceAll(prompt, "{{DOC_TYPES}}", docTypes)
	prompt = strings.ReplaceAll(prompt, "{{TIME_RANGE}}", timeRange)
	prompt = strings.ReplaceAll(prompt, "{{CATEGORIES}}", categories)
	prompt = strings.ReplaceAll(prompt, "{{DEPARTMENTS}}", departments)
	prompt = strings.ReplaceAll(prompt, "{{TOTAL_FILES}}", totalFiles)
	
	return prompt
}

// fallbackTerms generates basic search terms when LLM is unavailable
func (r *RoyalSearchProcessor) fallbackTerms(query string) *RoyalSearchTerms {
	// Clean and tokenize the query
	query = strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(query)
	
	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"about": true, "as": true, "into": true, "through": true,
		"find": true, "show": true, "list": true, "get": true, "all": true,
		"that": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "must": true, "can": true,
		"is": true, "are": true, "was": true, "were": true, "been": true,
	}
	
	var textTerms []string
	for _, word := range words {
		if !stopWords[word] && len(word) > 2 {
			textTerms = append(textTerms, word)
		}
	}
	
	// Generate simple tsquery
	tsquery := strings.Join(textTerms, " & ")
	if tsquery == "" {
		tsquery = query
	}
	
	return &RoyalSearchTerms{
		VectorTerms:         []string{query},
		TextTerms:           textTerms,
		PgTsQuery:           tsquery,
		SearchStrategy:      "Fallback: basic keyword extraction",
		MetadataFilters:     make(map[string]interface{}),
		MetadataBoostFields: []string{},
	}
}

// parsePartialResponse attempts to extract useful terms from malformed JSON
func (r *RoyalSearchProcessor) parsePartialResponse(response, query string) *RoyalSearchTerms {
	terms := r.fallbackTerms(query)
	
	// Try to extract vector_terms
	if idx := strings.Index(response, `"vector_terms"`); idx >= 0 {
		if endIdx := strings.Index(response[idx:], "]"); endIdx > 0 {
			substr := response[idx : idx+endIdx+1]
			// Try to extract terms between quotes
			var extracted []string
			parts := strings.Split(substr, `"`)
			for i, part := range parts {
				if i%2 == 1 && len(part) > 2 && !strings.Contains(part, ":") && !strings.Contains(part, "[") {
					extracted = append(extracted, part)
				}
			}
			if len(extracted) > 0 {
				terms.VectorTerms = extracted
			}
		}
	}
	
	// Try to extract text_terms
	if idx := strings.Index(response, `"text_terms"`); idx >= 0 {
		if endIdx := strings.Index(response[idx:], "]"); endIdx > 0 {
			substr := response[idx : idx+endIdx+1]
			var extracted []string
			parts := strings.Split(substr, `"`)
			for i, part := range parts {
				if i%2 == 1 && len(part) > 1 && !strings.Contains(part, ":") && !strings.Contains(part, "[") {
					extracted = append(extracted, strings.ToLower(part))
				}
			}
			if len(extracted) > 0 {
				terms.TextTerms = extracted
			}
		}
	}
	
	// Try to extract pg_tsquery
	if idx := strings.Index(response, `"pg_tsquery"`); idx >= 0 {
		if startIdx := strings.Index(response[idx:], `"`); startIdx > 0 {
			queryStart := idx + startIdx + 1
			if endIdx := strings.Index(response[queryStart:], `"`); endIdx > 0 {
				terms.PgTsQuery = response[queryStart : queryStart+endIdx]
			}
		}
	}
	
	terms.SearchStrategy = "Partial extraction from malformed response"
	return terms
}

// validateAndClean ensures the extracted terms are valid and useful
func (r *RoyalSearchProcessor) validateAndClean(terms RoyalSearchTerms, originalQuery string) RoyalSearchTerms {
	// Ensure we have at least some terms
	if len(terms.VectorTerms) == 0 {
		terms.VectorTerms = []string{originalQuery}
	}
	
	if len(terms.TextTerms) == 0 {
		fallback := r.fallbackTerms(originalQuery)
		terms.TextTerms = fallback.TextTerms
	}
	
	// Clean vector terms
	var cleanedVector []string
	for _, term := range terms.VectorTerms {
		term = strings.TrimSpace(term)
		if term != "" && len(term) > 2 && len(term) < 200 {
			cleanedVector = append(cleanedVector, term)
		}
	}
	terms.VectorTerms = cleanedVector
	
	// Clean text terms - ensure they're lowercase and valid
	var cleanedText []string
	seen := make(map[string]bool)
	for _, term := range terms.TextTerms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && len(term) > 1 && len(term) < 50 && !seen[term] {
			cleanedText = append(cleanedText, term)
			seen[term] = true
		}
	}
	terms.TextTerms = cleanedText
	
	// Validate pg_tsquery - ensure it's not empty and clean it
	if terms.PgTsQuery == "" || len(terms.PgTsQuery) < 3 {
		// Generate simple query from text terms
		if len(terms.TextTerms) > 0 {
			// Take first 3-5 terms for the query
			limit := len(terms.TextTerms)
			if limit > 5 {
				limit = 5
			}
			terms.PgTsQuery = strings.Join(terms.TextTerms[:limit], " & ")
		} else {
			terms.PgTsQuery = strings.ToLower(originalQuery)
		}
	} else {
		// Clean the tsquery - remove query meta words that don't appear in content
		queryMetaWords := []string{"files", "contain", "word", "find", "all", "show", "list", "get", "that", "the", "a", "an", "with"}
		cleanedQuery := terms.PgTsQuery
		
		// Remove meta words from the query
		for _, metaWord := range queryMetaWords {
			cleanedQuery = strings.ReplaceAll(cleanedQuery, metaWord+" & ", "")
			cleanedQuery = strings.ReplaceAll(cleanedQuery, " & "+metaWord, "")
			cleanedQuery = strings.ReplaceAll(cleanedQuery, metaWord+" | ", "")
			cleanedQuery = strings.ReplaceAll(cleanedQuery, " | "+metaWord, "")
			if cleanedQuery == metaWord {
				cleanedQuery = ""
			}
		}
		
		// Clean up remaining operators
		cleanedQuery = strings.TrimSpace(cleanedQuery)
		cleanedQuery = strings.Trim(cleanedQuery, "&|")
		cleanedQuery = strings.TrimSpace(cleanedQuery)
		
		// If we removed everything, fall back to extracting key terms from original query
		if cleanedQuery == "" {
			// Extract actual content words from original query
			words := strings.Fields(strings.ToLower(originalQuery))
			var contentWords []string
			stopWords := map[string]bool{
				"find": true, "all": true, "files": true, "that": true, "contain": true, "the": true, "word": true, "show": true, "list": true, "get": true,
			}
			for _, word := range words {
				if !stopWords[word] && len(word) > 2 {
					contentWords = append(contentWords, word)
				}
			}
			if len(contentWords) > 0 {
				cleanedQuery = strings.Join(contentWords, " | ")
			}
		}
		
		if cleanedQuery != "" {
			terms.PgTsQuery = cleanedQuery
		}
	}
	
	// Ensure we have a search strategy
	if terms.SearchStrategy == "" {
		terms.SearchStrategy = "Hybrid search with vector and text matching"
	}
	
	// Limit vector terms to 5 and text terms to 10
	if len(terms.VectorTerms) > 5 {
		terms.VectorTerms = terms.VectorTerms[:5]
	}
	if len(terms.TextTerms) > 10 {
		terms.TextTerms = terms.TextTerms[:10]
	}
	
	return terms
}

// BatchProcess processes multiple queries in batch
func (r *RoyalSearchProcessor) BatchProcess(queries []string, searchContext string) ([]*RoyalSearchTerms, error) {
	results := make([]*RoyalSearchTerms, len(queries))
	
	for i, query := range queries {
		terms, err := r.GenerateSearchTerms(query, searchContext)
		if err != nil {
			// Use fallback for failed queries
			results[i] = r.fallbackTerms(query)
		} else {
			results[i] = terms
		}
	}
	
	return results, nil
}

// SetEnabled enables or disables the royal processor
func (r *RoyalSearchProcessor) SetEnabled(enabled bool) {
	r.enabled = enabled
}

// IsEnabled returns whether the processor is enabled
func (r *RoyalSearchProcessor) IsEnabled() bool {
	return r.enabled
}

// SetModel changes the LLM model used for processing
func (r *RoyalSearchProcessor) SetModel(model string) {
	r.model = model
}

// ReloadPromptTemplate reloads the prompt template from the database
func (r *RoyalSearchProcessor) ReloadPromptTemplate() {
	r.loadPromptTemplate()
}

// GetPromptTemplate returns the current prompt template
func (r *RoyalSearchProcessor) GetPromptTemplate() string {
	return r.promptTemplate
}

// RemoveJSONComments removes single-line comments (// ...) from JSON strings
// It's exported so it can be used from other files in the package
func RemoveJSONComments(jsonStr string) string {
	var result strings.Builder
	lines := strings.Split(jsonStr, "\n")
	
	for _, line := range lines {
		processedLine := ""
		inString := false
		escaped := false
		
		// Process character by character to track if we're in a string
		for i := 0; i < len(line); i++ {
			ch := line[i]
			
			// Handle escape sequences
			if escaped {
				processedLine += string(ch)
				escaped = false
				continue
			}
			
			// Check for escape character
			if ch == '\\' && inString {
				processedLine += string(ch)
				escaped = true
				continue
			}
			
			// Toggle string state on unescaped quotes
			if ch == '"' {
				processedLine += string(ch)
				inString = !inString
				continue
			}
			
			// Check for comment start when not in a string
			if !inString && i+1 < len(line) && ch == '/' && line[i+1] == '/' {
				// Found a comment outside of a string, stop processing this line
				break
			}
			
			// Add the character to the processed line
			processedLine += string(ch)
		}
		
		// Add the processed line to the result
		if processedLine != "" || line == "" {
			result.WriteString(processedLine)
			result.WriteString("\n")
		}
	}
	
	return strings.TrimSpace(result.String())
}