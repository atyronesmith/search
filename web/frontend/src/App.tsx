import React, { useEffect, useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';
import Layout from './components/Layout';
import SearchPage from './pages/SearchPage';
import DashboardPage from './pages/DashboardPage';
import FilesPage from './pages/FilesPage';
import SettingsPage from './pages/SettingsPage';
import { ApiProvider } from './contexts/ApiContext';
import { WebSocketProvider } from './contexts/WebSocketContext';
import './App.css';

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: '#90caf9',
    },
    secondary: {
      main: '#f48fb1',
    },
    background: {
      default: '#121212',
      paper: '#1e1e1e',
    },
  },
  typography: {
    fontFamily: [
      '-apple-system',
      'BlinkMacSystemFont',
      '"Segoe UI"',
      'Roboto',
      '"Helvetica Neue"',
      'Arial',
      'sans-serif',
    ].join(','),
  },
});

const lightTheme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#1976d2',
    },
    secondary: {
      main: '#dc004e',
    },
  },
  typography: {
    fontFamily: [
      '-apple-system',
      'BlinkMacSystemFont',
      '"Segoe UI"',
      'Roboto',
      '"Helvetica Neue"',
      'Arial',
      'sans-serif',
    ].join(','),
  },
});

function App() {
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem('darkMode');
    return saved ? JSON.parse(saved) : true;
  });

  useEffect(() => {
    localStorage.setItem('darkMode', JSON.stringify(darkMode));
  }, [darkMode]);

  const theme = darkMode ? darkTheme : lightTheme;

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ApiProvider>
        <WebSocketProvider>
          <Router>
            <Layout darkMode={darkMode} setDarkMode={setDarkMode}>
              <Routes>
                <Route path="/" element={<Navigate to="/search" replace />} />
                <Route path="/search" element={<SearchPage />} />
                <Route path="/dashboard" element={<DashboardPage />} />
                <Route path="/files" element={<FilesPage />} />
                <Route path="/settings" element={<SettingsPage />} />
              </Routes>
            </Layout>
          </Router>
        </WebSocketProvider>
      </ApiProvider>
    </ThemeProvider>
  );
}

export default App;