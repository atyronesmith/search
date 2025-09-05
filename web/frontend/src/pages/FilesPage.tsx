import React, { useState, useEffect } from 'react';
import {
  Box,
  Paper,
  Typography,
  Button,
  TextField,
  InputAdornment,
  IconButton,
  Alert,
  Chip,
  Menu,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Tooltip,
} from '@mui/material';
import { DataGrid, GridColDef, GridRowSelectionModel } from '@mui/x-data-grid';
import {
  Search as SearchIcon,
  Refresh as RefreshIcon,
  Delete as DeleteIcon,
  Sync as ReindexIcon,
  OpenInNew as OpenIcon,
  FilterList as FilterIcon,
  Download as DownloadIcon,
  Info as InfoIcon,
} from '@mui/icons-material';
import { useApi } from '../contexts/ApiContext';
import { FileInfo } from '../services/ApiService';
import { format } from 'date-fns';

const FilesPage: React.FC = () => {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [fileDetailsOpen, setFileDetailsOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(25);
  
  const { api } = useApi();

  useEffect(() => {
    loadFiles();
  }, [page, pageSize]);

  const loadFiles = async () => {
    try {
      setLoading(true);
      const filesData = await api.getFiles(pageSize, page * pageSize);
      setFiles(filesData);
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load files');
    } finally {
      setLoading(false);
    }
  };

  const handleReindex = async (ids: string[]) => {
    try {
      for (const id of ids) {
        await api.reindexFile(id);
      }
      loadFiles();
    } catch (err: any) {
      setError(err.message || 'Failed to reindex files');
    }
  };

  const handleDelete = async (ids: string[]) => {
    if (!window.confirm(`Are you sure you want to delete ${ids.length} file(s) from the index?`)) {
      return;
    }
    
    try {
      for (const id of ids) {
        await api.deleteFile(id);
      }
      setSelectedRows([]);
      loadFiles();
    } catch (err: any) {
      setError(err.message || 'Failed to delete files');
    }
  };

  const handleOpenFile = (path: string) => {
    if (window.electronAPI) {
      window.open(`file://${path}`);
    }
  };

  const handleShowDetails = async (file: FileInfo) => {
    setSelectedFile(file);
    setFileDetailsOpen(true);
  };

  const formatFileSize = (bytes: number): string => {
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let unitIndex = 0;
    
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }
    
    return `${size.toFixed(2)} ${units[unitIndex]}`;
  };

  const getFileTypeColor = (type: string): 'default' | 'primary' | 'secondary' | 'success' | 'warning' => {
    if (type.includes('code') || type.includes('javascript') || type.includes('python')) {
      return 'primary';
    }
    if (type.includes('document') || type.includes('pdf')) {
      return 'secondary';
    }
    if (type.includes('image')) {
      return 'success';
    }
    if (type.includes('archive')) {
      return 'warning';
    }
    return 'default';
  };

  const columns: GridColDef[] = [
    {
      field: 'name',
      headerName: 'Name',
      flex: 2,
      minWidth: 200,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2">{params.value}</Typography>
        </Box>
      ),
    },
    {
      field: 'type',
      headerName: 'Type',
      width: 120,
      renderCell: (params) => (
        <Chip
          label={params.value}
          size="small"
          color={getFileTypeColor(params.value)}
          variant="outlined"
        />
      ),
    },
    {
      field: 'size',
      headerName: 'Size',
      width: 100,
      valueFormatter: (params: any) => formatFileSize(params.value as number),
    },
    {
      field: 'chunks',
      headerName: 'Chunks',
      width: 80,
      align: 'center',
    },
    {
      field: 'modifiedAt',
      headerName: 'Modified',
      width: 180,
      valueFormatter: (params: any) => format(new Date(params.value as string), 'PPp'),
    },
    {
      field: 'lastIndexed',
      headerName: 'Last Indexed',
      width: 180,
      valueFormatter: (params: any) => format(new Date(params.value as string), 'PPp'),
    },
    {
      field: 'actions',
      headerName: 'Actions',
      width: 150,
      sortable: false,
      renderCell: (params) => (
        <Box>
          <Tooltip title="Open file">
            <IconButton
              size="small"
              onClick={() => handleOpenFile(params.row.path)}
            >
              <OpenIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Reindex">
            <IconButton
              size="small"
              onClick={() => handleReindex([params.row.id])}
            >
              <ReindexIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Details">
            <IconButton
              size="small"
              onClick={() => handleShowDetails(params.row)}
            >
              <InfoIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Delete from index">
            <IconButton
              size="small"
              onClick={() => handleDelete([params.row.id])}
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      ),
    },
  ];

  const filteredFiles = files.filter((file) =>
    searchTerm === '' ||
    file.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    file.path.toLowerCase().includes(searchTerm.toLowerCase())
  );

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
          <TextField
            fullWidth
            variant="outlined"
            placeholder="Search files..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon />
                </InputAdornment>
              ),
            }}
          />
          
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={loadFiles}
          >
            Refresh
          </Button>

          {selectedRows.length > 0 && (
            <>
              <Button
                variant="outlined"
                startIcon={<ReindexIcon />}
                onClick={() => handleReindex(selectedRows as string[])}
              >
                Reindex ({selectedRows.length})
              </Button>
              <Button
                variant="outlined"
                color="error"
                startIcon={<DeleteIcon />}
                onClick={() => handleDelete(selectedRows as string[])}
              >
                Delete ({selectedRows.length})
              </Button>
            </>
          )}
        </Box>
      </Paper>

      <Paper sx={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <DataGrid
          rows={filteredFiles}
          columns={columns}
          loading={loading}
          pageSizeOptions={[10, 25, 50, 100]}
          checkboxSelection
          disableRowSelectionOnClick
          onRowSelectionModelChange={setSelectedRows}
          rowSelectionModel={selectedRows}
          paginationModel={{ page, pageSize }}
          onPaginationModelChange={(model) => {
            setPage(model.page);
            setPageSize(model.pageSize);
          }}
          sx={{
            '& .MuiDataGrid-cell': {
              fontSize: '0.875rem',
            },
          }}
        />
      </Paper>

      <Dialog
        open={fileDetailsOpen}
        onClose={() => setFileDetailsOpen(false)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>File Details</DialogTitle>
        <DialogContent>
          {selectedFile && (
            <Box sx={{ pt: 2 }}>
              <Typography variant="subtitle2" gutterBottom>
                Name
              </Typography>
              <Typography variant="body2" sx={{ mb: 2, fontFamily: 'monospace' }}>
                {selectedFile.name}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Path
              </Typography>
              <Typography variant="body2" sx={{ mb: 2, fontFamily: 'monospace' }}>
                {selectedFile.path}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Type
              </Typography>
              <Chip
                label={selectedFile.type}
                size="small"
                color={getFileTypeColor(selectedFile.type)}
                sx={{ mb: 2 }}
              />

              <Typography variant="subtitle2" gutterBottom>
                Size
              </Typography>
              <Typography variant="body2" sx={{ mb: 2 }}>
                {formatFileSize(selectedFile.size)}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Hash
              </Typography>
              <Typography
                variant="body2"
                sx={{ mb: 2, fontFamily: 'monospace', wordBreak: 'break-all' }}
              >
                {selectedFile.hash}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Chunks
              </Typography>
              <Typography variant="body2" sx={{ mb: 2 }}>
                {selectedFile.chunks}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Created
              </Typography>
              <Typography variant="body2" sx={{ mb: 2 }}>
                {format(new Date(selectedFile.createdAt), 'PPpp')}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Modified
              </Typography>
              <Typography variant="body2" sx={{ mb: 2 }}>
                {format(new Date(selectedFile.modifiedAt), 'PPpp')}
              </Typography>

              <Typography variant="subtitle2" gutterBottom>
                Last Indexed
              </Typography>
              <Typography variant="body2">
                {format(new Date(selectedFile.lastIndexed), 'PPpp')}
              </Typography>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setFileDetailsOpen(false)}>Close</Button>
          {selectedFile && (
            <>
              <Button
                startIcon={<OpenIcon />}
                onClick={() => handleOpenFile(selectedFile.path)}
              >
                Open File
              </Button>
              <Button
                startIcon={<ReindexIcon />}
                onClick={() => {
                  handleReindex([selectedFile.id]);
                  setFileDetailsOpen(false);
                }}
              >
                Reindex
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default FilesPage;