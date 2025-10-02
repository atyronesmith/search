package chunker

import (
	"strings"

	"github.com/file-search/file-search-system/pkg/extractor"
)

// MaxPostgreSQLTsvectorBytes defines the PostgreSQL tsvector limit
const MaxPostgreSQLTsvectorBytes = 1048575

// Chunk represents a text chunk
type Chunk struct {
	Content   string                 `json:"content"`
	Index     int                    `json:"index"`
	Type      string                 `json:"type"`
	StartChar int                    `json:"start_char"`
	EndChar   int                    `json:"end_char"`
	StartLine int                    `json:"start_line,omitempty"`
	EndLine   int                    `json:"end_line,omitempty"`
	StartPage int                    `json:"start_page,omitempty"`
	EndPage   int                    `json:"end_page,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Config holds configuration for chunking
type Config struct {
	ChunkSize        int  `json:"chunk_size"`     // Target chunk size in tokens/characters
	ChunkOverlap     int  `json:"chunk_overlap"`  // Overlap between chunks
	MaxChunkSize     int  `json:"max_chunk_size"` // Maximum chunk size
	MinChunkSize     int  `json:"min_chunk_size"` // Minimum chunk size
	SplitOnSentences bool `json:"split_on_sentences"`
}

// DefaultConfig returns default chunker configuration
func DefaultConfig() *Config {
	return &Config{
		ChunkSize:        512,
		ChunkOverlap:     64,
		MaxChunkSize:     800000, // ~800KB - well below PostgreSQL's 1MB tsvector limit
		MinChunkSize:     10,     // Reduced to handle very small content
		SplitOnSentences: true,
	}
}

// Chunker interface for different chunking strategies
type Chunker interface {
	Chunk(content *extractor.ExtractedContent, config *Config) ([]Chunk, error)
	GetName() string
	SupportsFileType(fileType string) bool
}

// Manager manages different chunking strategies
type Manager struct {
	chunkers map[string]Chunker
	config   *Config
}

// NewManager creates a new chunker manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	chunkers := make(map[string]Chunker)

	// Add default chunkers
	chunkers["element"] = NewElementChunker()
	chunkers["semantic"] = NewSemanticChunker()
	chunkers["sliding"] = NewSlidingWindowChunker()
	chunkers["code"] = NewCodeChunker()

	return &Manager{
		chunkers: chunkers,
		config:   config,
	}
}

// ChunkContent chunks content using the appropriate strategy
func (cm *Manager) ChunkContent(content *extractor.ExtractedContent, fileType string) ([]Chunk, error) {
	chunker := cm.selectChunker(fileType, content)
	chunks, err := chunker.Chunk(content, cm.config)
	if err != nil {
		return nil, err
	}

	// Enforce size limits to prevent PostgreSQL tsvector errors
	return cm.enforceSizeLimits(chunks), nil
}

// selectChunker selects the appropriate chunker based on content type
func (cm *Manager) selectChunker(fileType string, content *extractor.ExtractedContent) Chunker {
	// Check if we have element data from Unstructured
	if elements, ok := content.Metadata["elements"].([]interface{}); ok && len(elements) > 0 {
		// Use element chunker when we have Unstructured elements
		return cm.chunkers["element"]
	}

	// Priority order for chunker selection
	chunkerOrder := []string{"code", "semantic", "sliding"}

	for _, name := range chunkerOrder {
		if chunker, exists := cm.chunkers[name]; exists {
			if chunker.SupportsFileType(fileType) {
				return chunker
			}
		}
	}

	// Default to sliding window
	return cm.chunkers["sliding"]
}

// AddChunker adds a custom chunker
func (cm *Manager) AddChunker(name string, chunker Chunker) {
	cm.chunkers[name] = chunker
}

// enforceSizeLimits ensures chunks don't exceed PostgreSQL limits
func (cm *Manager) enforceSizeLimits(chunks []Chunk) []Chunk {
	const SafeChunkSizeBytes = 800000 // Safe limit well below PostgreSQL maximum

	var result []Chunk

	for _, chunk := range chunks {
		chunkSize := len([]byte(chunk.Content))

		// If chunk is within safe limits, keep it as-is
		if chunkSize <= SafeChunkSizeBytes {
			// Skip empty chunks
			if strings.TrimSpace(chunk.Content) != "" {
				result = append(result, chunk)
			}
			continue
		}

		// Split oversized chunk into smaller pieces
		subChunks := cm.splitOversizedChunk(chunk, SafeChunkSizeBytes)
		result = append(result, subChunks...)
	}

	return result
}

// splitOversizedChunk splits a chunk that's too large into smaller chunks
func (cm *Manager) splitOversizedChunk(chunk Chunk, maxSize int) []Chunk {
	content := chunk.Content
	var subChunks []Chunk

	// Try to split on paragraph boundaries first
	paragraphs := strings.Split(content, "\n\n")
	currentChunk := ""
	chunkIndex := 0

	for _, paragraph := range paragraphs {
		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n\n"
		}
		testChunk += paragraph

		// If adding this paragraph would exceed limit, save current chunk and start new one
		if len([]byte(testChunk)) > maxSize && currentChunk != "" {
			if strings.TrimSpace(currentChunk) != "" {
				subChunks = append(subChunks, Chunk{
					Content:   strings.TrimSpace(currentChunk),
					Index:     chunk.Index*1000 + chunkIndex, // Unique index for sub-chunks
					Type:      chunk.Type,
					StartChar: chunk.StartChar,
					EndChar:   chunk.EndChar,
					Metadata:  chunk.Metadata,
				})
				chunkIndex++
			}
			currentChunk = paragraph
		} else {
			currentChunk = testChunk
		}
	}

	// Add the last chunk
	if strings.TrimSpace(currentChunk) != "" {
		subChunks = append(subChunks, Chunk{
			Content:   strings.TrimSpace(currentChunk),
			Index:     chunk.Index*1000 + chunkIndex,
			Type:      chunk.Type,
			StartChar: chunk.StartChar,
			EndChar:   chunk.EndChar,
			Metadata:  chunk.Metadata,
		})
	}

	// If we still have oversized chunks, split by sentences
	var finalChunks []Chunk
	for _, subChunk := range subChunks {
		if len([]byte(subChunk.Content)) > maxSize {
			finalChunks = append(finalChunks, cm.splitBySentences(subChunk, maxSize)...)
		} else {
			finalChunks = append(finalChunks, subChunk)
		}
	}

	return finalChunks
}

// splitBySentences splits content by sentences when paragraphs are still too large
func (cm *Manager) splitBySentences(chunk Chunk, maxSize int) []Chunk {
	content := chunk.Content
	var subChunks []Chunk

	// Split by sentence-ending punctuation
	sentences := strings.FieldsFunc(content, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	currentChunk := ""
	chunkIndex := 0

	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// Add punctuation back (except for last sentence)
		if i < len(sentences)-1 {
			sentence += "."
		}

		testChunk := currentChunk
		if testChunk != "" {
			testChunk += " "
		}
		testChunk += sentence

		// If adding this sentence would exceed limit, save current chunk
		if len([]byte(testChunk)) > maxSize && currentChunk != "" {
			subChunks = append(subChunks, Chunk{
				Content:   strings.TrimSpace(currentChunk),
				Index:     chunk.Index*1000 + chunkIndex,
				Type:      chunk.Type,
				StartChar: chunk.StartChar,
				EndChar:   chunk.EndChar,
				Metadata:  chunk.Metadata,
			})
			chunkIndex++
			currentChunk = sentence
		} else {
			currentChunk = testChunk
		}
	}

	// Add the last chunk
	if strings.TrimSpace(currentChunk) != "" {
		subChunks = append(subChunks, Chunk{
			Content:   strings.TrimSpace(currentChunk),
			Index:     chunk.Index*1000 + chunkIndex,
			Type:      chunk.Type,
			StartChar: chunk.StartChar,
			EndChar:   chunk.EndChar,
			Metadata:  chunk.Metadata,
		})
	}

	return subChunks
}

// Utility functions

// countTokensApproximate provides a rough token count estimation
func countTokensApproximate(text string) int {
	// Simple approximation: average 4 characters per token
	return len(text) / 4
}

// splitIntoSentences splits text into sentences
func splitIntoSentences(text string) []string {
	// Simple sentence splitting on common punctuation
	sentences := []string{}
	current := strings.Builder{}

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)

		// Check for sentence endings
		if r == '.' || r == '!' || r == '?' {
			// Look ahead to see if this is actually end of sentence
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == ' ' || next == '\n' || next == '\t' {
					// Check if next word starts with capital letter (simple heuristic)
					for j := i + 1; j < len(runes); j++ {
						if runes[j] != ' ' && runes[j] != '\n' && runes[j] != '\t' {
							if j+1 < len(runes) && isUpper(runes[j]) {
								sentences = append(sentences, strings.TrimSpace(current.String()))
								current.Reset()
							}
							break
						}
					}
				}
			} else {
				// End of text
				sentences = append(sentences, strings.TrimSpace(current.String()))
				current.Reset()
			}
		}
	}

	// Add remaining text
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}

	return sentences
}

// splitIntoParagraphs splits text into paragraphs
func splitIntoParagraphs(text string) []string {
	paragraphs := strings.Split(text, "\n\n")
	result := make([]string, 0, len(paragraphs))

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// splitIntoLines splits text into lines
func splitIntoLines(text string) []string {
	return strings.Split(text, "\n")
}

// isUpper checks if a rune is uppercase
func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// trimToWordBoundary trims text to the nearest word boundary
func trimToWordBoundary(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Find last space within limit
	for i := maxLen - 1; i >= 0; i-- {
		if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
			return strings.TrimSpace(text[:i])
		}
	}

	// If no word boundary found, cut at limit
	return text[:maxLen]
}

// findOverlapPosition finds a good position for chunk overlap
func findOverlapPosition(text string, targetOverlap int) int {
	if len(text) <= targetOverlap {
		return 0
	}

	start := len(text) - targetOverlap

	// Try to find a sentence boundary
	for i := start; i < len(text); i++ {
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			if i+1 < len(text) && text[i+1] == ' ' {
				return i + 2
			}
		}
	}

	// Try to find a word boundary
	for i := start; i < len(text); i++ {
		if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
			return i + 1
		}
	}

	return start
}

// calculateCharPositions calculates character positions within original text
func calculateCharPositions(originalText string, chunks []Chunk) []Chunk {
	currentPos := 0

	for i := range chunks {
		chunks[i].StartChar = currentPos
		chunks[i].EndChar = currentPos + len(chunks[i].Content)
		currentPos = chunks[i].EndChar
	}

	return chunks
}

// calculateLineNumbers calculates line numbers for chunks
func calculateLineNumbers(originalText string, chunks []Chunk) []Chunk {

	for i := range chunks {
		// Simple line calculation based on character position
		textUpToStart := originalText[:chunks[i].StartChar]
		chunks[i].StartLine = strings.Count(textUpToStart, "\n") + 1

		textUpToEnd := originalText[:chunks[i].EndChar]
		chunks[i].EndLine = strings.Count(textUpToEnd, "\n") + 1
	}

	return chunks
}

// Prevent unused items linter warnings
func init() {
	// Reference unused functions and constants to silence linter
	if false {
		_ = MaxPostgreSQLTsvectorBytes
		_ = splitIntoLines
		_ = trimToWordBoundary
		_ = findOverlapPosition  
		_ = calculateCharPositions
		_ = calculateLineNumbers
	}
}
