import React, { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import io, { Socket } from 'socket.io-client';

interface WebSocketContextType {
  socket: Socket | null;
  connected: boolean;
  indexingStatus: IndexingStatus | null;
  searchProgress: SearchProgress | null;
}

interface IndexingStatus {
  state: string;
  filesProcessed: number;
  totalFiles: number;
  currentFile: string;
  errors: number;
  startTime: string;
  elapsedTime: number;
}

interface SearchProgress {
  queryId: string;
  stage: string;
  progress: number;
  message: string;
}

const WebSocketContext = createContext<WebSocketContextType | undefined>(undefined);

export const useWebSocket = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error('useWebSocket must be used within a WebSocketProvider');
  }
  return context;
};

interface WebSocketProviderProps {
  children: ReactNode;
}

export const WebSocketProvider: React.FC<WebSocketProviderProps> = ({ children }) => {
  const [socket, setSocket] = useState<Socket | null>(null);
  const [connected, setConnected] = useState(false);
  const [indexingStatus, setIndexingStatus] = useState<IndexingStatus | null>(null);
  const [searchProgress, setSearchProgress] = useState<SearchProgress | null>(null);

  useEffect(() => {
    const wsUrl = process.env.REACT_APP_WS_URL || 'ws://localhost:8080';
    const newSocket = io(wsUrl, {
      path: '/api/v1/ws',
      transports: ['websocket'],
      reconnectionAttempts: 5,
      reconnectionDelay: 1000,
    });

    newSocket.on('connect', () => {
      console.log('WebSocket connected');
      setConnected(true);
    });

    newSocket.on('disconnect', () => {
      console.log('WebSocket disconnected');
      setConnected(false);
    });

    newSocket.on('indexing_status', (data: IndexingStatus) => {
      setIndexingStatus(data);
    });

    newSocket.on('search_progress', (data: SearchProgress) => {
      setSearchProgress(data);
    });

    newSocket.on('error', (error: any) => {
      console.error('WebSocket error:', error);
    });

    setSocket(newSocket);

    return () => {
      newSocket.close();
    };
  }, []);

  return (
    <WebSocketContext.Provider value={{ socket, connected, indexingStatus, searchProgress }}>
      {children}
    </WebSocketContext.Provider>
  );
};