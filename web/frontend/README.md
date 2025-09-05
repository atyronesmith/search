# File Search System - Frontend

Electron + React frontend for the File Search System.

## Features

- **Search Interface**: Advanced search with filters, suggestions, and history
- **Results Display**: Code highlighting, file previews, and relevance scoring
- **Dashboard**: Real-time indexing status, system metrics, and resource monitoring
- **File Management**: Browse, reindex, and manage indexed files
- **Settings**: Configure indexing paths, exclusions, and system parameters
- **Real-time Updates**: WebSocket integration for live status updates

## Tech Stack

- **Electron**: Desktop application framework
- **React 18**: UI framework with TypeScript
- **Material-UI**: Component library
- **Socket.io**: WebSocket client for real-time updates
- **Chart.js**: Data visualization
- **React Router**: Navigation
- **Axios**: HTTP client

## Installation

```bash
cd web/frontend
npm install
```

## Development

Run the React development server and Electron in development mode:

```bash
npm run dev
```

This will:
1. Start the React development server on http://localhost:3000
2. Launch Electron in development mode once React is ready

## Building

### Build React app:
```bash
npm run build
```

### Package Electron app:
```bash
npm run pack
```

This creates distributable packages in the `dist` folder:
- **macOS**: .dmg file
- **Windows**: .exe installer
- **Linux**: AppImage

## Configuration

Edit `.env` file to configure API endpoints:

```env
REACT_APP_API_URL=http://localhost:8080
REACT_APP_WS_URL=ws://localhost:8080
```

## Project Structure

```
src/
├── components/       # Reusable UI components
│   ├── Layout.tsx   # Main app layout with navigation
│   ├── SearchResults.tsx
│   └── SearchFilters.tsx
├── pages/           # Page components
│   ├── SearchPage.tsx
│   ├── DashboardPage.tsx
│   ├── FilesPage.tsx
│   └── SettingsPage.tsx
├── contexts/        # React contexts
│   ├── ApiContext.tsx
│   └── WebSocketContext.tsx
├── services/        # API services
│   └── ApiService.ts
├── types/          # TypeScript definitions
│   └── electron.d.ts
├── App.tsx         # Main app component
└── index.tsx       # Entry point
```

## Keyboard Shortcuts

- `Cmd/Ctrl + N`: New search
- `Cmd/Ctrl + F`: Focus search
- `Cmd/Ctrl + D`: Open dashboard
- `Cmd/Ctrl + Shift + F`: Open files
- `Cmd/Ctrl + ,`: Open settings
- `Cmd/Ctrl + O`: Index directory
- `F11`: Toggle fullscreen
- `F12`: Toggle DevTools

## Features in Detail

### Search Page
- Full-text and semantic search
- Auto-suggestions and search history
- Advanced filters (file type, date, size, path)
- List/grid view toggle
- Code syntax highlighting
- File preview with highlights

### Dashboard
- Real-time indexing status
- System resource monitoring
- File type distribution
- Indexing rate charts
- Start/stop/pause/resume controls
- Error tracking

### Files Page
- DataGrid with sorting and filtering
- Batch operations
- File details dialog
- Quick actions (open, reindex, delete)
- Export functionality

### Settings Page
- Index path management
- Exclude pattern configuration
- Chunking parameters
- Embedding model selection
- Resource limits
- Auto-save functionality

## Scripts

- `npm start`: Start React development server
- `npm run build`: Build React app for production
- `npm test`: Run tests
- `npm run electron`: Run Electron with built app
- `npm run electron-dev`: Run Electron in development mode
- `npm run dev`: Run both React and Electron in development
- `npm run pack`: Package Electron app for distribution

## Requirements

- Node.js 16+
- npm 8+
- Backend API server running on port 8080

## License

MIT