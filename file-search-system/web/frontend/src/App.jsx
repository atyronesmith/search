import React, { useState, useEffect } from 'react';
import SearchInterface from './components/SearchInterface';
import IndexingDashboard from './components/IndexingDashboard';
import SettingsPanel from './components/SettingsPanel';
import FileManager from './components/FileManager';
import StatusBar from './components/StatusBar';
import { ApiService } from './services/ApiService';
import { useWebSocket } from './hooks/useWebSocket';

function App() {
  const [activeTab, setActiveTab] = useState('search');
  const [systemStatus, setSystemStatus] = useState({
    connected: false,
    indexing: false,
    indexed_files: 0,
    total_files: 0
  });

  const { lastMessage, isConnected } = useWebSocket('ws://localhost:8080/api/v1/ws');

  useEffect(() => {
    // Initialize API service and check system status
    const checkStatus = async () => {
      try {
        const status = await ApiService.getSystemStatus();
        setSystemStatus(prev => ({ ...prev, ...status, connected: true }));
      } catch (error) {
        console.error('Failed to connect to backend:', error);
        setSystemStatus(prev => ({ ...prev, connected: false }));
      }
    };

    checkStatus();
    const interval = setInterval(checkStatus, 5000); // Check every 5 seconds

    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    // Handle WebSocket messages
    if (lastMessage) {
      try {
        const data = JSON.parse(lastMessage.data);
        if (data.type === 'indexing_status') {
          setSystemStatus(prev => ({
            ...prev,
            indexing: data.data.is_running,
            indexed_files: data.data.indexed_files || 0,
            total_files: data.data.total_files || 0
          }));
        }
      } catch (error) {
        console.error('Error parsing WebSocket message:', error);
      }
    }
  }, [lastMessage]);

  useEffect(() => {
    // Handle Electron menu events
    if (window.electronAPI) {
      const handleMenuOpenFolder = () => {
        setActiveTab('files');
      };

      const handleMenuSettings = () => {
        setActiveTab('settings');
      };

      const handleMenuSearch = () => {
        setActiveTab('search');
      };

      const handleMenuStartIndexing = async () => {
        try {
          await ApiService.startIndexing();
        } catch (error) {
          console.error('Failed to start indexing:', error);
        }
      };

      const handleMenuStopIndexing = async () => {
        try {
          await ApiService.stopIndexing();
        } catch (error) {
          console.error('Failed to stop indexing:', error);
        }
      };

      window.electronAPI.onMenuOpenFolder(handleMenuOpenFolder);
      window.electronAPI.onMenuSettings(handleMenuSettings);
      window.electronAPI.onMenuSearch(handleMenuSearch);
      window.electronAPI.onMenuStartIndexing(handleMenuStartIndexing);
      window.electronAPI.onMenuStopIndexing(handleMenuStopIndexing);

      return () => {
        window.electronAPI.removeAllListeners('menu-open-folder');
        window.electronAPI.removeAllListeners('menu-settings');
        window.electronAPI.removeAllListeners('menu-search');
        window.electronAPI.removeAllListeners('menu-start-indexing');
        window.electronAPI.removeAllListeners('menu-stop-indexing');
      };
    }
  }, []);

  const tabs = [
    { id: 'search', label: 'Search', icon: '🔍' },
    { id: 'indexing', label: 'Indexing', icon: '📊' },
    { id: 'files', label: 'Files', icon: '📁' },
    { id: 'settings', label: 'Settings', icon: '⚙️' }
  ];

  const renderContent = () => {
    switch (activeTab) {
      case 'search':
        return <SearchInterface />;
      case 'indexing':
        return <IndexingDashboard systemStatus={systemStatus} />;
      case 'files':
        return <FileManager />;
      case 'settings':
        return <SettingsPanel />;
      default:
        return <SearchInterface />;
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-full mx-auto px-6 py-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <h1 className="text-xl font-semibold text-gray-900">
                File Search System
              </h1>
              <div className={`flex items-center gap-2 px-2 py-1 rounded-md text-xs ${
                systemStatus.connected 
                  ? 'bg-green-50 text-green-600' 
                  : 'bg-red-50 text-red-600'
              }`}>
                <div className={`w-2 h-2 rounded-full ${
                  systemStatus.connected ? 'bg-green-500' : 'bg-red-500'
                }`} />
                {systemStatus.connected ? 'Connected' : 'Disconnected'}
              </div>
            </div>
            <nav className="flex gap-1">
              {tabs.map(tab => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all ${
                    activeTab === tab.id
                      ? 'bg-blue-500 text-white'
                      : 'text-gray-600 hover:bg-gray-100'
                  }`}
                >
                  <span>{tab.icon}</span>
                  {tab.label}
                </button>
              ))}
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 p-6">
        {renderContent()}
      </main>

      {/* Status Bar */}
      <StatusBar 
        systemStatus={systemStatus} 
        websocketConnected={isConnected}
      />
    </div>
  );
}

export default App;