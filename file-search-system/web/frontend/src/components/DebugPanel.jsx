import React, { useState, useEffect } from 'react';
import { ApiService } from '../services/ApiService';

const DebugPanel = () => {
  const [debugInfo, setDebugInfo] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(5);

  useEffect(() => {
    loadDebugInfo();
  }, []);

  useEffect(() => {
    if (autoRefresh && refreshInterval > 0) {
      const interval = setInterval(loadDebugInfo, refreshInterval * 1000);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, refreshInterval]);

  const loadDebugInfo = async () => {
    try {
      setIsLoading(true);
      // Fetch the latest search query debug info
      const response = await ApiService.getDebugInfo();
      setDebugInfo(response);
    } catch (error) {
      console.error('Failed to load debug info:', error);
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading && !debugInfo) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-semibold text-gray-900">LLM Debug Information</h2>
        <div className="flex items-center gap-4">
          <button
            onClick={loadDebugInfo}
            className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
          >
            Refresh
          </button>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 text-sm text-gray-600">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                className="rounded"
              />
              Auto-refresh every
            </label>
            <input
              type="number"
              min="1"
              max="60"
              value={refreshInterval}
              onChange={(e) => setRefreshInterval(parseInt(e.target.value) || 5)}
              className="w-16 px-2 py-1 border border-gray-300 rounded-md"
            />
            <span className="text-sm text-gray-600">seconds</span>
          </div>
        </div>
      </div>

      {/* Current Model */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4 mb-6">
        <div className="flex items-center gap-4">
          <span className="text-sm font-medium text-gray-700">Current LLM Model:</span>
          <span className="text-sm font-mono bg-gray-100 px-2 py-1 rounded">
            {debugInfo?.model || 'phi3:mini'}
          </span>
        </div>
      </div>

      {/* Query Information - Compact Layout */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 mb-6">
        <div className="px-4 py-3 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Query Information</h3>
        </div>
        <div className="p-4 space-y-2">
          <div className="grid grid-cols-2 gap-x-8 gap-y-2">
            <div className="flex">
              <span className="text-sm font-medium text-gray-600 w-32">Timestamp:</span>
              <span className="text-sm text-gray-900">{debugInfo?.timestamp || new Date().toLocaleString()}</span>
            </div>
            <div className="flex">
              <span className="text-sm font-medium text-gray-600 w-32">Processing Time:</span>
              <span className="text-sm text-gray-900">{debugInfo?.processingTime || '0'}ms</span>
            </div>
            <div className="flex">
              <span className="text-sm font-medium text-gray-600 w-32">Model Used:</span>
              <span className="text-sm text-gray-900">{debugInfo?.model || 'phi3:mini'}</span>
            </div>
            <div className="flex">
              <span className="text-sm font-medium text-gray-600 w-32">Query Type:</span>
              <span className="text-sm text-gray-900">{debugInfo?.queryType || 'analytical'}</span>
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-gray-100">
            <div className="flex">
              <span className="text-sm font-medium text-gray-600 w-32">Original Query:</span>
              <span className="text-sm text-gray-900 font-mono bg-gray-50 px-2 py-1 rounded">
                {debugInfo?.originalQuery || 'No recent query'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* LLM Prompt - Left Justified */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200">
        <div className="px-4 py-3 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">LLM Prompt</h3>
        </div>
        <div className="p-4">
          <pre className="text-sm text-gray-800 font-mono whitespace-pre-wrap text-left">
{debugInfo?.prompt || `You are an expert search term extraction system with access to document metadata for optimized hybrid search.

TASK: Generate search terms considering both the query and document metadata patterns in the corpus. For emotional or abstract queries, generate related terms, synonyms, and conceptually similar words.

USER QUERY: {query}

AVAILABLE METADATA CONTEXT:
- Document Types in Corpus: pdf, docx, xlsx, csv, txt
- Time Range of Documents: 2020-01-01 to 2025-12-31
- Common Categories: NarrativeText, Title, Table, ListItem
- Departments/Projects: engineering, finance, hr, legal, marketing
- Total Files: {fileCount}

SEARCH CONTEXT: Query type: {queryType}, Intent: {intent}

METADATA-AWARE EXTRACTION RULES:
1. If query implies time-sensitivity, emphasize temporal search terms
2. If mentions document types (report, email, memo), include type-specific terms
3. For departmental queries, include relevant organizational terms
4. Consider document hierarchy (section headers, chapters) for navigation queries

RESPONSE FORMAT:
{
  "enhanced_query": "optimized search string",
  "search_terms": ["term1", "term2", "term3"],
  "metadata_filters": {
    "file_types": ["pdf", "docx"],
    "date_range": {"from": "2024-01-01", "to": "2024-12-31"},
    "departments": ["engineering"]
  },
  "semantic_expansion": ["related1", "synonym1", "concept1"],
  "search_strategy": "vector_priority|keyword_priority|hybrid_balanced"
}`}
          </pre>
        </div>
      </div>

      {/* Enhanced Query Response */}
      {debugInfo?.enhancedQuery && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 mt-6">
          <div className="px-4 py-3 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Enhanced Query Response</h3>
          </div>
          <div className="p-4">
            <pre className="text-sm text-gray-800 font-mono whitespace-pre-wrap text-left bg-gray-50 p-3 rounded">
              {JSON.stringify(debugInfo.enhancedQuery, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};

export default DebugPanel;