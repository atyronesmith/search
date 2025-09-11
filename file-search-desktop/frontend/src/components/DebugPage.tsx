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
        <div className="model-info">
          <strong>Current LLM Model:</strong> 
          <span className="model-name">{currentModel}</span>
        </div>

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
              <div className="debug-item">
                <strong>Timestamp:</strong> {formatTimestamp(debugInfo.timestamp)}
              </div>
              <div className="debug-item">
                <strong>Original Query:</strong> 
                <pre>{debugInfo.query}</pre>
              </div>
              <div className="debug-item">
                <strong>Processing Time:</strong> {debugInfo.process_time_ms}ms
              </div>
              {debugInfo.model && (
                <div className="debug-item">
                  <strong>Model Used:</strong> {debugInfo.model}
                </div>
              )}
            </div>

            {debugInfo.prompt && (
              <div className="debug-section">
                <h3>LLM Prompt</h3>
                <pre className="debug-prompt">{debugInfo.prompt}</pre>
              </div>
            )}

            {debugInfo.response && (
              <div className="debug-section">
                <h3>LLM Response</h3>
                <pre className="debug-response">{formatJson(debugInfo.response)}</pre>
              </div>
            )}

            {(debugInfo.vector_query || debugInfo.text_query) && (
              <div className="debug-section">
                <h3>Processed Queries</h3>
                {debugInfo.vector_query && (
                  <div className="debug-item">
                    <strong>Vector Query:</strong>
                    <pre>{debugInfo.vector_query}</pre>
                  </div>
                )}
                {debugInfo.text_query && (
                  <div className="debug-item">
                    <strong>Text Query:</strong>
                    <pre>{debugInfo.text_query}</pre>
                  </div>
                )}
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