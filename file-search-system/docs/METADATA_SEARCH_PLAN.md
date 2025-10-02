# Plan: Incorporate Rich Metadata into Search Pipeline

## Current State (Updated: 2025-09-11)
We now extract extensive metadata from documents via Unstructured.io:
- **Document structure**: headers, paragraphs, lists, tables
- **Royal metadata**: titles, authors, dates, categories, languages
- **File properties**: creation/modification dates, file types, sizes
- **Content metadata**: page numbers, sections, emphasis levels
- ✅ **Element-based chunking**: Implemented! Each chunk now preserves element type, emphasis scores, and hierarchy
- ✅ **Enhanced Search Ranking**: Metadata-aware scoring boosts titles and headers in search results

## Progress Tracker
- ✅ Phase 1: Database Schema Enhancement - COMPLETED
  - ✅ Phase 1.1: Element metadata columns added to chunks table
  - ✅ Phase 1.2: Element-based Chunking - Fully implemented and tested
- ✅ Phase 2: Enhanced Search Integration - Completed 2025-09-11
  - Added emphasis_score, element_type, is_title, is_header to Result struct
  - Updated vector and text search queries to include element metadata
  - Implemented metadata-aware scoring with emphasis boost (2x for titles, 1.5x for headers)
  - Adjusted hybrid search weights: Vector(40%), BM25(35%), Metadata(15%)
  - Tested with element_test.md file confirming proper boost
- ✅ Phase 3: Search Ranking Enhancement - Completed 2025-09-11
  - Metadata-aware scoring integrated into calculateMetadataScore()
  - Hybrid search weights properly adjusted
  - Element emphasis working in production
- ⏳ Phase 4: Advanced Filtering - Next up
- ⏳ Phase 5: Smart Result Grouping
- ⏳ Phase 6: LLM Enhancement Integration
- ⏳ Phase 7: Search UI Enhancements

## Phase 1: Database Schema Enhancement (Week 1)

### 1.1 Add Metadata Search Tables
```sql
-- Store structured metadata for advanced filtering
CREATE TABLE file_metadata (
    file_id BIGINT PRIMARY KEY REFERENCES files(id),
    title TEXT,
    author TEXT,
    creation_date TIMESTAMP,
    modification_date TIMESTAMP,
    language VARCHAR(10),
    category VARCHAR(100),
    document_type VARCHAR(50),
    page_count INTEGER,
    word_count INTEGER,
    has_tables BOOLEAN,
    has_images BOOLEAN,
    metadata_json JSONB -- Full metadata for complex queries
);

-- Index for fast metadata queries
CREATE INDEX idx_metadata_author ON file_metadata(author);
CREATE INDEX idx_metadata_dates ON file_metadata(creation_date, modification_date);
CREATE INDEX idx_metadata_json ON file_metadata USING GIN(metadata_json);
```

### 1.2 Add Chunk-Level Metadata
```sql
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS 
    emphasis_level INTEGER DEFAULT 0, -- 0=normal, 1=bold, 2=header, 3=title
    section_path TEXT[], -- ["Chapter 1", "Section 2", "Subsection A"]
    is_table BOOLEAN DEFAULT FALSE,
    is_list BOOLEAN DEFAULT FALSE,
    element_type VARCHAR(50); -- paragraph, header, list_item, table_cell, etc.
```

## Phase 2: Metadata Extraction & Storage (Week 1-2)

### 2.1 Enhanced Metadata Processor
```go
// internal/service/metadata_processor.go
type MetadataProcessor struct {
    db *database.DB
    log *logrus.Logger
}

func (m *MetadataProcessor) ProcessRoyalMetadata(fileID int64, metadata map[string]interface{}) error {
    // Extract structured fields
    extracted := ExtractStructuredMetadata(metadata)
    
    // Store in file_metadata table
    err := m.storeFileMetadata(fileID, extracted)
    
    // Update chunks with element-level metadata
    err = m.updateChunkMetadata(fileID, metadata)
    
    return err
}
```

### 2.2 Update Service Pipeline
- Modify `processFileComplete()` to call metadata processor after extraction
- Parse Unstructured's element types (NarrativeText, Title, Header, Table, etc.)
- Map emphasis levels from royal metadata to chunk importance

## Phase 3: Search Ranking Enhancement (Week 2) ✅ COMPLETED

### 3.1 Metadata-Aware Scoring ✅ IMPLEMENTED
The metadata scoring has been integrated directly into `internal/search/engine.go`:
- `calculateMetadataScore()` function applies emphasis boosts
- Title elements get 2x emphasis score multiplier + 20% additional boost
- Header elements get 1.5x emphasis score multiplier + 10% additional boost
- File type, path, and extension relevance factors also included

### 3.2 Update Hybrid Search ✅ COMPLETED
- Metadata scoring integrated as component in RRF scoring
- Weights adjusted in `DefaultConfig()`: Vector(40%), BM25(35%), Metadata(15%)
- Remaining 10% reserved for future recency scoring implementation

## Phase 4: Advanced Filtering (Week 2-3)

### 4.1 Metadata Filter Syntax
Support queries like:
- `author:"John Doe" type:pdf created:2024`
- `has:tables language:en modified:last-week`
- `section:"Introduction" emphasis:high`

### 4.2 Query Processor Enhancement
```go
// internal/search/query_processor.go
func (qp *QueryProcessor) ExtractMetadataFilters(query string) *MetadataFilters {
    filters := &MetadataFilters{}
    
    // Parse author filter
    if match := authorRegex.FindStringSubmatch(query); match != nil {
        filters.Author = match[1]
    }
    
    // Parse date ranges
    if match := createdRegex.FindStringSubmatch(query); match != nil {
        filters.CreatedAfter = parseTimeExpression(match[1])
    }
    
    // Parse structural filters
    filters.HasTables = strings.Contains(query, "has:tables")
    filters.HasImages = strings.Contains(query, "has:images")
    
    return filters
}
```

## Phase 5: Smart Result Grouping (Week 3)

### 5.1 Document-Level Aggregation
- Group chunks by parent document
- Show document metadata in results (title, author, date)
- Aggregate chunk scores for document-level ranking

### 5.2 Section-Aware Results
```go
type DocumentResult struct {
    FileID      int64
    Title       string
    Author      string
    BestChunks  []ChunkResult  // Top scoring chunks
    Sections    []string       // Sections containing matches
    TotalScore  float64        // Aggregated score
}
```

## Phase 6: LLM Enhancement Integration (Week 3-4)

### 6.1 Metadata-Aware LLM Queries
- Include document metadata in LLM context
- Let LLM understand document structure and authority
- Example: "Find recent technical documents by senior authors about database optimization"

### 6.2 Intelligent Summarization
- Use section headers to structure summaries
- Prioritize content from high-emphasis sections
- Include metadata context in answer generation

## Phase 7: Search UI Enhancements (Week 4)

### 7.1 Faceted Search Display
- Show metadata facets in search results
- Allow filtering by author, date, type, language
- Display document structure in results

### 7.2 Rich Result Cards
```typescript
interface SearchResult {
    document: {
        title: string
        author: string
        date: Date
        type: string
        preview: string
    }
    matches: {
        chunk: string
        section: string
        emphasis: number
        score: number
    }[]
    metadata: {
        pageCount: number
        hasTables: boolean
        language: string
    }
}
```

## Implementation Priority

### High Priority (Do First)
- Database schema updates
- Basic metadata extraction and storage
- Emphasis-based scoring boost

### Medium Priority (Core Features)
- Metadata filtering syntax
- Document-level aggregation
- Recency scoring

### Low Priority (Nice to Have)
- LLM metadata integration
- Advanced faceted search
- Section-aware navigation

## Success Metrics
- **Search Relevance**: Measure if results improve with metadata scoring
- **Query Expressiveness**: Track usage of metadata filters
- **Performance**: Ensure <100ms query time with metadata
- **User Satisfaction**: A/B test metadata-enhanced vs. basic search

## Next Steps
1. Create database migration scripts
2. Implement metadata extraction from existing royal_metadata
3. Add metadata scoring to search engine
4. Update query processor for metadata filters
5. Test with real queries and tune weights

## Technical Implementation Notes

### Database Migration Script
```sql
-- Run this migration to add metadata support
BEGIN;

-- Create metadata table
CREATE TABLE IF NOT EXISTS file_metadata (
    file_id BIGINT PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    title TEXT,
    author TEXT,
    creation_date TIMESTAMP,
    modification_date TIMESTAMP,
    language VARCHAR(10),
    category VARCHAR(100),
    document_type VARCHAR(50),
    page_count INTEGER,
    word_count INTEGER,
    has_tables BOOLEAN DEFAULT FALSE,
    has_images BOOLEAN DEFAULT FALSE,
    metadata_json JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add chunk metadata columns
ALTER TABLE chunks 
ADD COLUMN IF NOT EXISTS emphasis_level INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS section_path TEXT[],
ADD COLUMN IF NOT EXISTS is_table BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS is_list BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS element_type VARCHAR(50);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_metadata_author ON file_metadata(author);
CREATE INDEX IF NOT EXISTS idx_metadata_dates ON file_metadata(creation_date, modification_date);
CREATE INDEX IF NOT EXISTS idx_metadata_json ON file_metadata USING GIN(metadata_json);
CREATE INDEX IF NOT EXISTS idx_chunks_emphasis ON chunks(emphasis_level);
CREATE INDEX IF NOT EXISTS idx_chunks_element_type ON chunks(element_type);

COMMIT;
```

### Metadata Extraction from Existing Data
```go
// Script to backfill metadata from existing royal_metadata
func BackfillMetadata(db *database.DB) error {
    query := `
        SELECT id, royal_metadata 
        FROM files 
        WHERE royal_metadata IS NOT NULL
    `
    
    rows, err := db.Query(context.Background(), query)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var fileID int64
        var royalMetadata json.RawMessage
        
        if err := rows.Scan(&fileID, &royalMetadata); err != nil {
            continue
        }
        
        // Extract and store structured metadata
        metadata := extractStructuredMetadata(royalMetadata)
        storeFileMetadata(db, fileID, metadata)
    }
    
    return nil
}
```

This plan leverages all the rich metadata we're now collecting to significantly enhance search quality and user experience.