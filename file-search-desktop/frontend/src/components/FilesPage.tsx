import { useState, useEffect } from 'react'

interface FileInfo {
  id: string
  path: string
  filename: string
  file_type: string
  size_bytes: number
  created_at: string
  modified_at: string
  content_hash: string
  parent_path: string
  indexing_status: string
}

interface DirectoryNode {
  name: string
  path: string
  isDirectory: boolean
  children: DirectoryNode[]
  files: FileInfo[]
  expanded: boolean
  loaded: boolean
}

function FilesPage() {
  const [rootDirectories, setRootDirectories] = useState<DirectoryNode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentPage, setCurrentPage] = useState(0)
  const [totalFiles, setTotalFiles] = useState(0)
  const [viewMode, setViewMode] = useState<'tree' | 'list'>('tree')
  
  // List view state
  const [files, setFiles] = useState<FileInfo[]>([])
  const pageSize = 50

  useEffect(() => {
    if (viewMode === 'tree') {
      loadDirectoryStructure()
    } else {
      loadFiles(0)
    }
  }, [viewMode])

  const loadDirectoryStructure = async () => {
    try {
      setLoading(true)
      // Get root directories by fetching unique parent paths
      const filesList = await window.go.main.App.GetFiles(1000, 0) as FileInfo[]
      const rootPaths = new Set<string>()
      
      filesList.forEach(file => {
        if (file.parent_path) {
          const topLevel = file.parent_path.split('/')[1] || file.parent_path
          if (topLevel && topLevel !== '') {
            rootPaths.add('/' + topLevel)
          }
        }
      })

      const roots: DirectoryNode[] = Array.from(rootPaths).map(path => ({
        name: path.split('/').pop() || path,
        path: path,
        isDirectory: true,
        children: [],
        files: [],
        expanded: false,
        loaded: false
      }))

      setRootDirectories(roots)
      setError(null)
    } catch (err: any) {
      setError(err.message || 'Failed to load directory structure')
    } finally {
      setLoading(false)
    }
  }

  const loadFiles = async (page: number) => {
    try {
      setLoading(true)
      const offset = page * pageSize
      const filesList = await window.go.main.App.GetFiles(pageSize, offset) as FileInfo[]
      setFiles(filesList)
      setCurrentPage(page)
      setError(null)
      
      // Get total count from system status
      const status = await window.go.main.App.GetSystemStatus()
      setTotalFiles(status.indexed_files || 0)
    } catch (err: any) {
      setError(err.message || 'Failed to load files')
      setFiles([])
    } finally {
      setLoading(false)
    }
  }

  const toggleDirectory = async (directory: DirectoryNode) => {
    if (!directory.expanded && !directory.loaded) {
      // Load directory contents
      try {
        const filesList = await window.go.main.App.GetFiles(500, 0) as FileInfo[]
        const dirFiles = filesList.filter(file => file.parent_path === directory.path)
        
        const subdirs = new Set<string>()
        const files: FileInfo[] = []
        
        dirFiles.forEach(file => {
          if (file.path.startsWith(directory.path + '/')) {
            const relativePath = file.path.substring(directory.path.length + 1)
            if (relativePath.includes('/')) {
              // This is in a subdirectory
              const subdirName = relativePath.split('/')[0]
              subdirs.add(subdirName)
            } else {
              // This is a direct file
              files.push(file)
            }
          }
        })

        directory.children = Array.from(subdirs).map(subdirName => ({
          name: subdirName,
          path: `${directory.path}/${subdirName}`,
          isDirectory: true,
          children: [],
          files: [],
          expanded: false,
          loaded: false
        }))
        
        directory.files = files
        directory.loaded = true
      } catch (err) {
        console.error('Failed to load directory:', err)
      }
    }
    
    directory.expanded = !directory.expanded
    setRootDirectories([...rootDirectories])
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

  const renderDirectoryTree = (directories: DirectoryNode[], level: number = 0) => {
    return directories.map((dir, index) => (
      <div key={`${dir.path}-${index}`} style={{ marginLeft: `${level * 20}px` }}>
        <div 
          onClick={() => toggleDirectory(dir)}
          style={{
            display: 'flex',
            alignItems: 'center',
            padding: '8px',
            cursor: 'pointer',
            backgroundColor: dir.expanded ? '#ecf0f1' : 'transparent',
            borderRadius: '4px'
          }}
        >
          <span style={{ marginRight: '8px' }}>
            {dir.expanded ? '📂' : '📁'}
          </span>
          <span style={{ fontWeight: '500' }}>{dir.name}</span>
          <span style={{ marginLeft: '8px', fontSize: '0.8rem', color: '#7f8c8d' }}>
            ({dir.files.length} files)
          </span>
        </div>
        
        {dir.expanded && (
          <div>
            {/* Render subdirectories */}
            {dir.children.length > 0 && renderDirectoryTree(dir.children, level + 1)}
            
            {/* Render files */}
            {dir.files.map(file => (
              <div key={file.id} style={{ 
                marginLeft: `${(level + 1) * 20}px`,
                padding: '4px 8px',
                display: 'flex',
                alignItems: 'center',
                fontSize: '0.9rem'
              }}>
                <span style={{ marginRight: '8px' }}>{getFileIcon(file.file_type)}</span>
                <span style={{ flex: 1 }}>{file.filename}</span>
                <span style={{ marginLeft: '8px', color: '#7f8c8d', fontSize: '0.8rem' }}>
                  {formatFileSize(file.size_bytes)}
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    navigator.clipboard.writeText(file.path)
                  }}
                  style={{
                    marginLeft: '8px',
                    padding: '2px 6px',
                    backgroundColor: '#ecf0f1',
                    border: '1px solid #bdc3c7',
                    borderRadius: '3px',
                    cursor: 'pointer',
                    fontSize: '0.7rem'
                  }}
                >
                  📋
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    ))
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
            onClick={() => viewMode === 'tree' ? loadDirectoryStructure() : loadFiles(0)}
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

  const totalPages = Math.ceil(totalFiles / pageSize)

  return (
    <div className="files-page">
      <div className="files-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
        <h1>Files ({totalFiles.toLocaleString()} total)</h1>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <div style={{ display: 'flex', border: '1px solid #bdc3c7', borderRadius: '6px', overflow: 'hidden' }}>
            <button 
              onClick={() => setViewMode('tree')}
              style={{
                padding: '8px 16px',
                backgroundColor: viewMode === 'tree' ? '#3498db' : '#ecf0f1',
                color: viewMode === 'tree' ? 'white' : '#2c3e50',
                border: 'none',
                cursor: 'pointer',
                fontSize: '0.9rem'
              }}
            >
              🌳 Tree
            </button>
            <button 
              onClick={() => setViewMode('list')}
              style={{
                padding: '8px 16px',
                backgroundColor: viewMode === 'list' ? '#3498db' : '#ecf0f1',
                color: viewMode === 'list' ? 'white' : '#2c3e50',
                border: 'none',
                cursor: 'pointer',
                fontSize: '0.9rem'
              }}
            >
              📄 List
            </button>
          </div>
          <button 
            onClick={() => viewMode === 'tree' ? loadDirectoryStructure() : loadFiles(currentPage)}
            style={{
              padding: '8px 16px',
              backgroundColor: '#27ae60',
              color: 'white',
              border: 'none',
              borderRadius: '6px',
              cursor: 'pointer',
              fontSize: '0.9rem'
            }}
          >
            🔄 Refresh
          </button>
        </div>
      </div>

      {viewMode === 'tree' ? (
        <div className="directory-tree" style={{ padding: '20px', maxHeight: '70vh', overflowY: 'auto' }}>
          {rootDirectories.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '50px', color: '#7f8c8d' }}>
              <h3>No directories found</h3>
              <p>Start indexing some directories to see files here.</p>
            </div>
          ) : (
            renderDirectoryTree(rootDirectories)
          )}
        </div>
      ) : (
        <div className="files-list">
          {files.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '50px', color: '#7f8c8d' }}>
              <h3>No files found</h3>
              <p>Start indexing some directories to see files here.</p>
            </div>
          ) : (
            <>
              <div className="files-table" style={{ marginBottom: '20px' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ backgroundColor: '#ecf0f1' }}>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>File</th>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>Type</th>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>Size</th>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>Modified</th>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {files.map((file) => (
                      <tr key={file.id} style={{ borderBottom: '1px solid #ecf0f1' }}>
                        <td style={{ padding: '12px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                            <span style={{ fontSize: '1.2rem' }}>{getFileIcon(file.file_type)}</span>
                            <div>
                              <div style={{ fontWeight: '500' }}>{file.filename}</div>
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
                        <td style={{ padding: '12px' }}>
                          <span style={{
                            background: file.indexing_status === 'completed' ? '#d5ecd5' : '#f8d7d7',
                            color: file.indexing_status === 'completed' ? '#27ae60' : '#e74c3c',
                            padding: '2px 8px',
                            borderRadius: '4px',
                            fontSize: '0.8rem'
                          }}>
                            {file.file_type}
                          </span>
                        </td>
                        <td style={{ padding: '12px' }}>{formatFileSize(file.size_bytes)}</td>
                        <td style={{ padding: '12px' }}>{formatDate(file.modified_at)}</td>
                        <td style={{ padding: '12px' }}>
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
              
              {/* Pagination */}
              {totalPages > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '10px', marginTop: '20px' }}>
                  <button
                    onClick={() => loadFiles(currentPage - 1)}
                    disabled={currentPage === 0}
                    style={{
                      padding: '8px 12px',
                      backgroundColor: currentPage === 0 ? '#ecf0f1' : '#3498db',
                      color: currentPage === 0 ? '#7f8c8d' : 'white',
                      border: 'none',
                      borderRadius: '4px',
                      cursor: currentPage === 0 ? 'not-allowed' : 'pointer'
                    }}
                  >
                    ← Previous
                  </button>
                  
                  <span style={{ color: '#7f8c8d', fontSize: '0.9rem' }}>
                    Page {currentPage + 1} of {totalPages}
                  </span>
                  
                  <button
                    onClick={() => loadFiles(currentPage + 1)}
                    disabled={currentPage >= totalPages - 1}
                    style={{
                      padding: '8px 12px',
                      backgroundColor: currentPage >= totalPages - 1 ? '#ecf0f1' : '#3498db',
                      color: currentPage >= totalPages - 1 ? '#7f8c8d' : 'white',
                      border: 'none',
                      borderRadius: '4px',
                      cursor: currentPage >= totalPages - 1 ? 'not-allowed' : 'pointer'
                    }}
                  >
                    Next →
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  )
}

export default FilesPage