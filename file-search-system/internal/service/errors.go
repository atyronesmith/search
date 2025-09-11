package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Common service errors
var (
	// ErrServiceNotStarted indicates the service is not running
	ErrServiceNotStarted = errors.New("service not started")
	
	// ErrServiceAlreadyStarted indicates the service is already running
	ErrServiceAlreadyStarted = errors.New("service already started")
	
	// ErrIndexingActive indicates indexing is already in progress
	ErrIndexingActive = errors.New("indexing already active")
	
	// ErrIndexingNotActive indicates indexing is not running
	ErrIndexingNotActive = errors.New("indexing not active")
	
	// ErrMonitoringActive indicates monitoring is already running
	ErrMonitoringActive = errors.New("monitoring already active")
	
	// ErrMonitoringNotActive indicates monitoring is not running
	ErrMonitoringNotActive = errors.New("monitoring not active")
	
	// ErrNoFilesToProcess indicates no pending files are available
	ErrNoFilesToProcess = errors.New("no files to process")
	
	// ErrContextCanceled indicates the operation was canceled
	ErrContextCanceled = errors.New("context canceled")
)

// FileProcessingError represents an error that occurred during file processing
type FileProcessingError struct {
	FilePath string
	Category string
	Err      error
}

// Error implements the error interface
func (e *FileProcessingError) Error() string {
	return fmt.Sprintf("processing %s failed (%s): %v", e.FilePath, e.Category, e.Err)
}

// Unwrap returns the underlying error
func (e *FileProcessingError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is temporary and can be retried
func (e *FileProcessingError) IsRetryable() bool {
	switch e.Category {
	case "permission_error", "file_not_found":
		return false // These won't change with retry
	case "database_error", "embedding_error", "network_error":
		return true // These might succeed on retry
	default:
		return false
	}
}

// ShouldSkip returns true if the file should be skipped rather than marked as failed
func (e *FileProcessingError) ShouldSkip() bool {
	switch e.Category {
	case "empty_content", "encoding_error", "unsupported_format",
	     "file_too_large", "permission_error", "file_not_found":
		return true
	default:
		return false
	}
}

// NewFileProcessingError creates a new file processing error with categorization
func NewFileProcessingError(filePath string, err error) *FileProcessingError {
	return &FileProcessingError{
		FilePath: filePath,
		Category: categorizeError(err),
		Err:      err,
	}
}

// categorizeError determines the error category based on the error message
func categorizeError(err error) string {
	if err == nil {
		return "unknown"
	}
	
	errStr := err.Error()
	
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case strings.Contains(errStr, "empty text provided for embedding"):
		return "empty_content"
	case strings.Contains(errStr, "file contains invalid UTF-8"),
	     strings.Contains(errStr, "invalid byte sequence for encoding"):
		return "encoding_error"
	case strings.Contains(errStr, "string is too long for tsvector"):
		return "content_too_large"
	case strings.Contains(errStr, "no extractor available"):
		return "unsupported_format"
	case strings.Contains(errStr, "file too large"):
		return "file_too_large"
	case strings.Contains(errStr, "permission denied"):
		return "permission_error"
	case strings.Contains(errStr, "no such file or directory"):
		return "file_not_found"
	case strings.Contains(errStr, "database"),
	     strings.Contains(errStr, "postgres"),
	     strings.Contains(errStr, "sql"):
		return "database_error"
	case strings.Contains(errStr, "embedding"),
	     strings.Contains(errStr, "ollama"):
		return "embedding_error"
	case strings.Contains(errStr, "connection"),
	     strings.Contains(errStr, "network"):
		return "network_error"
	default:
		return "processing_error"
	}
}

// DatabaseError represents a database-specific error
type DatabaseError struct {
	Operation string
	Query     string
	Err       error
}

// Error implements the error interface
func (e *DatabaseError) Error() string {
	if e.Query != "" {
		return fmt.Sprintf("database %s failed (query: %s): %v", e.Operation, e.Query, e.Err)
	}
	return fmt.Sprintf("database %s failed: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error
func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the database error might succeed on retry
func (e *DatabaseError) IsRetryable() bool {
	if e.Err == nil {
		return false
	}
	
	errStr := e.Err.Error()
	// Connection errors and locks are retryable
	return strings.Contains(errStr, "connection") ||
	       strings.Contains(errStr, "locked") ||
	       strings.Contains(errStr, "deadlock") ||
	       strings.Contains(errStr, "timeout")
}