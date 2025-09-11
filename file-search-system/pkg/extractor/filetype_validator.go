package extractor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
)

// FileTypeValidator validates file types based on content rather than extension
type FileTypeValidator struct {
	log *logrus.Logger
}

// NewFileTypeValidator creates a new file type validator
func NewFileTypeValidator(log *logrus.Logger) *FileTypeValidator {
	return &FileTypeValidator{
		log: log,
	}
}

// ValidatedFileType represents the result of file type validation
type ValidatedFileType struct {
	Extension     string // Original file extension
	DetectedType  string // Type detected by content analysis
	DetectedMIME  string // MIME type detected
	IsValid       bool   // Whether extension matches content
	ActualType    string // The type to use for processing (may differ from extension)
}

// ValidateFile detects the actual file type based on content and compares with extension
func (v *FileTypeValidator) ValidateFile(filePath string) (*ValidatedFileType, error) {
	// Get the file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Detect actual MIME type from content
	mtype, err := mimetype.DetectFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}
	
	result := &ValidatedFileType{
		Extension:    ext,
		DetectedType: strings.TrimPrefix(mtype.Extension(), "."),
		DetectedMIME: mtype.String(),
		IsValid:      true, // Assume valid unless proven otherwise
	}
	
	// Special handling for email and markup formats
	switch ext {
	case ".eml", ".msg":
		// Email formats - often detected as text/plain or application/octet-stream
		if mtype.Is("text/plain") || mtype.Is("application/octet-stream") {
			v.log.WithFields(logrus.Fields{
				"file":          filePath,
				"extension":     ext,
				"detected_mime": mtype.String(),
			}).Debug("Email format detected as text/plain or octet-stream, trusting extension")
			result.ActualType = strings.TrimPrefix(ext, ".")
			return result, nil
		}
	case ".rst", ".org":
		// Markup formats - usually detected as text/plain
		if mtype.Is("text/plain") {
			v.log.WithFields(logrus.Fields{
				"file":          filePath,
				"extension":     ext,
				"detected_mime": mtype.String(),
			}).Debug("Markup format detected as text/plain, trusting extension")
			result.ActualType = strings.TrimPrefix(ext, ".")
			return result, nil
		}
	}
	
	// Check if the detected type matches the extension
	expectedMIME := v.getMIMEForExtension(ext)
	
	// Compare detected type with expected type
	if expectedMIME != "" && !mtype.Is(expectedMIME) {
		// Mismatch detected
		result.IsValid = false
		
		// Log warning about mismatch
		v.log.WithFields(logrus.Fields{
			"file":          filePath,
			"extension":     ext,
			"expected_type": strings.TrimPrefix(ext, "."),
			"detected_type": result.DetectedType,
			"detected_mime": result.DetectedMIME,
		}).Warn("File extension does not match detected content type")
	}
	
	// Always use the detected type for processing
	result.ActualType = result.DetectedType
	
	// Handle cases where mimetype doesn't provide an extension
	if result.ActualType == "" {
		// Check if this might be a temporary/lock file
		baseName := filepath.Base(filePath)
		if strings.HasPrefix(baseName, "~$") || strings.HasPrefix(baseName, ".~") {
			v.log.WithFields(logrus.Fields{
				"file":          filePath,
				"extension":     ext,
				"detected_mime": result.DetectedMIME,
			}).Debug("Detected temporary/lock file")
			// Mark as invalid for temporary files
			result.IsValid = false
			result.ActualType = "temp_" + strings.TrimPrefix(ext, ".")
		} else if result.DetectedMIME == "application/octet-stream" {
			// Generic binary file - use extension but mark as uncertain
			result.ActualType = strings.TrimPrefix(ext, ".")
			v.log.WithFields(logrus.Fields{
				"file":          filePath,
				"extension":     ext,
				"detected_mime": result.DetectedMIME,
			}).Debug("Generic binary file detected, using extension")
		} else {
			// Fall back to extension-based type
			result.ActualType = strings.TrimPrefix(ext, ".")
			v.log.WithFields(logrus.Fields{
				"file":          filePath,
				"extension":     ext,
				"detected_mime": result.DetectedMIME,
			}).Debug("No extension from mimetype, using file extension")
		}
	}
	
	return result, nil
}

// getMIMEForExtension returns the expected MIME type for common extensions
func (v *FileTypeValidator) getMIMEForExtension(ext string) string {
	// Map of extensions to their expected MIME types
	mimeMap := map[string]string{
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".doc":  "application/msword",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".xls":  "application/vnd.ms-excel",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".ppt":  "application/vnd.ms-powerpoint",
		".html": "text/html",
		".htm":  "text/html",
		".xml":  "text/xml",
		".csv":  "text/csv",
		".tsv":  "text/tab-separated-values",
		".txt":  "text/plain",
		".rtf":  "text/rtf",
		".odt":  "application/vnd.oasis.opendocument.text",
		".epub": "application/epub+zip",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".zip":  "application/zip",
		".json": "application/json",
		".md":   "text/markdown",
	}
	
	return mimeMap[ext]
}

// GetProcessingType determines the file type to use for processing
func (v *FileTypeValidator) GetProcessingType(filePath string) (string, error) {
	validation, err := v.ValidateFile(filePath)
	if err != nil {
		return "", err
	}
	
	// Return the actual type for processing
	return validation.ActualType, nil
}

// MapToDBFileType maps detected file types to allowed database file_type values
func (v *FileTypeValidator) MapToDBFileType(detectedType string) string {
	// Handle temporary files
	if strings.HasPrefix(detectedType, "temp_") {
		// Extract the actual type after "temp_"
		actualType := strings.TrimPrefix(detectedType, "temp_")
		return v.MapToDBFileType(actualType)
	}
	
	// Map of detected types to database-allowed types
	typeMap := map[string]string{
		// Document types
		"pdf":  "pdf",
		"doc":  "doc",
		"docx": "docx",
		"rtf":  "rtf",
		"odt":  "odt",
		
		// Spreadsheet types
		"xls":  "xls",
		"xlsx": "xlsx",
		"csv":  "csv",
		"ods":  "ods",
		
		// Code types
		"py":     "python",
		"python": "python",
		"js":     "javascript",
		"javascript": "javascript",
		"ts":     "typescript",
		"typescript": "typescript",
		"java":   "java",
		"cpp":    "cpp",
		"cc":     "cpp",
		"cxx":    "cpp",
		"c":      "c",
		"go":     "go",
		"rs":     "rust",
		"rust":   "rust",
		"json":   "json",
		"yaml":   "yaml",
		"yml":    "yaml",
		
		// Text types
		"txt":  "text",
		"text": "text",
		"md":   "markdown",
		"markdown": "markdown",
		
		// Image types
		"png":  "image",
		"jpg":  "image",
		"jpeg": "image",
		"gif":  "image",
		"bmp":  "image",
		"tiff": "image",
		"tif":  "image",
		"svg":  "image",
		"webp": "image",
		
		// Generic categories for unrecognized types
		"zip":  "document", // ZIP files often contain documents
		"html": "code",
		"htm":  "code",
		"xml":  "code",
		"css":  "code",
		"sh":   "code",
		"bash": "code",
		"zsh":  "code",
		"fish": "code",
		
		// Email types
		"eml": "document",
		"msg": "document",
		
		// Markup types
		"rst": "text",
		"org": "text",
		"adoc": "text",
		"asciidoc": "text",
	}
	
	// Check if we have a mapping
	if dbType, ok := typeMap[strings.ToLower(detectedType)]; ok {
		return dbType
	}
	
	// Default fallbacks based on patterns
	detectedLower := strings.ToLower(detectedType)
	
	// Programming languages default to "code"
	if strings.Contains(detectedLower, "script") || 
	   strings.Contains(detectedLower, "source") ||
	   strings.HasSuffix(detectedLower, "ml") { // xml, html, etc.
		return "code"
	}
	
	// Images
	if strings.Contains(detectedLower, "image") ||
	   strings.Contains(detectedLower, "photo") ||
	   strings.Contains(detectedLower, "picture") {
		return "image"
	}
	
	// Documents
	if strings.Contains(detectedLower, "document") ||
	   strings.Contains(detectedLower, "text") ||
	   strings.Contains(detectedLower, "word") {
		return "document"
	}
	
	// Spreadsheets
	if strings.Contains(detectedLower, "sheet") ||
	   strings.Contains(detectedLower, "excel") ||
	   strings.Contains(detectedLower, "calc") {
		return "spreadsheet"
	}
	
	// Default to "document" for unknown types
	v.log.WithFields(logrus.Fields{
		"detected_type": detectedType,
		"mapped_to":     "document",
	}).Debug("Unknown file type mapped to 'document'")
	
	return "document"
}