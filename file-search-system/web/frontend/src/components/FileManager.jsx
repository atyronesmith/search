import React, { useState, useEffect } from 'react';
import { 
  RefreshCcw, Filter, Grid, List, 
  File, Folder, Calendar, HardDrive,
  Search, MoreHorizontal, ExternalLink,
  RotateCcw, Trash2, Eye, ArrowUpDown
} from 'lucide-react';
import { ApiService } from '../services/ApiService';

const FileManager = () => {
  const [files, setFiles] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [viewMode, setViewMode] = useState('list'); // 'list' or 'grid'
  const [sortBy, setSortBy] = useState('modified_at');
  const [sortOrder, setSortOrder] = useState('desc');
  const [filterType, setFilterType] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 50,
    total: 0
  });
  const [selectedFiles, setSelectedFiles] = useState(new Set());

  useEffect(() => {
    loadFiles();
  }, [pagination.page, sortBy, sortOrder, filterType]);

  const loadFiles = async () => {
    setIsLoading(true);
    try {
      const options = {
        page: pagination.page,
        limit: pagination.limit,
        sort_by: sortBy,
        sort_order: sortOrder,
        ...(filterType && { file_type: filterType }),
        ...(searchQuery && { search: searchQuery })
      };

      const response = await ApiService.getFiles(options);
      setFiles(response.files || []);
      setPagination(prev => ({
        ...prev,
        total: response.total || 0
      }));
    } catch (error) {
      console.error('Failed to load files:', error);
      setFiles([]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSort = (column) => {
    if (sortBy === column) {
      setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(column);
      setSortOrder('desc');
    }
  };

  const handleFileSelect = (fileId) => {
    setSelectedFiles(prev => {
      const newSet = new Set(prev);
      if (newSet.has(fileId)) {
        newSet.delete(fileId);
      } else {
        newSet.add(fileId);
      }
      return newSet;
    });
  };

  const handleSelectAll = () => {
    if (selectedFiles.size === files.length) {
      setSelectedFiles(new Set());
    } else {
      setSelectedFiles(new Set(files.map(f => f.id)));
    }
  };

  const handleReindexSelected = async () => {
    const fileIds = Array.from(selectedFiles);
    if (fileIds.length === 0) return;

    try {
      await Promise.all(fileIds.map(id => ApiService.reindexFile(id)));
      setSelectedFiles(new Set());
      await loadFiles();
    } catch (error) {
      console.error('Failed to reindex files:', error);
    }
  };

  const handleOpenFile = (file) => {
    if (window.electronAPI) {
      console.log('Open file:', file.path);
    } else {
      window.open(`/api/v1/files/${file.id}/content`, '_blank');
    }
  };

  const handlePreviewFile = async (file) => {
    try {
      const content = await ApiService.getFileContent(file.id);
      // Show preview in modal or new window
      console.log('Preview content:', content);
    } catch (error) {
      console.error('Failed to preview file:', error);
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'indexed': return 'text-green-600 bg-green-50';
      case 'pending': return 'text-yellow-600 bg-yellow-50';
      case 'error': return 'text-red-600 bg-red-50';
      case 'processing': return 'text-blue-600 bg-blue-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  const fileTypes = [
    { value: '', label: 'All Types' },
    { value: 'pdf', label: 'PDF' },
    { value: 'doc,docx', label: 'Documents' },
    { value: 'txt,md', label: 'Text Files' },
    { value: 'js,ts,jsx,tsx', label: 'JavaScript/TypeScript' },
    { value: 'py', label: 'Python' },
    { value: 'java', label: 'Java' },
    { value: 'go', label: 'Go' },
    { value: 'json,yaml,yml', label: 'Config Files' },
    { value: 'csv', label: 'CSV' },
    { value: 'jpg,jpeg,png,gif', label: 'Images' }
  ];

  const totalPages = Math.ceil(pagination.total / pagination.limit);

  return (
    <div className="max-w-7xl mx-auto">
      {/* Header */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">File Manager</h1>
            <p className="text-gray-600">
              {pagination.total.toLocaleString()} files indexed
            </p>
          </div>

          <div className="flex items-center gap-3">
            {/* Search */}
            <div className="relative">
              <Search className="h-4 w-4 absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && loadFiles()}
                placeholder="Search files..."
                className="pl-9 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
              />
            </div>

            {/* Filter */}
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
            >
              {fileTypes.map(type => (
                <option key={type.value} value={type.value}>
                  {type.label}
                </option>
              ))}
            </select>

            {/* View Mode */}
            <div className="flex border border-gray-300 rounded-md">
              <button
                onClick={() => setViewMode('list')}
                className={`p-2 ${viewMode === 'list' ? 'bg-blue-500 text-white' : 'text-gray-600 hover:bg-gray-100'}`}
              >
                <List className="h-4 w-4" />
              </button>
              <button
                onClick={() => setViewMode('grid')}
                className={`p-2 ${viewMode === 'grid' ? 'bg-blue-500 text-white' : 'text-gray-600 hover:bg-gray-100'}`}
              >
                <Grid className="h-4 w-4" />
              </button>
            </div>

            {/* Refresh */}
            <button
              onClick={loadFiles}
              className="p-2 text-gray-600 hover:bg-gray-100 rounded-md"
              title="Refresh"
            >
              <RefreshCcw className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* Selected Files Actions */}
        {selectedFiles.size > 0 && (
          <div className="mt-4 p-3 bg-blue-50 border border-blue-200 rounded-md">
            <div className="flex items-center justify-between">
              <span className="text-sm text-blue-700">
                {selectedFiles.size} file{selectedFiles.size > 1 ? 's' : ''} selected
              </span>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleReindexSelected}
                  className="flex items-center gap-1 px-3 py-1 bg-blue-500 text-white rounded-md hover:bg-blue-600 text-sm"
                >
                  <RotateCcw className="h-3 w-3" />
                  Reindex
                </button>
                <button
                  onClick={() => setSelectedFiles(new Set())}
                  className="flex items-center gap-1 px-3 py-1 text-gray-600 hover:bg-gray-100 rounded-md text-sm"
                >
                  Clear
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* File List */}
      {isLoading ? (
        <div className="bg-white rounded-lg shadow-md p-8">
          <div className="flex items-center justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
            <span className="ml-3 text-gray-600">Loading files...</span>
          </div>
        </div>
      ) : files.length === 0 ? (
        <div className="bg-white rounded-lg shadow-md p-8 text-center">
          <File className="h-12 w-12 text-gray-400 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No files found</h3>
          <p className="text-gray-600">
            {searchQuery || filterType 
              ? 'Try adjusting your search or filter criteria'
              : 'No files have been indexed yet'
            }
          </p>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow-md">
          {viewMode === 'list' ? (
            /* List View */
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-gray-50 border-b border-gray-200">
                  <tr>
                    <th className="w-8 px-4 py-3">
                      <input
                        type="checkbox"
                        checked={selectedFiles.size === files.length && files.length > 0}
                        onChange={handleSelectAll}
                        className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                    </th>
                    <th 
                      className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                      onClick={() => handleSort('filename')}
                    >
                      <div className="flex items-center gap-1">
                        Name
                        <ArrowUpDown className="h-3 w-3" />
                      </div>
                    </th>
                    <th 
                      className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                      onClick={() => handleSort('size')}
                    >
                      <div className="flex items-center gap-1">
                        Size
                        <ArrowUpDown className="h-3 w-3" />
                      </div>
                    </th>
                    <th 
                      className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                      onClick={() => handleSort('modified_at')}
                    >
                      <div className="flex items-center gap-1">
                        Modified
                        <ArrowUpDown className="h-3 w-3" />
                      </div>
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {files.map((file) => (
                    <tr 
                      key={file.id} 
                      className={`hover:bg-gray-50 ${selectedFiles.has(file.id) ? 'bg-blue-50' : ''}`}
                    >
                      <td className="px-4 py-3">
                        <input
                          type="checkbox"
                          checked={selectedFiles.has(file.id)}
                          onChange={() => handleFileSelect(file.id)}
                          className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        />
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3">
                          <span className="text-xl">
                            {ApiService.getFileTypeIcon(file.filename)}
                          </span>
                          <div className="min-w-0 flex-1">
                            <div className="text-sm font-medium text-gray-900 truncate">
                              {file.filename}
                            </div>
                            <div className="text-xs text-gray-500 truncate">
                              {file.path}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-600">
                        {ApiService.formatFileSize(file.size || 0)}
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-600">
                        {ApiService.formatDate(file.modified_at)}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex px-2 py-1 text-xs font-medium rounded-md ${
                          getStatusColor(file.index_status)
                        }`}>
                          {file.index_status || 'unknown'}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            onClick={() => handlePreviewFile(file)}
                            className="p-1 text-gray-400 hover:text-gray-600 rounded"
                            title="Preview"
                          >
                            <Eye className="h-4 w-4" />
                          </button>
                          <button
                            onClick={() => handleOpenFile(file)}
                            className="p-1 text-gray-400 hover:text-gray-600 rounded"
                            title="Open"
                          >
                            <ExternalLink className="h-4 w-4" />
                          </button>
                          <button
                            onClick={() => ApiService.reindexFile(file.id)}
                            className="p-1 text-gray-400 hover:text-gray-600 rounded"
                            title="Reindex"
                          >
                            <RotateCcw className="h-4 w-4" />
                          </button>
                          <button className="p-1 text-gray-400 hover:text-gray-600 rounded">
                            <MoreHorizontal className="h-4 w-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            /* Grid View */
            <div className="p-6">
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
                {files.map((file) => (
                  <div 
                    key={file.id}
                    className={`p-4 border border-gray-200 rounded-lg hover:bg-gray-50 cursor-pointer ${
                      selectedFiles.has(file.id) ? 'border-blue-500 bg-blue-50' : ''
                    }`}
                    onClick={() => handleFileSelect(file.id)}
                  >
                    <div className="text-center">
                      <div className="text-3xl mb-2">
                        {ApiService.getFileTypeIcon(file.filename)}
                      </div>
                      <div className="text-sm font-medium text-gray-900 truncate mb-1">
                        {file.filename}
                      </div>
                      <div className="text-xs text-gray-500">
                        {ApiService.formatFileSize(file.size || 0)}
                      </div>
                      <div className={`inline-flex px-1 py-0.5 text-xs rounded-md mt-2 ${
                        getStatusColor(file.index_status)
                      }`}>
                        {file.index_status || 'unknown'}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="border-t border-gray-200 px-6 py-3">
              <div className="flex items-center justify-between">
                <div className="text-sm text-gray-700">
                  Showing {((pagination.page - 1) * pagination.limit) + 1} to{' '}
                  {Math.min(pagination.page * pagination.limit, pagination.total)} of{' '}
                  {pagination.total} files
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setPagination(prev => ({ ...prev, page: prev.page - 1 }))}
                    disabled={pagination.page === 1}
                    className="px-3 py-1 text-sm border border-gray-300 rounded-md disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
                  >
                    Previous
                  </button>
                  <span className="px-3 py-1 text-sm">
                    Page {pagination.page} of {totalPages}
                  </span>
                  <button
                    onClick={() => setPagination(prev => ({ ...prev, page: prev.page + 1 }))}
                    disabled={pagination.page === totalPages}
                    className="px-3 py-1 text-sm border border-gray-300 rounded-md disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
                  >
                    Next
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default FileManager;