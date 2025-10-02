package chunker

import (
	"strings"

	"github.com/file-search/file-search-system/pkg/extractor"
)

// ElementChunker uses Unstructured.io elements directly as chunks
type ElementChunker struct {
	minGroupSize int // Minimum size for grouping tiny elements
	maxGroupSize int // Maximum size when grouping elements
}

// ElementChunk extends Chunk with element-specific metadata
type ElementChunk struct {
	Chunk
	ElementType      string   // Single element type
	ElementTypes     []string // Multiple types if grouped
	CategoryDepth    int      // Hierarchy level
	ParentElementID  string   // Parent element reference
	IsTitle          bool     // Quick flag for titles
	IsHeader         bool     // Quick flag for headers
	EmphasisScore    float64  // Search boost factor
}

// NewElementChunker creates a new element-based chunker
func NewElementChunker() *ElementChunker {
	return &ElementChunker{
		minGroupSize: 50,   // Group elements smaller than this
		maxGroupSize: 2000, // Maximum size when grouping
	}
}

// Chunk converts Unstructured elements into chunks
func (c *ElementChunker) Chunk(content *extractor.ExtractedContent, config *Config) ([]Chunk, error) {
	// Check if we have element data
	elements, ok := content.Metadata["elements"].([]interface{})
	if !ok || len(elements) == 0 {
		// Fallback to semantic chunking if no elements
		return NewSemanticChunker().Chunk(content, config)
	}
	
	var chunks []Chunk
	var tinyBuffer []interface{} // Buffer for grouping tiny elements
	chunkIndex := 0

	for i, elem := range elements {
		element, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}

		text := c.getElementText(element)
		if text == "" {
			continue
		}

		elementType := c.getElementType(element)
		textLen := len(text)

		// Check if this is a tiny element that should be grouped
		if textLen < c.minGroupSize && !c.isStandaloneElement(elementType) {
			tinyBuffer = append(tinyBuffer, element)
			
			// Check if we should flush the buffer
			bufferSize := c.calculateBufferSize(tinyBuffer)
			nextIsLarge := false
			if i+1 < len(elements) {
				if nextElem, ok := elements[i+1].(map[string]interface{}); ok {
					nextType := c.getElementType(nextElem)
					nextText := c.getElementText(nextElem)
					nextIsLarge = len(nextText) >= c.minGroupSize || c.isStandaloneElement(nextType)
				}
			}

			// Flush buffer if it's getting large, next is large, or we're at the end
			if bufferSize >= c.maxGroupSize || nextIsLarge || i == len(elements)-1 {
				if len(tinyBuffer) > 0 {
					chunk := c.createGroupedChunk(tinyBuffer, chunkIndex)
					chunks = append(chunks, chunk)
					chunkIndex++
					tinyBuffer = nil
				}
			}
		} else {
			// First flush any buffered tiny elements
			if len(tinyBuffer) > 0 {
				chunk := c.createGroupedChunk(tinyBuffer, chunkIndex)
				chunks = append(chunks, chunk)
				chunkIndex++
				tinyBuffer = nil
			}

			// Create chunk from this element
			chunk := c.createElementChunk(element, chunkIndex)
			chunks = append(chunks, chunk)
			chunkIndex++
		}
	}

	// Flush any remaining buffered elements
	if len(tinyBuffer) > 0 {
		chunk := c.createGroupedChunk(tinyBuffer, chunkIndex)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// createElementChunk creates a chunk from a single element
func (c *ElementChunker) createElementChunk(element map[string]interface{}, index int) Chunk {
	text := c.getElementText(element)
	elementType := c.getElementType(element)
	
	// Calculate character positions
	startChar := 0
	endChar := len(text)
	if start, ok := element["start"].(float64); ok {
		startChar = int(start)
	}
	if end, ok := element["end"].(float64); ok {
		endChar = int(end)
	}

	// Extract metadata
	metadata := make(map[string]interface{})
	metadata["element_type"] = elementType
	metadata["element_id"] = c.getElementID(element)
	
	// Add hierarchy information
	if depth, ok := element["category_depth"].(float64); ok {
		metadata["category_depth"] = int(depth)
	}
	if parentID, ok := element["parent_id"].(string); ok {
		metadata["parent_element_id"] = parentID
	}

	// Calculate emphasis score
	emphasisScore := c.calculateEmphasisScore(elementType)
	metadata["emphasis_score"] = emphasisScore
	
	// Set flags
	metadata["is_title"] = c.isTitle(elementType)
	metadata["is_header"] = c.isHeader(elementType)

	// Add page information if available
	if pageNum, ok := element["page_number"].(float64); ok {
		metadata["page_number"] = int(pageNum)
	}

	return Chunk{
		Content:   text,
		Index:     index,
		Type:      "element",
		StartChar: startChar,
		EndChar:   endChar,
		Metadata:  metadata,
	}
}

// createGroupedChunk creates a chunk from multiple tiny elements
func (c *ElementChunker) createGroupedChunk(elements []interface{}, index int) Chunk {
	var texts []string
	var elementTypes []string
	metadata := make(map[string]interface{})
	
	// Track unique element types
	typeSet := make(map[string]bool)
	minDepth := 999
	var pageNumbers []int
	totalEmphasis := 0.0
	hasTitle := false
	hasHeader := false

	for _, elem := range elements {
		element, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}

		text := c.getElementText(element)
		if text != "" {
			texts = append(texts, text)
		}

		elementType := c.getElementType(element)
		if !typeSet[elementType] {
			typeSet[elementType] = true
			elementTypes = append(elementTypes, elementType)
		}

		// Track hierarchy depth
		if depth, ok := element["category_depth"].(float64); ok {
			if int(depth) < minDepth {
				minDepth = int(depth)
			}
		}

		// Collect page numbers
		if pageNum, ok := element["page_number"].(float64); ok {
			pageNumbers = append(pageNumbers, int(pageNum))
		}

		// Accumulate emphasis scores
		totalEmphasis += c.calculateEmphasisScore(elementType)
		
		// Check for titles/headers
		if c.isTitle(elementType) {
			hasTitle = true
		}
		if c.isHeader(elementType) {
			hasHeader = true
		}
	}

	// Join texts with appropriate separator
	content := strings.Join(texts, "\n\n")
	
	// Set metadata for grouped chunk
	metadata["element_type"] = "grouped"
	metadata["element_types"] = elementTypes
	metadata["element_count"] = len(elements)
	
	if minDepth < 999 {
		metadata["category_depth"] = minDepth
	}
	
	// Average emphasis score
	if len(elements) > 0 {
		metadata["emphasis_score"] = totalEmphasis / float64(len(elements))
	} else {
		metadata["emphasis_score"] = 1.0
	}
	
	metadata["is_title"] = hasTitle
	metadata["is_header"] = hasHeader
	
	// Add page range if applicable
	if len(pageNumbers) > 0 {
		minPage, maxPage := pageNumbers[0], pageNumbers[0]
		for _, p := range pageNumbers {
			if p < minPage {
				minPage = p
			}
			if p > maxPage {
				maxPage = p
			}
		}
		metadata["start_page"] = minPage
		metadata["end_page"] = maxPage
	}

	return Chunk{
		Content:   content,
		Index:     index,
		Type:      "element",
		StartChar: 0,
		EndChar:   len(content),
		Metadata:  metadata,
	}
}

// Helper methods

func (c *ElementChunker) getElementText(element map[string]interface{}) string {
	if text, ok := element["text"].(string); ok {
		return strings.TrimSpace(text)
	}
	if content, ok := element["content"].(string); ok {
		return strings.TrimSpace(content)
	}
	return ""
}

func (c *ElementChunker) getElementType(element map[string]interface{}) string {
	if elemType, ok := element["type"].(string); ok {
		return elemType
	}
	if elemType, ok := element["element_type"].(string); ok {
		return elemType
	}
	return "Unknown"
}

func (c *ElementChunker) getElementID(element map[string]interface{}) string {
	if id, ok := element["element_id"].(string); ok {
		return id
	}
	if id, ok := element["id"].(string); ok {
		return id
	}
	return ""
}

func (c *ElementChunker) isStandaloneElement(elementType string) bool {
	// These elements should always be their own chunks
	standalone := map[string]bool{
		"Title":      true,
		"Header":     true,
		"Table":      true,
		"Image":      true,
		"FigureCaption": true,
	}
	return standalone[elementType]
}

func (c *ElementChunker) isTitle(elementType string) bool {
	return elementType == "Title"
}

func (c *ElementChunker) isHeader(elementType string) bool {
	return elementType == "Header" || elementType == "Title"
}

func (c *ElementChunker) calculateEmphasisScore(elementType string) float64 {
	scores := map[string]float64{
		"Title":         2.0,
		"Header":        1.5,
		"Table":         1.3,
		"ListItem":      0.9,
		"Footer":        0.7,
		"PageBreak":     0.5,
		"NarrativeText": 1.0,
		"Unknown":       1.0,
	}
	
	if score, ok := scores[elementType]; ok {
		return score
	}
	return 1.0
}

func (c *ElementChunker) calculateBufferSize(buffer []interface{}) int {
	size := 0
	for _, elem := range buffer {
		if element, ok := elem.(map[string]interface{}); ok {
			text := c.getElementText(element)
			size += len(text)
		}
	}
	return size
}

// GetName returns the name of this chunker
func (c *ElementChunker) GetName() string {
	return "element"
}

// SupportsFileType checks if this chunker supports the given file type
func (c *ElementChunker) SupportsFileType(fileType string) bool {
	// Element chunker works with any file that has been processed by Unstructured
	// It will fallback to semantic chunking if no elements are present
	return true
}