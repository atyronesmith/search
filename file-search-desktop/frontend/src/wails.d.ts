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
          GetIndexingStatus: () => Promise<any>
          GetSystemStatus: () => Promise<any>
          GetConfig: () => Promise<string>
          UpdateConfig: (config: string) => Promise<void>
          GetFiles: (limit: number, offset: number) => Promise<any[]>
          ResetDatabase: () => Promise<void>
          CallAPI: (method: string, endpoint: string, body: string) => Promise<string>
        }
      }
    }
  }
}

export {}