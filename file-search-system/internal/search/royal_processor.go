package search

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	
	"context"
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

// RoyalSearchProcessor handles advanced search term extraction using LLM
type RoyalSearchProcessor struct {
	ollamaClient    *OllamaClient
	model           string
	enabled         bool
	corpusMetadata  *CorpusMetadata
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
func NewRoyalSearchProcessor(ollamaURL string) *RoyalSearchProcessor {
	processor := &RoyalSearchProcessor{
		ollamaClient: NewOllamaClient(ollamaURL),
		model:        "qwen3:4b", // Better semantic understanding for search
		enabled:      true,
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
	
	return processor
}

// GenerateSearchTerms generates comprehensive search terms for hybrid search
func (r *RoyalSearchProcessor) GenerateSearchTerms(query string, searchContext string) (*RoyalSearchTerms, error) {
	if !r.enabled {
		return r.fallbackTerms(query), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := r.buildPrompt(query, searchContext)
	
	response, err := r.ollamaClient.Generate(ctx, r.model, prompt)
	if err != nil {
		fmt.Printf("DEBUG: Royal processor Ollama error: %v\n", err)
		// Fallback to basic extraction on error
		return r.fallbackTerms(query), nil
	}
	
	fmt.Printf("DEBUG: Royal processor raw response: %s\n", response)
	fmt.Printf("DEBUG: Royal processor raw response length: %d\n", len(response))

	// Parse the JSON response
	var terms RoyalSearchTerms
	if err := json.Unmarshal([]byte(response), &terms); err != nil {
		// Try to extract what we can from malformed response
		return r.parsePartialResponse(response, query), nil
	}

	// Validate and clean the terms
	terms = r.validateAndClean(terms, query)
	
	return &terms, nil
}

// buildPrompt constructs the comprehensive prompt for search term extraction
func (r *RoyalSearchProcessor) buildPrompt(query, searchContext string) string {
	if searchContext == "" {
		searchContext = "General file search"
	}

	// Join corpus metadata for context
	docTypes := strings.Join(r.corpusMetadata.DocumentTypes, ", ")
	departments := strings.Join(r.corpusMetadata.Departments, ", ")
	categories := strings.Join(r.corpusMetadata.Categories, ", ")
	timeRange := r.corpusMetadata.TimeRange.Start + " to " + r.corpusMetadata.TimeRange.End
	if timeRange == " to " {
		timeRange = "2020-01-01 to 2025-12-31"
	}

	return fmt.Sprintf(`You are an expert search term extraction system with access to document metadata for optimized hybrid search.

TASK: Generate search terms considering both the query and document metadata patterns in the corpus. For emotional or abstract queries, generate related terms, synonyms, and conceptually similar words.

USER QUERY: %s

AVAILABLE METADATA CONTEXT:
- Document Types in Corpus: %s
- Time Range of Documents: %s
- Common Categories: %s
- Departments/Projects: %s
- Total Files: %d

SEARCH CONTEXT: %s

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
    "departments": ["relevant departments if applicable"],
    "document_types": ["report", "email", "memo", etc.],
    "confidence_threshold": 0.7
  },
  "pg_tsquery": "optimized PostgreSQL query",
  "search_strategy": "explanation of approach",
  "metadata_boost_fields": ["fields to prioritize in ranking"]
}

METADATA-INFORMED EXAMPLES:

Query: "Find files that are sad"
{
  "vector_terms": [
    "sad emotional content",
    "depressing melancholy documents",
    "unhappy sorrowful text",
    "tragic loss grief",
    "negative emotional tone"
  ],
  "text_terms": [
    "sad",
    "unhappy",
    "depressed",
    "melancholy",
    "sorrow",
    "grief",
    "loss",
    "tragic",
    "tears",
    "mourning"
  ],
  "metadata_filters": {
    "file_types": ["pdf", "docx", "txt"],
    "categories": ["NarrativeText"],
    "confidence_threshold": 0.6
  },
  "pg_tsquery": "sad | unhappy | depressed | melancholy | sorrow | grief | loss | tragic",
  "search_strategy": "Semantic search for emotional content with negative sentiment",
  "metadata_boost_fields": ["content", "title"]
}

Query: "Q3 financial reports from finance team"
{
  "vector_terms": [
    "third quarter financial reports",
    "Q3 finance department documents", 
    "quarterly financial statements Q3",
    "finance team Q3 reporting",
    "third quarter fiscal documentation"
  ],
  "text_terms": [
    "q3",
    "third", 
    "quarter",
    "financial",
    "finance",
    "report", 
    "fiscal",
    "revenue",
    "expense",
    "budget"
  ],
  "metadata_filters": {
    "file_types": ["pdf", "xlsx", "docx"],
    "date_range": {
      "start": "2024-07-01",
      "end": "2024-09-30"
    },
    "categories": ["Table", "FigureCaption", "NarrativeText"],
    "departments": ["finance", "accounting"],
    "document_types": ["report", "spreadsheet"],
    "confidence_threshold": 0.8
  },
  "pg_tsquery": "(q3 | third & quarter) & (financial | finance) & report",
  "search_strategy": "Focus on Q3 time period with finance department filtering",
  "metadata_boost_fields": ["created_date", "department", "document_type"]
}

IMPORTANT:
- Return ONLY the JSON object, no additional text
- Ensure all JSON is properly formatted and valid
- Generate 5 vector_terms and 8-10 text_terms
- The pg_tsquery must be valid PostgreSQL syntax
- Adapt term complexity based on query sophistication

JSON OUTPUT:`, query, docTypes, timeRange, categories, departments, r.corpusMetadata.TotalFiles, searchContext)
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
	
	// Validate pg_tsquery - ensure it's not empty
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