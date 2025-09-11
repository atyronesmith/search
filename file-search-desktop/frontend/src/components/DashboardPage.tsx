interface IndexingStatus {
  state: string
  filesProcessed: number
  totalFiles: number
  pendingFiles: number
  processingFiles: number
  currentFile: string
  errors: number
  skippedFiles: number
  elapsedTime: number
}

interface FileTypeBreakdown {
  extension: string
  type: string
  count: number
}

interface SystemStatus {
  status: string
  uptime: number
  database: any
  embeddings: any
  indexing: any
  resources: any
  file_type_breakdown?: FileTypeBreakdown[]
  skipped_files?: number
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
        case 'reindex-failed':
          await window.go.main.App.ReindexFailed()
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
        <div className="dashboard-card" title="Number of files successfully processed and indexed">
          <h3>Files Processed</h3>
          <div className="value">{indexingStatus?.filesProcessed || 0}</div>
          <div className="label">Total indexed files</div>
        </div>

        <div className="dashboard-card" title="Number of files currently being processed by workers">
          <h3>Processing Now</h3>
          <div className="value" style={{ color: (indexingStatus?.processingFiles || 0) > 0 ? '#28a745' : undefined }}>
            {indexingStatus?.processingFiles || 0}
          </div>
          <div className="label">Files being processed</div>
        </div>

        <div className="dashboard-card" title="Number of files waiting to be indexed">
          <h3>Files Queued</h3>
          <div className="value">{indexingStatus?.pendingFiles || 0}</div>
          <div className="label">Pending for indexing</div>
        </div>

        <div className="dashboard-card" title="Number of files that failed to be indexed">
          <h3>Failed Files</h3>
          <div className="value">{indexingStatus?.errors || 0}</div>
          <div className="label">Indexing failures</div>
        </div>

        <div className="dashboard-card" title="Number of files skipped during indexing">
          <h3>Skipped Files</h3>
          <div className="value">{indexingStatus?.skippedFiles || 0}</div>
          <div className="label">Files skipped</div>
        </div>

        <div className="dashboard-card" title="Total disk space used by the database">
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
              {indexingStatus.processingFiles > 0 && (
                <span style={{ marginLeft: '10px', color: '#28a745', fontWeight: 'bold' }}>
                  ({indexingStatus.processingFiles} processing)
                </span>
              )}
            </span>
          )}
        </div>
        
        <div className="control-buttons">
          <button
            className="control-button start"
            onClick={() => handleIndexingControl('start')}
            disabled={indexingStatus?.state === 'running'}
            title="Start indexing files from configured directories"
          >
            🎬 Start Indexing
          </button>
          
          <button
            className="control-button stop"
            onClick={() => handleIndexingControl('stop')}
            disabled={indexingStatus?.state !== 'running'}
            title="Stop the currently running indexing process"
          >
            🛑 Stop Indexing
          </button>
          
          <button
            className="control-button pause"
            onClick={() => handleIndexingControl('pause')}
            disabled={indexingStatus?.state !== 'running'}
            title="Temporarily pause the indexing process"
          >
            ⏸️ Pause
          </button>
          
          <button
            className="control-button pause"
            onClick={() => handleIndexingControl('resume')}
            disabled={indexingStatus?.state !== 'paused'}
            title="Resume a paused indexing process"
          >
            ▶️ Resume
          </button>
          
          <button
            className="control-button reindex-failed"
            onClick={() => handleIndexingControl('reindex-failed')}
            disabled={indexingStatus?.state === 'running'}
            title="Retry indexing all files that previously failed"
          >
            🔄 Retry Failed Files
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

      {systemStatus?.file_type_breakdown && systemStatus.file_type_breakdown.length > 0 && (
        <div className="indexing-controls">
          <h3>Indexed Files by Type</h3>
          <div className="file-type-breakdown">
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #ecf0f1' }}>
                  <th style={{ textAlign: 'left', padding: '10px', color: '#2c3e50', fontWeight: 600 }}>File Type</th>
                  <th style={{ textAlign: 'left', padding: '10px', color: '#2c3e50', fontWeight: 600 }}>Extension</th>
                  <th style={{ textAlign: 'right', padding: '10px', color: '#2c3e50', fontWeight: 600 }}>Count</th>
                </tr>
              </thead>
              <tbody>
                {systemStatus.file_type_breakdown.slice(0, 10).map((item: any, index: number) => (
                  <tr key={index} style={{ borderBottom: '1px solid #ecf0f1' }}>
                    <td style={{ padding: '8px 10px', color: '#495057' }}>{item.type}</td>
                    <td style={{ padding: '8px 10px', color: '#6c757d', fontFamily: 'monospace', fontSize: '0.9em' }}>
                      {item.extension}
                    </td>
                    <td style={{ padding: '8px 10px', textAlign: 'right', fontWeight: 500, color: '#2c3e50' }}>
                      {item.count.toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {systemStatus.file_type_breakdown.length > 10 && (
              <div style={{ padding: '10px', textAlign: 'center', color: '#6c757d', fontSize: '0.9em' }}>
                ...and {systemStatus.file_type_breakdown.length - 10} more file types
              </div>
            )}
          </div>
        </div>
      )}

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