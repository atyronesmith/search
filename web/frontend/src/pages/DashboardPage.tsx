import React, { useEffect, useState } from 'react';
import {
  Box,
  Grid,
  Paper,
  Typography,
  Button,
  LinearProgress,
  Card,
  CardContent,
  IconButton,
  Chip,
  Alert,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Pause as PauseIcon,
  Stop as StopIcon,
  Refresh as RefreshIcon,
  Storage as StorageIcon,
  Speed as SpeedIcon,
  Error as ErrorIcon,
  CheckCircle as SuccessIcon,
  Memory as MemoryIcon,
  Folder as FolderIcon,
} from '@mui/icons-material';
import { Line, Doughnut, Bar } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';
import { useApi } from '../contexts/ApiContext';
import { useWebSocket } from '../contexts/WebSocketContext';
import { IndexingStats, SystemStatus } from '../services/ApiService';
import { format } from 'date-fns';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
);

const DashboardPage: React.FC = () => {
  const [stats, setStats] = useState<IndexingStats | null>(null);
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [metrics, setMetrics] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const { api } = useApi();
  const { indexingStatus } = useWebSocket();

  useEffect(() => {
    loadDashboardData();
    const interval = setInterval(loadDashboardData, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadDashboardData = async () => {
    try {
      const [statsData, statusData, metricsData] = await Promise.all([
        api.getIndexingStats(),
        api.getSystemStatus(),
        api.getMetrics(),
      ]);
      
      setStats(statsData);
      setStatus(statusData);
      setMetrics(metricsData);
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  };

  const handleIndexingAction = async (action: 'start' | 'stop' | 'pause' | 'resume') => {
    try {
      switch (action) {
        case 'start':
          await api.startIndexing();
          break;
        case 'stop':
          await api.stopIndexing();
          break;
        case 'pause':
          await api.pauseIndexing();
          break;
        case 'resume':
          await api.resumeIndexing();
          break;
      }
      loadDashboardData();
    } catch (err: any) {
      setError(err.message || `Failed to ${action} indexing`);
    }
  };

  const formatBytes = (bytes: number): string => {
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let size = bytes;
    let unitIndex = 0;
    
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }
    
    return `${size.toFixed(2)} ${units[unitIndex]}`;
  };

  const getIndexingStateColor = (state: string) => {
    switch (state) {
      case 'running':
        return 'success';
      case 'paused':
        return 'warning';
      case 'stopped':
        return 'error';
      default:
        return 'default';
    }
  };

  const resourceChartData = {
    labels: ['CPU', 'Memory', 'Disk'],
    datasets: [{
      label: 'Resource Usage %',
      data: [
        status?.resources.cpu || 0,
        status?.resources.memory || 0,
        status?.resources.disk || 0,
      ],
      backgroundColor: [
        'rgba(255, 99, 132, 0.5)',
        'rgba(54, 162, 235, 0.5)',
        'rgba(255, 206, 86, 0.5)',
      ],
      borderColor: [
        'rgba(255, 99, 132, 1)',
        'rgba(54, 162, 235, 1)',
        'rgba(255, 206, 86, 1)',
      ],
      borderWidth: 1,
    }],
  };

  const fileTypeChartData = {
    labels: metrics?.fileTypes?.map((ft: any) => ft.type) || [],
    datasets: [{
      data: metrics?.fileTypes?.map((ft: any) => ft.count) || [],
      backgroundColor: [
        'rgba(255, 99, 132, 0.5)',
        'rgba(54, 162, 235, 0.5)',
        'rgba(255, 206, 86, 0.5)',
        'rgba(75, 192, 192, 0.5)',
        'rgba(153, 102, 255, 0.5)',
        'rgba(255, 159, 64, 0.5)',
      ],
    }],
  };

  const indexingRateChartData = {
    labels: metrics?.indexingHistory?.map((h: any) => 
      format(new Date(h.timestamp), 'HH:mm')
    ) || [],
    datasets: [{
      label: 'Files/min',
      data: metrics?.indexingHistory?.map((h: any) => h.rate) || [],
      fill: true,
      backgroundColor: 'rgba(75, 192, 192, 0.2)',
      borderColor: 'rgba(75, 192, 192, 1)',
      tension: 0.4,
    }],
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
        <Typography>Loading dashboard...</Typography>
      </Box>
    );
  }

  return (
    <Box>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Status Cards */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Total Files
                  </Typography>
                  <Typography variant="h4">
                    {stats?.totalFiles.toLocaleString() || 0}
                  </Typography>
                </Box>
                <FolderIcon sx={{ fontSize: 40, opacity: 0.3 }} />
              </Box>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Indexed Files
                  </Typography>
                  <Typography variant="h4">
                    {stats?.indexedFiles.toLocaleString() || 0}
                  </Typography>
                  <LinearProgress
                    variant="determinate"
                    value={(stats?.indexedFiles || 0) / (stats?.totalFiles || 1) * 100}
                    sx={{ mt: 1 }}
                  />
                </Box>
                <SuccessIcon sx={{ fontSize: 40, opacity: 0.3, color: 'success.main' }} />
              </Box>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Total Size
                  </Typography>
                  <Typography variant="h4">
                    {formatBytes(stats?.totalSize || 0)}
                  </Typography>
                </Box>
                <StorageIcon sx={{ fontSize: 40, opacity: 0.3 }} />
              </Box>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Box>
                  <Typography color="textSecondary" gutterBottom>
                    Indexing Rate
                  </Typography>
                  <Typography variant="h4">
                    {stats?.indexingRate || 0}/min
                  </Typography>
                </Box>
                <SpeedIcon sx={{ fontSize: 40, opacity: 0.3 }} />
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Indexing Control */}
      <Paper sx={{ p: 2, mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <Typography variant="h6">Indexing Control</Typography>
            <Chip
              label={status?.indexing.state || 'Unknown'}
              color={getIndexingStateColor(status?.indexing.state || '')}
              size="small"
            />
          </Box>
          
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button
              variant="contained"
              color="success"
              startIcon={<PlayIcon />}
              onClick={() => handleIndexingAction('start')}
              disabled={status?.indexing.active}
            >
              Start
            </Button>
            <Button
              variant="contained"
              color="warning"
              startIcon={<PauseIcon />}
              onClick={() => handleIndexingAction('pause')}
              disabled={!status?.indexing.active || status?.indexing.state === 'paused'}
            >
              Pause
            </Button>
            <Button
              variant="contained"
              color="info"
              startIcon={<PlayIcon />}
              onClick={() => handleIndexingAction('resume')}
              disabled={status?.indexing.state !== 'paused'}
            >
              Resume
            </Button>
            <Button
              variant="contained"
              color="error"
              startIcon={<StopIcon />}
              onClick={() => handleIndexingAction('stop')}
              disabled={!status?.indexing.active}
            >
              Stop
            </Button>
            <IconButton onClick={loadDashboardData}>
              <RefreshIcon />
            </IconButton>
          </Box>
        </Box>

        {indexingStatus && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2" color="text.secondary">
              Processing: {indexingStatus.currentFile}
            </Typography>
            <LinearProgress
              variant="determinate"
              value={(indexingStatus.filesProcessed / indexingStatus.totalFiles) * 100}
              sx={{ mt: 1 }}
            />
            <Typography variant="caption" color="text.secondary">
              {indexingStatus.filesProcessed} / {indexingStatus.totalFiles} files processed
              {indexingStatus.errors > 0 && ` (${indexingStatus.errors} errors)`}
            </Typography>
          </Box>
        )}
      </Paper>

      {/* Charts */}
      <Grid container spacing={3}>
        <Grid item xs={12} md={4}>
          <Paper sx={{ p: 2, height: 300 }}>
            <Typography variant="h6" gutterBottom>
              Resource Usage
            </Typography>
            <Box sx={{ height: 230 }}>
              <Bar data={resourceChartData} options={{ maintainAspectRatio: false }} />
            </Box>
          </Paper>
        </Grid>

        <Grid item xs={12} md={4}>
          <Paper sx={{ p: 2, height: 300 }}>
            <Typography variant="h6" gutterBottom>
              File Types
            </Typography>
            <Box sx={{ height: 230 }}>
              <Doughnut data={fileTypeChartData} options={{ maintainAspectRatio: false }} />
            </Box>
          </Paper>
        </Grid>

        <Grid item xs={12} md={4}>
          <Paper sx={{ p: 2, height: 300 }}>
            <Typography variant="h6" gutterBottom>
              Indexing Rate
            </Typography>
            <Box sx={{ height: 230 }}>
              <Line data={indexingRateChartData} options={{ maintainAspectRatio: false }} />
            </Box>
          </Paper>
        </Grid>
      </Grid>

      {/* System Status */}
      <Paper sx={{ p: 2, mt: 3 }}>
        <Typography variant="h6" gutterBottom>
          System Status
        </Typography>
        <Grid container spacing={2}>
          <Grid item xs={12} md={6}>
            <TableContainer>
              <Table size="small">
                <TableBody>
                  <TableRow>
                    <TableCell>Status</TableCell>
                    <TableCell>
                      <Chip label={status?.status || 'Unknown'} color="success" size="small" />
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Uptime</TableCell>
                    <TableCell>{Math.floor((status?.uptime || 0) / 3600)}h {Math.floor(((status?.uptime || 0) % 3600) / 60)}m</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Version</TableCell>
                    <TableCell>{status?.version || 'Unknown'}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Last Indexed</TableCell>
                    <TableCell>
                      {stats?.lastIndexed ? format(new Date(stats.lastIndexed), 'PPpp') : 'Never'}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          </Grid>
          
          <Grid item xs={12} md={6}>
            <TableContainer>
              <Table size="small">
                <TableBody>
                  <TableRow>
                    <TableCell>Database</TableCell>
                    <TableCell>
                      <Chip
                        label={status?.database.connected ? 'Connected' : 'Disconnected'}
                        color={status?.database.connected ? 'success' : 'error'}
                        size="small"
                      />
                      {status?.database.connected && ` (${status.database.latency}ms)`}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Embeddings</TableCell>
                    <TableCell>
                      <Chip
                        label={status?.embeddings.available ? 'Available' : 'Unavailable'}
                        color={status?.embeddings.available ? 'success' : 'error'}
                        size="small"
                      />
                      {status?.embeddings.model && ` (${status.embeddings.model})`}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Total Chunks</TableCell>
                    <TableCell>{stats?.totalChunks.toLocaleString() || 0}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Errors</TableCell>
                    <TableCell>
                      {stats?.errors || 0}
                      {(stats?.errors || 0) > 0 && <ErrorIcon sx={{ ml: 1, fontSize: 16, color: 'error.main' }} />}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          </Grid>
        </Grid>
      </Paper>
    </Box>
  );
};

export default DashboardPage;