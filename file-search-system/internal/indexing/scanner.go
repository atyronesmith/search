package indexing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/pkg/filesystem"
	"github.com/sirupsen/logrus"
)

// Scanner handles file system scanning
type Scanner struct {
	db              *database.DB
	config          *ScannerConfig
	log             *logrus.Logger
	supportedExts   map[string]bool
	ignorePatterns  []string
}

type ScannerConfig struct {
	WatchPaths      []string
	MaxFileSizeMB   int
	IgnorePatterns  []string
	SupportedTypes  []string
}

func NewScanner(db *database.DB, config *ScannerConfig, log *logrus.Logger) *Scanner {
	supportedExts := make(map[string]bool)
	for _, ext := range config.SupportedTypes {
		supportedExts[ext] = true
	}

	return &Scanner{
		db:             db,
		config:         config,
		log:            log,
		supportedExts:  supportedExts,
		ignorePatterns: config.IgnorePatterns,
	}
}

// ScanDirectory scans a directory recursively for files to index
func (s *Scanner) ScanDirectory(ctx context.Context, rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.log.WithError(err).WithField("path", path).Warn("Error accessing path")
			return nil // Continue walking
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Check if directory should be ignored
			if s.shouldIgnore(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Process file
		if err := s.processFile(ctx, path, info); err != nil {
			s.log.WithError(err).WithField("path", path).Error("Failed to process file")
		}

		return nil
	})
}

// processFile checks if a file should be indexed and adds it to the database
func (s *Scanner) processFile(ctx context.Context, path string, info os.FileInfo) error {
	// Check if file should be ignored
	if s.shouldIgnore(path) {
		return nil
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if !s.isSupported(ext) {
		return nil
	}

	// Check file size
	maxSize := int64(s.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		s.log.WithFields(logrus.Fields{
			"path": path,
			"size": info.Size(),
		}).Debug("File too large, skipping")
		return nil
	}

	// Calculate file hash
	hash, err := s.calculateFileHash(path)
	if err != nil {
		return err
	}

	// Check if file already exists in database
	existingFile, err := s.getFileByPath(ctx, path)
	if err != nil {
		return err
	}

	if existingFile != nil {
		// File exists, check if it needs reindexing
		if existingFile.ContentHash != nil && *existingFile.ContentHash == hash {
			// File hasn't changed
			return nil
		}
		// File has changed, mark for reindexing
		return s.markFileForReindexing(ctx, existingFile.ID, path, info, hash)
	}

	// New file, add to database
	return s.addNewFile(ctx, path, info, hash)
}

// shouldIgnore checks if a path matches any ignore patterns
func (s *Scanner) shouldIgnore(path string) bool {
	basename := filepath.Base(path)
	
	for _, pattern := range s.ignorePatterns {
		// Check basename patterns
		if matched, _ := filepath.Match(pattern, basename); matched {
			return true
		}
		// Check path patterns
		if strings.Contains(path, pattern) {
			return true
		}
	}
	
	// Check for hidden files
	if strings.HasPrefix(basename, ".") {
		return true
	}
	
	return false
}

// isSupported checks if a file extension is supported
func (s *Scanner) isSupported(ext string) bool {
	// Default supported extensions if not configured
	if len(s.supportedExts) == 0 {
		defaultExts := map[string]bool{
			".pdf": true, ".doc": true, ".docx": true,
			".xls": true, ".xlsx": true, ".csv": true,
			".txt": true, ".md": true, ".rtf": true,
			".py": true, ".js": true, ".ts": true,
			".jsx": true, ".tsx": true, ".java": true,
			".cpp": true, ".c": true, ".go": true,
			".rs": true, ".json": true, ".yaml": true,
			".yml": true,
		}
		return defaultExts[ext]
	}
	return s.supportedExts[ext]
}

// calculateFileHash calculates SHA-256 hash of a file
func (s *Scanner) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// getFileByPath retrieves a file from the database by path
func (s *Scanner) getFileByPath(ctx context.Context, path string) (*database.File, error) {
	query := `
		SELECT id, path, parent_path, filename, extension, file_type,
		       size_bytes, created_at, modified_at, last_indexed,
		       content_hash, indexing_status, error_message, metadata
		FROM files
		WHERE path = $1
	`
	
	var file database.File
	err := s.db.QueryRow(ctx, query, path).Scan(
		&file.ID, &file.Path, &file.ParentPath, &file.Filename,
		&file.Extension, &file.FileType, &file.SizeBytes,
		&file.CreatedAt, &file.ModifiedAt, &file.LastIndexed,
		&file.ContentHash, &file.IndexingStatus, &file.ErrorMessage,
		&file.Metadata,
	)
	
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	
	return &file, nil
}

// markFileForReindexing marks a file for reindexing
func (s *Scanner) markFileForReindexing(ctx context.Context, fileID int64, path string, info os.FileInfo, newHash string) error {
	// Get proper filesystem timestamps - we only need the modification time for updates
	timestamps := filesystem.GetFileTimestampsFromInfo(path, info)
	if s.log != nil {
		s.log.Debugf("File %s changed: new modified time=%v", path, timestamps.ModifiedAt)
	}
	
	query := `
		UPDATE files 
		SET content_hash = $1, 
		    indexing_status = 'pending',
		    modified_at = $2
		WHERE id = $3
	`
	_, err := s.db.Exec(ctx, query, newHash, timestamps.ModifiedAt, fileID)
	if err != nil {
		return err
	}

	// Add to file changes table
	changeQuery := `
		INSERT INTO file_changes (file_path, change_type, detected_at)
		SELECT path, 'modified', NOW() FROM files WHERE id = $1
	`
	_, err = s.db.Exec(ctx, changeQuery, fileID)
	return err
}

// addNewFile adds a new file to the database
func (s *Scanner) addNewFile(ctx context.Context, path string, info os.FileInfo, hash string) error {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	fileType := s.determineFileType(ext)
	
	// Get proper filesystem timestamps
	timestamps := filesystem.GetFileTimestampsFromInfo(path, info)
	if s.log != nil {
		if timestamps.HasBirthTime {
			s.log.Debugf("File %s: created=%v, modified=%v", path, timestamps.CreatedAt, timestamps.ModifiedAt)
		} else {
			s.log.Debugf("File %s: using modtime for both timestamps (%v)", path, timestamps.ModifiedAt)
		}
	}
	
	query := `
		INSERT INTO files (
			path, filename, extension, file_type, size_bytes,
			created_at, modified_at, content_hash, indexing_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
		ON CONFLICT (path) DO UPDATE SET
			content_hash = EXCLUDED.content_hash,
			modified_at = EXCLUDED.modified_at,
			indexing_status = 'pending'
	`
	
	_, err := s.db.Exec(ctx, query,
		path, filename, ext, fileType, info.Size(),
		timestamps.CreatedAt, timestamps.ModifiedAt, hash,
	)
	
	if err != nil {
		return err
	}

	// Add to file changes table
	changeQuery := `
		INSERT INTO file_changes (file_path, change_type, detected_at)
		VALUES ($1, 'created', NOW())
	`
	_, err = s.db.Exec(ctx, changeQuery, path)
	return err
}

// determineFileType determines the file type based on extension
func (s *Scanner) determineFileType(ext string) string {
	ext = strings.ToLower(ext)
	
	documentExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true,
	}
	
	spreadsheetExts := map[string]bool{
		".xls": true, ".xlsx": true, ".csv": true,
	}
	
	codeExts := map[string]bool{
		".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".java": true,
		".cpp": true, ".c": true, ".go": true,
		".rs": true, ".json": true, ".yaml": true,
		".yml": true,
	}
	
	if documentExts[ext] {
		return "document"
	}
	if spreadsheetExts[ext] {
		return "spreadsheet"
	}
	if codeExts[ext] {
		return "code"
	}
	return "text"
}