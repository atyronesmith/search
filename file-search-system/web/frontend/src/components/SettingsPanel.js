import React, { useState, useEffect } from 'react';
import {
  Save, RefreshCcw, FolderPlus, Trash2,
  Settings, Database, Zap, Shield,
  AlertCircle, CheckCircle, Info
} from 'lucide-react';
import { ApiService } from '../services/ApiService';

const SettingsPanel = () => {
  const [config, setConfig] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState(null); // 'success', 'error', null
  const [activeTab, setActiveTab] = useState('paths');

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await ApiService.getConfig();
      setConfig(response);
      setIsLoading(false);
    } catch (error) {
      console.error('Failed to load config:', error);
      setIsLoading(false);
    }
  };

  const saveConfig = async () => {
    if (!config) return;

    setIsSaving(true);
    setSaveStatus(null);

    try {
      await ApiService.updateConfig(config);
      setSaveStatus('success');
      setTimeout(() => setSaveStatus(null), 3000);
    } catch (error) {
      console.error('Failed to save config:', error);
      setSaveStatus('error');
      setTimeout(() => setSaveStatus(null), 5000);
    } finally {
      setIsSaving(false);
    }
  };

  const updateConfig = (path, value) => {
    setConfig(prev => {
      const newConfig = { ...prev };
      const keys = path.split('.');
      let current = newConfig;

      for (let i = 0; i < keys.length - 1; i++) {
        if (!current[keys[i]]) current[keys[i]] = {};
        current = current[keys[i]];
      }

      current[keys[keys.length - 1]] = value;
      return newConfig;
    });
  };

  const addPath = () => {
    const newPath = '';
    updateConfig('indexing.paths', [...(config.indexing?.paths || []), newPath]);
  };

  const removePath = (index) => {
    const paths = [...(config.indexing?.paths || [])];
    paths.splice(index, 1);
    updateConfig('indexing.paths', paths);
  };

  const updatePath = (index, value) => {
    const paths = [...(config.indexing?.paths || [])];
    paths[index] = value;
    updateConfig('indexing.paths', paths);
  };

  const addIgnorePattern = () => {
    const patterns = [...(config.indexing?.ignore_patterns || []), ''];
    updateConfig('indexing.ignore_patterns', patterns);
  };

  const removeIgnorePattern = (index) => {
    const patterns = [...(config.indexing?.ignore_patterns || [])];
    patterns.splice(index, 1);
    updateConfig('indexing.ignore_patterns', patterns);
  };

  const updateIgnorePattern = (index, value) => {
    const patterns = [...(config.indexing?.ignore_patterns || [])];
    patterns[index] = value;
    updateConfig('indexing.ignore_patterns', patterns);
  };

  if (isLoading) {
    return (
      <div className="max-w-4xl mx-auto">
        <div className="bg-white rounded-lg shadow-md p-8">
          <div className="flex items-center justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
            <span className="ml-3 text-gray-600">Loading settings...</span>
          </div>
        </div>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="max-w-4xl mx-auto">
        <div className="bg-white rounded-lg shadow-md p-8 text-center">
          <AlertCircle className="h-12 w-12 text-red-400 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">Failed to Load Settings</h3>
          <p className="text-gray-600 mb-4">
            Could not load configuration from the server.
          </p>
          <button
            onClick={loadConfig}
            className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
          >
            Try Again
          </button>
        </div>
      </div>
    );
  }

  const tabs = [
    { id: 'paths', label: 'Indexing Paths', icon: <FolderPlus className="h-4 w-4" /> },
    { id: 'performance', label: 'Performance', icon: <Zap className="h-4 w-4" /> },
    { id: 'database', label: 'Database', icon: <Database className="h-4 w-4" /> },
    { id: 'advanced', label: 'Advanced', icon: <Settings className="h-4 w-4" /> }
  ];

  return (
    <div className="max-w-4xl mx-auto">
      {/* Header */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Settings</h1>
            <p className="text-gray-600 mt-1">Configure the file search system</p>
          </div>
          <div className="flex items-center gap-3">
            {saveStatus === 'success' && (
              <div className="flex items-center gap-2 text-green-600 bg-green-50 px-3 py-1 rounded-md">
                <CheckCircle className="h-4 w-4" />
                Settings saved
              </div>
            )}
            {saveStatus === 'error' && (
              <div className="flex items-center gap-2 text-red-600 bg-red-50 px-3 py-1 rounded-md">
                <AlertCircle className="h-4 w-4" />
                Save failed
              </div>
            )}
            <button
              onClick={loadConfig}
              className="flex items-center gap-2 px-4 py-2 text-gray-600 hover:text-gray-800 transition-colors"
              title="Reload settings"
            >
              <RefreshCcw className="h-4 w-4" />
              Reload
            </button>
            <button
              onClick={saveConfig}
              disabled={isSaving}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 disabled:opacity-50 transition-colors"
            >
              <Save className="h-4 w-4" />
              {isSaving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="bg-white rounded-lg shadow-md">
        <div className="border-b border-gray-200">
          <nav className="flex">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-6 py-3 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab.id
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="p-6">
          {/* Indexing Paths Tab */}
          {activeTab === 'paths' && (
            <div className="space-y-6">
              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-3">Indexing Paths</h3>
                <p className="text-sm text-gray-600 mb-4">
                  Specify the directories to index for file search.
                </p>

                <div className="space-y-3">
                  {(config.indexing?.paths || []).map((path, index) => (
                    <div key={index} className="flex items-center gap-3">
                      <input
                        type="text"
                        value={path}
                        onChange={(e) => updatePath(index, e.target.value)}
                        placeholder="/path/to/directory"
                        className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                      />
                      <button
                        onClick={() => removePath(index)}
                        className="p-2 text-red-600 hover:bg-red-50 rounded-md transition-colors"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))}

                  <button
                    onClick={addPath}
                    className="flex items-center gap-2 px-4 py-2 text-blue-600 hover:bg-blue-50 rounded-md transition-colors"
                  >
                    <FolderPlus className="h-4 w-4" />
                    Add Path
                  </button>
                </div>
              </div>

              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-3">Ignore Patterns</h3>
                <p className="text-sm text-gray-600 mb-4">
                  File patterns to exclude from indexing (glob patterns supported).
                </p>

                <div className="space-y-3">
                  {(config.indexing?.ignore_patterns || []).map((pattern, index) => (
                    <div key={index} className="flex items-center gap-3">
                      <input
                        type="text"
                        value={pattern}
                        onChange={(e) => updateIgnorePattern(index, e.target.value)}
                        placeholder="*.tmp, node_modules/**, .git/**"
                        className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                      />
                      <button
                        onClick={() => removeIgnorePattern(index)}
                        className="p-2 text-red-600 hover:bg-red-50 rounded-md transition-colors"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))}

                  <button
                    onClick={addIgnorePattern}
                    className="flex items-center gap-2 px-4 py-2 text-blue-600 hover:bg-blue-50 rounded-md transition-colors"
                  >
                    <FolderPlus className="h-4 w-4" />
                    Add Pattern
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Performance Tab */}
          {activeTab === 'performance' && (
            <div className="space-y-6">
              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-3">Resource Limits</h3>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      CPU Threshold (%)
                    </label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      value={config.performance?.cpu_threshold || 90}
                      onChange={(e) => updateConfig('performance.cpu_threshold', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Pause indexing when CPU usage exceeds this threshold
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Memory Threshold (%)
                    </label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      value={config.performance?.memory_threshold || 90}
                      onChange={(e) => updateConfig('performance.memory_threshold', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Pause indexing when memory usage exceeds this threshold
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Files per Minute
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="1000"
                      value={config.performance?.files_per_minute || 60}
                      onChange={(e) => updateConfig('performance.files_per_minute', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Embeddings per Minute
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="1000"
                      value={config.performance?.embeddings_per_minute || 120}
                      onChange={(e) => updateConfig('performance.embeddings_per_minute', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Database Tab */}
          {activeTab === 'database' && (
            <div className="space-y-6">
              <div className="flex justify-end">
                <button
                  onClick={async () => {
                    if (window.confirm('Are you sure you want to reset the database? This will erase all data.')) {
                      try {
                        await ApiService.resetDatabase();
                        alert('Database reset successfully.');
                      } catch (err) {
                        alert('Failed to reset database.');
                      }
                    }
                  }}
                  className="px-4 py-2 bg-red-500 text-white rounded-md hover:bg-red-600 transition-colors"
                >
                  Reset Database
                </button>
              </div>
              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-3">Database Configuration</h3>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Host
                    </label>
                    <input
                      type="text"
                      value={config.database?.host || 'localhost'}
                      onChange={(e) => updateConfig('database.host', e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Port
                    </label>
                    <input
                      type="number"
                      value={config.database?.port || 5432}
                      onChange={(e) => updateConfig('database.port', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Database Name
                    </label>
                    <input
                      type="text"
                      value={config.database?.name || 'file_search'}
                      onChange={(e) => updateConfig('database.name', e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Max Connections
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="100"
                      value={config.database?.max_connections || 10}
                      onChange={(e) => updateConfig('database.max_connections', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Advanced Tab */}
          {activeTab === 'advanced' && (
            <div className="space-y-6">
              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-3">Search Configuration</h3>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Vector Search Weight
                    </label>
                    <input
                      type="number"
                      min="0"
                      max="1"
                      step="0.1"
                      value={config.search?.vector_weight || 0.6}
                      onChange={(e) => updateConfig('search.vector_weight', parseFloat(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      BM25 Search Weight
                    </label>
                    <input
                      type="number"
                      min="0"
                      max="1"
                      step="0.1"
                      value={config.search?.bm25_weight || 0.3}
                      onChange={(e) => updateConfig('search.bm25_weight', parseFloat(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Chunk Size
                    </label>
                    <input
                      type="number"
                      min="100"
                      max="2000"
                      value={config.chunking?.chunk_size || 512}
                      onChange={(e) => updateConfig('chunking.chunk_size', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Chunk Overlap
                    </label>
                    <input
                      type="number"
                      min="0"
                      max="200"
                      value={config.chunking?.overlap || 50}
                      onChange={(e) => updateConfig('chunking.overlap', parseInt(e.target.value))}
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                    />
                  </div>
                </div>
              </div>

              <div className="bg-blue-50 border border-blue-200 rounded-md p-4">
                <div className="flex items-start gap-3">
                  <Info className="h-5 w-5 text-blue-500 mt-0.5" />
                  <div>
                    <h4 className="text-sm font-medium text-blue-900">Configuration Note</h4>
                    <p className="text-sm text-blue-700 mt-1">
                      Some settings require a restart of the indexing service to take effect.
                      Monitor the performance metrics after making changes.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default SettingsPanel;