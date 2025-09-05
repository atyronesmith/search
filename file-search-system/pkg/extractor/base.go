package extractor

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedContent represents content extracted from a file
type ExtractedContent struct {
	Text     string            `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
	Pages    []PageContent     `json:"pages,omitempty"`
	Sections []SectionContent  `json:"sections,omitempty"`
}

// PageContent represents content from a specific page (for PDFs)
type PageContent struct {
	Number  int    `json:"number"`
	Text    string `json:"text"`
	Objects []DocumentObject `json:"objects,omitempty"`
}

// SectionContent represents a section of structured content
type SectionContent struct {
	Type     string `json:"type"` // "heading", "paragraph", "list", "table", "code"
	Level    int    `json:"level,omitempty"`
	Text     string `json:"text"`
	Language string `json:"language,omitempty"` // For code sections
}

// DocumentObject represents an object in a document (table, image, etc.)
type DocumentObject struct {
	Type     string                 `json:"type"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Extractor interface for content extraction
type Extractor interface {
	// CanExtract returns true if this extractor can handle the file
	CanExtract(filePath string) bool
	
	// Extract extracts content from the file
	Extract(ctx context.Context, filePath string) (*ExtractedContent, error)
	
	// GetName returns the name of the extractor
	GetName() string
	
	// GetSupportedExtensions returns supported file extensions
	GetSupportedExtensions() []string
}

// ExtractorManager manages multiple extractors
type ExtractorManager struct {
	extractors []Extractor
}

// NewExtractorManager creates a new extractor manager
func NewExtractorManager() *ExtractorManager {
	return &ExtractorManager{
		extractors: make([]Extractor, 0),
	}
}

// AddExtractor adds an extractor to the manager
func (em *ExtractorManager) AddExtractor(extractor Extractor) {
	em.extractors = append(em.extractors, extractor)
}

// Extract finds an appropriate extractor and extracts content
func (em *ExtractorManager) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	for _, extractor := range em.extractors {
		if extractor.CanExtract(filePath) {
			return extractor.Extract(ctx, filePath)
		}
	}
	
	return nil, fmt.Errorf("no extractor available for file: %s", filePath)
}

// GetExtractorForFile returns the extractor that can handle a file
func (em *ExtractorManager) GetExtractorForFile(filePath string) Extractor {
	for _, extractor := range em.extractors {
		if extractor.CanExtract(filePath) {
			return extractor
		}
	}
	return nil
}

// ListExtractors returns all available extractors
func (em *ExtractorManager) ListExtractors() []Extractor {
	return em.extractors
}

// GetSupportedExtensions returns all supported file extensions
func (em *ExtractorManager) GetSupportedExtensions() []string {
	extensionSet := make(map[string]bool)
	
	for _, extractor := range em.extractors {
		for _, ext := range extractor.GetSupportedExtensions() {
			extensionSet[strings.ToLower(ext)] = true
		}
	}
	
	extensions := make([]string, 0, len(extensionSet))
	for ext := range extensionSet {
		extensions = append(extensions, ext)
	}
	
	return extensions
}

// ExtractorConfig holds configuration for extractors
type ExtractorConfig struct {
	MaxFileSizeMB   int               `json:"max_file_size_mb"`
	Timeout         int               `json:"timeout_seconds"`
	TempDir         string            `json:"temp_dir"`
	ExternalTools   map[string]string `json:"external_tools,omitempty"`
}

// DefaultConfig returns default extractor configuration
func DefaultConfig() *ExtractorConfig {
	return &ExtractorConfig{
		MaxFileSizeMB: 100,
		Timeout:       60,
		TempDir:       "/tmp",
		ExternalTools: make(map[string]string),
	}
}