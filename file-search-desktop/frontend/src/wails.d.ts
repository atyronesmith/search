interface DebugInfo {
  timestamp: string
  query: string
  model: string
  prompt: string
  response: string
  process_time_ms: number
  error?: string
  vector_query?: string
  text_query?: string
}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          Search: (request: any) => Promise<any[]>
          SearchWithDetails: (request: any) => Promise<any>
          IsLLMQuery: (query: string) => Promise<boolean>
          StartIndexing: (path: string) => Promise<void>
          StopIndexing: () => Promise<void>
          PauseIndexing: () => Promise<void>
          ResumeIndexing: () => Promise<void>
          ReindexFailed: () => Promise<void>
          GetIndexingStatus: () => Promise<any>
          GetSystemStatus: () => Promise<any>
          GetConfig: () => Promise<string>
          UpdateConfig: (config: string) => Promise<void>
          GetFiles: (limit: number, offset: number) => Promise<any[]>
          GetFilesSorted: (limit: number, offset: number, sortBy: string, sortDir: string) => Promise<any[]>
          GetRootDirectories: () => Promise<any>
          GetDirectoryContents: (path: string) => Promise<any>
          ResetDatabase: () => Promise<void>
          CallAPI: (method: string, endpoint: string, body: string) => Promise<string>
          GetCurrentLLMModel: () => Promise<string>
          GetLLMDebugInfo: () => Promise<DebugInfo | null>
          GetPrompt: () => Promise<string>
          UpdatePrompt: (prompt: string) => Promise<string>
        }
      }
    }
  }
}

export {}