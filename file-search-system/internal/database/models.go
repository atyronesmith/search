package database

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/pgvector/pgvector-go"
)

// File represents a file in the index
type File struct {
	ID             int64           `json:"id"`
	Path           string          `json:"path"`
	ParentPath     *string         `json:"parent_path,omitempty"`
	Filename       string          `json:"filename"`
	Extension      *string         `json:"extension,omitempty"`
	FileType       *string         `json:"file_type,omitempty"`
	SizeBytes      int64           `json:"size_bytes"`
	CreatedAt      *time.Time      `json:"created_at,omitempty"`
	ModifiedAt     *time.Time      `json:"modified_at,omitempty"`
	LastIndexed    *time.Time      `json:"last_indexed,omitempty"`
	ContentHash    *string         `json:"content_hash,omitempty"`
	IndexingStatus string          `json:"indexing_status"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// Chunk represents a text chunk with embedding
type Chunk struct {
	ID         int64           `json:"id"`
	FileID     int64           `json:"file_id"`
	ChunkIndex int             `json:"chunk_index"`
	Content    string          `json:"content"`
	Embedding  pgvector.Vector `json:"-"`
	StartPage  *int            `json:"start_page,omitempty"`
	StartLine  *int            `json:"start_line,omitempty"`
	CharStart  int             `json:"char_start"`
	CharEnd    int             `json:"char_end"`
	ChunkType  string          `json:"chunk_type"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

// TextSearch represents a full-text search entry
type TextSearch struct {
	ID       int64   `json:"id"`
	FileID   int64   `json:"file_id"`
	ChunkID  int64   `json:"chunk_id"`
	Content  string  `json:"content"`
	TitleTSV *string `json:"-"`
	Language string  `json:"language"`
}

// FileChange represents a detected file system change
type FileChange struct {
	ID           int64      `json:"id"`
	FilePath     string     `json:"file_path"`
	ChangeType   string     `json:"change_type"`
	OldPath      *string    `json:"old_path,omitempty"`
	DetectedAt   time.Time  `json:"detected_at"`
	Processed    bool       `json:"processed"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	FileID        int64           `json:"file_id"`
	FilePath      string          `json:"file_path"`
	Filename      string          `json:"filename"`
	ChunkID       int64           `json:"chunk_id"`
	Content       string          `json:"content"`
	Score         float64         `json:"score"`
	VectorScore   float64         `json:"vector_score"`
	TextScore     float64         `json:"text_score"`
	MetadataScore float64         `json:"metadata_score"`
	Highlights    []string        `json:"highlights,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// IndexingRule represents a directory indexing rule
type IndexingRule struct {
	ID              int       `json:"id"`
	PathPattern     string    `json:"path_pattern"`
	Priority        int       `json:"priority"`
	Enabled         bool      `json:"enabled"`
	Recursive       bool      `json:"recursive"`
	FilePatterns    []string  `json:"file_patterns"`
	ExcludePatterns []string  `json:"exclude_patterns"`
	MaxFileSizeMB   int       `json:"max_file_size_mb"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// IndexingStats represents system indexing statistics
type IndexingStats struct {
	ID             int       `json:"id"`
	TotalFiles     int64     `json:"total_files"`
	IndexedFiles   int64     `json:"indexed_files"`
	FailedFiles    int64     `json:"failed_files"`
	TotalChunks    int64     `json:"total_chunks"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	LastUpdated    time.Time `json:"last_updated"`
}

// SearchCache represents a cached search result
type SearchCache struct {
	ID           int64           `json:"id"`
	QueryHash    string          `json:"query_hash"`
	QueryText    string          `json:"query_text"`
	Results      json.RawMessage `json:"results"`
	CreatedAt    time.Time       `json:"created_at"`
	AccessCount  int             `json:"access_count"`
	LastAccessed time.Time       `json:"last_accessed"`
	TTLMinutes   int             `json:"ttl_minutes"`
}

// StringArray is a custom type for PostgreSQL text[] columns
type StringArray []string

// Value implements driver.Valuer for StringArray
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return a, nil
}

// Scan implements sql.Scanner for StringArray
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = []string{}
		return nil
	}
	// PostgreSQL array parsing would go here
	// For simplicity, using JSON for now
	return json.Unmarshal(value.([]byte), a)
}

// FileStatus represents the indexing status of a file
type FileStatus string

// File indexing status constants
const (
	// FileStatusPending indicates file is waiting to be indexed
	FileStatusPending FileStatus = "pending"
	// FileStatusProcessing indicates file is being processed
	FileStatusProcessing FileStatus = "processing"
	// FileStatusCompleted indicates file indexing is complete
	FileStatusCompleted FileStatus = "completed"
	// FileStatusError indicates file indexing failed
	FileStatusError FileStatus = "error"
	// FileStatusSkipped indicates file was skipped
	FileStatusSkipped FileStatus = "skipped"
)

// ChangeType represents the type of file system change
type ChangeType string

// File change type constants
const (
	// ChangeTypeCreated indicates a new file was created
	ChangeTypeCreated ChangeType = "created"
	// ChangeTypeModified indicates a file was modified
	ChangeTypeModified ChangeType = "modified"
	// ChangeTypeDeleted indicates a file was deleted
	ChangeTypeDeleted ChangeType = "deleted"
	// ChangeTypeRenamed indicates a file was renamed
	ChangeTypeRenamed ChangeType = "renamed"
)

// ChunkType represents the type of text chunk
type ChunkType string

// Chunk type constants
const (
	// ChunkTypeSemantic indicates semantic chunking strategy
	ChunkTypeSemantic ChunkType = "semantic"
	// ChunkTypeCode indicates code-aware chunking
	ChunkTypeCode ChunkType = "code"
	// ChunkTypeTable indicates table chunking
	ChunkTypeTable ChunkType = "table"
	// ChunkTypeList indicates list chunking
	ChunkTypeList ChunkType = "list"
	// ChunkTypeSliding indicates sliding window chunking
	ChunkTypeSliding ChunkType = "sliding"
)
