-- Database migration for docling integration
-- Adds structured document elements table and related functionality

-- New table for document elements (structured content from docling)
CREATE TABLE IF NOT EXISTS document_elements (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT REFERENCES files(id) ON DELETE CASCADE,
    element_type VARCHAR(50) NOT NULL, -- 'heading', 'paragraph', 'table', 'figure', 'list', 'title', 'slide_title', 'slide_content'
    content TEXT NOT NULL,
    page_number INTEGER DEFAULT 0, -- Page number (0-indexed, 0 for non-paginated documents)
    structure_data JSONB, -- Hierarchy info, table structure, list items, etc.
    bbox JSONB, -- Bounding box: {"x": 100, "y": 200, "width": 300, "height": 50}
    parent_element_id BIGINT REFERENCES document_elements(id) ON DELETE SET NULL, -- For hierarchical elements
    element_order INTEGER DEFAULT 0, -- Order within document/page
    extraction_method VARCHAR(50), -- 'docling', 'pypdfium2', 'python-docx', etc.
    metadata JSONB DEFAULT '{}'::jsonb, -- Additional element-specific metadata
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add reference from chunks to document elements
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS element_id BIGINT REFERENCES document_elements(id) ON DELETE SET NULL;

-- Add docling-specific metadata columns to files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS has_structured_content BOOLEAN DEFAULT FALSE;
ALTER TABLE files ADD COLUMN IF NOT EXISTS extraction_method VARCHAR(50);
ALTER TABLE files ADD COLUMN IF NOT EXISTS structure_version VARCHAR(20) DEFAULT '1.0';

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_document_elements_file_id ON document_elements(file_id);
CREATE INDEX IF NOT EXISTS idx_document_elements_type ON document_elements(element_type);
CREATE INDEX IF NOT EXISTS idx_document_elements_page ON document_elements(page_number);
CREATE INDEX IF NOT EXISTS idx_document_elements_order ON document_elements(element_order);
CREATE INDEX IF NOT EXISTS idx_document_elements_parent ON document_elements(parent_element_id);
CREATE INDEX IF NOT EXISTS idx_document_elements_structure ON document_elements USING GIN (structure_data);
CREATE INDEX IF NOT EXISTS idx_document_elements_bbox ON document_elements USING GIN (bbox);
CREATE INDEX IF NOT EXISTS idx_document_elements_extraction ON document_elements(extraction_method);

-- Index for chunks element reference
CREATE INDEX IF NOT EXISTS idx_chunks_element_id ON chunks(element_id);

-- Index for files with structured content
CREATE INDEX IF NOT EXISTS idx_files_structured ON files(has_structured_content) WHERE has_structured_content = TRUE;
CREATE INDEX IF NOT EXISTS idx_files_extraction_method ON files(extraction_method);

-- Full-text search index for document elements
CREATE INDEX IF NOT EXISTS idx_document_elements_content_fts ON document_elements USING GIN (to_tsvector('english', content));

-- Function to get document structure for a file
CREATE OR REPLACE FUNCTION get_document_structure(file_id_param BIGINT)
RETURNS TABLE (
    element_id BIGINT,
    element_type VARCHAR(50),
    content TEXT,
    page_number INTEGER,
    element_order INTEGER,
    parent_element_id BIGINT,
    level INTEGER
) AS $$
WITH RECURSIVE element_hierarchy AS (
    -- Root elements (no parent)
    SELECT 
        de.id as element_id,
        de.element_type,
        CASE 
            WHEN length(de.content) > 200 THEN left(de.content, 200) || '...'
            ELSE de.content
        END as content,
        de.page_number,
        de.element_order,
        de.parent_element_id,
        0 as level
    FROM document_elements de
    WHERE de.file_id = file_id_param 
      AND de.parent_element_id IS NULL
    
    UNION ALL
    
    -- Child elements
    SELECT 
        de.id as element_id,
        de.element_type,
        CASE 
            WHEN length(de.content) > 200 THEN left(de.content, 200) || '...'
            ELSE de.content
        END as content,
        de.page_number,
        de.element_order,
        de.parent_element_id,
        eh.level + 1
    FROM document_elements de
    INNER JOIN element_hierarchy eh ON de.parent_element_id = eh.element_id
    WHERE eh.level < 10 -- Prevent infinite recursion
)
SELECT * FROM element_hierarchy
ORDER BY page_number, element_order, level;
$$ LANGUAGE sql;

-- Function to search within document elements
CREATE OR REPLACE FUNCTION search_document_elements(
    search_query TEXT,
    element_types TEXT[] DEFAULT NULL,
    page_number_filter INTEGER DEFAULT NULL,
    file_id_filter BIGINT DEFAULT NULL
)
RETURNS TABLE (
    file_id BIGINT,
    file_path TEXT,
    element_id BIGINT,
    element_type VARCHAR(50),
    content TEXT,
    page_number INTEGER,
    rank REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        de.file_id,
        f.path as file_path,
        de.id as element_id,
        de.element_type,
        de.content,
        de.page_number,
        ts_rank(to_tsvector('english', de.content), plainto_tsquery('english', search_query)) as rank
    FROM document_elements de
    JOIN files f ON de.file_id = f.id
    WHERE to_tsvector('english', de.content) @@ plainto_tsquery('english', search_query)
      AND (element_types IS NULL OR de.element_type = ANY(element_types))
      AND (page_number_filter IS NULL OR de.page_number = page_number_filter)
      AND (file_id_filter IS NULL OR de.file_id = file_id_filter)
    ORDER BY rank DESC, de.file_id, de.page_number, de.element_order;
END;
$$ LANGUAGE plpgsql;

-- Function to get element statistics for a file
CREATE OR REPLACE FUNCTION get_element_stats(file_id_param BIGINT)
RETURNS TABLE (
    element_type VARCHAR(50),
    count BIGINT,
    total_chars BIGINT,
    pages INTEGER[]
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        de.element_type,
        COUNT(*) as count,
        SUM(length(de.content)) as total_chars,
        array_agg(DISTINCT de.page_number ORDER BY de.page_number) as pages
    FROM document_elements de
    WHERE de.file_id = file_id_param
    GROUP BY de.element_type
    ORDER BY count DESC;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update has_structured_content flag when document elements are added
CREATE OR REPLACE FUNCTION update_structured_content_flag() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE files 
        SET has_structured_content = TRUE 
        WHERE id = NEW.file_id AND has_structured_content = FALSE;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        -- Check if this was the last element for this file
        IF NOT EXISTS (SELECT 1 FROM document_elements WHERE file_id = OLD.file_id AND id != OLD.id) THEN
            UPDATE files 
            SET has_structured_content = FALSE 
            WHERE id = OLD.file_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER document_elements_structured_flag
    AFTER INSERT OR DELETE ON document_elements
    FOR EACH ROW
    EXECUTE FUNCTION update_structured_content_flag();

-- Update indexing stats function to include document elements
CREATE OR REPLACE FUNCTION update_indexing_stats() RETURNS void AS $$
BEGIN
    UPDATE indexing_stats SET
        total_files = (SELECT COUNT(*) FROM files),
        indexed_files = (SELECT COUNT(*) FROM files WHERE indexing_status = 'completed'),
        failed_files = (SELECT COUNT(*) FROM files WHERE indexing_status = 'error'),
        total_chunks = (SELECT COUNT(*) FROM chunks),
        total_size_bytes = (SELECT COALESCE(SUM(size_bytes), 0) FROM files),
        last_updated = NOW()
    WHERE id = 1;
    
    -- Add document elements stats if they don't exist
    INSERT INTO indexing_stats (id, total_files, indexed_files, failed_files, total_chunks, total_size_bytes)
    SELECT 2, 0, 0, 0, COUNT(*), 0
    FROM document_elements
    WHERE NOT EXISTS (SELECT 1 FROM indexing_stats WHERE id = 2);
    
    -- Update document elements stats
    UPDATE indexing_stats SET
        total_files = (SELECT COUNT(DISTINCT file_id) FROM document_elements),
        indexed_files = (SELECT COUNT(DISTINCT file_id) FROM document_elements),
        failed_files = 0,
        total_chunks = (SELECT COUNT(*) FROM document_elements),
        last_updated = NOW()
    WHERE id = 2;
END;
$$ LANGUAGE plpgsql;

-- Add comments for new schema elements
COMMENT ON TABLE document_elements IS 'Structured document elements extracted by docling service';
COMMENT ON COLUMN document_elements.element_type IS 'Type of element: heading, paragraph, table, figure, list, title, slide_title, slide_content';
COMMENT ON COLUMN document_elements.structure_data IS 'Hierarchical and structural metadata (table structure, list items, etc.)';
COMMENT ON COLUMN document_elements.bbox IS 'Bounding box coordinates for spatial search';
COMMENT ON COLUMN document_elements.parent_element_id IS 'Reference to parent element for hierarchical structure';
COMMENT ON COLUMN document_elements.element_order IS 'Order of element within document or page';

COMMENT ON COLUMN files.has_structured_content IS 'Whether file has been processed for structured content extraction';
COMMENT ON COLUMN files.extraction_method IS 'Method used for content extraction (docling, pypdfium2, etc.)';
COMMENT ON COLUMN files.structure_version IS 'Version of structure extraction schema used';

COMMENT ON FUNCTION get_document_structure(BIGINT) IS 'Returns hierarchical structure of document elements for a file';
COMMENT ON FUNCTION search_document_elements(TEXT, TEXT[], INTEGER, BIGINT) IS 'Full-text search within document elements with filtering';
COMMENT ON FUNCTION get_element_stats(BIGINT) IS 'Returns statistics about element types in a document';

-- Insert sample element types for reference
INSERT INTO indexing_rules (path_pattern, priority, file_patterns, exclude_patterns) VALUES
    ('*/enhanced_documents/*', 1, ARRAY['*.pdf', '*.docx', '*.pptx'], ARRAY['~*', '.*', '*.tmp'])
ON CONFLICT (path_pattern) DO NOTHING;

-- Success message
DO $$
BEGIN
    RAISE NOTICE 'Docling migration completed successfully!';
    RAISE NOTICE 'New table: document_elements';
    RAISE NOTICE 'Added columns: chunks.element_id, files.has_structured_content, files.extraction_method, files.structure_version';
    RAISE NOTICE 'New functions: get_document_structure(), search_document_elements(), get_element_stats()';
    RAISE NOTICE 'Indexes and triggers configured for structured content';
END $$;