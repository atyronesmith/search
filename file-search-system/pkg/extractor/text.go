package extractor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	ErrFileTooLarge = errors.New("file too large to process")
)

// TextExtractor handles plain text files
type TextExtractor struct {
	config *ExtractorConfig
}

// NewTextExtractor creates a new text extractor
func NewTextExtractor(config *ExtractorConfig) *TextExtractor {
	if config == nil {
		config = DefaultConfig()
	}
	return &TextExtractor{config: config}
}

// CanExtract checks if this extractor can handle the file
func (e *TextExtractor) CanExtract(filePath string) bool {
	// First check if it's a known binary file
	if IsBinaryFile(filePath) {
		return false
	}
	
	// Check if it's a known text file
	if IsTextFile(filePath) {
		return true
	}
	
	// For unknown extensions, check encoding
	return HasValidEncoding(filePath)
}

// Extract extracts content from a text file
func (e *TextExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Check file size
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	maxSize := int64(e.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		return nil, ErrFileTooLarge
	}

	// Check if file is empty
	if info.Size() == 0 {
		return &ExtractedContent{
			Text: "",
			Metadata: map[string]interface{}{
				"file_size":  0,
				"encoding":   "utf-8",
				"line_count": 0,
				"char_count": 0,
				"file_type":  "empty",
			},
		}, nil
	}

	// Detect encoding and read content
	content, encoding, err := e.readWithEncoding(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file with proper encoding: %w", err)
	}

	// Additional validation for UTF-8 content
	if !utf8.ValidString(content) {
		return nil, fmt.Errorf("file contains invalid UTF-8 sequences")
	}

	// Create metadata
	metadata := map[string]interface{}{
		"file_size":    info.Size(),
		"encoding":     encoding,
		"line_count":   countLines(content),
		"char_count":   len(content),
		"file_type":    e.detectTextType(filePath, content),
	}

	// Parse structured content based on file type
	sections := e.parseStructuredContent(filePath, content)

	return &ExtractedContent{
		Text:     content,
		Metadata: metadata,
		Sections: sections,
	}, nil
}

// GetName returns the extractor name
func (e *TextExtractor) GetName() string {
	return "TextExtractor"
}

// GetSupportedExtensions returns supported extensions
func (e *TextExtractor) GetSupportedExtensions() []string {
	return []string{".txt", ".md", ".rtf", ".csv", ".tsv", ".log", ".conf", ".cfg", ".ini", ".env"}
}

// readWithEncoding reads file content with encoding detection
func (e *TextExtractor) readWithEncoding(file *os.File) (string, string, error) {
	// Read first 1024 bytes for encoding detection
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", "", err
	}

	// Reset file position
	file.Seek(0, 0)

	// Detect encoding
	encoding := "utf-8"
	if !utf8.Valid(buf[:n]) {
		// Try common encodings
		encodings := []struct {
			name string
			dec  transform.Transformer
		}{
			{"utf-16", unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()},
			{"windows-1252", charmap.Windows1252.NewDecoder()},
			{"iso-8859-1", charmap.ISO8859_1.NewDecoder()},
		}

		for _, enc := range encodings {
			file.Seek(0, 0)
			reader := transform.NewReader(file, enc.dec)
			decoded := make([]byte, 1024)
			if n, err := reader.Read(decoded); err == nil && utf8.Valid(decoded[:n]) {
				encoding = enc.name
				file.Seek(0, 0)
				reader = transform.NewReader(file, enc.dec)
				content, err := io.ReadAll(reader)
				return string(content), encoding, err
			}
		}
	}

	// Default to UTF-8
	file.Seek(0, 0)
	content, err := io.ReadAll(file)
	return string(content), encoding, err
}

// detectTextType detects the type of text file
func (e *TextExtractor) detectTextType(filePath, content string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".md":
		return "markdown"
	case ".csv", ".tsv":
		return "tabular"
	case ".log":
		return "log"
	case ".conf", ".cfg", ".ini":
		return "configuration"
	case ".env":
		return "environment"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "plain_text"
	}
}

// parseStructuredContent parses content into sections based on file type
func (e *TextExtractor) parseStructuredContent(filePath, content string) []SectionContent {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".md":
		return e.parseMarkdown(content)
	case ".csv", ".tsv":
		return e.parseTabular(content, ext)
	default:
		return e.parsePlainText(content)
	}
}

// parseMarkdown parses markdown content into sections
func (e *TextExtractor) parseMarkdown(content string) []SectionContent {
	var sections []SectionContent
	lines := strings.Split(content, "\n")
	
	var currentSection SectionContent
	var sectionLines []string
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Check for headings
		if strings.HasPrefix(trimmedLine, "#") {
			// Save previous section
			if currentSection.Type != "" {
				currentSection.Text = strings.Join(sectionLines, "\n")
				sections = append(sections, currentSection)
			}
			
			// Start new section
			level := 0
			for i, r := range trimmedLine {
				if r == '#' {
					level++
				} else {
					currentSection = SectionContent{
						Type:  "heading",
						Level: level,
						Text:  strings.TrimSpace(trimmedLine[i:]),
					}
					break
				}
			}
			sectionLines = []string{}
		} else if strings.HasPrefix(trimmedLine, "```") {
			// Code block
			if currentSection.Type == "code" {
				// End code block
				currentSection.Text = strings.Join(sectionLines, "\n")
				sections = append(sections, currentSection)
				currentSection = SectionContent{Type: "paragraph"}
				sectionLines = []string{}
			} else {
				// Start code block
				if currentSection.Type != "" {
					currentSection.Text = strings.Join(sectionLines, "\n")
					sections = append(sections, currentSection)
				}
				
				language := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "```"))
				currentSection = SectionContent{
					Type:     "code",
					Language: language,
				}
				sectionLines = []string{}
			}
		} else {
			sectionLines = append(sectionLines, line)
		}
	}
	
	// Save last section
	if currentSection.Type != "" || len(sectionLines) > 0 {
		if currentSection.Type == "" {
			currentSection.Type = "paragraph"
		}
		currentSection.Text = strings.Join(sectionLines, "\n")
		sections = append(sections, currentSection)
	}
	
	return sections
}

// parseTabular parses CSV/TSV content
func (e *TextExtractor) parseTabular(content, ext string) []SectionContent {
	// TODO: Implement proper CSV/TSV parsing with separator
	
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	
	// First line as header
	header := SectionContent{
		Type: "table_header",
		Text: lines[0],
	}
	
	// Rest as table body
	body := SectionContent{
		Type: "table",
		Text: strings.Join(lines[1:], "\n"),
	}
	
	return []SectionContent{header, body}
}

// parsePlainText parses plain text into paragraphs
func (e *TextExtractor) parsePlainText(content string) []SectionContent {
	paragraphs := strings.Split(content, "\n\n")
	sections := make([]SectionContent, 0, len(paragraphs))
	
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) != "" {
			sections = append(sections, SectionContent{
				Type: "paragraph",
				Text: paragraph,
			})
		}
	}
	
	return sections
}

// countLines counts the number of lines in content
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}