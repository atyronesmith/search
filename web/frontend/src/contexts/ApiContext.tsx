import React, { createContext, useContext, ReactNode } from 'react';
import ApiService from '../services/ApiService';

interface ApiContextType {
  api: ApiService;
}

const ApiContext = createContext<ApiContextType | undefined>(undefined);

export const useApi = () => {
  const context = useContext(ApiContext);
  if (!context) {
    throw new Error('useApi must be used within an ApiProvider');
  }
  return context;
};

interface ApiProviderProps {
  children: ReactNode;
}

export const ApiProvider: React.FC<ApiProviderProps> = ({ children }) => {
  const api = new ApiService(process.env.REACT_APP_API_URL || 'http://localhost:8080');

  return (
    <ApiContext.Provider value={{ api }}>
      {children}
    </ApiContext.Provider>
  );
};