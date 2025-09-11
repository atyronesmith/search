package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// DoclingClient handles communication with the docling service
type DoclingClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	enabled    bool
}

// DoclingConfig holds configuration for the docling client
type DoclingConfig struct {
	ServiceURL string
	Timeout    time.Duration
	Enabled    bool
}

// DoclingElement represents a document element from the docling service
type DoclingElement struct {
	ElementType   string                 `json:"element_type"`
	Content       string                 `json:"content"`
	PageNumber    int                    `json:"page_number"`
	StructureData map[string]interface{} `json:"structure_data,omitempty"`
	BoundingBox   *BoundingBox           `json:"bbox,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// BoundingBox represents element positioning
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DoclingResult represents the response from docling service
type DoclingResult struct {
	Success          bool                   `json:"success"`
	Elements         []DoclingElement       `json:"elements"`
	Metadata         map[string]interface{} `json:"metadata"`
	ProcessingTime   float64                `json:"processing_time"`
	ErrorMessage     *string                `json:"error_message,omitempty"`
	ExtractionMethod string                 `json:"extraction_method"`
}

// NewDoclingClient creates a new docling client
func NewDoclingClient(config *DoclingConfig) *DoclingClient {
	if config == nil {
		config = &DoclingConfig{
			ServiceURL: "http://localhost:8081",
			Timeout:    30 * time.Second,
			Enabled:    false,
		}
	}

	return &DoclingClient{
		baseURL: config.ServiceURL,
		timeout: config.Timeout,
		enabled: config.Enabled,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// IsAvailable checks if the docling service is available
func (c *DoclingClient) IsAvailable(ctx context.Context) bool {
	if !c.enabled {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ExtractFromFile extracts content from a file using the docling service
func (c *DoclingClient) ExtractFromFile(ctx context.Context, filePath string, method string) (*DoclingResult, error) {
	if !c.enabled {
		return nil, fmt.Errorf("docling service is disabled")
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add extraction method parameter
	if method != "" {
		err = writer.WriteField("extraction_method", method)
		if err != nil {
			return nil, fmt.Errorf("failed to write extraction method field: %w", err)
		}
	}

	writer.Close()

	// Create HTTP request
	url := c.baseURL + "/extract"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docling service request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docling service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result DoclingResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse docling response: %w", err)
	}

	return &result, nil
}

// ExtractFromPath extracts content from a file path using the docling service
func (c *DoclingClient) ExtractFromPath(ctx context.Context, filePath string, method string, options map[string]interface{}) (*DoclingResult, error) {
	if !c.enabled {
		return nil, fmt.Errorf("docling service is disabled")
	}

	// Create request payload
	payload := map[string]interface{}{
		"file_path":         filePath,
		"extraction_method": method,
		"options":           options,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/extract/path"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docling service request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docling service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result DoclingResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse docling response: %w", err)
	}

	return &result, nil
}

// convertDoclingResult converts DoclingResult to ExtractedContent
func (c *DoclingClient) convertDoclingResult(result *DoclingResult, filePath string) *ExtractedContent {
	var fullText strings.Builder
	var pages []PageContent
	pageMap := make(map[int]*PageContent)

	// Process elements
	for _, element := range result.Elements {
		// Add to full text
		if fullText.Len() > 0 {
			fullText.WriteString("\n")
		}
		fullText.WriteString(element.Content)

		// Group by page
		pageNum := element.PageNumber
		if pageNum == 0 {
			pageNum = 1 // Default to page 1 if not specified
		}

		if _, exists := pageMap[pageNum]; !exists {
			pageMap[pageNum] = &PageContent{
				Number: pageNum,
				Text:   "",
			}
		}

		if pageMap[pageNum].Text != "" {
			pageMap[pageNum].Text += "\n"
		}
		pageMap[pageNum].Text += element.Content
	}

	// Convert page map to slice
	for i := 1; i <= len(pageMap); i++ {
		if page, exists := pageMap[i]; exists {
			pages = append(pages, *page)
		}
	}

	// Create metadata
	metadata := map[string]interface{}{
		"extractor":         "docling",
		"extraction_method": result.ExtractionMethod,
		"processing_time":   result.ProcessingTime,
		"element_count":     len(result.Elements),
		"page_count":        len(pages),
		"char_count":        fullText.Len(),
		"file_type":         getFileType(filePath),
		"extracted_at":      time.Now(),
	}

	// Merge service metadata
	for key, value := range result.Metadata {
		metadata[key] = value
	}

	return &ExtractedContent{
		Text:     fullText.String(),
		Metadata: metadata,
		Pages:    pages,
	}
}

// getFileType determines file type from extension
func getFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".pptx":
		return "pptx"
	default:
		return "unknown"
	}
}

// EnhancedPDFExtractor provides enhanced PDF extraction with Docling integration
type EnhancedPDFExtractor struct {
	*PDFExtractor
	doclingClient   *DoclingClient
	fallbackEnabled bool
}

// NewEnhancedPDFExtractor creates a new enhanced PDF extractor with docling integration
func NewEnhancedPDFExtractor(config *Config, doclingConfig *DoclingConfig) *EnhancedPDFExtractor {
	return &EnhancedPDFExtractor{
		PDFExtractor:    NewPDFExtractor(config),
		doclingClient:   NewDoclingClient(doclingConfig),
		fallbackEnabled: true,
	}
}

// Extract extracts content from PDF using docling service with fallback
func (e *EnhancedPDFExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	// Check if file exists and is valid
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access PDF file: %w", err)
	}

	// Check file size
	maxSize := int64(e.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		return nil, fmt.Errorf("PDF file too large: %d bytes (max: %d bytes)", info.Size(), maxSize)
	}

	// Try docling service first
	if e.doclingClient.IsAvailable(ctx) {
		result, err := e.doclingClient.ExtractFromFile(ctx, filePath, "auto")
		if err == nil && result.Success {
			// Convert docling result to ExtractedContent
			content := e.doclingClient.convertDoclingResult(result, filePath)
			content.Metadata["primary_extractor"] = "docling"
			return content, nil
		}

		// Log docling failure but continue with fallback
		if err != nil {
			fmt.Printf("Docling extraction failed: %v\n", err)
		}
	}

	// Fallback to original PDF extraction
	if e.fallbackEnabled {
		content, err := e.PDFExtractor.Extract(ctx, filePath)
		if err != nil {
			return nil, err
		}
		content.Metadata["primary_extractor"] = "fallback"
		content.Metadata["docling_attempted"] = true
		return content, nil
	}

	return nil, fmt.Errorf("docling service unavailable and fallback disabled")
}

// GetName returns the enhanced extractor name
func (e *EnhancedPDFExtractor) GetName() string {
	return "EnhancedPDFExtractor"
}

// GetSupportedExtensions returns supported file extensions for EnhancedPDFExtractor
func (e *EnhancedPDFExtractor) GetSupportedExtensions() []string {
	return []string{".pdf"}
}

// DoclingExtractor handles all file types supported by Docling service
type DoclingExtractor struct {
	config          *Config
	doclingClient   *DoclingClient
	fallbackEnabled bool
}

// NewDoclingExtractor creates a new comprehensive Docling extractor
func NewDoclingExtractor(config *Config, doclingConfig *DoclingConfig) *DoclingExtractor {
	if config == nil {
		config = DefaultConfig()
	}

	return &DoclingExtractor{
		config:          config,
		doclingClient:   NewDoclingClient(doclingConfig),
		fallbackEnabled: true,
	}
}

// CanExtract returns true if this extractor can handle the file type
func (e *DoclingExtractor) CanExtract(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	supportedExts := map[string]bool{
		".pdf":  true, // PDF documents
		".docx": true, // Microsoft Word documents
		".pptx": true, // PowerPoint presentations
		".html": true, // HTML documents
		".htm":  true, // HTML documents
		".png":  true, // Images with OCR
		".jpg":  true, // Images with OCR
		".jpeg": true, // Images with OCR
		".md":   true, // Markdown files
		".adoc": true, // AsciiDoc documents
	}

	return supportedExts[ext]
}

// Extract extracts content using Docling service with fallback for basic text
func (e *DoclingExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	// Check if file exists and is valid
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	// Check file size
	maxSize := int64(e.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d bytes)", info.Size(), maxSize)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// Try Docling service first for all supported formats
	if e.doclingClient.IsAvailable(ctx) {
		result, err := e.doclingClient.ExtractFromFile(ctx, filePath, "auto")
		if err == nil && result.Success {
			// Convert docling result to ExtractedContent
			content := e.doclingClient.convertDoclingResult(result, filePath)
			content.Metadata["primary_extractor"] = "docling"
			content.Metadata["file_type"] = e.getFileTypeDescription(ext)
			return content, nil
		}

		// Log docling failure but continue with fallback for certain types
		if err != nil {
			fmt.Printf("Docling extraction failed for %s: %v\n", filePath, err)
		}
	}

	// Fallback handling for specific file types
	if e.fallbackEnabled {
		switch ext {
		case ".md", ".html", ".htm":
			// For markdown and HTML, provide basic text extraction
			content, err := e.extractAsText(filePath)
			if err != nil {
				return nil, err
			}
			content.Metadata["primary_extractor"] = "fallback_text"
			content.Metadata["docling_attempted"] = true
			content.Metadata["file_type"] = e.getFileTypeDescription(ext)
			return content, nil
		case ".png", ".jpg", ".jpeg":
			// For images, we can't provide meaningful fallback without OCR
			return nil, fmt.Errorf("image extraction requires Docling service (OCR unavailable)")
		case ".docx", ".pptx":
			// For complex documents, we can't provide meaningful fallback
			return nil, fmt.Errorf("document extraction requires Docling service")
		case ".adoc":
			// For AsciiDoc, try basic text extraction
			content, err := e.extractAsText(filePath)
			if err != nil {
				return nil, err
			}
			content.Metadata["primary_extractor"] = "fallback_text"
			content.Metadata["docling_attempted"] = true
			content.Metadata["file_type"] = "asciidoc"
			return content, nil
		}
	}

	return nil, fmt.Errorf("Docling service unavailable and no fallback for file type: %s", ext)
}

// extractAsText provides basic text extraction for fallback scenarios
func (e *DoclingExtractor) extractAsText(filePath string) (*ExtractedContent, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)

	// Basic validation
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("file contains invalid UTF-8")
	}

	metadata := map[string]interface{}{
		"file_size":  len(content),
		"char_count": len(text),
		"line_count": strings.Count(text, "\n") + 1,
		"encoding":   "utf-8",
	}

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
	}, nil
}

// getFileTypeDescription returns a human-readable description of the file type
func (e *DoclingExtractor) getFileTypeDescription(ext string) string {
	switch ext {
	case ".pdf":
		return "PDF document"
	case ".docx":
		return "Microsoft Word document"
	case ".pptx":
		return "PowerPoint presentation"
	case ".html", ".htm":
		return "HTML document"
	case ".png":
		return "PNG image"
	case ".jpg", ".jpeg":
		return "JPEG image"
	case ".md":
		return "Markdown document"
	case ".adoc":
		return "AsciiDoc document"
	default:
		return "document"
	}
}

// GetName returns the extractor name
func (e *DoclingExtractor) GetName() string {
	return "DoclingExtractor"
}

// GetSupportedExtensions returns supported file extensions
func (e *DoclingExtractor) GetSupportedExtensions() []string {
	return []string{".pdf", ".docx", ".pptx", ".html", ".htm", ".png", ".jpg", ".jpeg", ".md", ".adoc"}
}
