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

	"github.com/sirupsen/logrus"
)

// UnstructuredExtractor handles extraction using the Unstructured service
type UnstructuredExtractor struct {
	apiURL         string
	timeout        time.Duration
	log            *logrus.Logger
	royalProcessor *RoyalDocumentProcessor
	httpClient     *http.Client
}

// UnstructuredConfig represents configuration for the Unstructured service
type UnstructuredConfig struct {
	APIURL     string
	PythonPath string // Deprecated - kept for backward compatibility
	VenvPath   string // Deprecated - kept for backward compatibility
	Timeout    time.Duration
}

// UnstructuredResponse represents a response from the Unstructured service
type UnstructuredResponse struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Error    string                 `json:"error,omitempty"`
}

// NewUnstructuredExtractor creates a new Unstructured extractor
func NewUnstructuredExtractor(config UnstructuredConfig, logger *logrus.Logger) *UnstructuredExtractor {
	if config.Timeout == 0 {
		config.Timeout = 300 * time.Second // Default to 5 minutes for large files
	}

	// Default to local container API if not specified
	if config.APIURL == "" {
		// Check if UNSTRUCTURED_API_URL env var is set
		if apiURL := os.Getenv("UNSTRUCTURED_API_URL"); apiURL != "" {
			config.APIURL = apiURL
		} else {
			config.APIURL = "http://localhost:8001"
		}
	}

	return &UnstructuredExtractor{
		apiURL:         config.APIURL,
		timeout:        config.Timeout,
		log:            logger,
		royalProcessor: nil, // Disabled - using Unstructured API metadata directly
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CanExtract checks if the file can be extracted by this extractor
func (e *UnstructuredExtractor) CanExtract(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return e.isSupported(ext)
}

// Extract extracts text content from the file
func (e *UnstructuredExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	startTime := time.Now()

	e.log.WithFields(logrus.Fields{
		"file":      filePath,
		"extractor": "unstructured",
	}).Debug("Starting extraction")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Get file extension to check if supported
	ext := strings.ToLower(filepath.Ext(filePath))
	if !e.isSupported(ext) {
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	// Create a timeout context to ensure the extraction doesn't hang
	extractCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Use HTTP API to extract content
	response, err := e.extractViaAPI(extractCtx, filePath)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %v", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("extraction error: %s", response.Error)
	}

	duration := time.Since(startTime)
	e.log.WithFields(logrus.Fields{
		"file":               filePath,
		"duration":           duration,
		"size":               len(response.Content),
		"has_api_metadata":   len(response.Metadata) > 0,
		"api_metadata_count": len(response.Metadata),
	}).Debug("Extraction completed")

	// Try royal metadata extraction for supported files
	var royalMetadata map[string]interface{}
	if e.royalProcessor != nil {
		if metadata, content, err := e.royalProcessor.ExtractRoyalMetadata(ctx, filePath); err == nil {
			royalMetadata = FlattenRoyalMetadata(metadata)
			e.log.WithField("file", filePath).Info("Royal metadata extracted successfully")

			// Use royal content if available and longer
			if len(content.Text) > len(response.Content) {
				response.Content = content.Text
				e.log.WithField("file", filePath).Debug("Using royal processor content")
			}
		} else {
			e.log.WithError(err).WithField("file", filePath).Debug("Royal metadata extraction failed, using fallback")
		}
	}

	// Merge regular and royal metadata
	finalMetadata := response.Metadata
	if royalMetadata != nil {
		if finalMetadata == nil {
			finalMetadata = make(map[string]interface{})
		}
		// Add royal metadata with prefix to avoid conflicts
		for key, value := range royalMetadata {
			finalMetadata[key] = value
		}
		finalMetadata["royal_extraction"] = true
	}

	e.log.WithFields(logrus.Fields{
		"file":                 filePath,
		"has_final_metadata":   len(finalMetadata) > 0,
		"final_metadata_count": len(finalMetadata),
		"has_royal":            royalMetadata != nil,
	}).Debug("Returning extracted content with metadata")

	return &ExtractedContent{
		Text:     response.Content,
		Metadata: finalMetadata,
	}, nil
}

func (e *UnstructuredExtractor) isSupported(ext string) bool {
	// Handle all formats that Unstructured.io supports
	supportedTypes := map[string]bool{
		// Document Formats
		".pdf":  true, // PDF documents
		".docx": true, // Microsoft Word
		".doc":  true, // Microsoft Word (older)
		".pptx": true, // PowerPoint
		".ppt":  true, // PowerPoint (older)
		".html": true, // HTML files
		".htm":  true, // HTML files
		".xml":  true, // XML files
		".md":   true, // Markdown
		".rtf":  true, // Rich Text Format
		".odt":  true, // OpenDocument Text
		".epub": true, // EPUB books
		".org":  true, // Org-mode files
		".rst":  true, // reStructuredText

		// Spreadsheet & Data Formats
		".xlsx": true, // Excel
		".xls":  true, // Excel (older)
		".csv":  true, // CSV files
		".tsv":  true, // TSV files
		// ".json": false, // JSON files - removed, handled by text/code extractors

		// Email Formats
		".eml": true, // Email files
		".msg": true, // Outlook messages

		// Image Formats (with OCR)
		".png":  true, // PNG images
		".jpg":  true, // JPEG images
		".jpeg": true, // JPEG images
		".tiff": true, // TIFF images
		".tif":  true, // TIFF images
		".bmp":  true, // BMP images
		".heic": true, // HEIC images

		// Plain Text
		".txt": true, // Text files
	}

	// Check both lowercase and uppercase extensions
	return supportedTypes[strings.ToLower(ext)]
}

// extractViaAPI extracts text content using the Unstructured HTTP API
func (e *UnstructuredExtractor) extractViaAPI(ctx context.Context, filePath string) (*UnstructuredResponse, error) {
	// Check context before starting
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled before extraction: %v", ctx.Err())
	}

	// Read the file with size check
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %v", err)
	}

	// Skip very large files that might cause issues
	maxSize := int64(100 * 1024 * 1024) // 100MB limit for Unstructured
	if fileInfo.Size() > maxSize {
		return nil, fmt.Errorf("file too large for Unstructured extraction: %d bytes (max: %d)", fileInfo.Size(), maxSize)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %v", err)
	}

	// Add parameters for maximum metadata extraction
	if err := writer.WriteField("strategy", "hi_res"); err != nil { // High resolution for maximum detail
		return nil, fmt.Errorf("failed to write strategy field: %v", err)
	}
	if err := writer.WriteField("include_metadata", "true"); err != nil {
		return nil, fmt.Errorf("failed to write include_metadata field: %v", err)
	}
	// coordinates parameter removed - causes conflicts with internal implementation
	if err := writer.WriteField("extract_images", "false"); err != nil { // Disabled to avoid errors
		return nil, fmt.Errorf("failed to write extract_images field: %v", err)
	}
	if err := writer.WriteField("infer_table_structure", "true"); err != nil {
		return nil, fmt.Errorf("failed to write infer_table_structure field: %v", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL+"/extract", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request with timeout handling
	type result struct {
		resp *http.Response
		err  error
	}
	respChan := make(chan result, 1)

	go func() {
		resp, err := e.httpClient.Do(req)
		respChan <- result{resp, err}
	}()

	var resp *http.Response
	select {
	case <-ctx.Done():
		// Context cancelled or timed out
		e.log.WithFields(logrus.Fields{
			"file":    filePath,
			"timeout": e.timeout,
		}).Warn("Unstructured extraction timed out")
		return nil, fmt.Errorf("extraction timed out after %v", e.timeout)
	case res := <-respChan:
		if res.err != nil {
			return nil, fmt.Errorf("failed to send request: %v", res.err)
		}
		resp = res.resp
	}
	defer resp.Body.Close()

	// Check HTTP status before reading body
	if resp.StatusCode != http.StatusOK {
		// Try to read error message from body
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errorBody))
	}

	// Read response with size limit to prevent memory issues
	maxResponseSize := int64(50 * 1024 * 1024) // 50MB max response
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var apiResponse struct {
		Success  bool                   `json:"success"`
		Content  string                 `json:"content"`
		Metadata map[string]interface{} `json:"metadata"`
		Error    string                 `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v, body: %s", err, string(body))
	}

	if !apiResponse.Success {
		return nil, fmt.Errorf("API extraction failed: %s", apiResponse.Error)
	}

	return &UnstructuredResponse{
		Content:  apiResponse.Content,
		Metadata: apiResponse.Metadata,
	}, nil
}

// GetSupportedExtensions returns the list of supported file extensions
func (e *UnstructuredExtractor) GetSupportedExtensions() []string {
	return []string{
		// Document Formats
		".pdf", ".docx", ".doc", ".pptx", ".ppt", ".html", ".htm",
		".xml", ".md", ".rtf", ".odt", ".epub", ".org", ".rst",
		// Spreadsheet & Data Formats
		".xlsx", ".xls", ".csv", ".tsv", // Removed .json - handled by text/code extractors
		// Email Formats
		".eml", ".msg",
		// Image Formats (with OCR)
		".png", ".jpg", ".jpeg", ".tiff", ".tif", ".bmp", ".heic",
		// Plain Text
		".txt",
	}
}

// GetName returns the name of the extractor
func (e *UnstructuredExtractor) GetName() string {
	return "UnstructuredExtractor"
}
