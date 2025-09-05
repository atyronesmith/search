# File Search Desktop Application

A native cross-platform desktop application built with Wails and React that provides a beautiful interface for the File Search System.

## Overview

This desktop application serves as the frontend client for the File Search System backend API. It provides:

- **Native Performance**: Built with Wails for true native desktop experience
- **Cross-Platform**: Runs natively on macOS, Windows, and Linux
- **Service Integration**: Connects to the backend API via HTTP client
- **Modern UI**: React frontend with Material-UI components
- **Offline Capability**: Graceful fallback when backend is unavailable

## Architecture

```
┌─────────────────────┐    HTTP API    ┌─────────────────────┐
│                     │                │                     │
│  Desktop App        │◄──────────────►│  Backend Service    │
│  (Wails + React)    │                │  (Go + PostgreSQL)  │
│                     │                │                     │
└─────────────────────┘                └─────────────────────┘
```

### Components

1. **app.go**: Main Wails application backend
2. **api_client.go**: HTTP client for backend API communication
3. **frontend/**: React application with UI components
4. **build/**: Built application artifacts

## Quick Start

### Prerequisites

- **Go 1.21+**: For Wails backend
- **Node.js 16+**: For React frontend
- **Wails v2**: Desktop application framework

```bash
# Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Development

```bash
# Start in development mode (with hot reload)
wails dev

# Development mode opens:
# - Backend API client ready to connect to localhost:8080
# - React dev server with hot reload
# - Native app window with developer tools
```

### Production Build

```bash
# Build for current platform
wails build

# Build with debug info
wails build -debug

# Cross-platform builds
wails build -platform darwin/amd64     # Intel macOS
wails build -platform darwin/arm64     # Apple Silicon macOS
wails build -platform windows/amd64    # Windows x64
wails build -platform linux/amd64      # Linux x64
```

### Running the App

```bash
# After building
open build/bin/file-search-desktop.app  # macOS
./build/bin/file-search-desktop.exe     # Windows
./build/bin/file-search-desktop          # Linux
```

## Configuration

### Backend Connection

The app connects to the backend API at `http://localhost:8080` by default. This is configured in `app.go`:

```go
func NewApp() *App {
    return &App{
        apiClient: NewAPIClient("http://localhost:8080"),
    }
}
```

### API Endpoints Used

- `POST /api/v1/search` - Search files
- `GET /api/v1/indexing/status` - Get indexing status
- `GET /api/v1/status` - Get system status
- `POST /api/v1/indexing/start` - Start indexing
- `POST /api/v1/indexing/stop` - Stop indexing
- `GET /api/v1/config` - Get configuration
- `PUT /api/v1/config` - Update configuration
- `GET /api/v1/files` - List files

## Features

### Search Interface
- **Smart Search**: Natural language and keyword search
- **Filters**: File type, date range, size, and path filters
- **Results**: Rich results with syntax highlighting and previews
- **History**: Search history and suggestions

### Dashboard
- **System Status**: Real-time backend connection status
- **Indexing Control**: Start, stop, pause, and resume indexing
- **Metrics**: System performance and indexing statistics
- **Resource Usage**: CPU, memory, and disk usage monitoring

### File Management
- **File Browser**: Browse and manage indexed files
- **File Details**: View file metadata and content previews
- **Actions**: Quick actions like open, reindex, and delete
- **Bulk Operations**: Batch operations on multiple files

### Settings
- **Index Paths**: Configure directories to index
- **Exclude Patterns**: Set file and directory exclusion patterns
- **Performance**: Adjust CPU/memory thresholds and rate limits
- **Search**: Configure search weights and caching
- **Backend**: Backend service connection settings

## Development

### Project Structure

```
file-search-desktop/
├── app.go                      # Main Wails application
├── api_client.go               # HTTP client for backend API
├── wails.json                  # Wails configuration
├── frontend/
│   ├── src/
│   │   ├── components/         # Reusable UI components
│   │   ├── pages/             # Main application pages
│   │   ├── contexts/          # React contexts for state
│   │   ├── types/             # TypeScript type definitions
│   │   └── App.tsx            # Main React application
│   ├── package.json           # Node.js dependencies
│   └── tsconfig.json          # TypeScript configuration
├── build/                      # Built application files
└── README.md                   # This file
```

### Adding New Features

1. **Backend Integration**: Add new API calls to `api_client.go`
2. **Frontend Components**: Create React components in `frontend/src/components/`
3. **App Methods**: Expose new functionality through `app.go` methods
4. **Type Definitions**: Add TypeScript types to `frontend/src/types/`

### Error Handling

The app includes robust error handling:

- **API Unavailable**: Falls back to demo data when backend is unreachable
- **Network Errors**: Displays connection status and retry options
- **Invalid Responses**: Graceful handling of malformed API responses
- **User Errors**: Clear error messages and validation feedback

## API Client

The `api_client.go` file provides a complete HTTP client for the backend API:

```go
type APIClient struct {
    baseURL    string
    httpClient *http.Client
}

// Main methods
func (c *APIClient) Search(request SearchRequest) ([]SearchResult, error)
func (c *APIClient) GetSystemStatus() (SystemStatus, error)
func (c *APIClient) StartIndexing(path string) error
func (c *APIClient) GetConfig() (string, error)
func (c *APIClient) UpdateConfig(configJSON string) error
```

## Troubleshooting

### App Won't Start

```bash
# Check Wails installation
wails doctor

# Rebuild with clean dependencies
rm -rf frontend/node_modules
cd frontend && npm install
cd .. && wails build
```

### Backend Connection Issues

```bash
# Check if backend is running
curl http://localhost:8080/api/v1/status

# Start backend service
cd ../file-search-system
go run cmd/server/main.go
```

### Build Failures

```bash
# Clean build cache
wails build -clean

# Check dependencies
cd frontend && npm audit
cd .. && go mod tidy
```

### Performance Issues

The app includes several optimizations:

- **Lazy Loading**: Components load only when needed
- **Debounced Search**: Search requests are debounced to reduce API calls
- **Caching**: Results are cached to improve response times
- **Virtual Scrolling**: Large result sets use virtual scrolling

## Deployment

### Distribution

```bash
# Build production version
wails build

# Package for distribution (macOS)
codesign -s "Developer ID Application" build/bin/file-search-desktop.app
hdiutil create -volname "File Search" -srcfolder build/bin/file-search-desktop.app file-search-desktop.dmg
```

### System Requirements

- **macOS**: 10.13+ (High Sierra or later)
- **Windows**: Windows 10/11 (64-bit)
- **Linux**: Modern distribution with GTK3
- **RAM**: 256MB minimum, 512MB recommended
- **Disk**: 100MB for application, plus storage for index data

## License

MIT License - see [LICENSE](../LICENSE) file for details.

## Related

- [Backend API Documentation](../file-search-system/README.md)
- [Wails Documentation](https://wails.io/docs/)
- [React Documentation](https://react.dev/)
