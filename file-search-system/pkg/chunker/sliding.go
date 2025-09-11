package chunker

import (
	"strings"

	"github.com/file-search/file-search-system/pkg/extractor"
)

// SlidingWindowChunker implements sliding window chunking strategy
type SlidingWindowChunker struct{}

// NewSlidingWindowChunker creates a new sliding window chunker
func NewSlidingWindowChunker() *SlidingWindowChunker {
	return &SlidingWindowChunker{}
}

// GetName returns the chunker name
func (c *SlidingWindowChunker) GetName() string {
	return "sliding"
}

// SupportsFileType checks if this chunker supports the file type
func (c *SlidingWindowChunker) SupportsFileType(fileType string) bool {
	// Sliding window chunker supports all file types as fallback
	return true
}

// Chunk chunks content using sliding window approach
func (c *SlidingWindowChunker) Chunk(content *extractor.ExtractedContent, config *Config) ([]Chunk, error) {
	text := content.Text
	if text == "" {
		return []Chunk{}, nil
	}
	
	// Choose chunking method based on configuration
	if config.SplitOnSentences {
		return c.chunkBySentences(text, config)
	}
	return c.chunkByCharacters(text, config)
}

// chunkBySentences chunks text by sentences with sliding window
func (c *SlidingWindowChunker) chunkBySentences(text string, config *Config) ([]Chunk, error) {
	sentences := splitIntoSentences(text)
	if len(sentences) == 0 {
		return []Chunk{}, nil
	}
	
	var chunks []Chunk
	chunkIndex := 0
	
	i := 0
	for i < len(sentences) {
		chunk := c.buildSentenceChunk(sentences, i, config)
		chunk.Index = chunkIndex
		chunks = append(chunks, chunk)
		chunkIndex++
		
		// Calculate next starting position with overlap
		nextStart := c.calculateNextStart(sentences, i, config)
		if nextStart <= i {
			nextStart = i + 1 // Ensure progress
		}
		i = nextStart
	}
	
	return chunks, nil
}

// chunkByCharacters chunks text by character count with sliding window
func (c *SlidingWindowChunker) chunkByCharacters(text string, config *Config) ([]Chunk, error) {
	if len(text) == 0 {
		return []Chunk{}, nil
	}
	
	var chunks []Chunk
	chunkIndex := 0
	
	start := 0
	for start < len(text) {
		// Calculate chunk end position
		end := start + config.ChunkSize
		if end > len(text) {
			end = len(text)
		}
		
		// Adjust end to word boundary if possible
		if end < len(text) {
			end = c.findWordBoundary(text, end)
		}
		
		// Extract chunk content
		chunkContent := strings.TrimSpace(text[start:end])
		if chunkContent != "" {
			chunk := Chunk{
				Content:   chunkContent,
				Index:     chunkIndex,
				Type:      "sliding",
				StartChar: start,
				EndChar:   end,
				Metadata: map[string]interface{}{
					"chunking_method": "character",
					"window_size":     config.ChunkSize,
					"overlap_size":    config.ChunkOverlap,
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		}
		
		// Calculate next start position with overlap
		nextStart := end - config.ChunkOverlap
		if nextStart <= start {
			nextStart = start + config.ChunkSize - config.ChunkOverlap
		}
		if nextStart >= len(text) {
			break
		}
		
		// Adjust start to word boundary if possible
		nextStart = c.findWordBoundary(text, nextStart)
		start = nextStart
	}
	
	return chunks, nil
}

// buildSentenceChunk builds a chunk from sentences starting at index i
func (c *SlidingWindowChunker) buildSentenceChunk(sentences []string, startIdx int, config *Config) Chunk {
	chunkBuilder := strings.Builder{}
	sentenceCount := 0
	
	for i := startIdx; i < len(sentences); i++ {
		sentence := sentences[i]
		
		// Check if adding this sentence would exceed the chunk size
		testContent := chunkBuilder.String()
		if testContent != "" {
			testContent += " "
		}
		testContent += sentence
		
		if countTokensApproximate(testContent) > config.ChunkSize && chunkBuilder.Len() > 0 {
			break
		}
		
		// Add sentence to chunk
		if chunkBuilder.Len() > 0 {
			chunkBuilder.WriteString(" ")
		}
		chunkBuilder.WriteString(sentence)
		sentenceCount++
		
		// Don't exceed maximum chunk size
		if countTokensApproximate(chunkBuilder.String()) >= config.MaxChunkSize {
			break
		}
	}
	
	return Chunk{
		Content: strings.TrimSpace(chunkBuilder.String()),
		Type:    "sliding",
		Metadata: map[string]interface{}{
			"chunking_method": "sentence",
			"sentence_count":  sentenceCount,
			"window_size":     config.ChunkSize,
			"overlap_size":    config.ChunkOverlap,
		},
	}
}

// calculateNextStart calculates the next starting sentence index with overlap
func (c *SlidingWindowChunker) calculateNextStart(sentences []string, currentStart int, config *Config) int {
	// Build current chunk to understand its size
	currentChunk := c.buildSentenceChunk(sentences, currentStart, config)
	currentSentences := c.countSentencesInChunk(currentChunk.Content)
	
	// Calculate overlap in sentences
	overlapSentences := (config.ChunkOverlap * currentSentences) / config.ChunkSize
	if overlapSentences < 1 {
		overlapSentences = 1
	}
	
	nextStart := currentStart + currentSentences - overlapSentences
	if nextStart <= currentStart {
		nextStart = currentStart + 1
	}
	
	return nextStart
}

// countSentencesInChunk counts sentences in a chunk (approximate)
func (c *SlidingWindowChunker) countSentencesInChunk(content string) int {
	if content == "" {
		return 0
	}
	
	// Count sentence ending punctuation
	count := strings.Count(content, ".") + 
	        strings.Count(content, "!") + 
	        strings.Count(content, "?")
	
	if count == 0 {
		return 1 // At least one sentence
	}
	
	return count
}

// findWordBoundary finds the nearest word boundary before the given position
func (c *SlidingWindowChunker) findWordBoundary(text string, pos int) int {
	if pos >= len(text) {
		return len(text)
	}
	
	// Look backwards for a space, newline, or punctuation
	for i := pos - 1; i > 0; i-- {
		char := text[i]
		if char == ' ' || char == '\n' || char == '\t' || 
		   char == '.' || char == ',' || char == ';' || char == ':' {
			return i + 1
		}
	}
	
	return pos
}

// chunkWithFixedOverlap creates chunks with fixed character overlap
func (c *SlidingWindowChunker) chunkWithFixedOverlap(text string, chunkSize, overlap int) []Chunk {
	var chunks []Chunk
	chunkIndex := 0
	
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		
		// Adjust to word boundary
		if end < len(text) {
			end = c.findWordBoundary(text, end)
		}
		
		chunkContent := strings.TrimSpace(text[start:end])
		if chunkContent != "" {
			chunk := Chunk{
				Content:   chunkContent,
				Index:     chunkIndex,
				Type:      "sliding",
				StartChar: start,
				EndChar:   end,
				Metadata: map[string]interface{}{
					"chunking_method": "fixed_overlap",
					"chunk_size":      chunkSize,
					"overlap_size":    overlap,
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		}
		
		// Move to next position
		start = end - overlap
		if start >= len(text) || start <= 0 {
			break
		}
		
		// Adjust start to word boundary
		start = c.findWordBoundary(text, start)
	}
	
	return chunks
}

// optimizeChunkBoundaries optimizes chunk boundaries to avoid splitting words
func (c *SlidingWindowChunker) optimizeChunkBoundaries(chunks []Chunk, originalText string) []Chunk {
	for i := range chunks {
		chunk := &chunks[i]
		
		// Optimize start boundary (except for first chunk)
		if i > 0 && chunk.StartChar > 0 {
			newStart := c.findWordBoundary(originalText, chunk.StartChar)
			if newStart != chunk.StartChar {
				chunk.StartChar = newStart
				chunk.Content = strings.TrimSpace(originalText[chunk.StartChar:chunk.EndChar])
			}
		}
		
		// Optimize end boundary (except for last chunk)
		if i < len(chunks)-1 && chunk.EndChar < len(originalText) {
			newEnd := c.findWordBoundary(originalText, chunk.EndChar)
			if newEnd != chunk.EndChar {
				chunk.EndChar = newEnd
				chunk.Content = strings.TrimSpace(originalText[chunk.StartChar:chunk.EndChar])
			}
		}
	}
	
	return chunks
}