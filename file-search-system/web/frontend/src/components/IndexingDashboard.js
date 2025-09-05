import React, { useState, useEffect } from 'react';
import { 
  Play, Pause, Square, RotateCcw, FolderOpen, 
  Activity, Clock, HardDrive, Cpu, BarChart3,
  FileText, Database, Zap
} from 'lucide-react';
import { ApiService } from '../services/ApiService';

const IndexingDashboard = ({ systemStatus }) => {
  const [indexingStats, setIndexingStats] = useState(null);
  const [metrics, setMetrics] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedPath, setSelectedPath] = useState('');

  useEffect(() => {
    loadDashboardData();
    const interval = setInterval(loadDashboardData, 2000); // Update every 2 seconds
    return () => clearInterval(interval);
  }, []);

  const loadDashboardData = async () => {
    try {
      const [stats, metricsData] = await Promise.all([
        ApiService.getIndexingStats(),
        ApiService.getMetrics()
      ]);
      
      setIndexingStats(stats);
      setMetrics(metricsData);
      setIsLoading(false);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
      setIsLoading(false);
    }
  };

  const handleStartIndexing = async () => {
    try {
      await ApiService.startIndexing(selectedPath);
      await loadDashboardData();
    } catch (error) {
      console.error('Failed to start indexing:', error);
    }
  };

  const handleStopIndexing = async () => {
    try {
      await ApiService.stopIndexing();
      await loadDashboardData();
    } catch (error) {
      console.error('Failed to stop indexing:', error);
    }
  };

  const handlePauseIndexing = async () => {
    try {
      await ApiService.pauseIndexing();
      await loadDashboardData();
    } catch (error) {
      console.error('Failed to pause indexing:', error);
    }
  };

  const handleResumeIndexing = async () => {
    try {
      await ApiService.resumeIndexing();
      await loadDashboardData();
    } catch (error) {
      console.error('Failed to resume indexing:', error);
    }
  };

  const handleScanFiles = async () => {
    try {
      await ApiService.scanFiles(selectedPath);
      await loadDashboardData();
    } catch (error) {
      console.error('Failed to scan files:', error);
    }
  };

  const formatDuration = (seconds) => {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'running': return 'text-green-600 bg-green-50';
      case 'paused': return 'text-yellow-600 bg-yellow-50';
      case 'stopped': return 'text-gray-600 bg-gray-50';
      case 'error': return 'text-red-600 bg-red-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  if (isLoading) {
    return (
      <div className="max-w-6xl mx-auto">
        <div className="bg-white rounded-lg shadow-md p-8">
          <div className="flex items-center justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
            <span className="ml-3 text-gray-600">Loading dashboard...</span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      {/* Control Panel */}
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-xl font-semibold text-gray-900 mb-4">Indexing Control</h2>
        
        <div className="flex flex-col md:flex-row gap-4 items-start">
          {/* Path Selection */}
          <div className="flex-1">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Target Path (optional)
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={selectedPath}
                onChange={(e) => setSelectedPath(e.target.value)}
                placeholder="Leave empty to index all configured paths"
                className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
              />
              <button
                onClick={() => setSelectedPath('')}
                className="px-3 py-2 text-gray-500 hover:text-gray-700"
                title="Clear path"
              >
                <FolderOpen className="h-5 w-5" />
              </button>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2">
            {!systemStatus.indexing ? (
              <button
                onClick={handleStartIndexing}
                className="flex items-center gap-2 px-4 py-2 bg-green-500 text-white rounded-md hover:bg-green-600 transition-colors"
              >
                <Play className="h-4 w-4" />
                Start
              </button>
            ) : (
              <>
                <button
                  onClick={handlePauseIndexing}
                  className="flex items-center gap-2 px-4 py-2 bg-yellow-500 text-white rounded-md hover:bg-yellow-600 transition-colors"
                >
                  <Pause className="h-4 w-4" />
                  Pause
                </button>
                <button
                  onClick={handleStopIndexing}
                  className="flex items-center gap-2 px-4 py-2 bg-red-500 text-white rounded-md hover:bg-red-600 transition-colors"
                >
                  <Square className="h-4 w-4" />
                  Stop
                </button>
              </>
            )}
            <button
              onClick={handleScanFiles}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
            >
              <RotateCcw className="h-4 w-4" />
              Scan
            </button>
          </div>
        </div>
      </div>

      {/* Status Overview */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex items-center">
            <div className="p-2 bg-blue-100 rounded-md">
              <Activity className="h-6 w-6 text-blue-600" />
            </div>
            <div className="ml-4">
              <h3 className="text-sm font-medium text-gray-500">Status</h3>
              <p className={`text-lg font-semibold px-2 py-1 rounded-md ${
                getStatusColor(indexingStats?.status || 'stopped')
              }`}>
                {indexingStats?.status || 'Stopped'}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex items-center">
            <div className="p-2 bg-green-100 rounded-md">
              <FileText className="h-6 w-6 text-green-600" />
            </div>
            <div className="ml-4">
              <h3 className="text-sm font-medium text-gray-500">Files Indexed</h3>
              <p className="text-2xl font-semibold text-gray-900">
                {(indexingStats?.indexed_files || 0).toLocaleString()}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex items-center">
            <div className="p-2 bg-purple-100 rounded-md">
              <Database className="h-6 w-6 text-purple-600" />
            </div>
            <div className="ml-4">
              <h3 className="text-sm font-medium text-gray-500">Total Files</h3>
              <p className="text-2xl font-semibold text-gray-900">
                {(indexingStats?.total_files || 0).toLocaleString()}
              </p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex items-center">
            <div className="p-2 bg-yellow-100 rounded-md">
              <Zap className="h-6 w-6 text-yellow-600" />
            </div>
            <div className="ml-4">
              <h3 className="text-sm font-medium text-gray-500">Rate</h3>
              <p className="text-2xl font-semibold text-gray-900">
                {indexingStats?.files_per_second || 0}/s
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Progress Bar */}
      {systemStatus.indexing && indexingStats?.total_files > 0 && (
        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-lg font-medium text-gray-900">Progress</h3>
            <span className="text-sm text-gray-500">
              {indexingStats.indexed_files} / {indexingStats.total_files}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-3">
            <div 
              className="bg-blue-500 h-3 rounded-full transition-all duration-300"
              style={{ 
                width: `${Math.min(100, (indexingStats.indexed_files / indexingStats.total_files) * 100)}%` 
              }}
            />
          </div>
          <div className="flex justify-between mt-2 text-sm text-gray-600">
            <span>
              {Math.round((indexingStats.indexed_files / indexingStats.total_files) * 100)}% complete
            </span>
            {indexingStats.estimated_time_remaining && (
              <span>
                ~{formatDuration(indexingStats.estimated_time_remaining)} remaining
              </span>
            )}
          </div>
        </div>
      )}

      {/* System Metrics */}
      {metrics && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Resource Usage */}
          <div className="bg-white rounded-lg shadow-md p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-4 flex items-center gap-2">
              <BarChart3 className="h-5 w-5" />
              Resource Usage
            </h3>
            <div className="space-y-4">
              {/* CPU Usage */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-700 flex items-center gap-2">
                    <Cpu className="h-4 w-4" />
                    CPU
                  </span>
                  <span className="text-sm text-gray-600">
                    {Math.round(metrics.cpu_usage || 0)}%
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div 
                    className={`h-2 rounded-full transition-all duration-300 ${
                      (metrics.cpu_usage || 0) > 80 ? 'bg-red-500' : 
                      (metrics.cpu_usage || 0) > 60 ? 'bg-yellow-500' : 'bg-green-500'
                    }`}
                    style={{ width: `${Math.min(100, metrics.cpu_usage || 0)}%` }}
                  />
                </div>
              </div>

              {/* Memory Usage */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-700 flex items-center gap-2">
                    <HardDrive className="h-4 w-4" />
                    Memory
                  </span>
                  <span className="text-sm text-gray-600">
                    {Math.round(metrics.memory_usage || 0)}%
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div 
                    className={`h-2 rounded-full transition-all duration-300 ${
                      (metrics.memory_usage || 0) > 80 ? 'bg-red-500' : 
                      (metrics.memory_usage || 0) > 60 ? 'bg-yellow-500' : 'bg-green-500'
                    }`}
                    style={{ width: `${Math.min(100, metrics.memory_usage || 0)}%` }}
                  />
                </div>
              </div>

              {/* Disk Usage */}
              {metrics.disk_usage && (
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium text-gray-700">
                      Disk
                    </span>
                    <span className="text-sm text-gray-600">
                      {Math.round(metrics.disk_usage)}%
                    </span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div 
                      className={`h-2 rounded-full transition-all duration-300 ${
                        metrics.disk_usage > 80 ? 'bg-red-500' : 
                        metrics.disk_usage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                      }`}
                      style={{ width: `${Math.min(100, metrics.disk_usage)}%` }}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Statistics */}
          <div className="bg-white rounded-lg shadow-md p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-4 flex items-center gap-2">
              <Clock className="h-5 w-5" />
              Statistics
            </h3>
            <div className="space-y-3">
              {indexingStats?.uptime && (
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600">Uptime</span>
                  <span className="text-sm font-medium">
                    {formatDuration(indexingStats.uptime)}
                  </span>
                </div>
              )}
              
              {indexingStats?.total_chunks && (
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600">Total Chunks</span>
                  <span className="text-sm font-medium">
                    {indexingStats.total_chunks.toLocaleString()}
                  </span>
                </div>
              )}
              
              {indexingStats?.total_embeddings && (
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600">Total Embeddings</span>
                  <span className="text-sm font-medium">
                    {indexingStats.total_embeddings.toLocaleString()}
                  </span>
                </div>
              )}
              
              {indexingStats?.avg_file_size && (
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600">Avg File Size</span>
                  <span className="text-sm font-medium">
                    {ApiService.formatFileSize(indexingStats.avg_file_size)}
                  </span>
                </div>
              )}
              
              {indexingStats?.queue_size !== undefined && (
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600">Queue Size</span>
                  <span className="text-sm font-medium">
                    {indexingStats.queue_size}
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Recent Activity */}
      {indexingStats?.recent_files && indexingStats.recent_files.length > 0 && (
        <div className="bg-white rounded-lg shadow-md p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">Recent Activity</h3>
          <div className="space-y-2 max-h-64 overflow-y-auto">
            {indexingStats.recent_files.map((file, index) => (
              <div key={index} className="flex items-center justify-between py-2 px-3 bg-gray-50 rounded-md">
                <div className="flex items-center gap-3">
                  <span className="text-lg">
                    {ApiService.getFileTypeIcon(file.filename)}
                  </span>
                  <div>
                    <div className="text-sm font-medium text-gray-900 truncate">
                      {file.filename}
                    </div>
                    <div className="text-xs text-gray-500 truncate">
                      {file.path}
                    </div>
                  </div>
                </div>
                <div className="text-xs text-gray-500">
                  {ApiService.formatDate(file.indexed_at)}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default IndexingDashboard;