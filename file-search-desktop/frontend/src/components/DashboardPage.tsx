interface IndexingStatus {
  state: string
  filesProcessed: number
  totalFiles: number
  pendingFiles: number
  currentFile: string
  errors: number
  elapsedTime: number
}

interface SystemStatus {
  status: string
  uptime: number
  database: any
  embeddings: any
  indexing: any
  resources: any
}

interface DashboardPageProps {
  indexingStatus: IndexingStatus | null
  systemStatus: SystemStatus | null
}

function DashboardPage({ indexingStatus, systemStatus }: DashboardPageProps) {
  const handleIndexingControl = async (action: string) => {
    try {
      switch (action) {
        case 'start':
          // Use empty path to let backend use configured paths from .env (WATCH_PATHS)
          const result = await window.go.main.App.StartIndexing('')
          console.log('Indexing started:', result)
          break
        case 'stop':
          await window.go.main.App.StopIndexing()
          break
        case 'pause':
          await window.go.main.App.PauseIndexing()
          break
        case 'resume':
          await window.go.main.App.ResumeIndexing()
          break
      }
    } catch (error) {
      console.error(`Failed to ${action} indexing:`, error)
      // If trying to start indexing when already active, don't show error
      if (action === 'start' && String(error).includes('already')) {
        console.log('Indexing is already running')
        return
      }
      // Could show user notification here in the future
    }
  }

  const formatUptime = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}h ${minutes}m`
  }

  return (
    <div className="dashboard-page">
      <div className="dashboard-header">
        <h1>Dashboard</h1>
      </div>

      <div className="dashboard-cards">
        <div className="dashboard-card">
          <h3>System Status</h3>
          <div className="value">{systemStatus?.status || 'Unknown'}</div>
          <div className="label">Current state</div>
        </div>

        <div className="dashboard-card">
          <h3>Uptime</h3>
          <div className="value">{systemStatus ? formatUptime(systemStatus.uptime) : '0h 0m'}</div>
          <div className="label">System running time</div>
        </div>

        <div className="dashboard-card">
          <h3>Files Processed</h3>
          <div className="value">{indexingStatus?.filesProcessed || 0}</div>
          <div className="label">Total indexed files</div>
        </div>

        <div className="dashboard-card">
          <h3>Files Queued</h3>
          <div className="value">{indexingStatus?.pendingFiles || 0}</div>
          <div className="label">Pending for indexing</div>
        </div>

        <div className="dashboard-card">
          <h3>Indexing Errors</h3>
          <div className="value">{indexingStatus?.errors || 0}</div>
          <div className="label">Failed file operations</div>
        </div>

        <div className="dashboard-card">
          <h3>Database Size</h3>
          <div className="value">{systemStatus?.database?.size_info?.total_db_size || 'N/A'}</div>
          <div className="label">Total disk space used</div>
        </div>
      </div>

      <div className="indexing-controls">
        <h3>Indexing Controls</h3>
        
        {/* Indexing Status Indicator */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          padding: '12px',
          marginBottom: '20px',
          backgroundColor: indexingStatus?.state === 'running' ? '#d4edda' : 
                          indexingStatus?.state === 'paused' ? '#fff3cd' : '#f8f9fa',
          border: `1px solid ${indexingStatus?.state === 'running' ? '#c3e6cb' : 
                               indexingStatus?.state === 'paused' ? '#ffeaa7' : '#dee2e6'}`,
          borderRadius: '8px'
        }}>
          <div style={{
            width: '12px',
            height: '12px',
            borderRadius: '50%',
            backgroundColor: indexingStatus?.state === 'running' ? '#28a745' :
                            indexingStatus?.state === 'paused' ? '#ffc107' : '#6c757d',
            animation: indexingStatus?.state === 'running' ? 'pulse 2s infinite' : 'none'
          }} />
          <span style={{
            fontWeight: '500',
            color: indexingStatus?.state === 'running' ? '#155724' :
                   indexingStatus?.state === 'paused' ? '#856404' : '#495057'
          }}>
            Indexing Status: {
              indexingStatus?.state === 'running' ? '🟢 Active - Processing files...' :
              indexingStatus?.state === 'paused' ? '🟡 Paused' :
              indexingStatus?.state === 'idle' ? '⚪ Idle - Ready to index' :
              '⚫ Stopped'
            }
          </span>
          {indexingStatus?.state === 'running' && (
            <span style={{ marginLeft: 'auto', fontSize: '14px', color: '#6c757d' }}>
              {indexingStatus.filesProcessed} / {indexingStatus.totalFiles} files
            </span>
          )}
        </div>
        
        <div className="control-buttons">
          <button
            className="control-button start"
            onClick={() => handleIndexingControl('start')}
            disabled={indexingStatus?.state === 'running'}
          >
            🎬 Start Indexing
          </button>
          
          <button
            className="control-button stop"
            onClick={() => handleIndexingControl('stop')}
            disabled={indexingStatus?.state !== 'running'}
          >
            🛑 Stop Indexing
          </button>
          
          <button
            className="control-button pause"
            onClick={() => handleIndexingControl('pause')}
            disabled={indexingStatus?.state !== 'running'}
          >
            ⏸️ Pause
          </button>
          
          <button
            className="control-button pause"
            onClick={() => handleIndexingControl('resume')}
            disabled={indexingStatus?.state !== 'paused'}
          >
            ▶️ Resume
          </button>
        </div>

        {indexingStatus && indexingStatus.state === 'running' && (
          <div style={{ marginTop: '20px' }}>
            <div style={{ marginBottom: '10px' }}>
              <strong>Current file:</strong> {indexingStatus.currentFile}
            </div>
            <div style={{ marginBottom: '10px' }}>
              <strong>Progress:</strong> {indexingStatus.filesProcessed} / {indexingStatus.totalFiles} files
            </div>
            <div style={{ 
              background: '#ecf0f1', 
              borderRadius: '10px', 
              height: '10px', 
              overflow: 'hidden'
            }}>
              <div style={{ 
                background: '#3498db', 
                height: '100%', 
                width: `${(indexingStatus.filesProcessed / indexingStatus.totalFiles) * 100}%`,
                transition: 'width 0.3s ease'
              }}></div>
            </div>
          </div>
        )}
      </div>

      {systemStatus && (
        <div className="indexing-controls">
          <h3>System Information</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '20px' }}>
            <div>
              <h4>Database</h4>
              <p>Status: {systemStatus.database?.connected ? '✅ Connected' : '❌ Disconnected'}</p>
              {systemStatus.database?.latency && (
                <p>Latency: {systemStatus.database.latency}ms</p>
              )}
              {systemStatus.database?.size_info && (
                <div style={{ marginTop: '10px' }}>
                  <p><strong>Database Disk Usage:</strong></p>
                  <p>Total: {systemStatus.database.size_info.total_db_size}</p>
                  <p>Files: {systemStatus.database.size_info.files_table_size}</p>
                  <p>Chunks: {systemStatus.database.size_info.chunks_table_size}</p>
                  <p>Search: {systemStatus.database.size_info.text_search_table_size}</p>
                </div>
              )}
            </div>
            
            <div>
              <h4>Embeddings</h4>
              <p>Status: {systemStatus.embeddings?.available ? '✅ Available' : '❌ Unavailable'}</p>
              {systemStatus.embeddings?.model && (
                <p>Model: {systemStatus.embeddings.model}</p>
              )}
            </div>
            
            <div>
              <h4>Resources</h4>
              {systemStatus.resources && (
                <>
                  <p>CPU: {systemStatus.resources.cpu?.toFixed(1)}%</p>
                  <p>Memory: {systemStatus.resources.memory?.toFixed(1)}%</p>
                  <p>Disk: {systemStatus.resources.disk?.toFixed(1)}%</p>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default DashboardPage