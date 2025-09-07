package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type UnstructuredExtractor struct {
	pythonPath    string
	venvPath      string
	timeout       time.Duration
	log           *logrus.Logger
	royalProcessor *RoyalDocumentProcessor
}

type UnstructuredConfig struct {
	PythonPath string
	VenvPath   string
	Timeout    time.Duration
}

type UnstructuredResponse struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Error    string                 `json:"error,omitempty"`
}

func NewUnstructuredExtractor(config UnstructuredConfig, logger *logrus.Logger) *UnstructuredExtractor {
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	return &UnstructuredExtractor{
		pythonPath:     config.PythonPath,
		venvPath:       config.VenvPath,
		timeout:        config.Timeout,
		log:            logger,
		royalProcessor: NewRoyalDocumentProcessor(config, logger),
	}
}

func (e *UnstructuredExtractor) CanExtract(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return e.isSupported(ext)
}

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

	// Create Python script content
	script := e.createPythonScript()

	// Create temporary script file
	scriptFile, err := os.CreateTemp("", "unstructured_extract_*.py")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %v", err)
	}
	defer os.Remove(scriptFile.Name())

	if _, err := scriptFile.WriteString(script); err != nil {
		return nil, fmt.Errorf("failed to write script: %v", err)
	}
	scriptFile.Close()

	// Run the Python script
	response, err := e.runPythonScript(scriptFile.Name(), filePath)
	if err != nil {
		return nil, fmt.Errorf("python extraction failed: %v", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("extraction error: %s", response.Error)
	}

	duration := time.Since(startTime)
	e.log.WithFields(logrus.Fields{
		"file":     filePath,
		"duration": duration,
		"size":     len(response.Content),
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

	return &ExtractedContent{
		Text:     response.Content,
		Metadata: finalMetadata,
	}, nil
}

func (e *UnstructuredExtractor) isSupported(ext string) bool {
	// Only handle the formats we have confirmed dependencies for
	// Let other extractors handle PDF, MD, JSON, etc.
	supportedTypes := map[string]bool{
		".docx": true,  // Office documents work well
		".xlsx": true,  // Office documents work well  
		".pptx": true,  // Office documents work well
		".doc":  true,  // Older Office formats
		".xls":  true,  // Older Office formats
		".ppt":  true,  // Older Office formats
	}
	return supportedTypes[ext]
}

func (e *UnstructuredExtractor) createPythonScript() string {
	return `#!/usr/bin/env python3
import sys
import json
import traceback
from pathlib import Path

try:
    from unstructured.partition.auto import partition
    from unstructured.staging.base import convert_to_dict
except ImportError as e:
    print(json.dumps({"error": f"Failed to import unstructured: {e}"}))
    sys.exit(1)

def extract_document(file_path):
    try:
        # Use auto-partition to detect and process the document
        elements = partition(
            filename=file_path,
            strategy="auto",
            include_page_breaks=True,
            infer_table_structure=True,
            chunking_strategy="by_title",
            max_characters=10000,
            new_after_n_chars=3800,
            combine_text_under_n_chars=2000,
        )
        
        # Extract text content from all elements
        content_parts = []
        metadata = {
            "num_elements": len(elements),
            "element_types": {},
            "page_count": 0,
        }
        
        for element in elements:
            if hasattr(element, 'text') and element.text.strip():
                content_parts.append(element.text.strip())
            
            # Count element types
            element_type = type(element).__name__
            metadata["element_types"][element_type] = metadata["element_types"].get(element_type, 0) + 1
            
            # Track page numbers if available
            if hasattr(element, 'metadata') and element.metadata:
                if hasattr(element.metadata, 'page_number') and element.metadata.page_number:
                    metadata["page_count"] = max(metadata["page_count"], element.metadata.page_number)
        
        content = '\n\n'.join(content_parts)
        
        return {
            "content": content,
            "metadata": metadata
        }
        
    except Exception as e:
        return {
            "error": f"Extraction failed: {str(e)}",
            "traceback": traceback.format_exc()
        }

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(json.dumps({"error": "Usage: script.py <file_path>"}))
        sys.exit(1)
    
    file_path = sys.argv[1]
    result = extract_document(file_path)
    print(json.dumps(result, ensure_ascii=False, indent=None))
`
}

func (e *UnstructuredExtractor) runPythonScript(scriptPath, filePath string) (*UnstructuredResponse, error) {
	var cmd *exec.Cmd
	
	if e.venvPath != "" {
		// Use virtual environment python
		pythonBin := filepath.Join(e.venvPath, "bin", "python")
		cmd = exec.Command(pythonBin, scriptPath, filePath)
	} else if e.pythonPath != "" {
		// Use specified python path
		cmd = exec.Command(e.pythonPath, scriptPath, filePath)
	} else {
		// Use system python3
		cmd = exec.Command("python3", scriptPath, filePath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout
	if e.timeout > 0 {
		go func() {
			time.Sleep(e.timeout)
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()
	}

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("command failed: %v, stderr: %s", err, stderr.String())
	}

	var response UnstructuredResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v, output: %s", err, stdout.String())
	}

	return &response, nil
}

func (e *UnstructuredExtractor) GetSupportedExtensions() []string {
	return []string{
		".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt",
	}
}

func (e *UnstructuredExtractor) GetName() string {
	return "UnstructuredExtractor"
}