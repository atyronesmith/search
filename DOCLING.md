# Docling Integration Plan

## Overview

This document outlines a comprehensive plan to integrate IBM's Docling document processing library into our file search system to enhance PDF processing and add support for additional document formats (DOCX, PPTX).

## Current State vs Target State

### Current Architecture
```
PDF Files → pdftotext → Basic text chunks → Embeddings → Search
```

### Target Architecture
```
Documents → Docling Service → Structured chunks + metadata → Enhanced embeddings → Advanced search
```

## Phase 1: Proof of Concept (Week 1-2)

### Goals
- Validate docling performance and quality vs current pdftotext
- Establish baseline metrics for processing time and accuracy
- Create initial Python integration script

### Tasks

#### 1.1 Environment Setup
- [ ] Create Python virtual environment for docling
- [ ] Install docling and dependencies: `pip install docling`
- [ ] Test docling on sample PDFs from `/Users/asmith/Downloads/files/`
- [ ] Document system requirements and dependencies

#### 1.2 Comparison Script
Create `scripts/compare_extraction.py`:
```python
import docling
import subprocess
import time
import json
from pathlib import Path

def compare_extraction(pdf_path):
    # Test pdftotext (current)
    start = time.time()
    pdftotext_result = subprocess.run(['pdftotext', '-layout', pdf_path, '-'], 
                                    capture_output=True, text=True)
    pdftotext_time = time.time() - start
    
    # Test docling
    start = time.time()
    doc = docling.DocumentConverter().convert(pdf_path)
    docling_result = doc.document.export_to_markdown()
    docling_time = time.time() - start
    
    return {
        'pdftotext': {'text': pdftotext_result.stdout, 'time': pdftotext_time},
        'docling': {'text': docling_result, 'time': docling_time, 'structure': doc.document.body}
    }
```

#### 1.3 Quality Assessment
- [ ] Process 10-20 sample PDFs with both methods
- [ ] Compare text extraction quality
- [ ] Evaluate structure preservation
- [ ] Measure processing time differences
- [ ] Document findings in `DOCLING_POC_RESULTS.md`

### Success Criteria
- Docling extracts more structured information than pdftotext
- Processing time is acceptable (< 30 seconds for typical documents)
- Clear improvement in content quality for complex documents

## Phase 2: Microservice Integration (Week 3-4)

### Goals
- Create Python microservice for document processing
- Integrate with existing Go backend
- Implement fallback mechanism

### Architecture

#### 2.1 Python Microservice (`docling-service/`)
```
docling-service/
├── app.py              # FastAPI application
├── models.py           # Data models
├── extractor.py        # Docling integration
├── requirements.txt    # Dependencies
├── Dockerfile         # Container configuration
└── docker-compose.yml # Service orchestration
```

#### 2.2 FastAPI Service Design
```python
# app.py
from fastapi import FastAPI, UploadFile, File
from pydantic import BaseModel
from typing import List, Optional

class DocumentElement(BaseModel):
    element_type: str  # 'heading', 'paragraph', 'table', 'figure'
    content: str
    page_number: int
    structure_data: Optional[dict] = None
    bbox: Optional[dict] = None  # Bounding box coordinates

class ExtractionResult(BaseModel):
    success: bool
    elements: List[DocumentElement]
    metadata: dict
    processing_time: float
    error_message: Optional[str] = None

@app.post("/extract", response_model=ExtractionResult)
async def extract_document(file: UploadFile = File(...)):
    # Implementation using docling
    pass
```

#### 2.3 Go Service Integration
Update `pkg/extractor/pdf.go`:
```go
type DoclingClient struct {
    baseURL string
    timeout time.Duration
}

func (e *PDFExtractor) extractWithDocling(ctx context.Context, filePath string) (*ExtractedContent, error) {
    // HTTP client to call docling service
    // Parse structured response
    // Convert to ExtractedContent format
}
```

#### 2.4 Database Schema Updates
```sql
-- New table for document elements
CREATE TABLE document_elements (
    id SERIAL PRIMARY KEY,
    file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
    element_type VARCHAR(50) NOT NULL, -- 'heading', 'paragraph', 'table', 'figure', 'list'
    content TEXT NOT NULL,
    page_number INTEGER,
    structure_data JSONB, -- Hierarchy info, table structure, etc.
    bbox JSONB,           -- Bounding box: {"x": 100, "y": 200, "width": 300, "height": 50}
    parent_element_id INTEGER REFERENCES document_elements(id),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_document_elements_file_id ON document_elements(file_id);
CREATE INDEX idx_document_elements_type ON document_elements(element_type);
CREATE INDEX idx_document_elements_page ON document_elements(page_number);
CREATE INDEX idx_document_elements_structure ON document_elements USING GIN (structure_data);

-- Update chunks table to reference document elements
ALTER TABLE chunks ADD COLUMN element_id INTEGER REFERENCES document_elements(id);
```

#### 2.5 Configuration Updates
Update `internal/config/config.go`:
```go
type Config struct {
    // ... existing fields
    
    // Docling service configuration
    DoclingEnabled      bool   `env:"DOCLING_ENABLED" envDefault:"false"`
    DoclingServiceURL   string `env:"DOCLING_SERVICE_URL" envDefault:"http://localhost:8081"`
    DoclingTimeout      int    `env:"DOCLING_TIMEOUT" envDefault:"300"`
    DoclingFallback     bool   `env:"DOCLING_FALLBACK" envDefault:"true"`
}
```

### Tasks
- [ ] Create Python microservice with FastAPI
- [ ] Implement document processing endpoints
- [ ] Add Go HTTP client for docling service
- [ ] Create database migration scripts
- [ ] Update extractor manager to use docling
- [ ] Implement fallback mechanism
- [ ] Add comprehensive error handling
- [ ] Create Docker containers for service

### Success Criteria
- Docling service processes documents successfully
- Go backend integrates seamlessly with fallback
- Database stores structured document elements
- No breaking changes to existing functionality

## Phase 3: Enhanced Search Features (Week 5-6)

### Goals
- Implement structure-aware search capabilities
- Add element-type filtering
- Enable spatial/page-based search

### Search Enhancements

#### 3.1 Query Processing Updates
Update `internal/search/query.go`:
```go
type ProcessedQuery struct {
    // ... existing fields
    
    // New docling-specific filters
    ElementTypes    []string `json:"element_types"`    // 'heading', 'table', 'figure'
    PageNumbers     []int    `json:"page_numbers"`     // Specific pages
    StructureQuery  string   `json:"structure_query"`  // Query within structure
}

// New filter extraction patterns
// "type:table", "page:5", "element:heading", "section:introduction"
```

#### 3.2 Search Engine Updates
Update `internal/search/engine.go`:
```go
func (e *Engine) searchDocumentElements(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
    query := `
        SELECT de.*, f.path, f.filename
        FROM document_elements de
        JOIN files f ON de.file_id = f.id
        WHERE ($1 = '' OR de.element_type = ANY($1))
          AND ($2 = 0 OR de.page_number = $2)
          AND to_tsvector('english', de.content) @@ plainto_tsquery('english', $3)
    `
    // Implementation
}
```

#### 3.3 New Search Types
```go
// Add to SearchRequest
type SearchRequest struct {
    // ... existing fields
    
    ElementTypes []string `json:"element_types,omitempty"`
    PageNumber   int      `json:"page_number,omitempty"`
    SearchMode   string   `json:"search_mode,omitempty"` // "standard", "structure_aware", "spatial"
}
```

#### 3.4 API Endpoint Updates
Update `internal/api/handlers.go`:
```go
// New endpoint for structure-aware search
func (s *Server) handleStructuredSearch(w http.ResponseWriter, r *http.Request) {
    // Parse structured search request
    // Call enhanced search engine
    // Return structured results with element metadata
}

// Enhanced search results
type StructuredSearchResult struct {
    SearchResult
    ElementType   string                 `json:"element_type"`
    PageNumber    int                    `json:"page_number"`
    StructureData map[string]interface{} `json:"structure_data"`
    BoundingBox   *BoundingBox          `json:"bounding_box,omitempty"`
}
```

### Tasks
- [ ] Implement element-type query filters
- [ ] Add page-number search capabilities
- [ ] Create structure-aware search endpoint
- [ ] Update search result formatting
- [ ] Add spatial search for bounding boxes
- [ ] Implement hierarchical result grouping
- [ ] Update search caching for new query types

### Success Criteria
- Users can search by document element type
- Page-specific search works correctly
- Results include rich structural metadata
- Performance remains acceptable for complex queries

## Phase 4: Multi-format Support (Week 7-8)

### Goals
- Add DOCX and PPTX support
- Implement format-specific processing
- Enhanced metadata extraction

### Format Support

#### 4.1 DOCX Processing
```python
# docling-service/extractors/docx_extractor.py
class DocxExtractor:
    def extract(self, file_path: str) -> ExtractionResult:
        # Use docling to process DOCX
        # Extract paragraphs, headings, tables, images
        # Preserve document structure and styles
        # Return structured elements
```

#### 4.2 PPTX Processing
```python
# docling-service/extractors/pptx_extractor.py
class PptxExtractor:
    def extract(self, file_path: str) -> ExtractionResult:
        # Process presentation slides
        # Extract slide titles, content, notes
        # Handle embedded objects and images
        # Maintain slide sequence and hierarchy
```

#### 4.3 Go Service Updates
Update scanner configuration in `internal/service/service.go`:
```go
SupportedTypes: []string{
    // Documents
    ".pdf", ".doc", ".docx",
    ".ppt", ".pptx", ".odp",
    // Spreadsheets
    ".xls", ".xlsx", ".csv", ".ods",
    // Text files
    ".txt", ".md", ".rtf",
    // Code files (existing)
    ".py", ".js", ".ts", ".jsx", ".tsx", ".java",
    ".cpp", ".c", ".go", ".rs", ".json", ".yaml", ".yml",
    ".h", ".hpp", ".css", ".html",
},
```

#### 4.4 File Type Detection Updates
Update `pkg/extractor/fileutil.go`:
```go
// Remove DOCX/PPTX from binary extensions since we can now process them
func isBinaryExtension(filePath string) bool {
    ext := strings.ToLower(filepath.Ext(filePath))
    binaryExtensions := map[string]bool{
        // Remove these - now supported by docling
        // ".docx": true, ".pptx": true,
        
        // Keep truly binary formats
        ".exe": true, ".dll": true, ".so": true,
        // ... other binary formats
    }
    
    return binaryExtensions[ext]
}
```

### Tasks
- [ ] Extend docling service for DOCX/PPTX
- [ ] Update file type classification
- [ ] Add format-specific metadata extraction
- [ ] Implement presentation-specific search features
- [ ] Add document-specific chunking strategies
- [ ] Create format-specific result templates
- [ ] Update file scanner configuration

### Success Criteria
- DOCX and PPTX files are successfully indexed
- Format-specific metadata is extracted and searchable
- Presentation slide structure is preserved
- Document formatting and styles are captured

## Phase 5: Production Optimization (Week 9-10)

### Goals
- Performance optimization and monitoring
- Production deployment configuration
- Comprehensive testing and error handling

### Optimization Areas

#### 5.1 Performance Improvements
```python
# Caching strategy
class DocumentCache:
    def __init__(self):
        self.redis_client = redis.Redis()
        self.ttl = 3600  # 1 hour
    
    def get_processed_document(self, file_hash: str):
        # Check if document already processed
        # Return cached results if available
    
    def cache_processed_document(self, file_hash: str, result: ExtractionResult):
        # Cache processing results
        # Set appropriate TTL
```

#### 5.2 Monitoring and Metrics
```python
# Metrics collection
from prometheus_client import Counter, Histogram, Gauge

DOCUMENTS_PROCESSED = Counter('documents_processed_total', ['format', 'status'])
PROCESSING_TIME = Histogram('document_processing_seconds', ['format'])
QUEUE_SIZE = Gauge('processing_queue_size')
```

#### 5.3 Error Handling and Recovery
```go
// Enhanced error handling in Go service
func (e *PDFExtractor) extractWithFallback(ctx context.Context, filePath string) (*ExtractedContent, error) {
    // Try docling first
    result, err := e.extractWithDocling(ctx, filePath)
    if err != nil {
        e.log.WithError(err).Warn("Docling extraction failed, falling back to pdftotext")
        
        // Fallback to pdftotext
        return e.extractWithPDFToText(ctx, filePath)
    }
    
    return result, nil
}
```

### Tasks
- [ ] Implement document result caching
- [ ] Add comprehensive monitoring and metrics
- [ ] Create health check endpoints
- [ ] Implement circuit breaker pattern
- [ ] Add retry mechanisms with exponential backoff
- [ ] Create performance benchmarking suite
- [ ] Optimize database queries and indexes
- [ ] Implement document processing queue
- [ ] Add comprehensive logging and tracing
- [ ] Create deployment automation

### Success Criteria
- System handles production workloads efficiently
- Comprehensive monitoring and alerting in place
- Graceful degradation when services are unavailable
- Sub-second response times for cached documents

## Implementation Timeline

| Phase | Duration | Key Deliverables |
|-------|----------|-----------------|
| Phase 1: POC | 2 weeks | Docling evaluation, comparison results |
| Phase 2: Integration | 2 weeks | Microservice, Go integration, database updates |
| Phase 3: Enhanced Search | 2 weeks | Structure-aware search, element filtering |
| Phase 4: Multi-format | 2 weeks | DOCX/PPTX support, format-specific features |
| Phase 5: Production | 2 weeks | Optimization, monitoring, deployment |

**Total Timeline: 10 weeks**

## Resource Requirements

### Infrastructure
- **Python service**: 2-4 GB RAM, 2 CPU cores
- **Model storage**: ~2 GB for docling ML models
- **Database**: Additional 20-30% storage for structured elements
- **Monitoring**: Prometheus/Grafana stack

### Dependencies
- **Python 3.8+** with docling and ML dependencies
- **Redis** for caching processed documents
- **Additional network bandwidth** for service communication

## Risk Assessment and Mitigation

### High Risks
1. **Performance degradation**: Docling is slower than pdftotext
   - *Mitigation*: Caching, async processing, fallback mechanisms

2. **Resource consumption**: High memory/CPU usage
   - *Mitigation*: Resource limits, horizontal scaling, queue management

3. **Service complexity**: Additional service to maintain
   - *Mitigation*: Comprehensive monitoring, health checks, documentation

### Medium Risks
1. **Model dependencies**: ML models may change or become unavailable
   - *Mitigation*: Version pinning, local model storage

2. **Format support gaps**: Some documents may not process correctly
   - *Mitigation*: Fallback extraction, comprehensive testing

## Success Metrics

### Quantitative Metrics
- **Extraction quality**: 15-20% improvement in content structure preservation
- **Search relevance**: 10-15% improvement in search result accuracy
- **Format coverage**: Support for 3+ additional document formats
- **Processing time**: <30 seconds for typical documents

### Qualitative Metrics
- **User satisfaction**: Better search results for complex documents
- **Developer experience**: Easier to add new document formats
- **System reliability**: Graceful fallback and error handling

## Rollback Plan

### Immediate Rollback (if critical issues)
1. Disable docling service via feature flag
2. Revert to pdftotext-only extraction
3. Database remains compatible (new tables are additive)

### Gradual Rollback (if performance issues)
1. Route only specific file types to docling
2. Implement A/B testing for extraction methods
3. Monitor metrics and adjust traffic routing

## Future Enhancements

### Potential Phase 6+ Features
- **Visual element search**: Search within extracted images and diagrams
- **Advanced table queries**: SQL-like queries on extracted table data
- **Multi-language support**: Enhanced OCR for non-English documents
- **Real-time processing**: WebSocket-based document processing updates
- **ML-powered summarization**: Auto-generate document summaries
- **Collaborative features**: Document annotations and sharing

---

## Getting Started

1. **Review this plan** with the development team
2. **Set up development environment** for Phase 1 POC
3. **Allocate resources** and establish timeline
4. **Begin Phase 1** with sample document processing
5. **Iterate and refine** based on POC results

This plan provides a comprehensive roadmap for integrating Docling while maintaining system stability and providing clear value to users through enhanced document processing capabilities.