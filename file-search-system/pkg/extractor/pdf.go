package extractor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PDFExtractor handles PDF files using pdftotext utility
type PDFExtractor struct {
	config *Config
}

// NewPDFExtractor creates a new PDF extractor
func NewPDFExtractor(config *Config) *PDFExtractor {
	if config == nil {
		config = DefaultConfig()
	}
	return &PDFExtractor{config: config}
}

// CanExtract checks if this extractor can handle the file
func (e *PDFExtractor) CanExtract(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".pdf"
}

// Extract extracts content from a PDF file
func (e *PDFExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access PDF file: %w", err)
	}

	// Check file size
	maxSize := int64(e.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		return nil, fmt.Errorf("PDF file too large: %d bytes (max: %d bytes)", info.Size(), maxSize)
	}

	// Try to extract text using pdftotext first
	text, err := e.extractWithPDFToText(ctx, filePath)
	if err != nil {
		// Fallback to basic PDF extraction (placeholder for now)
		return e.extractBasic(filePath, info)
	}

	// Parse pages if we have structured content
	pages := e.parsePages(text)

	// Create metadata
	metadata := map[string]interface{}{
		"file_size":    info.Size(),
		"page_count":   len(pages),
		"char_count":   len(text),
		"file_type":    "pdf",
		"extractor":    "pdftotext",
		"extracted_at": time.Now(),
	}

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
		Pages:    pages,
	}, nil
}

// GetName returns the extractor name
func (e *PDFExtractor) GetName() string {
	return "PDFExtractor"
}

// GetSupportedExtensions returns supported extensions
func (e *PDFExtractor) GetSupportedExtensions() []string {
	return []string{".pdf"}
}

// extractWithPDFToText extracts text using the pdftotext command-line utility
func (e *PDFExtractor) extractWithPDFToText(ctx context.Context, filePath string) (string, error) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(e.config.Timeout)*time.Second)
	defer cancel()

	// Run pdftotext command
	cmd := exec.CommandContext(timeoutCtx, "pdftotext", "-layout", filePath, "-")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext extraction failed: %w", err)
	}

	text := string(output)

	// Basic cleanup
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	return strings.TrimSpace(text), nil
}

// extractBasic provides basic PDF extraction without external tools
func (e *PDFExtractor) extractBasic(filePath string, info os.FileInfo) (*ExtractedContent, error) {
	// For now, return minimal metadata without text content
	// This could be extended with a pure Go PDF library in the future
	metadata := map[string]interface{}{
		"file_size":    info.Size(),
		"file_type":    "pdf",
		"extractor":    "basic",
		"extracted_at": time.Now(),
		"note":         "PDF text extraction requires pdftotext utility",
	}

	return &ExtractedContent{
		Text:     "", // Empty text since we can't extract without tools
		Metadata: metadata,
	}, nil
}

// parsePages attempts to split PDF text into pages
func (e *PDFExtractor) parsePages(text string) []PageContent {
	if text == "" {
		return nil
	}

	var pages []PageContent

	// Split on form feed characters (often used as page separators in pdftotext output)
	pageTexts := strings.Split(text, "\f")

	for i, pageText := range pageTexts {
		pageText = strings.TrimSpace(pageText)
		if pageText != "" {
			pages = append(pages, PageContent{
				Number: i + 1,
				Text:   pageText,
			})
		}
	}

	// If no form feed characters found, treat entire text as single page
	if len(pages) == 0 && text != "" {
		pages = append(pages, PageContent{
			Number: 1,
			Text:   text,
		})
	}

	return pages
}
