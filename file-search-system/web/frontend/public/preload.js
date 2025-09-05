const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electronAPI', {
  // App info
  getAppVersion: () => ipcRenderer.invoke('get-app-version'),
  getPlatform: () => ipcRenderer.invoke('get-platform'),
  
  // Menu events
  onMenuOpenFolder: (callback) => ipcRenderer.on('menu-open-folder', callback),
  onMenuSettings: (callback) => ipcRenderer.on('menu-settings', callback),
  onMenuSearch: (callback) => ipcRenderer.on('menu-search', callback),
  onMenuStartIndexing: (callback) => ipcRenderer.on('menu-start-indexing', callback),
  onMenuStopIndexing: (callback) => ipcRenderer.on('menu-stop-indexing', callback),
  
  // Remove listeners
  removeAllListeners: (channel) => ipcRenderer.removeAllListeners(channel)
});