package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// RoyalDocumentProcessor handles comprehensive metadata extraction using Unstructured.io
type RoyalDocumentProcessor struct {
	pythonPath string
	venvPath   string
	timeout    time.Duration
	log        *logrus.Logger
}

// RoyalMetadata represents the comprehensive metadata schema
type RoyalMetadata struct {
	// Core Document Metadata
	DocumentMetadata struct {
		Filename  string `json:"filename"`
		FileType  string `json:"file_type"`
		FileSize  int64  `json:"file_size"`
		PageCount int    `json:"page_count"`
		WordCount int    `json:"word_count"`
		Language  string `json:"language"`
		Encoding  string `json:"encoding"`
	} `json:"document_metadata"`

	// Temporal Metadata
	TemporalMetadata struct {
		CreatedDate   *time.Time `json:"created_date,omitempty"`
		ModifiedDate  *time.Time `json:"modified_date,omitempty"`
		ProcessedDate *time.Time `json:"processed_date"`
		ContentDate   *time.Time `json:"content_date,omitempty"`
		DateRange     *struct {
			Start *time.Time `json:"start,omitempty"`
			End   *time.Time `json:"end,omitempty"`
		} `json:"date_range,omitempty"`
	} `json:"temporal_metadata"`

	// Structural Metadata
	StructuralMetadata struct {
		Title                string         `json:"title,omitempty"`
		Authors              []string       `json:"authors,omitempty"`
		SectionHeaders       []string       `json:"section_headers,omitempty"`
		TableCount           int            `json:"table_count"`
		ImageCount           int            `json:"image_count"`
		HasTableOfContents   bool           `json:"has_table_of_contents"`
		DocumentType         string         `json:"document_type,omitempty"`
		Categories           []string       `json:"categories,omitempty"`
		UnstructuredElements map[string]int `json:"unstructured_elements,omitempty"`
	} `json:"structural_metadata"`

	// Content-Derived Metadata
	ContentMetadata struct {
		Summary         string   `json:"summary,omitempty"`
		KeyEntities     []string `json:"key_entities,omitempty"`
		Topics          []string `json:"topics,omitempty"`
		Keywords        []string `json:"keywords,omitempty"`
		Department      string   `json:"department,omitempty"`
		ProjectName     string   `json:"project_name,omitempty"`
		DocumentClass   string   `json:"document_class,omitempty"`
		ConfidenceScore float64  `json:"confidence_score,omitempty"`
	} `json:"content_metadata"`

	// Hierarchical Metadata
	HierarchyMetadata struct {
		ParentDocument string `json:"parent_document,omitempty"`
		SectionPath    string `json:"section_path,omitempty"`
		DepthLevel     int    `json:"depth_level"`
		ElementType    string `json:"element_type,omitempty"`
	} `json:"hierarchy_metadata"`
}

// NewRoyalDocumentProcessor creates a new royal document processor
func NewRoyalDocumentProcessor(config UnstructuredConfig, logger *logrus.Logger) *RoyalDocumentProcessor {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second // Longer timeout for comprehensive processing
	}

	return &RoyalDocumentProcessor{
		pythonPath: config.PythonPath,
		venvPath:   config.VenvPath,
		timeout:    config.Timeout,
		log:        logger,
	}
}

// ExtractRoyalMetadata extracts comprehensive metadata using the royal schema
func (r *RoyalDocumentProcessor) ExtractRoyalMetadata(ctx context.Context, filePath string) (*RoyalMetadata, *ExtractedContent, error) {
	startTime := time.Now()

	r.log.WithField("file", filePath).Info("Starting royal metadata extraction")

	// Create temporary Python script for extraction
	scriptFile, err := os.CreateTemp("", "royal_extract_*.py")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())

	script := r.createRoyalPythonScript()
	if _, err := scriptFile.WriteString(script); err != nil {
		return nil, nil, fmt.Errorf("failed to write script: %v", err)
	}
	scriptFile.Close()

	// Execute the script
	response, err := r.runRoyalPythonScript(ctx, scriptFile.Name(), filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("royal extraction failed: %v", err)
	}

	// Parse the response
	var result struct {
		Content  string        `json:"content"`
		Metadata RoyalMetadata `json:"royal_metadata"`
		Error    string        `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse extraction result: %v", err)
	}

	if result.Error != "" {
		return nil, nil, fmt.Errorf("extraction error: %s", result.Error)
	}

	// Set processed date
	now := time.Now()
	result.Metadata.TemporalMetadata.ProcessedDate = &now

	duration := time.Since(startTime)
	r.log.WithFields(logrus.Fields{
		"file":     filePath,
		"duration": duration,
		"elements": len(result.Metadata.StructuralMetadata.UnstructuredElements),
	}).Info("Royal metadata extraction completed")

	return &result.Metadata, &ExtractedContent{
		Text: result.Content,
		Metadata: map[string]interface{}{
			"royal_processing": true,
			"extraction_time":  duration.String(),
			"element_count":    len(result.Metadata.StructuralMetadata.UnstructuredElements),
		},
	}, nil
}

// createRoyalPythonScript creates the comprehensive Python script for royal extraction
func (r *RoyalDocumentProcessor) createRoyalPythonScript() string {
	return `#!/usr/bin/env python3
import sys
import json
import traceback
import hashlib
import re
from pathlib import Path
from datetime import datetime
from collections import defaultdict

try:
    from unstructured.partition.auto import partition
    from unstructured.staging.base import convert_to_dict
    import nltk
    
    # Download required NLTK data if not present
    try:
        nltk.data.find('tokenizers/punkt')
    except LookupError:
        nltk.download('punkt', quiet=True)
        
    try:
        nltk.data.find('corpora/stopwords')
    except LookupError:
        nltk.download('stopwords', quiet=True)
        
    from nltk.corpus import stopwords
    from nltk.tokenize import word_tokenize, sent_tokenize
    
except ImportError as e:
    print(json.dumps({"error": f"Failed to import required libraries: {e}"}))
    sys.exit(1)

def extract_royal_metadata(file_path):
    """Extract comprehensive royal metadata from document"""
    try:
        # Configure Unstructured for maximum quality extraction
        elements = partition(
            filename=file_path,
            strategy="hi_res",  # Maximum quality
            extract_images_in_pdf=True,
            include_page_breaks=True,
            infer_table_structure=True,
            chunking_strategy="by_title",
            max_characters=1500,
            new_after_n_chars=800,
            combine_text_under_n_chars=100,
            extract_element_metadata=True
        )
        
        # Initialize royal metadata structure
        royal_metadata = {
            "document_metadata": {
                "filename": Path(file_path).name,
                "file_type": Path(file_path).suffix.lower(),
                "file_size": Path(file_path).stat().st_size,
                "page_count": 0,
                "word_count": 0,
                "language": "en",  # Default, could be detected
                "encoding": "utf-8"
            },
            "temporal_metadata": {
                "processed_date": datetime.utcnow().isoformat(),
                "created_date": datetime.fromtimestamp(Path(file_path).stat().st_ctime).isoformat(),
                "modified_date": datetime.fromtimestamp(Path(file_path).stat().st_mtime).isoformat()
            },
            "structural_metadata": {
                "title": "",
                "authors": [],
                "section_headers": [],
                "table_count": 0,
                "image_count": 0,
                "has_table_of_contents": False,
                "document_type": "",
                "categories": [],
                "unstructured_elements": {}
            },
            "content_metadata": {
                "summary": "",
                "key_entities": [],
                "topics": [],
                "keywords": [],
                "department": "",
                "project_name": "",
                "document_class": "",
                "confidence_score": 0.0
            },
            "hierarchy_metadata": {
                "parent_document": "",
                "section_path": "",
                "depth_level": 0,
                "element_type": ""
            }
        }
        
        # Extract content and analyze elements
        content_parts = []
        element_types = defaultdict(int)
        titles = []
        
        for element in elements:
            if hasattr(element, 'text') and element.text.strip():
                content_parts.append(element.text.strip())
                
                # Count element types
                element_type = type(element).__name__
                element_types[element_type] += 1
                
                # Extract titles and headers
                if element.category in ["Title", "Header"]:
                    titles.append(element.text.strip())
                    royal_metadata["structural_metadata"]["section_headers"].append(element.text.strip())
                
                # Count specific elements
                if element.category == "Table":
                    royal_metadata["structural_metadata"]["table_count"] += 1
                elif element.category in ["Image", "Figure"]:
                    royal_metadata["structural_metadata"]["image_count"] += 1
                
                # Track page numbers
                if hasattr(element, 'metadata') and element.metadata:
                    if hasattr(element.metadata, 'page_number') and element.metadata.page_number:
                        royal_metadata["document_metadata"]["page_count"] = max(
                            royal_metadata["document_metadata"]["page_count"], 
                            element.metadata.page_number
                        )
        
        # Combine content
        full_content = '\n\n'.join(content_parts)
        
        # Calculate word count
        try:
            words = word_tokenize(full_content)
            royal_metadata["document_metadata"]["word_count"] = len(words)
        except:
            royal_metadata["document_metadata"]["word_count"] = len(full_content.split())
        
        # Set title (first title or filename)
        if titles:
            royal_metadata["structural_metadata"]["title"] = titles[0]
        else:
            royal_metadata["structural_metadata"]["title"] = Path(file_path).stem
        
        # Generate summary (first paragraph or first 200 words)
        sentences = sent_tokenize(full_content)
        if sentences:
            # Use first few sentences as summary, up to 200 words
            summary_parts = []
            word_count = 0
            for sentence in sentences[:5]:  # Max 5 sentences
                sentence_words = len(sentence.split())
                if word_count + sentence_words > 200:
                    break
                summary_parts.append(sentence)
                word_count += sentence_words
            royal_metadata["content_metadata"]["summary"] = ' '.join(summary_parts)
        
        # Extract keywords (simple frequency-based approach)
        try:
            stop_words = set(stopwords.words('english'))
            words = word_tokenize(full_content.lower())
            words = [w for w in words if w.isalpha() and w not in stop_words and len(w) > 3]
            
            # Count word frequency
            word_freq = defaultdict(int)
            for word in words:
                word_freq[word] += 1
            
            # Get top keywords
            royal_metadata["content_metadata"]["keywords"] = [
                word for word, count in sorted(word_freq.items(), key=lambda x: x[1], reverse=True)[:20]
            ]
        except:
            royal_metadata["content_metadata"]["keywords"] = []
        
        # Detect document type based on content patterns
        royal_metadata["structural_metadata"]["document_type"] = detect_document_type(full_content, titles)
        
        # Detect department/project from content
        department, project = extract_organizational_info(full_content)
        royal_metadata["content_metadata"]["department"] = department
        royal_metadata["content_metadata"]["project_name"] = project
        
        # Set categories from element types
        royal_metadata["structural_metadata"]["categories"] = list(element_types.keys())
        royal_metadata["structural_metadata"]["unstructured_elements"] = dict(element_types)
        
        # Calculate confidence score based on extraction completeness
        confidence = calculate_confidence_score(royal_metadata, len(elements))
        royal_metadata["content_metadata"]["confidence_score"] = confidence
        
        return {
            "content": full_content,
            "royal_metadata": royal_metadata
        }
        
    except Exception as e:
        return {
            "error": f"Royal extraction failed: {str(e)}",
            "traceback": traceback.format_exc()
        }

def detect_document_type(content, titles):
    """Detect document type based on content patterns"""
    content_lower = content.lower()
    
    # Define patterns for different document types
    patterns = {
        "report": [r"executive summary", r"conclusion", r"recommendations", r"findings"],
        "email": [r"from:", r"to:", r"subject:", r"sent:", r"dear", r"best regards"],
        "memo": [r"memorandum", r"memo", r"from:", r"date:", r"re:"],
        "invoice": [r"invoice", r"bill to", r"total amount", r"payment due"],
        "contract": [r"agreement", r"party", r"terms and conditions", r"hereby"],
        "presentation": [r"slide", r"agenda", r"overview", r"next steps"],
        "manual": [r"instructions", r"step", r"procedure", r"guide"],
        "specification": [r"requirements", r"specifications", r"technical", r"standards"]
    }
    
    scores = {}
    for doc_type, type_patterns in patterns.items():
        score = sum(1 for pattern in type_patterns if re.search(pattern, content_lower))
        if score > 0:
            scores[doc_type] = score
    
    return max(scores.items(), key=lambda x: x[1])[0] if scores else "document"

def extract_organizational_info(content):
    """Extract department and project information from content"""
    content_lower = content.lower()
    
    # Common department patterns
    dept_patterns = {
        "engineering": [r"engineering", r"development", r"software", r"technical"],
        "finance": [r"finance", r"accounting", r"budget", r"financial"],
        "hr": [r"human resources", r"personnel", r"hr", r"employee"],
        "marketing": [r"marketing", r"promotion", r"campaign", r"brand"],
        "sales": [r"sales", r"revenue", r"customer", r"client"],
        "legal": [r"legal", r"compliance", r"contract", r"law"],
        "operations": [r"operations", r"logistics", r"supply", r"process"]
    }
    
    department = ""
    for dept, patterns in dept_patterns.items():
        if any(re.search(pattern, content_lower) for pattern in patterns):
            department = dept
            break
    
    # Extract project name (simple pattern matching)
    project_patterns = [
        r"project (\w+)",
        r"(\w+) project",
        r"initiative (\w+)",
        r"program (\w+)"
    ]
    
    project_name = ""
    for pattern in project_patterns:
        match = re.search(pattern, content_lower)
        if match:
            project_name = match.group(1).title()
            break
    
    return department, project_name

def calculate_confidence_score(metadata, element_count):
    """Calculate confidence score based on extraction completeness"""
    score = 0.0
    
    # Base score for having elements
    if element_count > 0:
        score += 0.3
    
    # Title extracted
    if metadata["structural_metadata"]["title"]:
        score += 0.2
    
    # Content metadata populated
    if metadata["content_metadata"]["keywords"]:
        score += 0.2
    
    # Summary extracted
    if metadata["content_metadata"]["summary"]:
        score += 0.1
    
    # Document type detected
    if metadata["structural_metadata"]["document_type"] != "document":
        score += 0.1
    
    # Organizational info
    if metadata["content_metadata"]["department"]:
        score += 0.05
    
    if metadata["content_metadata"]["project_name"]:
        score += 0.05
    
    return min(score, 1.0)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(json.dumps({"error": "Usage: script.py <file_path>"}))
        sys.exit(1)
    
    file_path = sys.argv[1]
    result = extract_royal_metadata(file_path)
    print(json.dumps(result, ensure_ascii=False, indent=None, default=str))`
}

// runRoyalPythonScript executes the royal Python script
func (r *RoyalDocumentProcessor) runRoyalPythonScript(ctx context.Context, scriptPath, filePath string) (string, error) {
	var cmd *exec.Cmd

	if r.venvPath != "" {
		pythonBin := filepath.Join(r.venvPath, "bin", "python")
		cmd = exec.CommandContext(ctx, pythonBin, scriptPath, filePath)
	} else if r.pythonPath != "" {
		cmd = exec.CommandContext(ctx, r.pythonPath, scriptPath, filePath)
	} else {
		cmd = exec.CommandContext(ctx, "python3", scriptPath, filePath)
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("royal extraction timeout")
		}
		return "", fmt.Errorf("script execution failed: %v", err)
	}

	return string(output), nil
}

// FlattenRoyalMetadata converts RoyalMetadata to a flat map for database storage
func FlattenRoyalMetadata(metadata *RoyalMetadata) map[string]interface{} {
	result := make(map[string]interface{})

	// Document metadata
	result["filename"] = metadata.DocumentMetadata.Filename
	result["file_type"] = metadata.DocumentMetadata.FileType
	result["file_size"] = metadata.DocumentMetadata.FileSize
	result["page_count"] = metadata.DocumentMetadata.PageCount
	result["word_count"] = metadata.DocumentMetadata.WordCount
	result["language"] = metadata.DocumentMetadata.Language
	result["encoding"] = metadata.DocumentMetadata.Encoding

	// Temporal metadata
	if metadata.TemporalMetadata.CreatedDate != nil {
		result["created_date"] = metadata.TemporalMetadata.CreatedDate.Format(time.RFC3339)
	}
	if metadata.TemporalMetadata.ModifiedDate != nil {
		result["modified_date"] = metadata.TemporalMetadata.ModifiedDate.Format(time.RFC3339)
	}
	if metadata.TemporalMetadata.ProcessedDate != nil {
		result["processed_date"] = metadata.TemporalMetadata.ProcessedDate.Format(time.RFC3339)
	}
	if metadata.TemporalMetadata.ContentDate != nil {
		result["content_date"] = metadata.TemporalMetadata.ContentDate.Format(time.RFC3339)
	}

	// Structural metadata
	result["title"] = metadata.StructuralMetadata.Title
	result["authors"] = metadata.StructuralMetadata.Authors
	result["section_headers"] = metadata.StructuralMetadata.SectionHeaders
	result["table_count"] = metadata.StructuralMetadata.TableCount
	result["image_count"] = metadata.StructuralMetadata.ImageCount
	result["has_table_of_contents"] = metadata.StructuralMetadata.HasTableOfContents
	result["document_type"] = metadata.StructuralMetadata.DocumentType
	result["categories"] = metadata.StructuralMetadata.Categories

	// Content metadata
	result["summary"] = metadata.ContentMetadata.Summary
	result["key_entities"] = metadata.ContentMetadata.KeyEntities
	result["topics"] = metadata.ContentMetadata.Topics
	result["keywords"] = metadata.ContentMetadata.Keywords
	result["department"] = metadata.ContentMetadata.Department
	result["project_name"] = metadata.ContentMetadata.ProjectName
	result["document_class"] = metadata.ContentMetadata.DocumentClass
	result["confidence_score"] = metadata.ContentMetadata.ConfidenceScore

	// Hierarchy metadata
	result["parent_document"] = metadata.HierarchyMetadata.ParentDocument
	result["section_path"] = metadata.HierarchyMetadata.SectionPath
	result["depth_level"] = metadata.HierarchyMetadata.DepthLevel
	result["element_type"] = metadata.HierarchyMetadata.ElementType

	return result
}
