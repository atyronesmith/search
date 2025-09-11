package service

import (
	"context"
	"fmt"
	"time"

	"github.com/file-search/file-search-system/internal/database"
)

// dbHelper contains common database operations to reduce code duplication
type dbHelper struct {
	db *database.DB
}

// newDBHelper creates a new database helper
func newDBHelper(db *database.DB) *dbHelper {
	return &dbHelper{db: db}
}

// updateFileStatus updates the indexing status of a file
func (h *dbHelper) updateFileStatus(ctx context.Context, fileID int64, status, errorMessage string) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return ErrContextCanceled
	}
	
	var query string
	var args []interface{}
	
	switch status {
	case "processing":
		query = `UPDATE files SET indexing_status = $1, error_message = NULL WHERE id = $2`
		args = []interface{}{status, fileID}
	case "completed":
		query = `UPDATE files SET indexing_status = $1, error_message = NULL, last_indexed = $2 WHERE id = $3`
		args = []interface{}{status, time.Now(), fileID}
	case "error", "skipped":
		query = `UPDATE files SET indexing_status = $1, error_message = $2, last_indexed = $3 WHERE id = $4`
		args = []interface{}{status, errorMessage, time.Now(), fileID}
	case "pending":
		query = `UPDATE files SET indexing_status = $1, last_indexed = NULL, error_message = NULL WHERE id = $2`
		args = []interface{}{status, fileID}
	default:
		return fmt.Errorf("invalid status: %s", status)
	}
	
	_, err := h.db.Exec(ctx, query, args...)
	if err != nil {
		return &DatabaseError{
			Operation: "updateFileStatus",
			Query:     query,
			Err:       err,
		}
	}
	return nil
}

// clearFileData removes all indexed data for a file (chunks and text_search)
func (h *dbHelper) clearFileData(ctx context.Context, fileID int64) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return ErrContextCanceled
	}
	
	// Start a transaction for atomic deletion
	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return &DatabaseError{
			Operation: "clearFileData",
			Err:       err,
		}
	}
	defer func() {
		_ = tx.Rollback() // Ignore error as tx may already be committed
	}()
	
	// Delete from text_search first (foreign key constraint)
	if _, err := tx.ExecContext(ctx, `DELETE FROM text_search WHERE file_id = $1`, fileID); err != nil {
		return &DatabaseError{
			Operation: "clearFileData",
			Query:     "DELETE FROM text_search",
			Err:       err,
		}
	}
	
	// Delete chunks
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE file_id = $1`, fileID); err != nil {
		return &DatabaseError{
			Operation: "clearFileData",
			Query:     "DELETE FROM chunks",
			Err:       err,
		}
	}
	
	return tx.Commit()
}


// updateFileMetadata updates a specific metadata field for a file
func (h *dbHelper) updateFileMetadata(ctx context.Context, fileID int64, field string, value interface{}) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return ErrContextCanceled
	}
	
	// Map field names to column names
	columnMap := map[string]string{
		"file_type":         "file_type",
		"extraction_method": "extraction_method",
		"royal_metadata":    "royal_metadata",
		"content_hash":      "content_hash",
	}
	
	column, ok := columnMap[field]
	if !ok {
		return fmt.Errorf("invalid metadata field: %s", field)
	}
	
	query := fmt.Sprintf(`UPDATE files SET %s = $1 WHERE id = $2`, column)
	_, err := h.db.Exec(ctx, query, value, fileID)
	if err != nil {
		return &DatabaseError{
			Operation: fmt.Sprintf("update %s", field),
			Query:     query,
			Err:       err,
		}
	}
	
	return nil
}