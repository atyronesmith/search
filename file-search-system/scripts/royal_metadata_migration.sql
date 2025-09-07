-- Royal Metadata Enhancement Migration
-- This migration adds comprehensive metadata fields for enhanced search capabilities

BEGIN;

-- Add new columns to files table for royal metadata
ALTER TABLE files ADD COLUMN IF NOT EXISTS royal_metadata JSONB DEFAULT '{}';

-- Create indexes for the new metadata fields
CREATE INDEX IF NOT EXISTS idx_files_royal_metadata ON files USING GIN (royal_metadata);

-- Add specific indexes for common metadata queries
CREATE INDEX IF NOT EXISTS idx_files_document_type ON files ((royal_metadata->>'document_type'));
CREATE INDEX IF NOT EXISTS idx_files_department ON files ((royal_metadata->>'department'));
CREATE INDEX IF NOT EXISTS idx_files_project_name ON files ((royal_metadata->>'project_name'));
CREATE INDEX IF NOT EXISTS idx_files_language ON files ((royal_metadata->>'language'));
CREATE INDEX IF NOT EXISTS idx_files_content_date ON files ((royal_metadata->>'content_date'));

-- Create a view for easy metadata access
CREATE OR REPLACE VIEW file_metadata_view AS
SELECT 
    f.id,
    f.path,
    f.filename,
    f.file_type,
    f.created_at,
    f.modified_at,
    f.size_bytes,
    
    -- Core Document Metadata
    royal_metadata->>'filename' as doc_filename,
    royal_metadata->>'file_type' as doc_file_type,
    (royal_metadata->>'file_size')::bigint as doc_file_size,
    (royal_metadata->>'page_count')::integer as page_count,
    (royal_metadata->>'word_count')::integer as word_count,
    royal_metadata->>'language' as language,
    royal_metadata->>'encoding' as encoding,
    
    -- Temporal Metadata
    (royal_metadata->>'created_date')::timestamp as doc_created_date,
    (royal_metadata->>'modified_date')::timestamp as doc_modified_date,
    (royal_metadata->>'processed_date')::timestamp as processed_date,
    (royal_metadata->>'content_date')::timestamp as content_date,
    royal_metadata->'date_range' as date_range,
    
    -- Structural Metadata
    royal_metadata->>'title' as title,
    royal_metadata->'authors' as authors,
    royal_metadata->'section_headers' as section_headers,
    (royal_metadata->>'table_count')::integer as table_count,
    (royal_metadata->>'image_count')::integer as image_count,
    (royal_metadata->>'has_table_of_contents')::boolean as has_toc,
    royal_metadata->>'document_type' as document_type,
    royal_metadata->'categories' as categories,
    
    -- Content-Derived Metadata
    royal_metadata->>'summary' as summary,
    royal_metadata->'key_entities' as key_entities,
    royal_metadata->'topics' as topics,
    royal_metadata->'keywords' as keywords,
    royal_metadata->>'department' as department,
    royal_metadata->>'project_name' as project_name,
    royal_metadata->>'document_class' as document_class,
    (royal_metadata->>'confidence_score')::float as confidence_score,
    
    -- Hierarchical Metadata
    royal_metadata->>'parent_document' as parent_document,
    royal_metadata->>'section_path' as section_path,
    (royal_metadata->>'depth_level')::integer as depth_level,
    royal_metadata->>'element_type' as element_type,
    
    -- Full royal metadata for complex queries
    royal_metadata as full_metadata
FROM files f;

-- Create function to update royal metadata
CREATE OR REPLACE FUNCTION update_royal_metadata(
    file_id bigint,
    metadata_json jsonb
) RETURNS void AS $$
BEGIN
    UPDATE files 
    SET royal_metadata = royal_metadata || metadata_json,
        last_indexed = now()
    WHERE id = file_id;
END;
$$ LANGUAGE plpgsql;

-- Create function to search by metadata
CREATE OR REPLACE FUNCTION search_by_royal_metadata(
    search_params jsonb
) RETURNS TABLE(
    file_id bigint,
    path text,
    filename text,
    relevance_score float
) AS $$
DECLARE
    query_conditions text := '';
    param_count int := 0;
BEGIN
    -- Build dynamic query based on search parameters
    IF search_params ? 'document_type' THEN
        query_conditions := query_conditions || 
            CASE WHEN param_count > 0 THEN ' AND ' ELSE '' END ||
            'royal_metadata->>''document_type'' = ''' || (search_params->>'document_type') || '''';
        param_count := param_count + 1;
    END IF;
    
    IF search_params ? 'department' THEN
        query_conditions := query_conditions || 
            CASE WHEN param_count > 0 THEN ' AND ' ELSE '' END ||
            'royal_metadata->>''department'' = ''' || (search_params->>'department') || '''';
        param_count := param_count + 1;
    END IF;
    
    IF search_params ? 'project_name' THEN
        query_conditions := query_conditions || 
            CASE WHEN param_count > 0 THEN ' AND ' ELSE '' END ||
            'royal_metadata->>''project_name'' = ''' || (search_params->>'project_name') || '''';
        param_count := param_count + 1;
    END IF;
    
    IF search_params ? 'date_from' AND search_params ? 'date_to' THEN
        query_conditions := query_conditions || 
            CASE WHEN param_count > 0 THEN ' AND ' ELSE '' END ||
            '(royal_metadata->>''content_date'')::timestamp BETWEEN ''' || 
            (search_params->>'date_from') || ''' AND ''' || (search_params->>'date_to') || '''';
        param_count := param_count + 1;
    END IF;
    
    -- If no conditions, return all files
    IF param_count = 0 THEN
        query_conditions := 'true';
    END IF;
    
    -- Execute dynamic query
    RETURN QUERY EXECUTE format('
        SELECT f.id, f.path, f.filename, 1.0::float as relevance_score
        FROM files f
        WHERE %s
        ORDER BY f.last_indexed DESC
    ', query_conditions);
END;
$$ LANGUAGE plpgsql;

-- Create aggregate functions for metadata analysis
CREATE OR REPLACE FUNCTION analyze_corpus_metadata()
RETURNS jsonb AS $$
DECLARE
    result jsonb;
BEGIN
    WITH metadata_stats AS (
        SELECT 
            array_agg(DISTINCT royal_metadata->>'document_type') FILTER (WHERE royal_metadata->>'document_type' IS NOT NULL) as doc_types,
            array_agg(DISTINCT royal_metadata->>'department') FILTER (WHERE royal_metadata->>'department' IS NOT NULL) as departments,
            array_agg(DISTINCT royal_metadata->>'project_name') FILTER (WHERE royal_metadata->>'project_name' IS NOT NULL) as projects,
            array_agg(DISTINCT royal_metadata->>'language') FILTER (WHERE royal_metadata->>'language' IS NOT NULL) as languages,
            min((royal_metadata->>'content_date')::timestamp) as min_date,
            max((royal_metadata->>'content_date')::timestamp) as max_date,
            count(*) as total_files
        FROM files 
        WHERE royal_metadata != '{}'
    )
    SELECT jsonb_build_object(
        'document_types', COALESCE(doc_types, ARRAY[]::text[]),
        'departments', COALESCE(departments, ARRAY[]::text[]),
        'projects', COALESCE(projects, ARRAY[]::text[]),
        'languages', COALESCE(languages, ARRAY[]::text[]),
        'date_range', jsonb_build_object(
            'start', min_date,
            'end', max_date
        ),
        'total_files', total_files,
        'analysis_date', now()
    ) INTO result
    FROM metadata_stats;
    
    RETURN COALESCE(result, '{}'::jsonb);
END;
$$ LANGUAGE plpgsql;

-- Create materialized view for fast metadata queries
CREATE MATERIALIZED VIEW IF NOT EXISTS royal_metadata_summary AS
SELECT 
    royal_metadata->>'document_type' as document_type,
    royal_metadata->>'department' as department,
    royal_metadata->>'project_name' as project_name,
    royal_metadata->>'language' as language,
    DATE_TRUNC('month', (royal_metadata->>'content_date')::timestamp) as content_month,
    COUNT(*) as file_count,
    AVG((royal_metadata->>'confidence_score')::float) as avg_confidence,
    AVG((royal_metadata->>'word_count')::integer) as avg_word_count
FROM files 
WHERE royal_metadata != '{}' AND royal_metadata->>'document_type' IS NOT NULL
GROUP BY 
    royal_metadata->>'document_type',
    royal_metadata->>'department', 
    royal_metadata->>'project_name',
    royal_metadata->>'language',
    DATE_TRUNC('month', (royal_metadata->>'content_date')::timestamp);

CREATE UNIQUE INDEX IF NOT EXISTS idx_royal_metadata_summary 
ON royal_metadata_summary (document_type, department, project_name, language, content_month);

-- Function to refresh the materialized view
CREATE OR REPLACE FUNCTION refresh_royal_metadata_summary()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY royal_metadata_summary;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically refresh summary when files are updated
CREATE OR REPLACE FUNCTION trigger_refresh_royal_metadata()
RETURNS trigger AS $$
BEGIN
    -- Refresh in background (you might want to use pg_background or similar)
    PERFORM refresh_royal_metadata_summary();
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Create trigger (only if you want automatic refresh - can be heavy)
-- DROP TRIGGER IF EXISTS royal_metadata_refresh_trigger ON files;
-- CREATE TRIGGER royal_metadata_refresh_trigger
--     AFTER INSERT OR UPDATE OF royal_metadata ON files
--     FOR EACH STATEMENT
--     EXECUTE FUNCTION trigger_refresh_royal_metadata();

-- Add comments for documentation
COMMENT ON COLUMN files.royal_metadata IS 'Comprehensive document metadata following the Royal Metadata Schema';
COMMENT ON VIEW file_metadata_view IS 'Structured view of royal metadata for easy querying';
COMMENT ON FUNCTION update_royal_metadata IS 'Updates royal metadata for a specific file';
COMMENT ON FUNCTION search_by_royal_metadata IS 'Searches files based on royal metadata criteria';
COMMENT ON FUNCTION analyze_corpus_metadata IS 'Analyzes metadata patterns across the document corpus';
COMMENT ON MATERIALIZED VIEW royal_metadata_summary IS 'Aggregated statistics of royal metadata for fast queries';

COMMIT;