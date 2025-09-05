import React, { useState, useCallback, useEffect, useRef } from 'react';
import {
  Box,
  TextField,
  Button,
  Paper,
  Typography,
  Chip,
  CircularProgress,
  InputAdornment,
  IconButton,
  Divider,
  Alert,
  Autocomplete,
  ToggleButton,
  ToggleButtonGroup,
} from '@mui/material';
import {
  Search as SearchIcon,
  Clear as ClearIcon,
  FilterList as FilterIcon,
  ViewList as ListViewIcon,
  ViewModule as GridViewIcon,
} from '@mui/icons-material';
import { useApi } from '../contexts/ApiContext';
import { useWebSocket } from '../contexts/WebSocketContext';
import SearchResults from '../components/SearchResults';
import SearchFilters from '../components/SearchFilters';
import { SearchRequest, SearchResult, SearchFilters as Filters } from '../services/ApiService';

const SearchPage: React.FC = () => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [showFilters, setShowFilters] = useState(false);
  const [filters, setFilters] = useState<Filters>({});
  const [viewMode, setViewMode] = useState<'list' | 'grid'>('list');
  const [searchHistory, setSearchHistory] = useState<string[]>([]);
  
  const { api } = useApi();
  const { searchProgress } = useWebSocket();
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    loadSearchHistory();
    // Focus search input on mount
    searchInputRef.current?.focus();
  }, []);

  useEffect(() => {
    // Listen for Electron IPC events
    if (window.electronAPI) {
      window.electronAPI.onFocusSearch(() => {
        searchInputRef.current?.focus();
      });

      window.electronAPI.onNewSearch(() => {
        handleClear();
        searchInputRef.current?.focus();
      });
    }

    return () => {
      if (window.electronAPI) {
        window.electronAPI.removeAllListeners('focus-search');
        window.electronAPI.removeAllListeners('new-search');
      }
    };
  }, []);

  const loadSearchHistory = async () => {
    try {
      const history = await api.getSearchHistory();
      setSearchHistory(history.map((h: any) => h.query).slice(0, 10));
    } catch (err) {
      console.error('Failed to load search history:', err);
    }
  };

  const handleSearch = async () => {
    if (!query.trim()) return;

    setLoading(true);
    setError(null);
    
    try {
      const searchRequest: SearchRequest = {
        query: query.trim(),
        filters,
        limit: 50,
      };
      
      const searchResult = await api.search(searchRequest);
      setResults(searchResult);
      
      // Add to search history
      if (!searchHistory.includes(query)) {
        setSearchHistory([query, ...searchHistory.slice(0, 9)]);
      }
    } catch (err: any) {
      setError(err.message || 'Search failed');
      setResults(null);
    } finally {
      setLoading(false);
    }
  };

  const handleSuggestions = useCallback(async (input: string) => {
    if (input.length < 2) {
      setSuggestions([]);
      return;
    }

    try {
      const suggestionList = await api.getSuggestions(input);
      setSuggestions(suggestionList);
    } catch (err) {
      console.error('Failed to get suggestions:', err);
      setSuggestions([]);
    }
  }, [api]);

  const handleClear = () => {
    setQuery('');
    setResults(null);
    setError(null);
    setFilters({});
    searchInputRef.current?.focus();
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
          <Autocomplete
            freeSolo
            fullWidth
            value={query}
            onChange={(event, newValue) => {
              setQuery(newValue || '');
            }}
            inputValue={query}
            onInputChange={(event, newInputValue) => {
              setQuery(newInputValue);
              handleSuggestions(newInputValue);
            }}
            options={[...suggestions, ...searchHistory]}
            renderInput={(params) => (
              <TextField
                {...params}
                inputRef={searchInputRef}
                placeholder="Search for files, code, or content..."
                variant="outlined"
                onKeyPress={handleKeyPress}
                InputProps={{
                  ...params.InputProps,
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon />
                    </InputAdornment>
                  ),
                  endAdornment: (
                    <>
                      {loading && <CircularProgress size={20} />}
                      {query && !loading && (
                        <InputAdornment position="end">
                          <IconButton size="small" onClick={handleClear}>
                            <ClearIcon />
                          </IconButton>
                        </InputAdornment>
                      )}
                    </>
                  ),
                }}
              />
            )}
          />
          
          <Button
            variant="contained"
            onClick={handleSearch}
            disabled={!query.trim() || loading}
            sx={{ minWidth: 100 }}
          >
            Search
          </Button>
          
          <IconButton
            onClick={() => setShowFilters(!showFilters)}
            color={showFilters ? 'primary' : 'default'}
          >
            <FilterIcon />
          </IconButton>
          
          <ToggleButtonGroup
            value={viewMode}
            exclusive
            onChange={(e, newMode) => newMode && setViewMode(newMode)}
            size="small"
          >
            <ToggleButton value="list">
              <ListViewIcon />
            </ToggleButton>
            <ToggleButton value="grid">
              <GridViewIcon />
            </ToggleButton>
          </ToggleButtonGroup>
        </Box>

        {showFilters && (
          <>
            <Divider sx={{ my: 2 }} />
            <SearchFilters filters={filters} onChange={setFilters} />
          </>
        )}

        {searchProgress && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="caption" color="text.secondary">
              {searchProgress.message}
            </Typography>
            <CircularProgress
              variant="determinate"
              value={searchProgress.progress}
              size={16}
              sx={{ ml: 1 }}
            />
          </Box>
        )}
      </Paper>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {results && (
        <Paper sx={{ flex: 1, overflow: 'auto', p: 2 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
            <Typography variant="h6">
              Results ({results.totalResults})
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Search completed in {results.processingTime}ms
            </Typography>
          </Box>
          
          <SearchResults results={results.results} viewMode={viewMode} />
        </Paper>
      )}

      {!results && !loading && !error && (
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'text.secondary',
          }}
        >
          <SearchIcon sx={{ fontSize: 64, mb: 2, opacity: 0.5 }} />
          <Typography variant="h6">Start searching</Typography>
          <Typography variant="body2">
            Enter a query to search through your indexed files
          </Typography>
          
          {searchHistory.length > 0 && (
            <Box sx={{ mt: 4 }}>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>
                Recent searches
              </Typography>
              <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                {searchHistory.map((term) => (
                  <Chip
                    key={term}
                    label={term}
                    onClick={() => {
                      setQuery(term);
                      handleSearch();
                    }}
                    variant="outlined"
                    size="small"
                  />
                ))}
              </Box>
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
};

export default SearchPage;