import { useState, useEffect } from 'react'

function SettingsPage() {
  const [config, setConfig] = useState<any>(null)
  const [originalConfig, setOriginalConfig] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [resetting, setResetting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [databaseStats, setDatabaseStats] = useState<any>(null)
  const [showResetConfirm, setShowResetConfirm] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)

  useEffect(() => {
    loadConfig()
    loadDatabaseStats()
    
    // Set up periodic updates for database stats (every 5 seconds)
    const interval = setInterval(() => {
      loadDatabaseStats()
    }, 5000)
    
    return () => clearInterval(interval)
  }, [])

  const loadConfig = async () => {
    try {
      setLoading(true)
      const configStr = await window.go.main.App.GetConfig()
      const configObj = JSON.parse(configStr)
      setConfig(configObj)
      setOriginalConfig(JSON.parse(configStr)) // Store a separate copy
      setHasChanges(false)
    } catch (error) {
      console.error('Failed to load config:', error)
      setMessage('Failed to load configuration')
    } finally {
      setLoading(false)
    }
  }

  const loadDatabaseStats = async () => {
    try {
      const status = await window.go.main.App.GetSystemStatus()
      console.log('Settings page received status:', status)
      console.log('Status.total_files:', status.total_files)
      console.log('Status.indexed_files:', status.indexed_files)
      if (status) {
        const stats = {
          totalFiles: status.total_files || 0,
          indexedFiles: status.indexed_files || 0,
          pendingFiles: status.pending_files || 0,
          failedFiles: status.failed_files || 0
        }
        console.log('Setting database stats to:', stats)
        setDatabaseStats(stats)
      }
    } catch (error) {
      console.error('Failed to load database stats:', error)
    }
  }

  const saveConfig = async () => {
    if (!config) return

    try {
      setSaving(true)
      await window.go.main.App.UpdateConfig(JSON.stringify(config))
      setMessage('Configuration saved successfully!')
      setOriginalConfig({...config}) // Update original after successful save
      setHasChanges(false)
      setTimeout(() => setMessage(null), 3000)
    } catch (error) {
      console.error('Failed to save config:', error)
      setMessage('Failed to save configuration')
    } finally {
      setSaving(false)
    }
  }

  const updateConfig = (key: string, value: any) => {
    const newConfig = {
      ...config,
      [key]: value
    }
    setConfig(newConfig)
    // Check if config has changed from original
    setHasChanges(JSON.stringify(newConfig) !== JSON.stringify(originalConfig))
  }

  const handleResetDatabase = () => {
    console.log('Reset database button clicked')
    setShowResetConfirm(true)
  }

  const confirmReset = async () => {
    console.log('User confirmed reset, calling backend...')
    setShowResetConfirm(false)
    try {
      setResetting(true)
      setMessage('Resetting database...')
      console.log('About to call window.go.main.App.ResetDatabase()')
      const result = await window.go.main.App.ResetDatabase()
      console.log('ResetDatabase call completed, result:', result)
      setMessage('Database reset successfully! All indexed data has been cleared.')
      // Immediately update stats to show zeros
      setDatabaseStats({
        totalFiles: 0,
        indexedFiles: 0,
        pendingFiles: 0,
        failedFiles: 0
      })
      // Then refresh from server after a short delay
      setTimeout(async () => {
        await loadDatabaseStats()
      }, 1000)
      setTimeout(() => setMessage(null), 5000)
    } catch (error) {
      console.error('Failed to reset database - full error:', error)
      setMessage('Failed to reset database: ' + ((error as any).message || error))
    } finally {
      setResetting(false)
    }
  }

  const cancelReset = () => {
    console.log('User cancelled reset')
    setShowResetConfirm(false)
  }

  if (loading) {
    return (
      <div className="settings-page">
        <div className="settings-header">
          <h1>Settings</h1>
        </div>
        <div style={{ textAlign: 'center', padding: '50px' }}>
          Loading settings...
        </div>
      </div>
    )
  }

  if (!config) {
    return (
      <div className="settings-page">
        <div className="settings-header">
          <h1>Settings</h1>
        </div>
        <div style={{ textAlign: 'center', padding: '50px', color: '#e74c3c' }}>
          Failed to load configuration
        </div>
      </div>
    )
  }

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Settings</h1>
      </div>

      {message && (
        <div style={{
          padding: '15px',
          marginBottom: '20px',
          backgroundColor: message.includes('successfully') ? '#d4edda' : '#f8d7da',
          color: message.includes('successfully') ? '#155724' : '#721c24',
          border: `1px solid ${message.includes('successfully') ? '#c3e6cb' : '#f5c6cb'}`,
          borderRadius: '6px'
        }}>
          {message}
        </div>
      )}

      <div className="settings-grid">
        {/* Database & Connection Settings */}
        <div className="settings-card">
          <div className="card-header">
            <h3>🗄️ Database Connection</h3>
            <p>Database configuration and connection settings</p>
          </div>
          <div className="card-content">
            <div className="form-group">
              <label>Database URL</label>
              <input
                type="text"
                value={config.DatabaseURL || ''}
                onChange={(e) => updateConfig('DatabaseURL', e.target.value)}
                placeholder="postgresql://username:@localhost/dbname"
              />
            </div>
          </div>
        </div>

        {/* AI & Embeddings Settings */}
        <div className="settings-card">
          <div className="card-header">
            <h3>🤖 AI & Embeddings</h3>
            <p>Machine learning and embedding model configuration</p>
          </div>
          <div className="card-content">
            <div className="form-group">
              <label>Ollama URL</label>
              <input
                type="text"
                value={config.OllamaURL || ''}
                onChange={(e) => updateConfig('OllamaURL', e.target.value)}
                placeholder="http://localhost:11434"
              />
            </div>
            <div className="form-group">
              <label>Embedding Model</label>
              <select
                value={config.EmbeddingModel || ''}
                onChange={(e) => updateConfig('EmbeddingModel', e.target.value)}
              >
                <option value="">Select a model</option>
                <option value="nomic-embed-text">nomic-embed-text</option>
                <option value="mxbai-embed-large">mxbai-embed-large</option>
                <option value="all-minilm">all-minilm</option>
              </select>
            </div>
          </div>
        </div>

        {/* File Processing Settings */}
        <div className="settings-card">
          <div className="card-header">
            <h3>📁 File Processing</h3>
            <p>Text chunking and content processing options</p>
          </div>
          <div className="card-content">
            <div className="form-row">
              <div className="form-group">
                <label>Chunk Size</label>
                <input
                  type="number"
                  value={config.ChunkSize || 1000}
                  onChange={(e) => updateConfig('ChunkSize', parseInt(e.target.value))}
                  min="100"
                  max="2000"
                />
              </div>
              <div className="form-group">
                <label>Chunk Overlap</label>
                <input
                  type="number"
                  value={config.ChunkOverlap || 100}
                  onChange={(e) => updateConfig('ChunkOverlap', parseInt(e.target.value))}
                  min="0"
                  max="500"
                />
              </div>
            </div>
          </div>
        </div>

        {/* Performance & Limits */}
        <div className="settings-card">
          <div className="card-header">
            <h3>⚡ Performance & Limits</h3>
            <p>File size limits and scanning intervals</p>
          </div>
          <div className="card-content">
            <div className="form-row">
              <div className="form-group">
                <label>Max File Size (MB)</label>
                <input
                  type="number"
                  value={config.MaxFileSize ? config.MaxFileSize / (1024 * 1024) : 10}
                  onChange={(e) => updateConfig('MaxFileSize', parseInt(e.target.value) * 1024 * 1024)}
                  min="1"
                  max="100"
                />
              </div>
              <div className="form-group">
                <label>Scan Interval (seconds)</label>
                <input
                  type="number"
                  value={config.ScanInterval || 60}
                  onChange={(e) => updateConfig('ScanInterval', parseInt(e.target.value))}
                  min="0"
                  max="3600"
                />
              </div>
            </div>
          </div>
        </div>

        {/* Indexing Paths - Full Width */}
        <div className="settings-card full-width">
          <div className="card-header">
            <h3>📂 Indexing Configuration</h3>
            <p>Directories to index and patterns to exclude</p>
          </div>
          <div className="card-content">
            <div className="form-row">
              <div className="form-group">
                <label>Index Paths (comma-separated)</label>
                <textarea
                  value={config.IndexPaths?.join(', ') || ''}
                  onChange={(e) => updateConfig('IndexPaths', e.target.value.split(',').map((s: string) => s.trim()))}
                  placeholder="/path/to/directory1, /path/to/directory2"
                  rows={3}
                />
              </div>
              <div className="form-group">
                <label>Exclude Patterns (comma-separated)</label>
                <textarea
                  value={config.ExcludePatterns?.join(', ') || ''}
                  onChange={(e) => updateConfig('ExcludePatterns', e.target.value.split(',').map((s: string) => s.trim()))}
                  placeholder="*.tmp, node_modules/*, .git/*"
                  rows={3}
                />
              </div>
            </div>
          </div>
        </div>

      </div>

      {/* Save Configuration Button */}
      <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end', marginBottom: '30px' }}>
        <button
          className="save-button"
          onClick={saveConfig}
          disabled={saving || !hasChanges}
          style={{
            opacity: (!hasChanges || saving) ? 0.6 : 1,
            cursor: (!hasChanges || saving) ? 'not-allowed' : 'pointer'
          }}
        >
          {saving ? '💾 Saving...' : hasChanges ? '💾 Save Configuration' : '✓ Configuration Saved'}
        </button>
      </div>

      {/* Database Management Section - Separate from Configuration */}
      <div style={{ 
        marginTop: '40px',
        paddingTop: '30px',
        borderTop: '2px solid #e0e0e0'
      }}>
        <h2 style={{ marginBottom: '20px', fontSize: '20px' }}>Database Management</h2>
        
        <div className="settings-card">
          <div className="card-header">
            <h3>🗄️ Database Statistics</h3>
            <p>View and manage the search database</p>
          </div>
          <div className="card-content">
            {/* Database Statistics */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(3, 1fr)',
              gap: '15px',
              marginBottom: '20px'
            }}>
              <div style={{
                padding: '12px',
                backgroundColor: '#f8f9fa',
                borderRadius: '6px',
                textAlign: 'center'
              }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#2c3e50' }}>
                  {databaseStats?.totalFiles || 0}
                </div>
                <div style={{ fontSize: '12px', color: '#6c757d', marginTop: '4px' }}>
                  Total Files
                </div>
              </div>
              
              <div style={{
                padding: '12px',
                backgroundColor: '#f8f9fa',
                borderRadius: '6px',
                textAlign: 'center'
              }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#28a745' }}>
                  {databaseStats?.indexedFiles || 0}
                </div>
                <div style={{ fontSize: '12px', color: '#6c757d', marginTop: '4px' }}>
                  Indexed
                </div>
              </div>
              
              <div style={{
                padding: '12px',
                backgroundColor: '#f8f9fa',
                borderRadius: '6px',
                textAlign: 'center'
              }}>
                <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#ffc107' }}>
                  {databaseStats?.pendingFiles || 0}
                </div>
                <div style={{ fontSize: '12px', color: '#6c757d', marginTop: '4px' }}>
                  Pending
                </div>
              </div>
            </div>
            
            <div style={{
              padding: '15px',
              marginBottom: '15px',
              backgroundColor: '#fff3cd',
              color: '#856404',
              border: '1px solid #ffeaa7',
              borderRadius: '6px'
            }}>
              <strong>⚠️ Warning:</strong> Resetting the database will permanently delete all indexed files, 
              search data, and metadata. This action cannot be undone.
            </div>
            
            <button
              onClick={handleResetDatabase}
              disabled={resetting}
              style={{
                backgroundColor: '#dc3545',
                color: 'white',
                border: 'none',
                padding: '10px 20px',
                borderRadius: '6px',
                cursor: resetting ? 'not-allowed' : 'pointer',
                opacity: resetting ? 0.6 : 1,
                fontSize: '14px',
                fontWeight: '500'
              }}
            >
              {resetting ? '🔄 Resetting...' : '⚠️ Reset Database'}
            </button>
            
            {/* Custom Confirmation Dialog */}
            {showResetConfirm && (
              <div style={{
                position: 'fixed',
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                backgroundColor: 'rgba(0, 0, 0, 0.5)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                zIndex: 9999
              }}>
                <div style={{
                  backgroundColor: 'white',
                  padding: '30px',
                  borderRadius: '12px',
                  maxWidth: '400px',
                  boxShadow: '0 10px 40px rgba(0, 0, 0, 0.2)'
                }}>
                  <h3 style={{ marginTop: 0, marginBottom: '20px' }}>⚠️ Reset Database?</h3>
                  <p style={{ marginBottom: '15px' }}>
                    This will permanently delete all indexed files from the database.
                  </p>
                  <p style={{ marginBottom: '20px', fontWeight: 'bold' }}>
                    Currently indexed: {databaseStats?.totalFiles || 0} files
                  </p>
                  <p style={{ marginBottom: '30px', fontSize: '14px', color: '#666' }}>
                    You can re-index your files afterward.
                  </p>
                  <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end' }}>
                    <button
                      onClick={cancelReset}
                      style={{
                        padding: '10px 20px',
                        backgroundColor: '#6c757d',
                        color: 'white',
                        border: 'none',
                        borderRadius: '6px',
                        cursor: 'pointer',
                        fontSize: '14px'
                      }}
                    >
                      Cancel
                    </button>
                    <button
                      onClick={confirmReset}
                      style={{
                        padding: '10px 20px',
                        backgroundColor: '#dc3545',
                        color: 'white',
                        border: 'none',
                        borderRadius: '6px',
                        cursor: 'pointer',
                        fontSize: '14px',
                        fontWeight: 'bold'
                      }}
                    >
                      Reset Database
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default SettingsPage