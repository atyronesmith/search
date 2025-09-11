import { useState, useEffect } from 'react'
import './App.css'
import SearchPage from './components/SearchPage'
import DashboardPage from './components/DashboardPage'
import FilesPage from './components/FilesPage'
import SettingsPage from './components/SettingsPage'
import PromptPage from './components/PromptPage'
import DebugPage from './components/DebugPage'

function App() {
  const [currentPage, setCurrentPage] = useState('search')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<any[]>([])
  const [indexingStatus, setIndexingStatus] = useState<any>(null)
  const [systemStatus, setSystemStatus] = useState<any>(null)

  useEffect(() => {
    // Periodically check system status
    const interval = setInterval(async () => {
      try {
        const status = await window.go.main.App.GetSystemStatus()
        setSystemStatus(status)
        
        const indexStatus = await window.go.main.App.GetIndexingStatus()
        setIndexingStatus(indexStatus)
      } catch (error) {
        console.error('Failed to get status:', error)
      }
    }, 5000)

    return () => clearInterval(interval)
  }, [])

  const handleSearch = async (query: string, offset: number = 0, append: boolean = false) => {
    try {
      if (!append) {
        setSearchQuery(query)
      }
      const results = await window.go.main.App.Search({
        query,
        limit: 10,
        offset
      })
      if (append) {
        setSearchResults(prev => [...prev, ...(results || [])])
      } else {
        setSearchResults(results || [])
      }
    } catch (error) {
      console.error('Search failed:', error)
      if (!append) {
        setSearchResults([])
      }
    }
  }

  // New handler for SearchWithDetails results
  const handleSearchWithDetails = (query: string, results: any[], append: boolean = false) => {
    if (!append) {
      setSearchQuery(query)
    }
    if (append) {
      setSearchResults(prev => [...prev, ...(results || [])])
    } else {
      setSearchResults(results || [])
    }
  }

  const renderPage = () => {
    switch (currentPage) {
      case 'search':
        return (
          <SearchPage
            onSearch={handleSearch}
            onSearchWithDetails={handleSearchWithDetails}
            searchQuery={searchQuery}
            searchResults={searchResults}
          />
        )
      case 'dashboard':
        return (
          <DashboardPage
            indexingStatus={indexingStatus}
            systemStatus={systemStatus}
          />
        )
      case 'files':
        return <FilesPage />
      case 'settings':
        return <SettingsPage />
      case 'prompt':
        return <PromptPage />
      case 'debug':
        return <DebugPage />
      default:
        return <SearchPage onSearch={handleSearch} onSearchWithDetails={handleSearchWithDetails} searchQuery={searchQuery} searchResults={searchResults} />
    }
  }

  return (
    <div className="app">
      <nav className="sidebar">
        <div className="sidebar-header">
          <h2>File Search</h2>
        </div>
        <ul className="sidebar-nav">
          <li className={currentPage === 'search' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('search')}>
              🔍 Search
            </button>
          </li>
          <li className={currentPage === 'dashboard' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('dashboard')}>
              📊 Dashboard
            </button>
          </li>
          <li className={currentPage === 'files' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('files')}>
              📁 Files
            </button>
          </li>
          <li className={currentPage === 'settings' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('settings')}>
              ⚙️ Settings
            </button>
          </li>
          <li className={currentPage === 'prompt' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('prompt')}>
              🤖 Prompt
            </button>
          </li>
          <li className={currentPage === 'debug' ? 'active' : ''}>
            <button onClick={() => setCurrentPage('debug')}>
              🐛 Debug
            </button>
          </li>
        </ul>
        
        {(indexingStatus || systemStatus) && (
          <div className="sidebar-status">
            {indexingStatus && (
              <>
                <div className="status-item">
                  <span className="status-label">Indexing Status:</span>
                  <span className={`status-value ${indexingStatus.state}`}>
                    {indexingStatus.state}
                  </span>
                </div>
                {indexingStatus.state === 'running' && (
                  <div className="status-item">
                    <span className="status-label">Progress:</span>
                    <span className="status-value">
                      {indexingStatus.filesProcessed} / {indexingStatus.totalFiles}
                    </span>
                  </div>
                )}
              </>
            )}
            {systemStatus && (
              <>
                <div className="status-item" style={{ marginTop: '8px' }}>
                  <span className="status-label" style={{ fontSize: '0.85em' }}>System Status:</span>
                  <span className={`status-value ${systemStatus.status === 'healthy' ? 'connected' : 'disconnected'}`} style={{ fontSize: '0.85em' }}>
                    {systemStatus.status === 'healthy' ? 'Healthy' : systemStatus.status === 'disconnected' ? 'Disconnected' : 'Error'}
                  </span>
                </div>
                {systemStatus.status === 'healthy' && systemStatus.uptime && (
                  <div className="status-item">
                    <span className="status-label" style={{ fontSize: '0.85em' }}>Uptime:</span>
                    <span className="status-value" style={{ fontSize: '0.85em' }}>
                      {Math.floor(systemStatus.uptime / 60)}m
                    </span>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </nav>
      
      <main className="main-content">
        {renderPage()}
      </main>
    </div>
  )
}

export default App