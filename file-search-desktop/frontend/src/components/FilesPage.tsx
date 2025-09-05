import { useState, useEffect } from 'react'

interface FileInfo {
  id: string
  path: string
  name: string
  type: string
  size: number
  createdAt: string
  modifiedAt: string
  hash: string
}

function FilesPage() {
  const [files, setFiles] = useState<FileInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    loadFiles()
  }, [])

  const loadFiles = async () => {
    try {
      setLoading(true)
      const filesList = await window.go.main.App.GetFiles(100, 0)
      setFiles(filesList)
      setError(null)
    } catch (err: any) {
      setError(err.message || 'Failed to load files')
    } finally {
      setLoading(false)
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
    return '📄'
  }

  if (loading) {
    return (
      <div className="files-page">
        <div className="files-header">
          <h1>Files</h1>
        </div>
        <div style={{ textAlign: 'center', padding: '50px' }}>
          <div>Loading files...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="files-page">
        <div className="files-header">
          <h1>Files</h1>
        </div>
        <div style={{ textAlign: 'center', padding: '50px', color: '#e74c3c' }}>
          <div>Error: {error}</div>
          <button 
            onClick={loadFiles}
            style={{
              marginTop: '20px',
              padding: '10px 20px',
              backgroundColor: '#3498db',
              color: 'white',
              border: 'none',
              borderRadius: '6px',
              cursor: 'pointer'
            }}
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="files-page">
      <div className="files-header">
        <h1>Files ({files.length})</h1>
        <button 
          onClick={loadFiles}
          style={{
            padding: '10px 20px',
            backgroundColor: '#3498db',
            color: 'white',
            border: 'none',
            borderRadius: '6px',
            cursor: 'pointer'
          }}
        >
          🔄 Refresh
        </button>
      </div>

      {files.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '50px', color: '#7f8c8d' }}>
          <h3>No files found</h3>
          <p>Start indexing some directories to see files here.</p>
        </div>
      ) : (
        <div className="files-table">
          <table>
            <thead>
              <tr>
                <th>File</th>
                <th>Type</th>
                <th>Size</th>
                <th>Modified</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {files.map((file) => (
                <tr key={file.id}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <span style={{ fontSize: '1.2rem' }}>{getFileIcon(file.type)}</span>
                      <div>
                        <div style={{ fontWeight: '500' }}>{file.name}</div>
                        <div style={{ 
                          fontSize: '0.8rem', 
                          color: '#7f8c8d',
                          fontFamily: 'Monaco, Consolas, monospace'
                        }}>
                          {file.path}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td>
                    <span style={{
                      background: '#ecf0f1',
                      padding: '2px 8px',
                      borderRadius: '4px',
                      fontSize: '0.8rem'
                    }}>
                      {file.type}
                    </span>
                  </td>
                  <td>{formatFileSize(file.size)}</td>
                  <td>{formatDate(file.modifiedAt)}</td>
                  <td>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(file.path)
                      }}
                      style={{
                        padding: '4px 8px',
                        backgroundColor: '#ecf0f1',
                        border: '1px solid #bdc3c7',
                        borderRadius: '4px',
                        cursor: 'pointer',
                        fontSize: '0.8rem'
                      }}
                    >
                      📋 Copy Path
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export default FilesPage