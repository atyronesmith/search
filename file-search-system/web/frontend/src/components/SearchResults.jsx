import React, { useState } from 'react';
import { File, Folder, ExternalLink, Eye, MoreHorizontal, Calendar, HardDrive } from 'lucide-react';
import { ApiService } from '../services/ApiService';

const SearchResults = ({ results, isLoading, query, searchStats }) => {
  const [selectedResult, setSelectedResult] = useState(null);
  const [previewContent, setPreviewContent] = useState('');
  const [isLoadingPreview, setIsLoadingPreview] = useState(false);

  const handlePreview = async (result) => {
    if (selectedResult?.id === result.id) {
      setSelectedResult(null);
      setPreviewContent('');
      return;
    }

    setSelectedResult(result);
    setIsLoadingPreview(true);
    setPreviewContent('');

    try {
      const content = await ApiService.getFileContent(result.id);
      setPreviewContent(content.content || 'No preview available');
    } catch (error) {
      console.error('Failed to load preview:', error);
      setPreviewContent('Error loading preview');
    } finally {
      setIsLoadingPreview(false);
    }
  };

  const handleOpenFile = (result) => {
    // In Electron, we could open the file with the default application
    if (window.electronAPI) {
      // This would need to be implemented in the Electron main process
      console.log('Open file:', result.path);
    } else {
      // In web version, we might download or show in new tab
      window.open(`/api/v1/files/${result.id}/content`, '_blank');
    }
  };

  const highlightText = (text, query) => {
    if (!query || !text) return text;
    
    const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
    const parts = text.split(regex);
    
    return parts.map((part, index) => 
      regex.test(part) ? (
        <mark key={index} className="bg-yellow-200 px-1 rounded">
          {part}
        </mark>
      ) : part
    );
  };

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-8">
        <div className="flex items-center justify-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
          <span className="ml-3 text-gray-600">Searching files...</span>
        </div>
      </div>
    );
  }

  if (!results || results.length === 0) {
    if (query) {
      return (
        <div className="bg-white rounded-lg shadow-md p-8 text-center">
          <File className="h-12 w-12 text-gray-400 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No results found</h3>
          <p className="text-gray-600">
            Try adjusting your search terms or removing filters
          </p>
        </div>
      );
    }
    return null;
  }

  return (
    <div className="space-y-4">
      {/* Results List */}
      <div className="bg-white rounded-lg shadow-md">
        {results.map((result, index) => (
          <div key={result.id} className={`${index !== 0 ? 'border-t border-gray-200' : ''}`}>
            <div className="p-4 hover:bg-gray-50 transition-colors">
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  {/* File Header */}
                  <div className="flex items-center gap-3 mb-2">
                    <span className="text-2xl">
                      {ApiService.getFileTypeIcon(result.filename)}
                    </span>
                    <div className="flex-1 min-w-0">
                      <h3 className="text-lg font-medium text-gray-900 truncate">
                        {highlightText(result.filename, query)}
                      </h3>
                      <p className="text-sm text-gray-500 truncate" title={result.path}>
                        {result.path}
                      </p>
                    </div>
                  </div>

                  {/* File Metadata */}
                  <div className="flex items-center gap-4 text-xs text-gray-500 mb-3">
                    <span className="flex items-center gap-1">
                      <HardDrive className="h-3 w-3" />
                      {ApiService.formatFileSize(result.size || 0)}
                    </span>
                    <span className="flex items-center gap-1">
                      <Calendar className="h-3 w-3" />
                      {ApiService.formatDate(result.modified_at)}
                    </span>
                    {result.file_type && (
                      <span className="px-2 py-1 bg-gray-100 rounded-md">
                        {result.file_type.toUpperCase()}
                      </span>
                    )}
                    {result.score && (
                      <span className="px-2 py-1 bg-blue-100 text-blue-800 rounded-md">
                        {Math.round(result.score * 100)}% match
                      </span>
                    )}
                  </div>

                  {/* Content Preview */}
                  {result.snippet && (
                    <div className="bg-gray-50 rounded-md p-3 mb-3">
                      <p className="text-sm text-gray-700 leading-relaxed">
                        {highlightText(result.snippet, query)}
                      </p>
                    </div>
                  )}

                  {/* Highlights */}
                  {result.highlights && result.highlights.length > 0 && (
                    <div className="space-y-1 mb-3">
                      {result.highlights.slice(0, 3).map((highlight, idx) => (
                        <div key={idx} className="text-sm text-gray-600">
                          <span className="text-gray-400">...{' '}</span>
                          {highlightText(highlight, query)}
                          <span className="text-gray-400">{' '}...</span>
                        </div>
                      ))}
                      {result.highlights.length > 3 && (
                        <div className="text-xs text-gray-400">
                          +{result.highlights.length - 3} more matches
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Action Buttons */}
                <div className="flex items-center gap-2 ml-4">
                  <button
                    onClick={() => handlePreview(result)}
                    className={`p-2 rounded-md transition-colors ${
                      selectedResult?.id === result.id
                        ? 'bg-blue-100 text-blue-600'
                        : 'text-gray-400 hover:text-gray-600 hover:bg-gray-100'
                    }`}
                    title="Preview file"
                  >
                    <Eye className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => handleOpenFile(result)}
                    className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition-colors"
                    title="Open file"
                  >
                    <ExternalLink className="h-4 w-4" />
                  </button>
                  <button className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition-colors">
                    <MoreHorizontal className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* File Preview */}
              {selectedResult?.id === result.id && (
                <div className="mt-4 border-t border-gray-200 pt-4">
                  <div className="bg-gray-900 rounded-lg overflow-hidden">
                    <div className="flex items-center justify-between px-4 py-2 bg-gray-800">
                      <span className="text-sm text-gray-300">Preview</span>
                      <button
                        onClick={() => setSelectedResult(null)}
                        className="text-gray-400 hover:text-white"
                      >
                        ×
                      </button>
                    </div>
                    <div className="p-4 max-h-96 overflow-y-auto">
                      {isLoadingPreview ? (
                        <div className="flex items-center justify-center py-8">
                          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-400"></div>
                          <span className="ml-3 text-gray-400">Loading preview...</span>
                        </div>
                      ) : (
                        <pre className="text-sm text-gray-300 whitespace-pre-wrap font-mono">
                          {previewContent}
                        </pre>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Load More Button */}
      {searchStats && results.length < searchStats.total && (
        <div className="text-center py-4">
          <button className="px-6 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors">
            Load More Results ({results.length} of {searchStats.total})
          </button>
        </div>
      )}
    </div>
  );
};

export default SearchResults;