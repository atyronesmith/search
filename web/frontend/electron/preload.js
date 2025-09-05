const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electronAPI', {
  getAppPath: () => ipcRenderer.invoke('get-app-path'),
  getPlatform: () => ipcRenderer.invoke('get-platform'),
  minimizeWindow: () => ipcRenderer.invoke('minimize-window'),
  maximizeWindow: () => ipcRenderer.invoke('maximize-window'),
  closeWindow: () => ipcRenderer.invoke('close-window'),
  
  // Listen for main process events
  onNewSearch: (callback) => {
    ipcRenderer.on('new-search', callback);
  },
  onIndexDirectory: (callback) => {
    ipcRenderer.on('index-directory', (event, path) => callback(path));
  },
  onOpenSettings: (callback) => {
    ipcRenderer.on('open-settings', callback);
  },
  onFocusSearch: (callback) => {
    ipcRenderer.on('focus-search', callback);
  },
  onNavigate: (callback) => {
    ipcRenderer.on('navigate', (event, path) => callback(path));
  },
  
  // Remove listeners
  removeAllListeners: (channel) => {
    ipcRenderer.removeAllListeners(channel);
  }
});