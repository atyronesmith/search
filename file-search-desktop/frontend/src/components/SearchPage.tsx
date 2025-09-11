import { useState, useEffect } from 'react'

interface SearchResult {
  id: string
  path: string
  name: string
  type: string
  size: number
  modifiedAt: string
  score: number
  highlights: string[]
  snippet: string
}

interface EnhancedQuery {
  original: string
  enhanced: string
  search_terms: string[]
  intent: string
  requires_count: boolean
}

interface SearchResponseWithDetails {
  results: SearchResult[]
  enhanced_query?: EnhancedQuery
  used_llm: boolean
  search_time: number
  total_count: number
}

interface SearchPageProps {
  onSearch: (query: string, offset?: number, append?: boolean) => void
  onSearchWithDetails: (query: string, results: SearchResult[], append?: boolean) => void
  searchQuery: string
  searchResults: SearchResult[]
}

function SearchPage({ onSearch, onSearchWithDetails, searchQuery, searchResults }: SearchPageProps) {
  const [query, setQuery] = useState(searchQuery)
  const [loading, setLoading] = useState(false)
  const [isLLMQuery, setIsLLMQuery] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalResults, setTotalResults] = useState(0)
  const [enhancedQuery, setEnhancedQuery] = useState<EnhancedQuery | null>(null)
  const [usedLLM, setUsedLLM] = useState(false)
  const [currentLLMModel, setCurrentLLMModel] = useState<string>('unknown')
  const resultsPerPage = 10

  const handleSearch = async () => {
    if (!query.trim()) return
    
    setLoading(true)
    setCurrentPage(1)  // Reset to first page
    
    // Check if this might be an LLM query (conservative prediction)
    try {
      const isLLM = await window.go.main.App.IsLLMQuery(query)
      setIsLLMQuery(isLLM)
      
      // If it's an LLM query, fetch the current LLM model
      if (isLLM) {
        try {
          const model = await window.go.main.App.GetCurrentLLMModel()
          setCurrentLLMModel(model)
        } catch (modelError) {
          console.warn('Failed to get current LLM model:', modelError)
          setCurrentLLMModel('unknown')
        }
      }
    } catch (error) {
      console.warn('Failed to check LLM query status:', error)
      setIsLLMQuery(false)
    }
    
    try {
      // Try SearchWithDetails first to get enhanced query information
      try {
        const searchResponse: SearchResponseWithDetails = await window.go.main.App.SearchWithDetails({
          query: query,
          limit: resultsPerPage,
          offset: 0
        })
        
        // Update enhanced query state based on backend response
        setEnhancedQuery(searchResponse.enhanced_query || null)
        setUsedLLM(searchResponse.used_llm)
        
        // Update LLM query status based on actual backend response
        setIsLLMQuery(searchResponse.used_llm)
        
        // Use the SearchWithDetails results directly instead of calling onSearch
        // This preserves the LLM-enhanced results instead of making a second regular search call
        console.log('Using SearchWithDetails results directly, bypassing onSearch to preserve LLM enhancement')
        
        // Update parent component with the enhanced search results
        onSearchWithDetails(query, searchResponse.results || [], false)
        
      } catch (detailsError) {
        console.warn('SearchWithDetails failed, falling back to regular search:', detailsError)
        // Fallback to regular search
        await onSearch(query, 0, false)
        setEnhancedQuery(null)
        setUsedLLM(false)
      }
    } finally {
      setLoading(false)
      setIsLLMQuery(false)
    }
  }

  const handleLoadMore = async () => {
    if (!query.trim()) return
    
    setLoading(true)
    try {
      const offset = currentPage * resultsPerPage
      await onSearch(query, offset, true)
      setCurrentPage(prev => prev + 1)
    } finally {
      setLoading(false)
    }
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch()
    }
  }

  const formatFileSize = (bytes: number): string => {
    const units = ['B', 'KB', 'MB', 'GB']
    let size = bytes
    let unitIndex = 0
    
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024
      unitIndex++
    }
    
    return `${size.toFixed(1)} ${units[unitIndex]}`
  }

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString()
  }

  const getFileIcon = (type: string): string => {
    if (type.includes('code') || type.includes('javascript') || type.includes('python')) {
      return '💻'
    }
    if (type.includes('document') || type.includes('pdf')) {
      return '📄'
    }
    if (type.includes('image')) {
      return '🖼️'
    }
    if (type.includes('video')) {
      return '🎥'
    }
    if (type.includes('audio')) {
      return '🎵'
    }
    return '📄'
  }

  return (
    <div className="search-page">
      <div className="search-header">
        <h1>Search Files</h1>
        <div className="search-box">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder="Search for files, code, or content..."
            className="search-input"
            disabled={loading}
          />
          <button
            onClick={handleSearch}
            className="search-button"
            disabled={loading || !query.trim()}
          >
            {loading ? '🔄' : '🔍'} Search
          </button>
          <button
            onClick={() => setShowHelp(true)}
            className="help-button"
            title="Search Help & Syntax"
          >
            ❓
          </button>
        </div>
      </div>

      {/* Enhanced Query Display */}
      {enhancedQuery && usedLLM && !loading && (
        <div style={{
          backgroundColor: '#f8f9fa',
          border: '2px solid #007bff',
          borderRadius: '8px',
          padding: '15px',
          margin: '15px 0',
          fontSize: '14px'
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            marginBottom: '10px',
            fontWeight: 'bold',
            color: '#007bff'
          }}>
            <span style={{ fontSize: '20px', marginRight: '8px' }}>🧠</span>
            AI Enhanced Search Query
          </div>
          
          {enhancedQuery.enhanced !== enhancedQuery.original && (
            <div style={{ marginBottom: '8px' }}>
              <strong>Enhanced terms:</strong>{' '}
              <code style={{
                backgroundColor: '#e9ecef',
                padding: '2px 6px',
                borderRadius: '4px',
                fontSize: '13px'
              }}>
                {enhancedQuery.enhanced}
              </code>
            </div>
          )}
          
          {enhancedQuery.search_terms && enhancedQuery.search_terms.length > 0 && (
            <div style={{ marginBottom: '8px' }}>
              <strong>Search terms:</strong>{' '}
              {enhancedQuery.search_terms.map((term, index) => (
                <span key={index}>
                  <code style={{
                    backgroundColor: '#e9ecef',
                    padding: '2px 6px',
                    borderRadius: '4px',
                    fontSize: '13px',
                    marginRight: '4px'
                  }}>
                    {term}
                  </code>
                </span>
              ))}
            </div>
          )}
          
          {enhancedQuery.intent && (
            <div style={{ marginBottom: '8px' }}>
              <strong>Query intent:</strong>{' '}
              <span style={{
                backgroundColor: '#d4edda',
                color: '#155724',
                padding: '2px 8px',
                borderRadius: '4px',
                fontSize: '12px',
                textTransform: 'uppercase',
                fontWeight: 'bold'
              }}>
                {enhancedQuery.intent}
              </span>
            </div>
          )}
          
          {enhancedQuery.requires_count && (
            <div style={{
              fontSize: '12px',
              color: '#666',
              fontStyle: 'italic'
            }}>
              ℹ️ This query was identified as requesting a count of results
            </div>
          )}
        </div>
      )}

      {/* LLM Processing Animation */}
      {loading && isLLMQuery && (
        <div className="llm-loading-container" style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '40px 20px',
          backgroundColor: '#f8f9fa',
          border: '2px dashed #007bff',
          borderRadius: '12px',
          margin: '20px 0',
          animation: 'pulse 2s ease-in-out infinite'
        }}>
          <div style={{
            fontSize: '48px',
            marginBottom: '20px',
            animation: 'bounce 1.5s ease-in-out infinite'
          }}>
            🧠
          </div>
          <div style={{
            fontSize: '20px',
            fontWeight: 'bold',
            color: '#007bff',
            marginBottom: '10px'
          }}>
            🤖 AI Enhancement Active ({currentLLMModel})
          </div>
          <div style={{
            fontSize: '16px',
            color: '#666',
            textAlign: 'center',
            marginBottom: '20px',
            maxWidth: '500px'
          }}>
            Your natural language query is being processed by our AI system for intelligent search...
          </div>
          <div style={{
            display: 'flex',
            gap: '8px',
            marginBottom: '15px'
          }}>
            {[0, 1, 2, 3, 4].map((i) => (
              <div
                key={i}
                style={{
                  width: '12px',
                  height: '12px',
                  backgroundColor: '#007bff',
                  borderRadius: '50%',
                  animation: `wave 1.4s ease-in-out ${i * 0.2}s infinite`
                }}
              />
            ))}
          </div>
          <div style={{
            fontSize: '14px',
            color: '#888',
            fontStyle: 'italic'
          }}>
            Analyzing • Classifying • Enhancing • Searching
          </div>
          
          <style>
            {`
              @keyframes pulse {
                0%, 100% { 
                  border-color: #007bff; 
                  background-color: #f8f9fa; 
                }
                50% { 
                  border-color: #0056b3; 
                  background-color: #e3f2fd; 
                }
              }
              
              @keyframes bounce {
                0%, 20%, 50%, 80%, 100% {
                  transform: translateY(0);
                }
                40% {
                  transform: translateY(-10px);
                }
                60% {
                  transform: translateY(-5px);
                }
              }
              
              @keyframes wave {
                0%, 40%, 100% {
                  transform: scaleY(0.4);
                  opacity: 0.5;
                }
                20% {
                  transform: scaleY(1);
                  opacity: 1;
                }
              }
            `}
          </style>
        </div>
      )}

      {/* Regular Loading Animation */}
      {loading && !isLLMQuery && (
        <div style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          padding: '40px',
          fontSize: '18px',
          color: '#666'
        }}>
          <div style={{
            marginRight: '10px',
            animation: 'spin 1s linear infinite'
          }}>
            🔄
          </div>
          Searching...
          
          <style>
            {`
              @keyframes spin {
                from { transform: rotate(0deg); }
                to { transform: rotate(360deg); }
              }
            `}
          </style>
        </div>
      )}

      {searchResults.length > 0 && (
        <div className="search-results">
          <div className="results-header">
            <h2>Results ({searchResults.length})</h2>
          </div>
          
          <div className="results-list">
            {searchResults.map((result) => (
              <div key={result.id} className="result-item">
                <div className="result-header">
                  <div className="result-icon">{getFileIcon(result.type)}</div>
                  <div className="result-info">
                    <h3 className="result-name">{result.name}</h3>
                    <p className="result-path">{result.path}</p>
                  </div>
                  <div className="result-meta">
                    <span className="result-score">Score: {result.score.toFixed(2)}</span>
                    <span className="result-size">{formatFileSize(result.size)}</span>
                    <span className="result-date">{formatDate(result.modifiedAt)}</span>
                  </div>
                </div>
                
                {result.snippet && (
                  <div className="result-snippet">
                    <p>{result.snippet}</p>
                  </div>
                )}
                
                {result.highlights.length > 0 && (
                  <div className="result-highlights">
                    <h4>Highlights:</h4>
                    {result.highlights.map((highlight, index) => (
                      <pre key={index} className="highlight">
                        {highlight}
                      </pre>
                    ))}
                  </div>
                )}
                
                <div className="result-actions">
                  <button
                    onClick={() => {
                      // Open file in system default application
                      window.open(`file://${result.path}`)
                    }}
                    className="action-button"
                  >
                    Open File
                  </button>
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(result.path)
                    }}
                    className="action-button"
                  >
                    Copy Path
                  </button>
                </div>
              </div>
            ))}
          </div>
          
          {/* Load More Button */}
          <div style={{ textAlign: 'center', marginTop: '20px', padding: '20px' }}>
            <button
              onClick={handleLoadMore}
              disabled={loading}
              style={{
                padding: '12px 24px',
                backgroundColor: loading ? '#ccc' : '#007bff',
                color: 'white',
                border: 'none',
                borderRadius: '6px',
                cursor: loading ? 'not-allowed' : 'pointer',
                fontSize: '14px',
                fontWeight: '500'
              }}
            >
              {loading ? '🔄 Loading...' : '📄 Load More Results'}
            </button>
            <div style={{ 
              marginTop: '10px', 
              fontSize: '14px', 
              color: '#666' 
            }}>
              Showing {searchResults.length} results • Page {currentPage}
            </div>
          </div>
        </div>
      )}

      {query && searchResults.length === 0 && !loading && (
        <div className="no-results">
          <h3>No results found</h3>
          <p>Try a different search term or check your indexing status.</p>
        </div>
      )}

      {!query && (
        <div className="search-help">
          <h3>Getting Started</h3>
          <p>Enter a search term to find files, code, or content in your indexed directories.</p>
          <div className="search-tips">
            <h4>Search Tips:</h4>
            <ul>
              <li>Use specific keywords for better results</li>
              <li>Search for code snippets, function names, or comments</li>
              <li>File names and paths are also searchable</li>
              <li>Use the Dashboard to monitor indexing progress</li>
            </ul>
          </div>
        </div>
      )}

      {/* Search Help Modal */}
      {showHelp && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.7)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 10000
        }}>
          <div style={{
            backgroundColor: 'white',
            padding: '30px',
            borderRadius: '12px',
            maxWidth: '600px',
            maxHeight: '80vh',
            overflow: 'auto',
            boxShadow: '0 10px 40px rgba(0, 0, 0, 0.3)'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
              <h2 style={{ margin: 0 }}>🔍 Search Help & Syntax</h2>
              <button 
                onClick={() => setShowHelp(false)}
                style={{
                  background: 'none',
                  border: 'none',
                  fontSize: '24px',
                  cursor: 'pointer',
                  color: '#666'
                }}
              >
                ✕
              </button>
            </div>

            <div style={{ lineHeight: '1.6' }}>
              <h3>🔤 Basic Search</h3>
              <div style={{ marginBottom: '20px', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '4px' }}>
                <div>search term &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Simple keyword search</div>
                <div>multiple keywords &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Any of the terms</div>
                <div>"exact phrase" &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Exact phrase search ✅</div>
              </div>

              <h3>📁 File Type Filters ✅</h3>
              <div style={{ marginBottom: '20px', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '4px' }}>
                <div>type:code &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Code files only</div>
                <div>filetype:text &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Text files</div>
              </div>
              <p><strong>Available types:</strong> code, text (YAML files are classified as code)</p>

              <h3>📅 Date Filters ⚠️</h3>
              <div style={{ marginBottom: '20px', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '4px' }}>
                <div>after:2024-01-01 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Files modified after date</div>
                <div>before:2024-12-31 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Files modified before date</div>
              </div>
              <p style={{ fontSize: '14px', color: '#d73527' }}>⚠️ Date filters are partially working - results change but may not filter correctly</p>

              <h3>🔍 Boolean Operators ✅</h3>
              <div style={{ marginBottom: '20px', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '4px' }}>
                <div>term1 AND term2 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Both terms required</div>
                <div>term1 OR term2 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Either term</div>
                <div>term1 NOT term2 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# First but not second</div>
              </div>

              <h3>❌ Not Currently Working</h3>
              <div style={{ marginBottom: '20px', fontSize: '14px', color: '#666' }}>
                <p><strong>Size filters:</strong> size:&gt;10MB, size:&lt;1KB</p>
                <p><strong>Must include/exclude:</strong> +term, -term</p>
                <p><strong>Extensions:</strong> ext:py, ext:js (use type: instead)</p>
              </div>

              <h3>💡 Working Examples</h3>
              <div style={{ marginBottom: '20px', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '10px', borderRadius: '4px' }}>
                <div>"test: chart1" &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Exact phrase search</div>
                <div>resources type:code &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Search in code files only</div>
                <div>test AND chart &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;# Both terms required</div>
                <div>resources filetype:text &nbsp;&nbsp;&nbsp;# Search in text files</div>
              </div>

              <p><strong>Note:</strong> Pagination with "Load More" is available for all searches.</p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default SearchPage