import React from 'react';
import { Wifi, WifiOff, Activity, HardDrive, Clock } from 'lucide-react';

const StatusBar = ({ systemStatus, websocketConnected }) => {
  const formatUptime = (seconds) => {
    if (!seconds) return 'N/A';
    
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    
    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    } else if (minutes > 0) {
      return `${minutes}m ${secs}s`;
    } else {
      return `${secs}s`;
    }
  };

  const getIndexingStatusText = () => {
    if (!systemStatus.connected) return 'Disconnected';
    if (systemStatus.indexing) return 'Indexing...';
    return 'Ready';
  };

  const getIndexingStatusColor = () => {
    if (!systemStatus.connected) return 'text-red-500';
    if (systemStatus.indexing) return 'text-blue-500';
    return 'text-green-500';
  };

  return (
    <div className="bg-gray-800 text-white px-6 py-2 text-sm">
      <div className="flex items-center justify-between">
        {/* Left side - Connection and indexing status */}
        <div className="flex items-center gap-6">
          {/* Backend Connection */}
          <div className="flex items-center gap-2">
            {systemStatus.connected ? (
              <>
                <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse" />
                <span className="text-gray-300">Backend Connected</span>
              </>
            ) : (
              <>
                <div className="w-2 h-2 bg-red-400 rounded-full" />
                <span className="text-gray-300">Backend Disconnected</span>
              </>
            )}
          </div>

          {/* WebSocket Connection */}
          <div className="flex items-center gap-2">
            {websocketConnected ? (
              <Wifi className="h-4 w-4 text-green-400" />
            ) : (
              <WifiOff className="h-4 w-4 text-red-400" />
            )}
            <span className="text-gray-300">
              WebSocket {websocketConnected ? 'Connected' : 'Disconnected'}
            </span>
          </div>

          {/* Indexing Status */}
          <div className="flex items-center gap-2">
            <Activity className={`h-4 w-4 ${getIndexingStatusColor()}`} />
            <span className={`${getIndexingStatusColor()}`}>
              {getIndexingStatusText()}
            </span>
            {systemStatus.indexing && systemStatus.total_files > 0 && (
              <span className="text-gray-400">
                ({systemStatus.indexed_files || 0}/{systemStatus.total_files} files)
              </span>
            )}
          </div>
        </div>

        {/* Right side - System info */}
        <div className="flex items-center gap-6">
          {/* File Statistics */}
          {systemStatus.connected && (
            <div className="flex items-center gap-2">
              <HardDrive className="h-4 w-4 text-gray-400" />
              <span className="text-gray-300">
                {(systemStatus.indexed_files || 0).toLocaleString()} files indexed
              </span>
            </div>
          )}

          {/* Uptime (if available) */}
          {systemStatus.uptime && (
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-gray-400" />
              <span className="text-gray-300">
                Uptime: {formatUptime(systemStatus.uptime)}
              </span>
            </div>
          )}

          {/* Version (if available from Electron) */}
          <div className="text-gray-400 text-xs">
            File Search System v1.0.0
          </div>
        </div>
      </div>
    </div>
  );
};

export default StatusBar;