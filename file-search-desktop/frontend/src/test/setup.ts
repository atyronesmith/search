import '@testing-library/jest-dom'

// Mock window.go for Wails
global.window = global.window || {}
;(global.window as any).go = {
  main: {
    App: {
      Search: vi.fn(),
      StartIndexing: vi.fn(),
      StopIndexing: vi.fn(),
      PauseIndexing: vi.fn(),
      ResumeIndexing: vi.fn(),
      GetIndexingStatus: vi.fn(),
      GetSystemStatus: vi.fn(),
      GetConfig: vi.fn(),
      UpdateConfig: vi.fn(),
      GetFiles: vi.fn(),
    }
  }
}