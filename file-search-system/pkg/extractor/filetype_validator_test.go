package extractor

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestFileTypeValidator(t *testing.T) {
	// Create a test logger
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	
	validator := NewFileTypeValidator(log)
	
	// Test case 1: Create a ZIP file with .docx extension
	t.Run("ZIP file with DOCX extension", func(t *testing.T) {
		// Create a temporary ZIP file with .docx extension
		tmpFile, err := os.CreateTemp("", "test*.docx")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		// Write ZIP file header (PK signature)
		zipHeader := []byte{0x50, 0x4B, 0x03, 0x04} // ZIP file signature
		if _, err := tmpFile.Write(zipHeader); err != nil {
			t.Fatal(err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}
		
		// Validate the file
		result, err := validator.ValidateFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		
		// Check results
		if result.IsValid {
			t.Error("Expected IsValid to be false for ZIP file with .docx extension")
		}
		
		if result.DetectedType != "zip" {
			t.Errorf("Expected detected type to be 'zip', got '%s'", result.DetectedType)
		}
		
		if result.ActualType != "zip" {
			t.Errorf("Expected actual type to be 'zip', got '%s'", result.ActualType)
		}
		
		t.Logf("Test passed: ZIP with .docx extension detected as %s", result.DetectedType)
	})
	
	// Test case 2: Plain text file with .txt extension
	t.Run("Text file with TXT extension", func(t *testing.T) {
		// Create a temporary text file
		tmpFile, err := os.CreateTemp("", "test*.txt")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		// Write some text content
		if _, err := tmpFile.WriteString("This is a test text file."); err != nil {
			t.Fatal(err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}
		
		// Validate the file
		result, err := validator.ValidateFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		
		// Check results
		if !result.IsValid {
			t.Error("Expected IsValid to be true for text file with .txt extension")
		}
		
		if result.DetectedType != "txt" {
			t.Errorf("Expected detected type to be 'txt', got '%s'", result.DetectedType)
		}
		
		t.Logf("Test passed: Text file correctly identified as %s", result.DetectedType)
	})
	
	// Test case 3: Email file with .eml extension
	t.Run("Email file with EML extension", func(t *testing.T) {
		// Create a temporary email file
		tmpFile, err := os.CreateTemp("", "test*.eml")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		// Write email content (will be detected as text/plain)
		emailContent := `From: sender@example.com
To: recipient@example.com
Subject: Test Email

This is a test email.`
		if _, err := tmpFile.WriteString(emailContent); err != nil {
			t.Fatal(err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}
		
		// Validate the file
		result, err := validator.ValidateFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		
		// Check results - should trust extension for .eml files
		if !result.IsValid {
			t.Error("Expected IsValid to be true for .eml file")
		}
		
		if result.ActualType != "eml" {
			t.Errorf("Expected actual type to be 'eml', got '%s'", result.ActualType)
		}
		
		t.Logf("Test passed: Email file handled with extension trust, actual type: %s", result.ActualType)
	})
	
	// Test case 4: Database type mapping
	t.Run("Database type mapping", func(t *testing.T) {
		testCases := []struct {
			detectedType string
			expectedDBType string
		}{
			{"zip", "document"},
			{"pdf", "pdf"},
			{"docx", "docx"},
			{"py", "python"},
			{"js", "javascript"},
			{"html", "code"},
			{"png", "image"},
			{"eml", "document"},
			{"unknown", "document"}, // Unknown types default to document
		}
		
		for _, tc := range testCases {
			dbType := validator.MapToDBFileType(tc.detectedType)
			if dbType != tc.expectedDBType {
				t.Errorf("MapToDBFileType(%s) = %s, want %s", tc.detectedType, dbType, tc.expectedDBType)
			} else {
				t.Logf("✓ %s -> %s", tc.detectedType, dbType)
			}
		}
	})
}