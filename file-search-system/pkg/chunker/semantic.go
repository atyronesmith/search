package chunker

import (
	"strings"

	"github.com/file-search/file-search-system/pkg/extractor"
)

// SemanticChunker chunks content based on semantic boundaries
type SemanticChunker struct{}

// NewSemanticChunker creates a new semantic chunker
func NewSemanticChunker() *SemanticChunker {
	return &SemanticChunker{}
}

// GetName returns the chunker name
func (c *SemanticChunker) GetName() string {
	return "semantic"
}

// SupportsFileType checks if this chunker supports the file type
func (c *SemanticChunker) SupportsFileType(fileType string) bool {
	supportedTypes := map[string]bool{
		"document": true,
		"text":     true,
		"markdown": true,
	}
	return supportedTypes[fileType]
}

// Chunk chunks content based on semantic structure
func (c *SemanticChunker) Chunk(content *extractor.ExtractedContent, config *ChunkerConfig) ([]Chunk, error) {
	// If structured content is available, use it
	if len(content.Sections) > 0 {
		return c.chunkBySections(content.Sections, config)
	}
	
	// If pages are available (PDF), chunk by pages first
	if len(content.Pages) > 0 {
		return c.chunkByPages(content.Pages, config)
	}
	
	// Fall back to paragraph-based chunking
	return c.chunkByParagraphs(content.Text, config)
}

// chunkBySections chunks content using extracted sections
func (c *SemanticChunker) chunkBySections(sections []extractor.SectionContent, config *ChunkerConfig) ([]Chunk, error) {
	var chunks []Chunk
	currentChunk := strings.Builder{}
	currentSections := []extractor.SectionContent{}
	chunkIndex := 0
	
	for _, section := range sections {
		sectionText := section.Text
		
		// If adding this section would exceed chunk size, finalize current chunk
		if currentChunk.Len() > 0 && 
		   countTokensApproximate(currentChunk.String()+sectionText) > config.ChunkSize {
			
			chunk := c.finalizeChunk(currentChunk.String(), chunkIndex, currentSections)
			chunks = append(chunks, chunk)
			chunkIndex++
			
			// Start new chunk with overlap
			currentChunk.Reset()
			currentSections = []extractor.SectionContent{}
			
			// Add overlap from previous chunk if configured
			if config.ChunkOverlap > 0 && len(chunks) > 0 {
				overlapText := c.getOverlapText(chunks[len(chunks)-1].Content, config.ChunkOverlap)
				currentChunk.WriteString(overlapText)
				if overlapText != "" {
					currentChunk.WriteString("\n\n")
				}
			}
		}
		
		// Add section to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(sectionText)
		currentSections = append(currentSections, section)
		
		// If this is a large section, split it further
		if countTokensApproximate(sectionText) > config.ChunkSize {
			// Finalize current chunk
			chunk := c.finalizeChunk(currentChunk.String(), chunkIndex, currentSections)
			chunks = append(chunks, chunk)
			chunkIndex++
			
			// Split the large section
			subChunks := c.splitLargeSection(sectionText, chunkIndex, config)
			chunks = append(chunks, subChunks...)
			chunkIndex += len(subChunks)
			
			// Reset for next sections
			currentChunk.Reset()
			currentSections = []extractor.SectionContent{}
		}
	}
	
	// Finalize last chunk
	if currentChunk.Len() > 0 {
		chunk := c.finalizeChunk(currentChunk.String(), chunkIndex, currentSections)
		chunks = append(chunks, chunk)
	}
	
	return chunks, nil
}

// chunkByPages chunks content by pages (for PDFs)
func (c *SemanticChunker) chunkByPages(pages []extractor.PageContent, config *ChunkerConfig) ([]Chunk, error) {
	var chunks []Chunk
	currentChunk := strings.Builder{}
	currentPages := []int{}
	chunkIndex := 0
	
	for _, page := range pages {
		pageText := page.Text
		
		// If adding this page would exceed chunk size, finalize current chunk
		if currentChunk.Len() > 0 && 
		   countTokensApproximate(currentChunk.String()+pageText) > config.ChunkSize {
			
			chunk := Chunk{
				Content:   strings.TrimSpace(currentChunk.String()),
				Index:     chunkIndex,
				Type:      "semantic",
				StartPage: currentPages[0],
				EndPage:   currentPages[len(currentPages)-1],
				Metadata: map[string]interface{}{
					"pages":      currentPages,
					"page_count": len(currentPages),
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
			
			// Start new chunk
			currentChunk.Reset()
			currentPages = []int{}
		}
		
		// Add page to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(pageText)
		currentPages = append(currentPages, page.Number)
	}
	
	// Finalize last chunk
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Content:   strings.TrimSpace(currentChunk.String()),
			Index:     chunkIndex,
			Type:      "semantic",
			StartPage: currentPages[0],
			EndPage:   currentPages[len(currentPages)-1],
			Metadata: map[string]interface{}{
				"pages":      currentPages,
				"page_count": len(currentPages),
			},
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks, nil
}

// chunkByParagraphs chunks content by paragraphs
func (c *SemanticChunker) chunkByParagraphs(text string, config *ChunkerConfig) ([]Chunk, error) {
	paragraphs := splitIntoParagraphs(text)
	var chunks []Chunk
	currentChunk := strings.Builder{}
	chunkIndex := 0
	
	for _, paragraph := range paragraphs {
		// If adding this paragraph would exceed chunk size, finalize current chunk
		if currentChunk.Len() > 0 && 
		   countTokensApproximate(currentChunk.String()+paragraph) > config.ChunkSize {
			
			chunk := Chunk{
				Content: strings.TrimSpace(currentChunk.String()),
				Index:   chunkIndex,
				Type:    "semantic",
				Metadata: map[string]interface{}{
					"chunking_method": "paragraph",
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
			
			// Start new chunk with overlap
			currentChunk.Reset()
			if config.ChunkOverlap > 0 && len(chunks) > 0 {
				overlapText := c.getOverlapText(chunks[len(chunks)-1].Content, config.ChunkOverlap)
				if overlapText != "" {
					currentChunk.WriteString(overlapText)
					currentChunk.WriteString("\n\n")
				}
			}
		}
		
		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(paragraph)
	}
	
	// Finalize last chunk
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Content: strings.TrimSpace(currentChunk.String()),
			Index:   chunkIndex,
			Type:    "semantic",
			Metadata: map[string]interface{}{
				"chunking_method": "paragraph",
			},
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks, nil
}

// finalizeChunk creates a finalized chunk from current content
func (c *SemanticChunker) finalizeChunk(content string, index int, sections []extractor.SectionContent) Chunk {
	metadata := map[string]interface{}{
		"chunking_method": "semantic",
		"section_count":   len(sections),
	}
	
	// Add section types to metadata
	sectionTypes := make([]string, len(sections))
	for i, section := range sections {
		sectionTypes[i] = section.Type
	}
	metadata["section_types"] = sectionTypes
	
	return Chunk{
		Content:  strings.TrimSpace(content),
		Index:    index,
		Type:     "semantic",
		Metadata: metadata,
	}
}

// splitLargeSection splits a section that's too large into smaller chunks
func (c *SemanticChunker) splitLargeSection(text string, startIndex int, config *ChunkerConfig) []Chunk {
	// Use sentence-based splitting for large sections
	sentences := splitIntoSentences(text)
	var chunks []Chunk
	currentChunk := strings.Builder{}
	chunkIndex := startIndex
	
	for _, sentence := range sentences {
		// If adding this sentence would exceed chunk size, finalize current chunk
		if currentChunk.Len() > 0 && 
		   countTokensApproximate(currentChunk.String()+sentence) > config.ChunkSize {
			
			chunk := Chunk{
				Content: strings.TrimSpace(currentChunk.String()),
				Index:   chunkIndex,
				Type:    "semantic",
				Metadata: map[string]interface{}{
					"chunking_method": "sentence",
					"from_large_section": true,
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
			
			// Start new chunk with overlap
			currentChunk.Reset()
			if config.ChunkOverlap > 0 && len(chunks) > 0 {
				overlapText := c.getOverlapText(chunks[len(chunks)-1].Content, config.ChunkOverlap)
				if overlapText != "" {
					currentChunk.WriteString(overlapText)
					currentChunk.WriteString(" ")
				}
			}
		}
		
		// Add sentence to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
	}
	
	// Finalize last chunk
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Content: strings.TrimSpace(currentChunk.String()),
			Index:   chunkIndex,
			Type:    "semantic",
			Metadata: map[string]interface{}{
				"chunking_method": "sentence",
				"from_large_section": true,
			},
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks
}

// getOverlapText gets overlap text from the end of previous chunk
func (c *SemanticChunker) getOverlapText(text string, overlapSize int) string {
	if len(text) <= overlapSize {
		return text
	}
	
	// Find a good breaking point (sentence or paragraph boundary)
	start := len(text) - overlapSize
	
	// Look for sentence boundary
	for i := start; i < len(text); i++ {
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			if i+1 < len(text) && text[i+1] == ' ' {
				return strings.TrimSpace(text[i+2:])
			}
		}
	}
	
	// Look for paragraph boundary
	for i := start; i < len(text); i++ {
		if i+1 < len(text) && text[i] == '\n' && text[i+1] == '\n' {
			return strings.TrimSpace(text[i+2:])
		}
	}
	
	// Fall back to word boundary
	for i := start; i < len(text); i++ {
		if text[i] == ' ' {
			return strings.TrimSpace(text[i+1:])
		}
	}
	
	return text[start:]
}