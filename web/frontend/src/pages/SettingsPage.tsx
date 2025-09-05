import React, { useState, useEffect } from 'react';
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  Switch,
  FormControlLabel,
  Divider,
  Alert,
  Grid,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  IconButton,
  Chip,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Slider,
  InputAdornment,
} from '@mui/material';
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Save as SaveIcon,
  Refresh as RefreshIcon,
  FolderOpen as FolderIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import { useApi } from '../contexts/ApiContext';
import { Config } from '../services/ApiService';

const SettingsPage: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [newPath, setNewPath] = useState('');
  const [newExclude, setNewExclude] = useState('');
  const [resetting, setResetting] = useState(false);
  
  const { api } = useApi();

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const configData = await api.getConfig();
      setConfig(configData);
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load configuration');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!config) return;
    
    try {
      setSaving(true);
      await api.updateConfig(config);
      setSuccess('Configuration saved successfully');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err: any) {
      setError(err.message || 'Failed to save configuration');
    } finally {
      setSaving(false);
    }
  };

  const handleAddPath = () => {
    if (!config || !newPath.trim()) return;
    
    setConfig({
      ...config,
      indexPaths: [...config.indexPaths, newPath.trim()],
    });
    setNewPath('');
  };

  const handleRemovePath = (index: number) => {
    if (!config) return;
    
    setConfig({
      ...config,
      indexPaths: config.indexPaths.filter((_, i) => i !== index),
    });
  };

  const handleAddExclude = () => {
    if (!config || !newExclude.trim()) return;
    
    setConfig({
      ...config,
      excludePatterns: [...config.excludePatterns, newExclude.trim()],
    });
    setNewExclude('');
  };

  const handleRemoveExclude = (index: number) => {
    if (!config) return;
    
    setConfig({
      ...config,
      excludePatterns: config.excludePatterns.filter((_, i) => i !== index),
    });
  };

  const handleBrowseFolder = async () => {
    if (window.electronAPI && window.showDirectoryPicker) {
      const result = await window.showDirectoryPicker();
      if (result) {
        setNewPath(result);
      }
    }
  };

  const handleResetDatabase = async () => {
    if (!window.confirm('Are you sure you want to reset the database? This will permanently delete all indexed files and search data. This action cannot be undone.')) {
      return;
    }
    
    try {
      setResetting(true);
      await api.resetDatabase();
      setSuccess('Database reset successfully. All indexed data has been cleared.');
      setTimeout(() => setSuccess(null), 5000);
    } catch (err: any) {
      setError(err.message || 'Failed to reset database');
    } finally {
      setResetting(false);
    }
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
        <Typography>Loading settings...</Typography>
      </Box>
    );
  }

  if (!config) {
    return (
      <Alert severity="error">Failed to load configuration</Alert>
    );
  }

  return (
    <Box>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Index Paths
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Directories to include in the search index
        </Typography>
        
        <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
          <TextField
            fullWidth
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            placeholder="/path/to/directory"
            onKeyPress={(e) => e.key === 'Enter' && handleAddPath()}
          />
          <IconButton onClick={handleBrowseFolder}>
            <FolderIcon />
          </IconButton>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={handleAddPath}
            disabled={!newPath.trim()}
          >
            Add
          </Button>
        </Box>
        
        <List>
          {config.indexPaths.map((path, index) => (
            <ListItem key={index}>
              <ListItemText
                primary={path}
                primaryTypographyProps={{ fontFamily: 'monospace' }}
              />
              <ListItemSecondaryAction>
                <IconButton edge="end" onClick={() => handleRemovePath(index)}>
                  <DeleteIcon />
                </IconButton>
              </ListItemSecondaryAction>
            </ListItem>
          ))}
        </List>
      </Paper>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Exclude Patterns
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          File patterns to exclude from indexing (supports wildcards)
        </Typography>
        
        <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
          <TextField
            fullWidth
            value={newExclude}
            onChange={(e) => setNewExclude(e.target.value)}
            placeholder="*.tmp, node_modules/*, .git/*"
            onKeyPress={(e) => e.key === 'Enter' && handleAddExclude()}
          />
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={handleAddExclude}
            disabled={!newExclude.trim()}
          >
            Add
          </Button>
        </Box>
        
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {config.excludePatterns.map((pattern, index) => (
            <Chip
              key={index}
              label={pattern}
              onDelete={() => handleRemoveExclude(index)}
              variant="outlined"
            />
          ))}
        </Box>
      </Paper>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Chunking Settings
        </Typography>
        
        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Typography gutterBottom>
              Chunk Size: {config.chunkSize} characters
            </Typography>
            <Slider
              value={config.chunkSize}
              onChange={(e, value) => setConfig({ ...config, chunkSize: value as number })}
              min={100}
              max={2000}
              step={100}
              marks={[
                { value: 100, label: '100' },
                { value: 500, label: '500' },
                { value: 1000, label: '1000' },
                { value: 1500, label: '1500' },
                { value: 2000, label: '2000' },
              ]}
            />
          </Grid>
          
          <Grid item xs={12} md={6}>
            <Typography gutterBottom>
              Chunk Overlap: {config.chunkOverlap} characters
            </Typography>
            <Slider
              value={config.chunkOverlap}
              onChange={(e, value) => setConfig({ ...config, chunkOverlap: value as number })}
              min={0}
              max={500}
              step={50}
              marks={[
                { value: 0, label: '0' },
                { value: 100, label: '100' },
                { value: 200, label: '200' },
                { value: 300, label: '300' },
                { value: 400, label: '400' },
                { value: 500, label: '500' },
              ]}
            />
          </Grid>
        </Grid>
      </Paper>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Embedding Settings
        </Typography>
        
        <FormControl fullWidth sx={{ mb: 2 }}>
          <InputLabel>Embedding Model</InputLabel>
          <Select
            value={config.embeddingModel}
            onChange={(e) => setConfig({ ...config, embeddingModel: e.target.value })}
            label="Embedding Model"
          >
            <MenuItem value="nomic-embed-text">nomic-embed-text</MenuItem>
            <MenuItem value="mxbai-embed-large">mxbai-embed-large</MenuItem>
            <MenuItem value="all-minilm">all-minilm</MenuItem>
            <MenuItem value="bge-small">bge-small</MenuItem>
          </Select>
        </FormControl>
      </Paper>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Resource Limits
        </Typography>
        
        <Grid container spacing={3}>
          <Grid item xs={12} md={4}>
            <TextField
              fullWidth
              type="number"
              label="Max File Size (MB)"
              value={config.maxFileSize / (1024 * 1024)}
              onChange={(e) => setConfig({
                ...config,
                maxFileSize: parseInt(e.target.value) * 1024 * 1024,
              })}
              InputProps={{
                endAdornment: <InputAdornment position="end">MB</InputAdornment>,
              }}
            />
          </Grid>
          
          <Grid item xs={12} md={4}>
            <TextField
              fullWidth
              type="number"
              label="Max CPU Usage (%)"
              value={config.resourceLimits.maxCpu}
              onChange={(e) => setConfig({
                ...config,
                resourceLimits: {
                  ...config.resourceLimits,
                  maxCpu: parseInt(e.target.value),
                },
              })}
              InputProps={{
                endAdornment: <InputAdornment position="end">%</InputAdornment>,
              }}
            />
          </Grid>
          
          <Grid item xs={12} md={4}>
            <TextField
              fullWidth
              type="number"
              label="Max Memory Usage (%)"
              value={config.resourceLimits.maxMemory}
              onChange={(e) => setConfig({
                ...config,
                resourceLimits: {
                  ...config.resourceLimits,
                  maxMemory: parseInt(e.target.value),
                },
              })}
              InputProps={{
                endAdornment: <InputAdornment position="end">%</InputAdornment>,
              }}
            />
          </Grid>
        </Grid>
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Scanning Settings
        </Typography>
        
        <TextField
          fullWidth
          type="number"
          label="Scan Interval (seconds)"
          value={config.scanInterval}
          onChange={(e) => setConfig({ ...config, scanInterval: parseInt(e.target.value) })}
          InputProps={{
            endAdornment: <InputAdornment position="end">seconds</InputAdornment>,
          }}
          helperText="How often to check for file changes (0 to disable)"
        />
      </Paper>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Database Management
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          Manage the search database and indexed content
        </Typography>
        
        <Alert severity="warning" sx={{ mb: 2 }}>
          <strong>Warning:</strong> Resetting the database will permanently delete all indexed files, 
          search data, and metadata. This action cannot be undone.
        </Alert>
        
        <Button
          variant="outlined"
          color="error"
          startIcon={<WarningIcon />}
          onClick={handleResetDatabase}
          disabled={resetting}
          sx={{ minWidth: 160 }}
        >
          {resetting ? 'Resetting...' : 'Reset Database'}
        </Button>
      </Paper>

      <Box sx={{ mt: 3, display: 'flex', gap: 2, justifyContent: 'flex-end' }}>
        <Button
          variant="outlined"
          startIcon={<RefreshIcon />}
          onClick={loadConfig}
        >
          Reset
        </Button>
        <Button
          variant="contained"
          startIcon={<SaveIcon />}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? 'Saving...' : 'Save Changes'}
        </Button>
      </Box>
    </Box>
  );
};

export default SettingsPage;