import React, { useState, useEffect, useRef } from 'react';
import { Search, Filter, Clock, X } from 'lucide-react';
import { ApiService } from '../services/ApiService';
import SearchResults from './SearchResults';

const SearchInterface = () => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [searchHistory, setSearchHistory] = useState([]);
  const [suggestions, setSuggestions] = useState([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [filters, setFilters] = useState({
    fileType: '',
    dateRange: '',
    sizeRange: '',
    path: ''
  });
  const [showFilters, setShowFilters] = useState(false);
  const [searchStats, setSearchStats] = useState(null);
  
  const searchInputRef = useRef(null);
  const suggestionsTimeoutRef = useRef(null);

  useEffect(() => {
    loadSearchHistory();
  }, []);

  useEffect(() => {
    // Auto-focus search input
    if (searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, []);

  const loadSearchHistory = async () => {
    try {
      const history = await ApiService.getSearchHistory();
      setSearchHistory(history.slice(0, 10)); // Show last 10 searches
    } catch (error) {
      console.error('Failed to load search history:', error);
    }
  };

  const handleSearch = async (searchQuery = query) => {
    if (!searchQuery.trim()) return;

    setIsLoading(true);
    setShowSuggestions(false);

    try {
      const searchOptions = {
        limit: 50,
        ...Object.fromEntries(
          Object.entries(filters).filter(([_, value]) => value)
        )
      };

      const startTime = Date.now();
      const response = await ApiService.search(searchQuery, searchOptions);
      const endTime = Date.now();

      setResults(response.results || []);
      setSearchStats({
        total: response.total || 0,
        duration: endTime - startTime,
        query: searchQuery
      });

      // Add to search history
      await loadSearchHistory();
    } catch (error) {
      console.error('Search failed:', error);
      setResults([]);
      setSearchStats(null);
    } finally {
      setIsLoading(false);
    }
  };

  const handleQueryChange = (e) => {
    const newQuery = e.target.value;
    setQuery(newQuery);

    // Clear previous timeout
    if (suggestionsTimeoutRef.current) {
      clearTimeout(suggestionsTimeoutRef.current);
    }

    // Get suggestions after a delay
    if (newQuery.length > 2) {
      suggestionsTimeoutRef.current = setTimeout(async () => {
        try {
          const response = await ApiService.searchSuggestions(newQuery);
          setSuggestions(response.suggestions || []);
          setShowSuggestions(true);
        } catch (error) {
          console.error('Failed to get suggestions:', error);
          setSuggestions([]);
        }
      }, 300);
    } else {
      setSuggestions([]);
      setShowSuggestions(false);
    }
  };

  const handleSuggestionClick = (suggestion) => {
    setQuery(suggestion);
    setShowSuggestions(false);
    handleSearch(suggestion);
  };

  const handleHistoryClick = (historyItem) => {
    setQuery(historyItem.query);
    handleSearch(historyItem.query);
  };

  const clearFilters = () => {
    setFilters({
      fileType: '',
      dateRange: '',
      sizeRange: '',
      path: ''
    });
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter') {
      handleSearch();
    } else if (e.key === 'Escape') {
      setShowSuggestions(false);
    }
  };

  return (
    <div className="max-w-6xl mx-auto">
      {/* Search Header */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <div className="relative">
          {/* Search Input */}
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
              <Search className="h-5 w-5 text-gray-400" />
            </div>
            <input
              ref={searchInputRef}
              type="text"
              value={query}
              onChange={handleQueryChange}
              onKeyPress={handleKeyPress}
              onFocus={() => query.length > 2 && setShowSuggestions(true)}
              placeholder="Search files... (e.g., 'react components', 'config.json', 'type:pdf')"
              className="block w-full pl-10 pr-12 py-3 text-lg border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
            <div className="absolute inset-y-0 right-0 flex items-center gap-2 pr-3">
              <button
                onClick={() => setShowFilters(!showFilters)}
                className={`p-1 rounded-md transition-colors ${
                  showFilters ? 'bg-blue-100 text-blue-600' : 'text-gray-400 hover:text-gray-600'
                }`}
                title="Advanced filters"
              >
                <Filter className="h-5 w-5" />
              </button>
              <button
                onClick={() => handleSearch()}
                disabled={isLoading || !query.trim()}
                className="px-4 py-1 bg-blue-500 text-white rounded-md hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {isLoading ? 'Searching...' : 'Search'}
              </button>
            </div>
          </div>

          {/* Suggestions Dropdown */}
          {showSuggestions && suggestions.length > 0 && (
            <div className="absolute z-10 w-full mt-1 bg-white border border-gray-200 rounded-lg shadow-lg max-h-60 overflow-y-auto">
              {suggestions.map((suggestion, index) => (
                <div
                  key={index}
                  onClick={() => handleSuggestionClick(suggestion)}
                  className="px-4 py-2 hover:bg-gray-100 cursor-pointer flex items-center gap-2"
                >
                  <Search className="h-4 w-4 text-gray-400" />
                  <span>{suggestion}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Advanced Filters */}
        {showFilters && (
          <div className="mt-4 p-4 bg-gray-50 rounded-lg">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  File Type
                </label>
                <select
                  value={filters.fileType}
                  onChange={(e) => setFilters(prev => ({ ...prev, fileType: e.target.value }))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="">All types</option>
                  <option value="pdf">PDF</option>
                  <option value="doc,docx">Documents</option>
                  <option value="txt,md">Text files</option>
                  <option value="js,ts,jsx,tsx">JavaScript/TypeScript</option>
                  <option value="py">Python</option>
                  <option value="java">Java</option>
                  <option value="go">Go</option>
                  <option value="json,yaml,yml">Config files</option>
                </select>
              </div>
              
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Date Range
                </label>
                <select
                  value={filters.dateRange}
                  onChange={(e) => setFilters(prev => ({ ...prev, dateRange: e.target.value }))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="">Any time</option>
                  <option value="today">Today</option>
                  <option value="week">This week</option>
                  <option value="month">This month</option>
                  <option value="year">This year</option>
                </select>
              </div>
              
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  File Size
                </label>
                <select
                  value={filters.sizeRange}
                  onChange={(e) => setFilters(prev => ({ ...prev, sizeRange: e.target.value }))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="">Any size</option>
                  <option value="small">Small (&lt; 1MB)</option>
                  <option value="medium">Medium (1-10MB)</option>
                  <option value="large">Large (&gt; 10MB)</option>
                </select>
              </div>
              
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Path Contains
                </label>
                <input
                  type="text"
                  value={filters.path}
                  onChange={(e) => setFilters(prev => ({ ...prev, path: e.target.value }))}
                  placeholder="e.g., /src/, Documents"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
            </div>
            
            <div className="mt-3 flex justify-between items-center">
              <button
                onClick={clearFilters}
                className="text-sm text-gray-500 hover:text-gray-700 flex items-center gap-1"
              >
                <X className="h-4 w-4" />
                Clear all filters
              </button>
              <button
                onClick={() => handleSearch()}
                className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
              >
                Apply Filters
              </button>
            </div>
          </div>
        )}

        {/* Search Stats */}
        {searchStats && (
          <div className="mt-4 text-sm text-gray-600">
            Found {searchStats.total.toLocaleString()} results for "{searchStats.query}" 
            in {searchStats.duration}ms
          </div>
        )}
      </div>

      {/* Search History */}
      {!query && searchHistory.length > 0 && (
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4 flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Recent Searches
          </h3>
          <div className="flex flex-wrap gap-2">
            {searchHistory.map((item, index) => (
              <button
                key={index}
                onClick={() => handleHistoryClick(item)}
                className="px-3 py-1 bg-gray-100 text-gray-700 rounded-full text-sm hover:bg-gray-200 transition-colors"
              >
                {item.query}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Search Results */}
      <SearchResults 
        results={results}
        isLoading={isLoading}
        query={query}
        searchStats={searchStats}
      />
    </div>
  );
};

export default SearchInterface;