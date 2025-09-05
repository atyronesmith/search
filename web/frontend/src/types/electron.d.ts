export {};

declare global {
  interface Window {
    electronAPI: {
      getAppPath: () => Promise<string>;
      getPlatform: () => Promise<string>;
      minimizeWindow: () => Promise<void>;
      maximizeWindow: () => Promise<void>;
      closeWindow: () => Promise<void>;
      onNewSearch: (callback: () => void) => void;
      onIndexDirectory: (callback: (path: string) => void) => void;
      onOpenSettings: (callback: () => void) => void;
      onFocusSearch: (callback: () => void) => void;
      onNavigate: (callback: (path: string) => void) => void;
      removeAllListeners: (channel: string) => void;
    };
    showDirectoryPicker?: () => Promise<string>;
  }
}