import React from 'react';
import {
  Box,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
  TextField,
  Typography,
  Slider,
  Grid,
  SelectChangeEvent,
  OutlinedInput,
} from '@mui/material';
import { DatePicker } from '@mui/x-date-pickers/DatePicker';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { SearchFilters as Filters } from '../services/ApiService';

interface SearchFiltersProps {
  filters: Filters;
  onChange: (filters: Filters) => void;
}

const fileTypes = [
  'javascript',
  'typescript',
  'python',
  'java',
  'cpp',
  'go',
  'rust',
  'html',
  'css',
  'json',
  'yaml',
  'markdown',
  'pdf',
  'word',
  'excel',
  'text',
  'image',
];

const SearchFilters: React.FC<SearchFiltersProps> = ({ filters, onChange }) => {
  const handleFileTypeChange = (event: SelectChangeEvent<string[]>) => {
    const value = event.target.value;
    onChange({
      ...filters,
      fileTypes: typeof value === 'string' ? value.split(',') : value,
    });
  };

  const handleSizeChange = (event: Event, newValue: number | number[]) => {
    if (Array.isArray(newValue)) {
      onChange({
        ...filters,
        sizeRange: {
          min: newValue[0] * 1024 * 1024, // Convert MB to bytes
          max: newValue[1] * 1024 * 1024,
        },
      });
    }
  };

  const handleDateChange = (field: 'start' | 'end') => (date: Date | null) => {
    if (date) {
      onChange({
        ...filters,
        dateRange: {
          ...filters.dateRange,
          start: field === 'start' ? date.toISOString() : filters.dateRange?.start || '',
          end: field === 'end' ? date.toISOString() : filters.dateRange?.end || '',
        },
      });
    }
  };

  const handlePathChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const paths = event.target.value.split(',').map(p => p.trim()).filter(p => p);
    onChange({
      ...filters,
      paths: paths.length > 0 ? paths : undefined,
    });
  };

  const sizeRangeValue = [
    (filters.sizeRange?.min || 0) / (1024 * 1024),
    (filters.sizeRange?.max || 100 * 1024 * 1024) / (1024 * 1024),
  ];

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Grid container spacing={2}>
        <Grid item xs={12} sm={6} md={3}>
          <FormControl fullWidth size="small">
            <InputLabel>File Types</InputLabel>
            <Select
              multiple
              value={filters.fileTypes || []}
              onChange={handleFileTypeChange}
              input={<OutlinedInput label="File Types" />}
              renderValue={(selected) => (
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  {selected.map((value) => (
                    <Chip key={value} label={value} size="small" />
                  ))}
                </Box>
              )}
            >
              {fileTypes.map((type) => (
                <MenuItem key={type} value={type}>
                  {type}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <TextField
            fullWidth
            size="small"
            label="Paths (comma-separated)"
            placeholder="/path/to/folder, /another/path"
            value={filters.paths?.join(', ') || ''}
            onChange={handlePathChange}
          />
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <DatePicker
            label="Modified After"
            value={filters.dateRange?.start ? new Date(filters.dateRange.start) : null}
            onChange={handleDateChange('start')}
            slotProps={{
              textField: {
                size: 'small',
                fullWidth: true,
              },
            }}
          />
        </Grid>

        <Grid item xs={12} sm={6} md={3}>
          <DatePicker
            label="Modified Before"
            value={filters.dateRange?.end ? new Date(filters.dateRange.end) : null}
            onChange={handleDateChange('end')}
            slotProps={{
              textField: {
                size: 'small',
                fullWidth: true,
              },
            }}
          />
        </Grid>

        <Grid item xs={12}>
          <Typography variant="subtitle2" gutterBottom>
            File Size Range (MB)
          </Typography>
          <Box sx={{ px: 2 }}>
            <Slider
              value={sizeRangeValue}
              onChange={handleSizeChange}
              valueLabelDisplay="auto"
              min={0}
              max={100}
              marks={[
                { value: 0, label: '0 MB' },
                { value: 25, label: '25 MB' },
                { value: 50, label: '50 MB' },
                { value: 75, label: '75 MB' },
                { value: 100, label: '100 MB' },
              ]}
            />
          </Box>
        </Grid>
      </Grid>
    </LocalizationProvider>
  );
};

export default SearchFilters;