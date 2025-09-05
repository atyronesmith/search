import axios from 'axios';

const API_BASE_URL = 'http://localhost:8080/api/v1';

class ApiServiceClass {
  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        console.error('API Error:', error);
        if (error.response) {
          // The request was made and the server responded with a status code
          // that falls out of the range of 2xx
          throw new Error(`API Error: ${error.response.status} - ${error.response.data?.error || error.message}`);
        } else if (error.request) {
          // The request was made but no response was received
          throw new Error('No response from server. Please check if the backend is running.');
        } else {
          // Something happened in setting up the request that triggered an Error
          throw new Error(`Request Error: ${error.message}`);
        }
      }
    );
  }

  // Search endpoints
  async search(query, options = {}) {
    const params = {
      q: query,
      ...options
    };
    
    const response = await this.client.get('/search', { params });
    return response.data;
  }

  async searchSuggestions(query) {
    const response = await this.client.get('/search/suggest', {
      params: { q: query }
    });
    return response.data;
  }

  async getSearchHistory() {
    const response = await this.client.get('/search/history');
    return response.data;
  }

  // File management endpoints
  async getFiles(options = {}) {
    const response = await this.client.get('/files', { params: options });
    return response.data;
  }

  async getFile(fileId) {
    const response = await this.client.get(`/files/${fileId}`);
    return response.data;
  }

  async getFileContent(fileId) {
    const response = await this.client.get(`/files/${fileId}/content`);
    return response.data;
  }

  async reindexFile(fileId) {
    const response = await this.client.post(`/files/${fileId}/reindex`);
    return response.data;
  }

  // Indexing control endpoints
  async startIndexing(path = '') {
    const response = await this.client.post('/indexing/start', { path });
    return response.data;
  }

  async stopIndexing() {
    const response = await this.client.post('/indexing/stop');
    return response.data;
  }

  async pauseIndexing() {
    const response = await this.client.post('/indexing/pause');
    return response.data;
  }

  async resumeIndexing() {
    const response = await this.client.post('/indexing/resume');
    return response.data;
  }

  async getIndexingStatus() {
    const response = await this.client.get('/indexing/status');
    return response.data;
  }

  async getIndexingStats() {
    const response = await this.client.get('/indexing/stats');
    return response.data;
  }

  async scanFiles(path = '') {
    const response = await this.client.post('/indexing/scan', { path });
    return response.data;
  }

  // System endpoints
  async getSystemStatus() {
    const response = await this.client.get('/status');
    return response.data;
  }

  async getHealthCheck() {
    const response = await this.client.get('/health');
    return response.data;
  }

  async getMetrics() {
    const response = await this.client.get('/metrics');
    return response.data;
  }

  async getConfig() {
    const response = await this.client.get('/config');
    return response.data;
  }

  async updateConfig(config) {
    const response = await this.client.put('/config', config);
    return response.data;
  }

  // Utility methods
  formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  formatDate(dateString) {
    return new Date(dateString).toLocaleString();
  }

  getFileTypeIcon(filename) {
    const ext = filename.split('.').pop()?.toLowerCase();
    
    const iconMap = {
      // Documents
      'pdf': '📄',
      'doc': '📄', 'docx': '📄',
      'txt': '📄', 'md': '📄',
      'rtf': '📄',
      
      // Spreadsheets
      'xls': '📊', 'xlsx': '📊',
      'csv': '📊',
      
      // Presentations
      'ppt': '📈', 'pptx': '📈',
      
      // Images
      'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️',
      'gif': '🖼️', 'svg': '🖼️', 'bmp': '🖼️',
      
      // Audio
      'mp3': '🎵', 'wav': '🎵', 'flac': '🎵',
      'aac': '🎵', 'ogg': '🎵',
      
      // Video
      'mp4': '🎬', 'avi': '🎬', 'mkv': '🎬',
      'mov': '🎬', 'wmv': '🎬',
      
      // Archives
      'zip': '📦', 'rar': '📦', '7z': '📦',
      'tar': '📦', 'gz': '📦',
      
      // Code files
      'js': '💻', 'ts': '💻', 'jsx': '💻', 'tsx': '💻',
      'py': '🐍', 'java': '☕', 'cpp': '⚙️', 'c': '⚙️',
      'go': '🐹', 'rs': '🦀', 'php': '🐘',
      'rb': '💎', 'swift': '🦉', 'kt': '🎯',
      'html': '🌐', 'css': '🎨', 'scss': '🎨',
      'json': '📋', 'xml': '📋', 'yaml': '📋', 'yml': '📋',
      
      // Default
      'default': '📄'
    };
    
    return iconMap[ext] || iconMap.default;
  }
}

export const ApiService = new ApiServiceClass();