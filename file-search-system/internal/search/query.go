package search

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// QueryProcessor handles query parsing and optimization
type QueryProcessor struct {
	stopWords  map[string]bool
	synonyms   map[string][]string
	maxTerms   int
	minTermLen int
}

// NewQueryProcessor creates a new query processor
func NewQueryProcessor() *QueryProcessor {
	return &QueryProcessor{
		stopWords:  defaultStopWords(),
		synonyms:   defaultSynonyms(),
		maxTerms:   10,
		minTermLen: 2,
	}
}

// ProcessedQuery represents a processed search query
type ProcessedQuery struct {
	Original      string         `json:"original"`
	Cleaned       string         `json:"cleaned"`
	Terms         []string       `json:"terms"`
	Phrases       []string       `json:"phrases"`
	MustInclude   []string       `json:"must_include"`
	MustExclude   []string       `json:"must_exclude"`
	FileTypes     []string       `json:"file_types"`
	DateRange     *DateRange     `json:"date_range,omitempty"`
	SizeRange     *SizeRange     `json:"size_range,omitempty"`
	IsQuestion    bool           `json:"is_question"`
	QueryType     string         `json:"query_type"`               // "keyword", "natural", "code", "path"
	EnhancedQuery *EnhancedQuery `json:"enhanced_query,omitempty"` // LLM-enhanced query with royal processor
}

// DateRange represents a date range filter
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// SizeRange represents a file size range filter
type SizeRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// ProcessQuery parses and optimizes a search query
func (qp *QueryProcessor) ProcessQuery(query string) (*ProcessedQuery, error) {
	pq := &ProcessedQuery{
		Original: query,
	}

	// Detect query type
	pq.QueryType = qp.detectQueryType(query)

	// Extract special operators and filters
	query = qp.extractFilters(query, pq)

	// Extract phrases (quoted strings)
	query, phrases := qp.extractPhrases(query)
	pq.Phrases = phrases

	// Extract must include/exclude terms
	query = qp.extractMustTerms(query, pq)

	// Clean and tokenize
	pq.Cleaned = qp.cleanQuery(query)
	pq.Terms = qp.tokenize(pq.Cleaned)

	// Remove stop words (except for code queries)
	if pq.QueryType != "code" {
		pq.Terms = qp.removeStopWords(pq.Terms)
	}

	// Expand with synonyms
	pq.Terms = qp.expandSynonyms(pq.Terms)

	// Detect if it's a question
	pq.IsQuestion = qp.isQuestion(pq.Original)

	// Limit number of terms
	if len(pq.Terms) > qp.maxTerms {
		pq.Terms = pq.Terms[:qp.maxTerms]
	}

	return pq, nil
}

// detectQueryType determines the type of query
func (qp *QueryProcessor) detectQueryType(query string) string {
	query = strings.ToLower(query)

	// Check for code patterns
	codePatterns := []string{
		"func ", "function ", "class ", "def ", "import ",
		"var ", "const ", "let ", "return ", "if ", "for ",
		"{", "}", "()", "[]", "=>", "::", "->",
	}
	for _, pattern := range codePatterns {
		if strings.Contains(query, pattern) {
			return "code"
		}
	}

	// Check for path patterns
	if strings.Contains(query, "/") || strings.Contains(query, "\\") {
		return "path"
	}

	// Check for natural language patterns
	if qp.isQuestion(query) || len(strings.Fields(query)) > 5 {
		return "natural"
	}

	return "keyword"
}

// extractFilters extracts special filters from the query
func (qp *QueryProcessor) extractFilters(query string, pq *ProcessedQuery) string {
	// Extract file type filters (e.g., "type:pdf", "type: pdf", "filetype:doc")
	typeRegex := regexp.MustCompile(`(?i)\b(?:type|filetype|ext|extension):\s*(\S+)`)
	matches := typeRegex.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		if len(match) > 1 {
			pq.FileTypes = append(pq.FileTypes, strings.ToLower(match[1]))
		}
	}
	query = typeRegex.ReplaceAllString(query, "")

	// Extract date filters (e.g., "after:2024-01-01", "after: 2024-01-01", "before:2024-12-31")
	afterRegex := regexp.MustCompile(`(?i)\bafter:\s*(\S+)`)
	beforeRegex := regexp.MustCompile(`(?i)\bbefore:\s*(\S+)`)

	afterMatches := afterRegex.FindStringSubmatch(query)
	if len(afterMatches) > 1 {
		if t, err := time.Parse("2006-01-02", afterMatches[1]); err == nil {
			if pq.DateRange == nil {
				pq.DateRange = &DateRange{}
			}
			pq.DateRange.From = t
		}
	}
	query = afterRegex.ReplaceAllString(query, "")

	beforeMatches := beforeRegex.FindStringSubmatch(query)
	if len(beforeMatches) > 1 {
		if t, err := time.Parse("2006-01-02", beforeMatches[1]); err == nil {
			if pq.DateRange == nil {
				pq.DateRange = &DateRange{}
			}
			pq.DateRange.To = t
		}
	}
	query = beforeRegex.ReplaceAllString(query, "")

	// Extract size filters (e.g., "size:>10MB", "size: <1GB")
	sizeRegex := regexp.MustCompile(`(?i)\bsize:\s*([<>])(\d+)(KB|MB|GB)?`)
	sizeMatches := sizeRegex.FindStringSubmatch(query)
	if len(sizeMatches) > 2 {
		size := qp.parseSize(sizeMatches[2], sizeMatches[3])
		if pq.SizeRange == nil {
			pq.SizeRange = &SizeRange{}
		}
		if sizeMatches[1] == ">" {
			pq.SizeRange.Min = size
		} else {
			pq.SizeRange.Max = size
		}
	}
	query = sizeRegex.ReplaceAllString(query, "")

	return strings.TrimSpace(query)
}

// extractPhrases extracts quoted phrases from the query
func (qp *QueryProcessor) extractPhrases(query string) (string, []string) {
	var phrases []string
	phraseRegex := regexp.MustCompile(`"([^"]+)"`)

	matches := phraseRegex.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		if len(match) > 1 {
			phrases = append(phrases, match[1])
		}
	}

	// Remove phrases from query
	query = phraseRegex.ReplaceAllString(query, "")

	return query, phrases
}

// extractMustTerms extracts required (+) and excluded (-) terms
func (qp *QueryProcessor) extractMustTerms(query string, pq *ProcessedQuery) string {
	// Extract must include terms (+ prefix)
	includeRegex := regexp.MustCompile(`\+(\S+)`)
	includeMatches := includeRegex.FindAllStringSubmatch(query, -1)
	for _, match := range includeMatches {
		if len(match) > 1 {
			pq.MustInclude = append(pq.MustInclude, match[1])
		}
	}
	query = includeRegex.ReplaceAllString(query, "")

	// Extract must exclude terms (- prefix)
	excludeRegex := regexp.MustCompile(`-(\S+)`)
	excludeMatches := excludeRegex.FindAllStringSubmatch(query, -1)
	for _, match := range excludeMatches {
		if len(match) > 1 {
			pq.MustExclude = append(pq.MustExclude, match[1])
		}
	}
	query = excludeRegex.ReplaceAllString(query, "")

	return query
}

// cleanQuery cleans the query string
func (qp *QueryProcessor) cleanQuery(query string) string {
	// Convert to lowercase
	query = strings.ToLower(query)

	// Remove extra whitespace
	query = strings.Join(strings.Fields(query), " ")

	// Remove special characters (keep alphanumeric, spaces, and some punctuation)
	reg := regexp.MustCompile(`[^a-z0-9\s\-_\.]`)
	query = reg.ReplaceAllString(query, " ")

	// Remove extra spaces again
	query = strings.Join(strings.Fields(query), " ")

	return query
}

// tokenize splits the query into terms
func (qp *QueryProcessor) tokenize(query string) []string {
	var terms []string

	// Split by whitespace
	words := strings.Fields(query)

	for _, word := range words {
		// Skip if too short
		if len(word) < qp.minTermLen {
			continue
		}

		// Remove trailing punctuation
		word = strings.TrimFunc(word, func(r rune) bool {
			return unicode.IsPunct(r)
		})

		if word != "" {
			terms = append(terms, word)
		}
	}

	return terms
}

// removeStopWords removes common stop words
func (qp *QueryProcessor) removeStopWords(terms []string) []string {
	var filtered []string

	for _, term := range terms {
		if !qp.stopWords[term] {
			filtered = append(filtered, term)
		}
	}

	return filtered
}

// expandSynonyms expands terms with synonyms
func (qp *QueryProcessor) expandSynonyms(terms []string) []string {
	expanded := make([]string, 0, len(terms)*2)
	seen := make(map[string]bool)

	for _, term := range terms {
		if !seen[term] {
			expanded = append(expanded, term)
			seen[term] = true
		}

		// Add synonyms
		if synonyms, ok := qp.synonyms[term]; ok {
			for _, syn := range synonyms {
				if !seen[syn] {
					expanded = append(expanded, syn)
					seen[syn] = true
				}
			}
		}
	}

	return expanded
}

// isQuestion detects if the query is a question
func (qp *QueryProcessor) isQuestion(query string) bool {
	query = strings.ToLower(query)
	questionWords := []string{"what", "where", "when", "why", "how", "who", "which", "whose", "whom"}

	for _, word := range questionWords {
		if strings.HasPrefix(query, word+" ") {
			return true
		}
	}

	return strings.Contains(query, "?")
}

// parseSize parses size string to bytes
func (qp *QueryProcessor) parseSize(value, unit string) int64 {
	var size int64
	fmt.Sscanf(value, "%d", &size)

	switch strings.ToUpper(unit) {
	case "KB":
		size *= 1024
	case "MB":
		size *= 1024 * 1024
	case "GB":
		size *= 1024 * 1024 * 1024
	default:
		// Assume bytes if no unit
	}

	return size
}

// defaultStopWords returns a set of common stop words
func defaultStopWords() map[string]bool {
	words := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for", "from",
		"has", "he", "in", "is", "it", "its", "of", "on", "that", "the",
		"to", "was", "will", "with", "the", "this", "these", "those",
		"there", "their", "they", "them", "then", "than", "can", "could",
		"would", "should", "shall", "may", "might", "must", "will", "would",
	}

	stopWords := make(map[string]bool)
	for _, word := range words {
		stopWords[word] = true
	}

	return stopWords
}

// defaultSynonyms returns a map of common synonyms
func defaultSynonyms() map[string][]string {
	return map[string][]string{
		"search":    {"find", "locate", "lookup"},
		"find":      {"search", "locate", "discover"},
		"file":      {"document", "doc"},
		"document":  {"file", "doc"},
		"folder":    {"directory", "dir"},
		"directory": {"folder", "dir"},
		"create":    {"make", "new", "generate"},
		"delete":    {"remove", "erase", "del"},
		"update":    {"modify", "change", "edit"},
		"config":    {"configuration", "settings", "setup"},
		"error":     {"bug", "issue", "problem"},
		"function":  {"func", "method", "procedure"},
		"variable":  {"var", "param", "parameter"},
	}
}

// BuildRequest converts a processed query to a search request
func (qp *QueryProcessor) BuildRequest(pq *ProcessedQuery) *Request {
	req := &Request{
		Query: pq.Cleaned,
	}

	// Add phrases to query
	if len(pq.Phrases) > 0 {
		req.Query = fmt.Sprintf("%s \"%s\"", req.Query, strings.Join(pq.Phrases, "\" \""))
	}

	// Add must include terms
	for _, term := range pq.MustInclude {
		req.Query = fmt.Sprintf("%s +%s", req.Query, term)
	}

	// Add must exclude terms
	for _, term := range pq.MustExclude {
		req.Query = fmt.Sprintf("%s -%s", req.Query, term)
	}

	// Set filters
	req.FileTypes = pq.FileTypes

	if pq.DateRange != nil {
		req.DateFrom = &pq.DateRange.From
		req.DateTo = &pq.DateRange.To
	}

	if pq.SizeRange != nil {
		req.MinSize = pq.SizeRange.Min
		req.MaxSize = pq.SizeRange.Max
	}

	// Set search type based on query type
	switch pq.QueryType {
	case "code":
		req.SearchType = "hybrid" // Use both for code
	case "natural":
		req.SearchType = "vector" // Prefer semantic for natural language
	case "path":
		req.SearchType = "text" // Use text search for paths
	default:
		req.SearchType = "hybrid"
	}

	return req
}
