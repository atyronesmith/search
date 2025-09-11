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
  fileCount?: number
  totalSize?: number
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
  const [sortColumn, setSortColumn] = useState<'filename' | 'file_type' | 'indexing_status' | 'size_bytes' | 'modified_at'>('filename')
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const pageSize = 50

  useEffect(() => {
    if (viewMode === 'tree') {
      loadDirectoryStructure()
    } else {
      loadFiles(0)
    }
  }, [viewMode, sortColumn, sortDirection])

  const loadDirectoryStructure = async () => {
    try {
      setLoading(true)
      // Use the new efficient API to get root directories
      const result = await window.go.main.App.GetRootDirectories() as any
      
      if (result.directories && Array.isArray(result.directories)) {
        const roots: DirectoryNode[] = result.directories.map((dir: any) => ({
          name: dir.name,
          path: dir.path,
          isDirectory: true,
          children: [],
          files: [], // Will be loaded on-demand
          expanded: false,
          loaded: false,
          fileCount: dir.file_count || 0,
          totalSize: dir.total_size || 0
        }))
        
        setRootDirectories(roots)
      } else {
        setRootDirectories([])
      }
      
      // Set total files from API response
      if (result.total_files !== undefined) {
        setTotalFiles(result.total_files)
      }
      
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
      const filesList = await window.go.main.App.GetFilesSorted(pageSize, offset, sortColumn, sortDirection) as FileInfo[]
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
      // Load directory contents using the new efficient API
      try {
        const result = await window.go.main.App.GetDirectoryContents(directory.path) as any
        
        if (result) {
          // Set files directly in this directory
          directory.files = result.files || []
          
          // Create subdirectory nodes
          directory.children = (result.directories || []).map((subdir: any) => ({
            name: subdir.name,
            path: subdir.path,
            isDirectory: true,
            children: [],
            files: [], // Will be loaded on-demand
            expanded: false,
            loaded: false,
            fileCount: subdir.file_count || 0,
            totalSize: subdir.total_size || 0
          }))
        }
        
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
    const lowerType = type.toLowerCase()
    if (lowerType.includes('python')) return '🐍'
    if (lowerType.includes('javascript') || lowerType.includes('js')) return '📜'
    if (lowerType.includes('typescript') || lowerType.includes('ts')) return '🔷'
    if (lowerType.includes('go')) return '🐹'
    if (lowerType.includes('rust')) return '🦀'
    if (lowerType.includes('java')) return '☕'
    if (lowerType.includes('c++') || lowerType.includes('cpp')) return '⚙️'
    if (lowerType.includes('code')) return '💻'
    if (lowerType.includes('pdf')) return '📕'
    if (lowerType.includes('doc') || lowerType.includes('docx')) return '📘'
    if (lowerType.includes('xls') || lowerType.includes('xlsx') || lowerType.includes('csv')) return '📊'
    if (lowerType.includes('text') || lowerType.includes('txt')) return '📝'
    if (lowerType.includes('markdown') || lowerType.includes('md')) return '📓'
    if (lowerType.includes('json')) return '🔧'
    if (lowerType.includes('yaml') || lowerType.includes('yml')) return '📋'
    if (lowerType.includes('html')) return '🌐'
    if (lowerType.includes('css')) return '🎨'
    if (lowerType.includes('image') || lowerType.includes('png') || lowerType.includes('jpg') || lowerType.includes('jpeg')) return '🖼️'
    return '📄'
  }

  const getStatusIcon = (status: string): { icon: string, color: string } => {
    switch(status) {
      case 'completed':
        return { icon: '✅', color: '#27ae60' }
      case 'failed':
        return { icon: '❌', color: '#e74c3c' }
      case 'pending':
        return { icon: '⏳', color: '#f39c12' }
      case 'processing':
        return { icon: '🔄', color: '#3498db' }
      default:
        return { icon: '❓', color: '#7f8c8d' }
    }
  }

  const [copiedPaths, setCopiedPaths] = useState<Set<string>>(new Set())
  const [selectedFileMetadata, setSelectedFileMetadata] = useState<FileInfo | null>(null)
  const [showMetadataDialog, setShowMetadataDialog] = useState(false)

  const handleShowMetadata = (file: FileInfo, e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedFileMetadata(file)
    setShowMetadataDialog(true)
  }

  const handleCloseMetadataDialog = () => {
    setShowMetadataDialog(false)
    setSelectedFileMetadata(null)
  }

  const handleSort = (column: 'filename' | 'file_type' | 'indexing_status' | 'size_bytes' | 'modified_at') => {
    if (sortColumn === column) {
      // Toggle direction if same column
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      // New column, default to ascending
      setSortColumn(column)
      setSortDirection('asc')
    }
    // Will trigger reload via useEffect
  }

  const handleCopyPath = (path: string, e: React.MouseEvent) => {
    e.stopPropagation()
    navigator.clipboard.writeText(path)
    setCopiedPaths(prev => new Set(prev).add(path))
    setTimeout(() => {
      setCopiedPaths(prev => {
        const newSet = new Set(prev)
        newSet.delete(path)
        return newSet
      })
    }, 800)
  }

  const renderDirectoryTree = (directories: DirectoryNode[], level: number = 0) => {
    return directories.map((dir, index) => {
      const hasContent = (dir.fileCount && dir.fileCount > 0) || (dir.loaded && (dir.children.length > 0 || dir.files.length > 0))
      
      return (
      <div key={`${dir.path}-${index}`} style={{ marginLeft: `${level * 20}px` }}>
        <div 
          onClick={() => hasContent ? toggleDirectory(dir) : undefined}
          onMouseEnter={(e) => {
            if (hasContent) {
              e.currentTarget.style.backgroundColor = '#e8f0f3'
            }
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = dir.expanded ? '#ecf0f1' : 'transparent'
          }}
          style={{
            display: 'flex',
            alignItems: 'center',
            padding: '8px',
            cursor: hasContent ? 'pointer' : 'default',
            backgroundColor: dir.expanded ? '#ecf0f1' : 'transparent',
            borderRadius: '4px',
            transition: 'background-color 0.2s ease'
          }}
        >
          {hasContent ? (
            <span style={{ 
              marginRight: '8px',
              width: '16px',
              height: '16px',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              backgroundColor: dir.expanded ? '#3498db' : '#ecf0f1',
              borderRadius: '3px',
              fontSize: '0.9rem',
              fontWeight: 'bold',
              color: dir.expanded ? 'white' : '#2c3e50',
              border: '1px solid #bdc3c7',
              flexShrink: 0,
              transition: 'all 0.2s ease'
            }}>
              {dir.expanded ? '−' : '+'}
            </span>
          ) : (
            <span style={{ 
              marginRight: '8px',
              width: '16px',
              height: '16px',
              display: 'inline-flex',
              flexShrink: 0
            }}>
            </span>
          )}
          <span style={{ marginRight: '8px' }}>
            {dir.expanded ? '📂' : '📁'}
          </span>
          <span style={{ fontWeight: '500' }}>{dir.name}</span>
          <span style={{ marginLeft: '8px', fontSize: '0.8rem', color: '#7f8c8d' }}>
            ({dir.fileCount || dir.files.length} files)
          </span>
        </div>
        
        {dir.expanded && (
          <div>
            {/* Render subdirectories */}
            {dir.children.length > 0 && renderDirectoryTree(dir.children, level + 1)}
            
            {/* Render files */}
            {dir.files.map(file => {
              const statusInfo = getStatusIcon(file.indexing_status)
              const isCopied = copiedPaths.has(file.path)
              
              return (
              <div key={file.id} style={{ 
                marginLeft: `${(level + 1) * 20}px`,
                padding: '4px 8px',
                display: 'flex',
                alignItems: 'center',
                fontSize: '0.9rem',
                gap: '8px'
              }}>
                {/* File name with icon */}
                <div style={{ 
                  flex: 1, 
                  display: 'flex', 
                  alignItems: 'center',
                  gap: '8px',
                  minWidth: 0
                }}>
                  <span 
                    style={{ fontSize: '1rem', flexShrink: 0 }}
                    title={`File type: ${file.file_type}`}
                  >
                    {getFileIcon(file.file_type)}
                  </span>
                  <span style={{ 
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    textAlign: 'left'
                  }}>
                    {file.filename}
                  </span>
                </div>
                
                {/* File type icon column */}
                <span 
                  style={{
                    width: '20px',
                    textAlign: 'center',
                    fontSize: '1rem',
                    flexShrink: 0,
                    cursor: 'help'
                  }}
                  title={`File type: ${file.file_type}`}
                >
                  {getFileIcon(file.file_type)}
                </span>
                
                {/* Status icon */}
                <span 
                  style={{
                    width: '20px',
                    textAlign: 'center',
                    fontSize: '0.9rem',
                    flexShrink: 0,
                    cursor: 'help'
                  }}
                  title={`Status: ${file.indexing_status}`}
                >
                  {statusInfo.icon}
                </span>
                
                {/* File size */}
                <span style={{ 
                  width: '60px',
                  textAlign: 'right',
                  color: '#7f8c8d', 
                  fontSize: '0.8rem',
                  flexShrink: 0
                }}>
                  {formatFileSize(file.size_bytes)}
                </span>
                
                {/* Copy button */}
                <button
                  onClick={(e) => handleCopyPath(file.path, e)}
                  title={isCopied ? "Path copied!" : "Copy file path to clipboard"}
                  style={{
                    padding: '2px 6px',
                    backgroundColor: isCopied ? '#27ae60' : '#ecf0f1',
                    border: `1px solid ${isCopied ? '#27ae60' : '#bdc3c7'}`,
                    borderRadius: '3px',
                    cursor: 'pointer',
                    fontSize: '0.7rem',
                    flexShrink: 0,
                    transition: 'all 0.2s ease',
                    color: isCopied ? 'white' : 'inherit',
                    marginRight: '4px'
                  }}
                >
                  {isCopied ? '✓' : '📋'}
                </button>
                
                {/* Metadata button */}
                <button
                  onClick={(e) => handleShowMetadata(file, e)}
                  title="View file metadata"
                  style={{
                    padding: '2px 6px',
                    backgroundColor: '#e8f4f8',
                    border: '1px solid #3498db',
                    borderRadius: '3px',
                    cursor: 'pointer',
                    fontSize: '0.7rem',
                    flexShrink: 0,
                    color: '#2c3e50'
                  }}
                >
                  ℹ️
                </button>
              </div>
            )})}
          </div>
        )}
      </div>
    )
    })
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
                      <th 
                        style={{ 
                          padding: '12px', 
                          textAlign: 'left', 
                          borderBottom: '1px solid #bdc3c7',
                          cursor: 'pointer',
                          userSelect: 'none'
                        }}
                        onClick={() => handleSort('filename')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span>File</span>
                          {sortColumn === 'filename' && (
                            <span style={{ fontSize: '0.8rem' }}>
                              {sortDirection === 'asc' ? '▲' : '▼'}
                            </span>
                          )}
                        </div>
                      </th>
                      <th 
                        style={{ 
                          padding: '12px', 
                          textAlign: 'center', 
                          borderBottom: '1px solid #bdc3c7', 
                          width: '50px',
                          cursor: 'pointer',
                          userSelect: 'none'
                        }}
                        onClick={() => handleSort('file_type')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}>
                          <span>Type</span>
                          {sortColumn === 'file_type' && (
                            <span style={{ fontSize: '0.8rem' }}>
                              {sortDirection === 'asc' ? '▲' : '▼'}
                            </span>
                          )}
                        </div>
                      </th>
                      <th 
                        style={{ 
                          padding: '12px', 
                          textAlign: 'center', 
                          borderBottom: '1px solid #bdc3c7', 
                          width: '50px',
                          cursor: 'pointer',
                          userSelect: 'none'
                        }}
                        onClick={() => handleSort('indexing_status')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}>
                          <span>Status</span>
                          {sortColumn === 'indexing_status' && (
                            <span style={{ fontSize: '0.8rem' }}>
                              {sortDirection === 'asc' ? '▲' : '▼'}
                            </span>
                          )}
                        </div>
                      </th>
                      <th 
                        style={{ 
                          padding: '12px', 
                          textAlign: 'left', 
                          borderBottom: '1px solid #bdc3c7',
                          cursor: 'pointer',
                          userSelect: 'none'
                        }}
                        onClick={() => handleSort('size_bytes')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span>Size</span>
                          {sortColumn === 'size_bytes' && (
                            <span style={{ fontSize: '0.8rem' }}>
                              {sortDirection === 'asc' ? '▲' : '▼'}
                            </span>
                          )}
                        </div>
                      </th>
                      <th 
                        style={{ 
                          padding: '12px', 
                          textAlign: 'left', 
                          borderBottom: '1px solid #bdc3c7',
                          cursor: 'pointer',
                          userSelect: 'none'
                        }}
                        onClick={() => handleSort('modified_at')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span>Modified</span>
                          {sortColumn === 'modified_at' && (
                            <span style={{ fontSize: '0.8rem' }}>
                              {sortDirection === 'asc' ? '▲' : '▼'}
                            </span>
                          )}
                        </div>
                      </th>
                      <th style={{ padding: '12px', textAlign: 'left', borderBottom: '1px solid #bdc3c7' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {files.map((file) => {
                      const statusInfo = getStatusIcon(file.indexing_status)
                      return (
                      <tr key={file.id} style={{ borderBottom: '1px solid #ecf0f1' }}>
                        <td style={{ padding: '12px' }}>
                          <div style={{ display: 'flex', alignItems: 'flex-start', gap: '10px' }}>
                            <span style={{ fontSize: '1.2rem', marginTop: '2px' }}>{getFileIcon(file.file_type)}</span>
                            <div style={{ flex: 1 }}>
                              <div style={{ fontWeight: '500', textAlign: 'left' }}>{file.filename}</div>
                              <div style={{ 
                                fontSize: '0.8rem', 
                                color: '#7f8c8d',
                                fontFamily: 'Monaco, Consolas, monospace',
                                textAlign: 'left'
                              }}>
                                {file.path}
                              </div>
                            </div>
                          </div>
                        </td>
                        <td style={{ padding: '12px', textAlign: 'center' }}>
                          <span 
                            title={`File type: ${file.file_type}`}
                            style={{
                              fontSize: '1.5rem',
                              cursor: 'help'
                            }}
                          >
                            {getFileIcon(file.file_type)}
                          </span>
                        </td>
                        <td style={{ padding: '12px', textAlign: 'center' }}>
                          <span 
                            title={`Status: ${file.indexing_status}`}
                            style={{
                              fontSize: '1.2rem',
                              cursor: 'help'
                            }}
                          >
                            {statusInfo.icon}
                          </span>
                        </td>
                        <td style={{ padding: '12px' }}>{formatFileSize(file.size_bytes)}</td>
                        <td style={{ padding: '12px' }}>{formatDate(file.modified_at)}</td>
                        <td style={{ padding: '12px' }}>
                          <div style={{ display: 'flex', gap: '8px' }}>
                            <button
                              onClick={() => {
                                navigator.clipboard.writeText(file.path)
                              }}
                              title="Copy file path to clipboard"
                              style={{
                                padding: '4px 8px',
                                backgroundColor: '#ecf0f1',
                                border: '1px solid #bdc3c7',
                                borderRadius: '4px',
                                cursor: 'pointer',
                                fontSize: '0.8rem'
                              }}
                            >
                              📋
                            </button>
                            <button
                              onClick={(e) => handleShowMetadata(file, e)}
                              title="View file metadata"
                              style={{
                                padding: '4px 8px',
                                backgroundColor: '#e8f4f8',
                                border: '1px solid #3498db',
                                borderRadius: '4px',
                                cursor: 'pointer',
                                fontSize: '0.8rem',
                                color: '#2c3e50'
                              }}
                            >
                              ℹ️
                            </button>
                          </div>
                        </td>
                      </tr>
                    )})}
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

      {/* Metadata Dialog */}
      {showMetadataDialog && selectedFileMetadata && (
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
          zIndex: 1000
        }}>
          <div style={{
            backgroundColor: 'white',
            borderRadius: '8px',
            padding: '20px',
            maxWidth: '600px',
            width: '90%',
            maxHeight: '80vh',
            overflowY: 'auto',
            boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)'
          }}>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '20px',
              borderBottom: '1px solid #ecf0f1',
              paddingBottom: '10px'
            }}>
              <h2 style={{ margin: 0, color: '#2c3e50' }}>File Metadata</h2>
              <button
                onClick={handleCloseMetadataDialog}
                style={{
                  padding: '8px 16px',
                  backgroundColor: '#e74c3c',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: 'pointer',
                  fontSize: '0.9rem'
                }}
              >
                Dismiss
              </button>
            </div>
            
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '12px' }}>
              <div style={{ fontWeight: '600', color: '#34495e' }}>Filename:</div>
              <div style={{ fontFamily: 'Monaco, Consolas, monospace', color: '#2c3e50' }}>
                {selectedFileMetadata.filename}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>File Path:</div>
              <div style={{ 
                fontFamily: 'Monaco, Consolas, monospace', 
                color: '#2c3e50',
                wordBreak: 'break-all'
              }}>
                {selectedFileMetadata.path}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>File Type:</div>
              <div style={{ color: '#2c3e50' }}>
                <span style={{ marginRight: '8px' }}>{getFileIcon(selectedFileMetadata.file_type)}</span>
                {selectedFileMetadata.file_type}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>File Size:</div>
              <div style={{ color: '#2c3e50' }}>{formatFileSize(selectedFileMetadata.size_bytes)}</div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>Created Date:</div>
              <div style={{ color: '#2c3e50' }}>
                {selectedFileMetadata.created_at ? new Date(selectedFileMetadata.created_at).toLocaleString() : 'Unknown'}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>Modified Date:</div>
              <div style={{ color: '#2c3e50' }}>
                {new Date(selectedFileMetadata.modified_at).toLocaleString()}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>Indexing Status:</div>
              <div style={{ color: '#2c3e50' }}>
                <span style={{ marginRight: '8px' }}>{getStatusIcon(selectedFileMetadata.indexing_status).icon}</span>
                {selectedFileMetadata.indexing_status}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>Content Hash:</div>
              <div style={{ 
                fontFamily: 'Monaco, Consolas, monospace', 
                fontSize: '0.8rem',
                color: '#7f8c8d',
                wordBreak: 'break-all'
              }}>
                {selectedFileMetadata.content_hash || 'Not available'}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>Parent Path:</div>
              <div style={{ 
                fontFamily: 'Monaco, Consolas, monospace', 
                color: '#2c3e50',
                wordBreak: 'break-all'
              }}>
                {selectedFileMetadata.parent_path || 'Root'}
              </div>
              
              <div style={{ fontWeight: '600', color: '#34495e' }}>File ID:</div>
              <div style={{ fontFamily: 'Monaco, Consolas, monospace', color: '#7f8c8d' }}>
                {selectedFileMetadata.id}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default FilesPage