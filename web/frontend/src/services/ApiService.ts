import axios, { AxiosInstance, AxiosResponse } from 'axios';

export interface SearchRequest {
  query: string;
  limit?: number;
  offset?: number;
  filters?: SearchFilters;
}

export interface SearchFilters {
  fileTypes?: string[];
  dateRange?: {
    start: string;
    end: string;
  };
  sizeRange?: {
    min: number;
    max: number;
  };
  paths?: string[];
}

export interface SearchResult {
  queryId: string;
  results: FileResult[];
  totalResults: number;
  processingTime: number;
}

export interface FileResult {
  id: string;
  path: string;
  name: string;
  type: string;
  size: number;
  modifiedAt: string;
  score: number;
  highlights: string[];
  snippet: string;
}

export interface FileInfo {
  id: string;
  path: string;
  name: string;
  type: string;
  size: number;
  createdAt: string;
  modifiedAt: string;
  lastIndexed: string;
  hash: string;
  chunks: number;
}

export interface IndexingStats {
  totalFiles: number;
  indexedFiles: number;
  totalChunks: number;
  totalSize: number;
  lastIndexed: string;
  indexingRate: number;
  errors: number;
}

export interface SystemStatus {
  status: string;
  uptime: number;
  version: string;
  database: {
    connected: boolean;
    latency: number;
  };
  embeddings: {
    available: boolean;
    model: string;
  };
  indexing: {
    active: boolean;
    state: string;
  };
  resources: {
    cpu: number;
    memory: number;
    disk: number;
  };
}

export interface Config {
  indexPaths: string[];
  excludePatterns: string[];
  fileTypes: string[];
  chunkSize: number;
  chunkOverlap: number;
  embeddingModel: string;
  maxFileSize: number;
  scanInterval: number;
  resourceLimits: {
    maxCpu: number;
    maxMemory: number;
  };
}

class ApiService {
  private api: AxiosInstance;

  constructor(baseURL: string) {
    this.api = axios.create({
      baseURL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.api.interceptors.response.use(
      (response) => response,
      (error) => {
        console.error('API Error:', error);
        return Promise.reject(error);
      }
    );
  }

  // Search endpoints
  async search(request: SearchRequest): Promise<SearchResult> {
    const response = await this.api.post<SearchResult>('/api/v1/search', request);
    return response.data;
  }

  async getSuggestions(query: string): Promise<string[]> {
    const response = await this.api.get<string[]>('/api/v1/search/suggest', {
      params: { query },
    });
    return response.data;
  }

  async getSearchHistory(): Promise<any[]> {
    const response = await this.api.get<any[]>('/api/v1/search/history');
    return response.data;
  }

  // File endpoints
  async getFiles(limit = 100, offset = 0): Promise<FileInfo[]> {
    const response = await this.api.get<FileInfo[]>('/api/v1/files', {
      params: { limit, offset },
    });
    return response.data;
  }

  async getFile(id: string): Promise<FileInfo> {
    const response = await this.api.get<FileInfo>(`/api/v1/files/${id}`);
    return response.data;
  }

  async getFileContent(id: string): Promise<string> {
    const response = await this.api.get<string>(`/api/v1/files/${id}/content`);
    return response.data;
  }

  async reindexFile(id: string): Promise<void> {
    await this.api.post(`/api/v1/files/${id}/reindex`);
  }

  async deleteFile(id: string): Promise<void> {
    await this.api.delete(`/api/v1/files/${id}`);
  }

  // Indexing endpoints
  async startIndexing(path?: string): Promise<void> {
    await this.api.post('/api/v1/indexing/start', { path });
  }

  async stopIndexing(): Promise<void> {
    await this.api.post('/api/v1/indexing/stop');
  }

  async pauseIndexing(): Promise<void> {
    await this.api.post('/api/v1/indexing/pause');
  }

  async resumeIndexing(): Promise<void> {
    await this.api.post('/api/v1/indexing/resume');
  }

  async getIndexingStatus(): Promise<any> {
    const response = await this.api.get('/api/v1/indexing/status');
    return response.data;
  }

  async getIndexingStats(): Promise<IndexingStats> {
    const response = await this.api.get<IndexingStats>('/api/v1/indexing/stats');
    return response.data;
  }

  async scanDirectory(path: string): Promise<void> {
    await this.api.post('/api/v1/indexing/scan', { path });
  }

  // System endpoints
  async getSystemStatus(): Promise<SystemStatus> {
    const response = await this.api.get<SystemStatus>('/api/v1/status');
    return response.data;
  }

  async getHealth(): Promise<any> {
    const response = await this.api.get('/api/v1/health');
    return response.data;
  }

  async getMetrics(): Promise<any> {
    const response = await this.api.get('/api/v1/metrics');
    return response.data;
  }

  async getConfig(): Promise<Config> {
    const response = await this.api.get<Config>('/api/v1/config');
    return response.data;
  }

  async updateConfig(config: Partial<Config>): Promise<void> {
    await this.api.put('/api/v1/config', config);
  }

  async resetDatabase(): Promise<void> {
    await this.api.post('/api/v1/database/reset');
  }
}

export default ApiService;