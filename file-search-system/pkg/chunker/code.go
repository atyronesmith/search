package chunker

import (
	"strings"

	"github.com/file-search/file-search-system/pkg/extractor"
)

// CodeChunker implements code-aware chunking strategy
type CodeChunker struct{}

// NewCodeChunker creates a new code chunker
func NewCodeChunker() *CodeChunker {
	return &CodeChunker{}
}

// GetName returns the chunker name
func (c *CodeChunker) GetName() string {
	return "code"
}

// SupportsFileType checks if this chunker supports the file type
func (c *CodeChunker) SupportsFileType(fileType string) bool {
	supportedTypes := map[string]bool{
		"code": true,
	}
	return supportedTypes[fileType]
}

// Chunk chunks code content respecting code structure
func (c *CodeChunker) Chunk(content *extractor.ExtractedContent, config *Config) ([]Chunk, error) {
	// If structured content is available, use it
	if len(content.Sections) > 0 {
		return c.chunkBySections(content.Sections, config)
	}

	// Fall back to function/class-based chunking
	return c.chunkByCodeStructure(content.Text, config)
}

// chunkBySections chunks code using extracted sections (functions, classes, etc.)
func (c *CodeChunker) chunkBySections(sections []extractor.SectionContent, config *Config) ([]Chunk, error) {
	var chunks []Chunk
	chunkIndex := 0

	for _, section := range sections {
		// Handle different section types
		switch section.Type {
		case "function", "class", "method":
			// Each function/class gets its own chunk if not too large
			if countTokensApproximate(section.Text) <= config.MaxChunkSize {
				chunk := Chunk{
					Content: strings.TrimSpace(section.Text),
					Index:   chunkIndex,
					Type:    "code",
					Metadata: map[string]interface{}{
						"chunking_method": "structural",
						"section_type":    section.Type,
						"language":        section.Language,
					},
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			} else {
				// Large function/class needs to be split
				subChunks := c.splitLargeCodeSection(section.Text, section.Language, chunkIndex, config)
				chunks = append(chunks, subChunks...)
				chunkIndex += len(subChunks)
			}

		case "imports", "header":
			// Group imports and headers with following code
			// This will be handled in the grouping phase
			continue

		default:
			// Regular code section
			if countTokensApproximate(section.Text) <= config.ChunkSize {
				chunk := Chunk{
					Content: strings.TrimSpace(section.Text),
					Index:   chunkIndex,
					Type:    "code",
					Metadata: map[string]interface{}{
						"chunking_method": "section",
						"section_type":    section.Type,
						"language":        section.Language,
					},
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			} else {
				subChunks := c.splitLargeCodeSection(section.Text, section.Language, chunkIndex, config)
				chunks = append(chunks, subChunks...)
				chunkIndex += len(subChunks)
			}
		}
	}

	// Group related chunks (imports with functions, etc.)
	chunks = c.groupRelatedChunks(chunks, config)

	return chunks, nil
}

// chunkByCodeStructure chunks code by detecting functions, classes, etc.
func (c *CodeChunker) chunkByCodeStructure(text string, config *Config) ([]Chunk, error) {
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	chunkIndex := 0

	// Detect code blocks
	codeBlocks := c.detectCodeBlocks(lines)

	for _, block := range codeBlocks {
		blockText := strings.Join(lines[block.StartLine:block.EndLine+1], "\n")

		if countTokensApproximate(blockText) <= config.ChunkSize {
			// Block fits in one chunk
			chunk := Chunk{
				Content:   strings.TrimSpace(blockText),
				Index:     chunkIndex,
				Type:      "code",
				StartLine: block.StartLine + 1, // 1-based line numbers
				EndLine:   block.EndLine + 1,
				Metadata: map[string]interface{}{
					"chunking_method": "code_block",
					"block_type":      block.Type,
					"language":        block.Language,
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		} else {
			// Block too large, split it
			subChunks := c.splitLargeCodeBlock(block, lines, chunkIndex, config)
			chunks = append(chunks, subChunks...)
			chunkIndex += len(subChunks)
		}
	}

	// If no code blocks detected, fall back to line-based chunking
	if len(chunks) == 0 {
		chunks = c.chunkByLines(lines, chunkIndex, config)
	}

	return chunks, nil
}

// CodeBlock represents a detected code block
type CodeBlock struct {
	Type      string // "function", "class", "block", "imports"
	StartLine int
	EndLine   int
	Language  string
	Name      string // function/class name if applicable
}

// detectCodeBlocks detects logical code blocks in the source
func (c *CodeChunker) detectCodeBlocks(lines []string) []CodeBlock {
	var blocks []CodeBlock

	currentBlock := &CodeBlock{
		Type:      "block",
		StartLine: 0,
	}

	inFunction := false
	inClass := false
	braceLevel := 0
	parenLevel := 0

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and comments for structure detection
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "//") || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Detect function definitions
		if c.isFunctionStart(trimmedLine) && !inFunction && !inClass {
			// Close previous block
			if i > currentBlock.StartLine {
				currentBlock.EndLine = i - 1
				blocks = append(blocks, *currentBlock)
			}

			// Start new function block
			currentBlock = &CodeBlock{
				Type:      "function",
				StartLine: i,
				Name:      c.extractFunctionName(trimmedLine),
			}
			inFunction = true
		}

		// Detect class definitions
		if c.isClassStart(trimmedLine) && !inClass {
			// Close previous block
			if i > currentBlock.StartLine {
				currentBlock.EndLine = i - 1
				blocks = append(blocks, *currentBlock)
			}

			// Start new class block
			currentBlock = &CodeBlock{
				Type:      "class",
				StartLine: i,
				Name:      c.extractClassName(trimmedLine),
			}
			inClass = true
		}

		// Track brace/paren levels to detect block ends
		braceLevel += strings.Count(trimmedLine, "{") - strings.Count(trimmedLine, "}")
		parenLevel += strings.Count(trimmedLine, "(") - strings.Count(trimmedLine, ")")

		// Detect end of function/class (simplified heuristic)
		if inFunction && braceLevel == 0 && parenLevel == 0 && trimmedLine != "" {
			if c.isFunctionEnd(trimmedLine) || i == len(lines)-1 {
				currentBlock.EndLine = i
				blocks = append(blocks, *currentBlock)

				// Start new general block
				currentBlock = &CodeBlock{
					Type:      "block",
					StartLine: i + 1,
				}
				inFunction = false
			}
		}

		if inClass && braceLevel == 0 && c.isClassEnd(trimmedLine) {
			currentBlock.EndLine = i
			blocks = append(blocks, *currentBlock)

			// Start new general block
			currentBlock = &CodeBlock{
				Type:      "block",
				StartLine: i + 1,
			}
			inClass = false
		}
	}

	// Close final block
	if currentBlock.StartLine < len(lines) {
		currentBlock.EndLine = len(lines) - 1
		blocks = append(blocks, *currentBlock)
	}

	return blocks
}

// Helper functions for code structure detection
func (c *CodeChunker) isFunctionStart(line string) bool {
	patterns := []string{
		"def ", "function ", "func ", "fn ", "void ", "int ", "bool ", "string ",
		"public ", "private ", "static ",
	}

	for _, pattern := range patterns {
		if strings.Contains(line, pattern) && strings.Contains(line, "(") {
			return true
		}
	}

	return false
}

func (c *CodeChunker) isClassStart(line string) bool {
	patterns := []string{"class ", "interface ", "struct ", "trait ", "impl "}

	for _, pattern := range patterns {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}

	return false
}

func (c *CodeChunker) isFunctionEnd(line string) bool {
	// Simple heuristics for function end
	return line == "}" || line == "end" || strings.HasPrefix(line, "}")
}

func (c *CodeChunker) isClassEnd(line string) bool {
	return line == "}" || line == "end" || strings.HasPrefix(line, "}")
}

func (c *CodeChunker) extractFunctionName(line string) string {
	// Extract function name (simplified)
	if strings.Contains(line, "(") {
		start := strings.LastIndex(line[:strings.Index(line, "(")], " ")
		if start != -1 {
			return strings.TrimSpace(line[start:strings.Index(line, "(")])
		}
	}
	return "unnamed"
}

func (c *CodeChunker) extractClassName(line string) string {
	// Extract class name (simplified)
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "unnamed"
}

// splitLargeCodeSection splits a large code section into smaller chunks
func (c *CodeChunker) splitLargeCodeSection(text, language string, startIndex int, config *Config) []Chunk {
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	chunkIndex := startIndex

	// Try to split at logical boundaries (empty lines, comments)
	currentChunk := []string{}
	currentSize := 0

	for _, line := range lines {
		lineSize := countTokensApproximate(line)

		if currentSize+lineSize > config.ChunkSize && len(currentChunk) > 0 {
			// Finalize current chunk
			chunk := Chunk{
				Content: strings.TrimSpace(strings.Join(currentChunk, "\n")),
				Index:   chunkIndex,
				Type:    "code",
				Metadata: map[string]interface{}{
					"chunking_method": "split_large",
					"language":        language,
					"is_partial":      true,
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++

			// Start new chunk with some overlap
			overlapLines := c.getOverlapLines(currentChunk, config.ChunkOverlap)
			currentChunk = overlapLines
			currentSize = c.calculateLinesSize(overlapLines)
		}

		currentChunk = append(currentChunk, line)
		currentSize += lineSize
	}

	// Add final chunk
	if len(currentChunk) > 0 {
		chunk := Chunk{
			Content: strings.TrimSpace(strings.Join(currentChunk, "\n")),
			Index:   chunkIndex,
			Type:    "code",
			Metadata: map[string]interface{}{
				"chunking_method": "split_large",
				"language":        language,
				"is_partial":      true,
			},
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// splitLargeCodeBlock splits a large code block
func (c *CodeChunker) splitLargeCodeBlock(block CodeBlock, lines []string, startIndex int, config *Config) []Chunk {
	blockLines := lines[block.StartLine : block.EndLine+1]
	text := strings.Join(blockLines, "\n")

	return c.splitLargeCodeSection(text, block.Language, startIndex, config)
}

// chunkByLines provides simple line-based chunking as fallback
func (c *CodeChunker) chunkByLines(lines []string, startIndex int, config *Config) []Chunk {
	var chunks []Chunk
	chunkIndex := startIndex

	currentChunk := []string{}
	currentSize := 0

	for _, line := range lines {
		lineSize := countTokensApproximate(line)

		if currentSize+lineSize > config.ChunkSize && len(currentChunk) > 0 {
			chunk := Chunk{
				Content: strings.TrimSpace(strings.Join(currentChunk, "\n")),
				Index:   chunkIndex,
				Type:    "code",
				Metadata: map[string]interface{}{
					"chunking_method": "lines",
				},
			}
			chunks = append(chunks, chunk)
			chunkIndex++

			// Start new chunk
			currentChunk = []string{}
			currentSize = 0
		}

		currentChunk = append(currentChunk, line)
		currentSize += lineSize
	}

	// Add final chunk
	if len(currentChunk) > 0 {
		chunk := Chunk{
			Content: strings.TrimSpace(strings.Join(currentChunk, "\n")),
			Index:   chunkIndex,
			Type:    "code",
			Metadata: map[string]interface{}{
				"chunking_method": "lines",
			},
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// groupRelatedChunks groups related code chunks (imports with functions, etc.)
func (c *CodeChunker) groupRelatedChunks(chunks []Chunk, config *Config) []Chunk {
	// Simple grouping: if total size allows, combine small adjacent chunks
	var grouped []Chunk

	i := 0
	for i < len(chunks) {
		current := chunks[i]

		// Look ahead to see if we can combine with next chunks
		combined := current.Content
		combinedMetadata := make(map[string]interface{})
		for k, v := range current.Metadata {
			combinedMetadata[k] = v
		}

		j := i + 1
		for j < len(chunks) {
			next := chunks[j]
			testCombined := combined + "\n\n" + next.Content

			if countTokensApproximate(testCombined) <= config.ChunkSize {
				combined = testCombined
				combinedMetadata["combined_chunks"] = j - i + 1
				j++
			} else {
				break
			}
		}

		// Create grouped chunk
		groupedChunk := Chunk{
			Content:  combined,
			Index:    current.Index,
			Type:     "code",
			Metadata: combinedMetadata,
		}
		grouped = append(grouped, groupedChunk)

		i = j
	}

	return grouped
}

// Helper functions
func (c *CodeChunker) getOverlapLines(lines []string, overlapSize int) []string {
	if len(lines) == 0 {
		return []string{}
	}

	// Calculate overlap in lines (approximate)
	overlapLines := overlapSize / 10 // Rough estimate: 10 tokens per line
	if overlapLines < 1 {
		overlapLines = 1
	}
	if overlapLines > len(lines) {
		overlapLines = len(lines)
	}

	return lines[len(lines)-overlapLines:]
}

func (c *CodeChunker) calculateLinesSize(lines []string) int {
	total := 0
	for _, line := range lines {
		total += countTokensApproximate(line)
	}
	return total
}
