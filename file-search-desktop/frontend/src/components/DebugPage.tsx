import { useState, useEffect } from 'react'
import './DebugPage.css'

interface DebugInfo {
  timestamp: string
  query: string
  model: string
  prompt: string
  response: string
  process_time_ms: number
  error?: string
  vector_query?: string
  text_query?: string
}

export default function DebugPage() {
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentModel, setCurrentModel] = useState<string>('unknown')
  const [refreshInterval, setRefreshInterval] = useState<number>(0)
  const [autoRefresh, setAutoRefresh] = useState(false)

  const fetchDebugInfo = async () => {
    setLoading(true)
    setError(null)
    try {
      const info = await window.go.main.App.GetLLMDebugInfo()
      setDebugInfo(info)
      
      // Also get current model
      const model = await window.go.main.App.GetCurrentLLMModel()
      setCurrentModel(model)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch debug info')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchDebugInfo()
  }, [])

  useEffect(() => {
    if (autoRefresh && refreshInterval > 0) {
      const interval = setInterval(fetchDebugInfo, refreshInterval * 1000)
      return () => clearInterval(interval)
    }
  }, [autoRefresh, refreshInterval])

  const formatTimestamp = (timestamp: string) => {
    try {
      const date = new Date(timestamp)
      return date.toLocaleString()
    } catch {
      return timestamp
    }
  }

  const formatJson = (jsonString: string) => {
    try {
      const parsed = JSON.parse(jsonString)
      return JSON.stringify(parsed, null, 2)
    } catch {
      return jsonString
    }
  }

  const copyToClipboard = async (text: string, buttonId: string) => {
    try {
      await navigator.clipboard.writeText(text)
      // Show feedback
      const button = document.getElementById(buttonId)
      if (button) {
        const originalText = button.textContent
        button.textContent = 'Copied!'
        setTimeout(() => {
          button.textContent = originalText
        }, 2000)
      }
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  const formatSqlQuery = (query: string) => {
    // If it already looks formatted (has newlines), return as-is
    if (query.includes('\n')) {
      return query
    }
    
    // First pass: basic replacements
    let formatted = query
      // Ensure main keywords are uppercase
      .replace(/\bselect\b/gi, 'SELECT')
      .replace(/\bfrom\b/gi, 'FROM')
      .replace(/\bjoin\b/gi, 'JOIN')
      .replace(/\binner join\b/gi, 'JOIN')
      .replace(/\bleft join\b/gi, 'JOIN')
      .replace(/\bright join\b/gi, 'JOIN')
      .replace(/\bwhere\b/gi, 'WHERE')
      .replace(/\band\b/gi, 'AND')
      .replace(/\bor\b/gi, 'OR')
      .replace(/\border by\b/gi, 'ORDER BY')
      .replace(/\bgroup by\b/gi, 'GROUP BY')
      .replace(/\blimit\b/gi, 'LIMIT')
      .replace(/\bas\b/gi, 'AS')
      .replace(/\bon\b/gi, 'ON')
    
    // Split on major SQL keywords to restructure
    const parts = formatted.split(/\b(SELECT|FROM|JOIN|WHERE|ORDER BY|GROUP BY|LIMIT)\b/)
    
    let result: string[] = []
    let currentKeyword = ''
    
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i].trim()
      
      if (['SELECT', 'FROM', 'JOIN', 'WHERE', 'ORDER BY', 'GROUP BY', 'LIMIT'].includes(part)) {
        currentKeyword = part
        result.push(part)
      } else if (part) {
        if (currentKeyword === 'SELECT') {
          // Split SELECT items by comma and indent each
          const items = part.split(',').map(item => item.trim()).filter(item => item)
          if (items.length > 0) {
            result.push('    ' + items[0])
            for (let j = 1; j < items.length; j++) {
              result.push('    ' + items[j] + ',')
            }
            // Remove comma from last item
            if (result[result.length - 1].endsWith(',')) {
              result[result.length - 1] = result[result.length - 1].slice(0, -1)
            }
          }
        } else if (currentKeyword === 'WHERE') {
          // Handle WHERE conditions
          const conditions = part.split(/\b(AND|OR)\b/)
          let firstCondition = true
          for (let j = 0; j < conditions.length; j++) {
            const cond = conditions[j].trim()
            if (cond === 'AND' || cond === 'OR') {
              result.push('    ' + cond)
            } else if (cond) {
              if (firstCondition) {
                result.push('    ' + cond)
                firstCondition = false
              } else {
                result[result.length - 1] += ' ' + cond
              }
            }
          }
        } else if (currentKeyword === 'JOIN') {
          // Handle JOIN with ON clause
          result[result.length - 1] += ' ' + part
        } else if (currentKeyword === 'FROM') {
          // Handle FROM clause
          result.push('    ' + part)
        } else if (currentKeyword === 'ORDER BY' || currentKeyword === 'GROUP BY') {
          // Handle ORDER BY and GROUP BY
          result.push('    ' + part)
        } else if (currentKeyword === 'LIMIT') {
          // Handle LIMIT on same line
          result[result.length - 1] += ' ' + part
        }
      }
    }
    
    // Clean up and join
    return result
      .filter(line => line.trim())
      .map(line => {
        // Ensure consistent spacing
        return line.replace(/\s+/g, ' ').replace(/\s*,\s*/g, ',')
      })
      .join('\n')
  }

  return (
    <div className="debug-page">
      <div className="debug-header">
        <h2>LLM Debug Information</h2>
        <div className="debug-controls">
          <button 
            onClick={fetchDebugInfo} 
            disabled={loading}
            className="refresh-button"
          >
            {loading ? 'Loading...' : 'Refresh'}
          </button>
          
          <div className="auto-refresh">
            <label>
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
              />
              Auto-refresh every
            </label>
            <input
              type="number"
              min="1"
              max="60"
              value={refreshInterval}
              onChange={(e) => setRefreshInterval(parseInt(e.target.value) || 0)}
              className="interval-input"
            />
            <span>seconds</span>
          </div>
        </div>
      </div>

      <div className="debug-content">
        {error && (
          <div className="error-message">
            Error: {error}
          </div>
        )}

        {!debugInfo && !loading && !error && (
          <div className="no-data">
            No debug information available. Run a search query with LLM enhancement to generate debug data.
          </div>
        )}

        {debugInfo && (
          <div className="debug-details">
            <div className="debug-section">
              <h3>Query Information</h3>
              <div className="query-info-grid">
                <div className="query-info-cell">
                  <div className="query-info-header">Timestamp</div>
                  <div className="query-info-textbox">{formatTimestamp(debugInfo.timestamp)}</div>
                </div>
                <div className="query-info-cell">
                  <div className="query-info-header">Processing Time</div>
                  <div className="query-info-textbox">{debugInfo.process_time_ms}ms</div>
                </div>
                <div className="query-info-cell">
                  <div className="query-info-header">Model Used</div>
                  <div className="query-info-textbox">{debugInfo.model || 'N/A'}</div>
                </div>
              </div>
              <div className="query-original">
                <strong>Original Query:</strong>
                <div className="query-text-box">{debugInfo.query}</div>
              </div>
            </div>

            {debugInfo.prompt && (
              <div className="debug-section">
                <h3>
                  LLM Prompt
                  <button 
                    id="copy-prompt"
                    className="header-copy-button"
                    onClick={() => copyToClipboard(debugInfo.prompt, 'copy-prompt')}
                  >
                    Copy
                  </button>
                </h3>
                <pre className="debug-prompt">{debugInfo.prompt}</pre>
              </div>
            )}

            {debugInfo.response && (
              <div className="debug-section">
                <h3>
                  LLM Response
                  <button 
                    id="copy-response"
                    className="header-copy-button"
                    onClick={() => copyToClipboard(formatJson(debugInfo.response), 'copy-response')}
                  >
                    Copy
                  </button>
                </h3>
                <pre className="debug-response">{formatJson(debugInfo.response)}</pre>
              </div>
            )}

            {(debugInfo.vector_query || debugInfo.text_query) && (
              <div className="debug-section">
                <h3>Processed Queries</h3>
                <div className="processed-queries-stack">
                  {debugInfo.text_query && (
                    <div className="query-block">
                      <div className="query-block-header">
                        Text Query
                        <button 
                          id="copy-text-query"
                          className="query-copy-button"
                          onClick={() => copyToClipboard(formatSqlQuery(debugInfo.text_query || ''), 'copy-text-query')}
                        >
                          Copy
                        </button>
                      </div>
                      <pre className="query-block-content">{formatSqlQuery(debugInfo.text_query)}</pre>
                    </div>
                  )}
                  {debugInfo.vector_query && (
                    <div className="query-block">
                      <div className="query-block-header">
                        Vector Query
                        <button 
                          id="copy-vector-query"
                          className="query-copy-button"
                          onClick={() => copyToClipboard(formatSqlQuery(debugInfo.vector_query || ''), 'copy-vector-query')}
                        >
                          Copy
                        </button>
                      </div>
                      <pre className="query-block-content">{formatSqlQuery(debugInfo.vector_query)}</pre>
                    </div>
                  )}
                </div>
              </div>
            )}

            {debugInfo.error && (
              <div className="debug-section error">
                <h3>Error</h3>
                <pre className="debug-error">{debugInfo.error}</pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}