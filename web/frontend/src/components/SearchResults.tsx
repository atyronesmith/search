import React, { useState } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Chip,
  IconButton,
  Tooltip,
  Grid,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Paper,
  Collapse,
  Button,
} from '@mui/material';
import {
  InsertDriveFile as FileIcon,
  Code as CodeIcon,
  Description as DocIcon,
  Image as ImageIcon,
  Folder as FolderIcon,
  OpenInNew as OpenIcon,
  ContentCopy as CopyIcon,
  ExpandMore as ExpandIcon,
  ExpandLess as CollapseIcon,
} from '@mui/icons-material';
import { FileResult } from '../services/ApiService';
import { format } from 'date-fns';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface SearchResultsProps {
  results: FileResult[];
  viewMode: 'list' | 'grid';
}

const SearchResults: React.FC<SearchResultsProps> = ({ results, viewMode }) => {
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set());

  const toggleExpanded = (id: string) => {
    const newExpanded = new Set(expandedItems);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    setExpandedItems(newExpanded);
  };

  const getFileIcon = (type: string) => {
    if (type.includes('code') || type.includes('javascript') || type.includes('python')) {
      return <CodeIcon />;
    }
    if (type.includes('document') || type.includes('pdf') || type.includes('word')) {
      return <DocIcon />;
    }
    if (type.includes('image')) {
      return <ImageIcon />;
    }
    if (type.includes('folder') || type.includes('directory')) {
      return <FolderIcon />;
    }
    return <FileIcon />;
  };

  const getLanguageFromType = (type: string): string => {
    const typeMap: { [key: string]: string } = {
      'javascript': 'javascript',
      'typescript': 'typescript',
      'python': 'python',
      'java': 'java',
      'cpp': 'cpp',
      'go': 'go',
      'rust': 'rust',
      'html': 'html',
      'css': 'css',
      'json': 'json',
      'yaml': 'yaml',
      'markdown': 'markdown',
    };
    
    for (const [key, value] of Object.entries(typeMap)) {
      if (type.toLowerCase().includes(key)) {
        return value;
      }
    }
    return 'text';
  };

  const formatFileSize = (bytes: number): string => {
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let unitIndex = 0;
    
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }
    
    return `${size.toFixed(1)} ${units[unitIndex]}`;
  };

  const handleOpenFile = (path: string) => {
    if (window.electronAPI) {
      // Open file in system default application
      window.open(`file://${path}`);
    }
  };

  const handleCopyPath = (path: string) => {
    navigator.clipboard.writeText(path);
  };

  const renderHighlight = (highlight: string, type: string) => {
    const isCode = type.includes('code') || 
                   type.includes('javascript') || 
                   type.includes('python') ||
                   type.includes('java') ||
                   type.includes('cpp') ||
                   type.includes('go');

    if (isCode) {
      return (
        <SyntaxHighlighter
          language={getLanguageFromType(type)}
          style={vscDarkPlus}
          customStyle={{
            margin: 0,
            padding: '8px',
            fontSize: '12px',
            borderRadius: '4px',
          }}
        >
          {highlight}
        </SyntaxHighlighter>
      );
    }

    return (
      <Paper
        variant="outlined"
        sx={{
          p: 1,
          bgcolor: 'background.default',
          fontFamily: 'monospace',
          fontSize: '12px',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        {highlight}
      </Paper>
    );
  };

  if (viewMode === 'grid') {
    return (
      <Grid container spacing={2}>
        {results.map((result) => (
          <Grid item xs={12} sm={6} md={4} key={result.id}>
            <Card
              sx={{
                height: '100%',
                display: 'flex',
                flexDirection: 'column',
                '&:hover': {
                  boxShadow: 3,
                },
              }}
            >
              <CardContent sx={{ flex: 1 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  {getFileIcon(result.type)}
                  <Typography
                    variant="subtitle1"
                    component="div"
                    sx={{ ml: 1, fontWeight: 500 }}
                    noWrap
                  >
                    {result.name}
                  </Typography>
                </Box>
                
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{
                    display: 'block',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    mb: 1,
                  }}
                >
                  {result.path}
                </Typography>

                <Box sx={{ display: 'flex', gap: 0.5, mb: 1 }}>
                  <Chip
                    label={`Score: ${result.score.toFixed(2)}`}
                    size="small"
                    color="primary"
                    variant="outlined"
                  />
                  <Chip
                    label={formatFileSize(result.size)}
                    size="small"
                    variant="outlined"
                  />
                </Box>

                <Typography variant="body2" color="text.secondary">
                  {result.snippet}
                </Typography>
              </CardContent>
              
              <Box sx={{ p: 1, display: 'flex', justifyContent: 'flex-end' }}>
                <Tooltip title="Open file">
                  <IconButton size="small" onClick={() => handleOpenFile(result.path)}>
                    <OpenIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Copy path">
                  <IconButton size="small" onClick={() => handleCopyPath(result.path)}>
                    <CopyIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </Box>
            </Card>
          </Grid>
        ))}
      </Grid>
    );
  }

  return (
    <List>
      {results.map((result, index) => (
        <React.Fragment key={result.id}>
          {index > 0 && <Box sx={{ my: 1 }} />}
          <Paper variant="outlined" sx={{ mb: 1 }}>
            <ListItem>
              <ListItemIcon>{getFileIcon(result.type)}</ListItemIcon>
              <ListItemText
                primary={
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography variant="subtitle1" component="span">
                      {result.name}
                    </Typography>
                    <Chip
                      label={`Score: ${result.score.toFixed(2)}`}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                    <Chip label={result.type} size="small" variant="outlined" />
                    <Chip label={formatFileSize(result.size)} size="small" variant="outlined" />
                  </Box>
                }
                secondary={
                  <Box>
                    <Typography
                      variant="caption"
                      component="div"
                      color="text.secondary"
                      sx={{ fontFamily: 'monospace' }}
                    >
                      {result.path}
                    </Typography>
                    <Typography variant="body2" component="div" sx={{ mt: 1 }}>
                      {result.snippet}
                    </Typography>
                    {result.highlights.length > 0 && (
                      <Button
                        size="small"
                        onClick={() => toggleExpanded(result.id)}
                        startIcon={expandedItems.has(result.id) ? <CollapseIcon /> : <ExpandIcon />}
                        sx={{ mt: 1 }}
                      >
                        {expandedItems.has(result.id) ? 'Hide' : 'Show'} highlights ({result.highlights.length})
                      </Button>
                    )}
                  </Box>
                }
              />
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Tooltip title="Open file">
                  <IconButton size="small" onClick={() => handleOpenFile(result.path)}>
                    <OpenIcon />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Copy path">
                  <IconButton size="small" onClick={() => handleCopyPath(result.path)}>
                    <CopyIcon />
                  </IconButton>
                </Tooltip>
              </Box>
            </ListItem>
            
            <Collapse in={expandedItems.has(result.id)}>
              <Box sx={{ px: 2, pb: 2 }}>
                {result.highlights.map((highlight, idx) => (
                  <Box key={idx} sx={{ mb: 1 }}>
                    {renderHighlight(highlight, result.type)}
                  </Box>
                ))}
              </Box>
            </Collapse>
          </Paper>
        </React.Fragment>
      ))}
    </List>
  );
};

export default SearchResults;