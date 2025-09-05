# File Search System Frontend

Modern Electron + React frontend for the File Search System.

## Features

- **Search Interface**: Advanced search with real-time results, suggestions, and filters
- **Results Display**: Rich result display with highlighting, previews, and file operations
- **Indexing Dashboard**: Live metrics, resource monitoring, and indexing control
- **Settings Panel**: Comprehensive configuration management
- **File Manager**: Browse, manage, and reindex files with multiple view modes
- **Real-time Updates**: WebSocket integration for live status updates

## Technology Stack

- **Electron**: Cross-platform desktop application framework
- **React 18**: Modern React with hooks and functional components
- **Lucide React**: Beautiful, customizable icons
- **Axios**: HTTP client for API communication
- **WebSocket**: Real-time communication with backend

## Quick Start

### Prerequisites

- Node.js 16+ and npm
- Backend Go service running on `localhost:8080`

### Installation

```bash
# Install dependencies
npm install

# Start development server (React only)
npm start

# Start Electron in development mode
npm run electron-dev

# Build for production
npm run build

# Build Electron app
npm run build-electron
```

### Development

The frontend connects to the backend API at `http://localhost:8080/api/v1` and WebSocket at `ws://localhost:8080/api/v1/ws`.

## Project Structure

```
src/
├── components/           # React components
│   ├── SearchInterface.js    # Main search page
│   ├── SearchResults.js     # Search results display
│   ├── IndexingDashboard.js # Indexing status and metrics
│   ├── SettingsPanel.js     # Configuration management
│   ├── FileManager.js       # File browsing and management
│   └── StatusBar.js         # Bottom status bar
├── services/            # API and service layers
│   └── ApiService.js        # Backend API client
├── hooks/              # Custom React hooks
│   └── useWebSocket.js      # WebSocket hook
├── utils/              # Utility functions
├── App.js              # Main application component
├── index.js            # React entry point
└── index.css           # Global styles
public/
├── electron.js         # Electron main process
├── preload.js          # Electron preload script
└── index.html          # HTML template
```

## Key Components

### SearchInterface
- Advanced search with query suggestions
- Real-time search results
- Configurable filters (file type, date, size, path)
- Search history management

### IndexingDashboard
- Real-time indexing status and progress
- System resource monitoring (CPU, memory)
- Indexing control (start, stop, pause, resume)
- Live metrics and statistics

### SettingsPanel
- Configuration management with tabbed interface
- Indexing paths and ignore patterns
- Performance tuning settings
- Database configuration

### FileManager
- File browsing with list and grid views
- File operations (preview, open, reindex)
- Bulk operations on selected files
- Sorting and filtering capabilities

## API Integration

The frontend communicates with the Go backend through:

- **REST API**: Search, file management, configuration
- **WebSocket**: Real-time updates for indexing status and metrics

See `src/services/ApiService.js` for complete API documentation.

## Building for Production

```bash
# Build React app
npm run build

# Package Electron app for current platform
npm run dist

# The packaged app will be in the dist/ directory
```

## Keyboard Shortcuts

- `Cmd/Ctrl + F`: Focus search input
- `Cmd/Ctrl + I`: Start indexing
- `Cmd/Ctrl + Shift + I`: Stop indexing
- `Cmd/Ctrl + ,`: Open settings
- `Cmd/Ctrl + O`: Open file manager

## Troubleshooting

### Backend Connection Issues
- Ensure the Go backend is running on `localhost:8080`
- Check that CORS is properly configured
- Verify WebSocket endpoint is accessible

### Development Issues
- Clear browser cache and reload
- Check browser console for JavaScript errors
- Ensure all dependencies are installed with `npm install`

### Build Issues
- Clear `node_modules` and reinstall: `rm -rf node_modules && npm install`
- Clear build cache: `rm -rf build && npm run build`