package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/file-search/file-search-system/internal/database"
)

// DBConfigService handles configuration stored in the database
type DBConfigService struct {
	db *database.DB
}

// Value represents a configuration value from the database
type Value struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewDBConfigService creates a new database configuration service
func NewDBConfigService(db *database.DB) *DBConfigService {
	return &DBConfigService{
		db: db,
	}
}

// GetConfig retrieves the current configuration from database
func (c *DBConfigService) GetConfig(ctx context.Context) (*Config, error) {
	// Query all configuration values
	query := `
		SELECT config_key, config_value, config_type, description, category, created_at, updated_at
		FROM system_config
		ORDER BY category, config_key
	`

	rows, err := c.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %w", err)
	}
	defer rows.Close()

	config := &Config{}

	for rows.Next() {
		var cv Value
		if err := rows.Scan(&cv.Key, &cv.Value, &cv.Type, &cv.Description, &cv.Category, &cv.CreatedAt, &cv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}

		// Map database values to Config struct
		if err := c.mapConfigValue(config, cv); err != nil {
			return nil, fmt.Errorf("failed to map config value %s: %w", cv.Key, err)
		}
	}

	return config, nil
}

// GetConfigMap returns configuration as a map for API responses
func (c *DBConfigService) GetConfigMap(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT config_key, config_value, config_type, category
		FROM system_config
		ORDER BY category, config_key
	`

	rows, err := c.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]interface{})
	categories := make(map[string]map[string]interface{})

	for rows.Next() {
		var key, value, configType, category string
		if err := rows.Scan(&key, &value, &configType, &category); err != nil {
			continue
		}

		// Convert value based on type
		var parsedValue interface{}
		switch configType {
		case "boolean":
			parsedValue, _ = strconv.ParseBool(value)
		case "number":
			if strings.Contains(value, ".") {
				parsedValue, _ = strconv.ParseFloat(value, 64)
			} else {
				parsedValue, _ = strconv.Atoi(value)
			}
		case "json":
			// Handle comma-separated lists as arrays
			if strings.Contains(value, ",") {
				parsedValue = strings.Split(value, ",")
				// Trim whitespace from each element
				arr := parsedValue.([]string)
				for i, v := range arr {
					arr[i] = strings.TrimSpace(v)
				}
			} else {
				// For json type, always return as array even for single values
				if key == "watch_paths" || key == "ignore_patterns" {
					parsedValue = []string{strings.TrimSpace(value)}
				} else {
					parsedValue = value
				}
			}
		default:
			parsedValue = value
		}

		// Group by category
		if categories[category] == nil {
			categories[category] = make(map[string]interface{})
		}
		categories[category][key] = parsedValue

		// Also add to flat structure for backward compatibility
		result[key] = parsedValue
	}

	// Add categorized structure
	result["categories"] = categories

	return result, nil
}

// UpdateConfig updates configuration values in the database
func (c *DBConfigService) UpdateConfig(ctx context.Context, updates map[string]interface{}) error {
	for key, value := range updates {
		if err := c.updateConfigValue(ctx, key, value); err != nil {
			return fmt.Errorf("failed to update config %s: %w", key, err)
		}
	}
	return nil
}

// updateConfigValue updates a single configuration value
func (c *DBConfigService) updateConfigValue(ctx context.Context, key string, value interface{}) error {
	var stringValue string
	var configType string

	switch v := value.(type) {
	case bool:
		stringValue = strconv.FormatBool(v)
		configType = "boolean"
	case int:
		stringValue = strconv.Itoa(v)
		configType = "number"
	case float64:
		stringValue = strconv.FormatFloat(v, 'f', -1, 64)
		configType = "number"
	case []string:
		stringValue = strings.Join(v, ",")
		configType = "json"
	case []interface{}:
		// Convert interface slice to string slice
		strSlice := make([]string, len(v))
		for i, item := range v {
			strSlice[i] = fmt.Sprintf("%v", item)
		}
		stringValue = strings.Join(strSlice, ",")
		configType = "json"
	default:
		stringValue = fmt.Sprintf("%v", v)
		configType = "string"
	}

	query := `
		UPDATE system_config 
		SET config_value = $1, config_type = $2, updated_at = CURRENT_TIMESTAMP
		WHERE config_key = $3
	`

	_, err := c.db.Exec(ctx, query, stringValue, configType, key)
	return err
}

// mapConfigValue maps a database config value to the Config struct
func (c *DBConfigService) mapConfigValue(config *Config, cv Value) error {
	switch cv.Key {
	// Database
	case "database_url":
		config.DatabaseURL = cv.Value
	case "database_max_connections":
		val, _ := strconv.Atoi(cv.Value)
		config.DatabasePool = val
	case "database_timeout":
		val, _ := strconv.Atoi(cv.Value)
		config.DatabaseTimeout = time.Duration(val) * time.Second

	// Ollama/AI
	case "ollama_host":
		config.OllamaHost = cv.Value
	case "embedding_model":
		config.OllamaModel = cv.Value
	case "embedding_dim":
		val, _ := strconv.Atoi(cv.Value)
		config.EmbeddingDim = val
	case "ollama_timeout":
		val, _ := strconv.Atoi(cv.Value)
		config.OllamaTimeout = time.Duration(val) * time.Second

	// API Server (keep from env for now since they're startup params)
	case "api_host":
		config.APIHost = cv.Value
	case "api_port":
		val, _ := strconv.Atoi(cv.Value)
		config.APIPort = val

	// Indexing
	case "batch_size":
		val, _ := strconv.Atoi(cv.Value)
		config.IndexBatchSize = val
	case "max_file_size_mb":
		val, _ := strconv.Atoi(cv.Value)
		config.IndexMaxFileSizeMB = val
	case "chunk_size":
		val, _ := strconv.Atoi(cv.Value)
		config.IndexChunkSize = val
	case "chunk_overlap":
		val, _ := strconv.Atoi(cv.Value)
		config.IndexChunkOverlap = val

	// File Monitoring
	case "watch_paths":
		config.WatchPaths = strings.Split(cv.Value, ",")
		// Trim whitespace and expand paths
		for i, path := range config.WatchPaths {
			config.WatchPaths[i] = expandPath(strings.TrimSpace(path))
		}
	case "ignore_patterns":
		config.WatchIgnorePatterns = strings.Split(cv.Value, ",")
		// Trim whitespace
		for i, pattern := range config.WatchIgnorePatterns {
			config.WatchIgnorePatterns[i] = strings.TrimSpace(pattern)
		}

	// Resource Management
	case "cpu_threshold":
		val, _ := strconv.ParseFloat(cv.Value, 64)
		config.CPUThreshold = val
	case "memory_threshold":
		val, _ := strconv.ParseFloat(cv.Value, 64)
		config.MemoryThreshold = val
	case "files_per_minute":
		val, _ := strconv.Atoi(cv.Value)
		config.RateLimitFiles = val
	case "embeddings_per_minute":
		val, _ := strconv.Atoi(cv.Value)
		config.RateLimitEmbeddings = val

	// Search
	case "search_vector_weight":
		val, _ := strconv.ParseFloat(cv.Value, 64)
		config.SearchVectorWeight = val
	case "search_bm25_weight":
		val, _ := strconv.ParseFloat(cv.Value, 64)
		config.SearchBM25Weight = val
	case "search_metadata_weight":
		val, _ := strconv.ParseFloat(cv.Value, 64)
		config.SearchMetadataWeight = val
	case "search_cache_ttl":
		val, _ := strconv.Atoi(cv.Value)
		config.SearchCacheTTL = time.Duration(val) * time.Second
	case "search_default_limit":
		val, _ := strconv.Atoi(cv.Value)
		config.SearchDefaultLimit = val

	// Docling
	case "docling_enabled":
		val, _ := strconv.ParseBool(cv.Value)
		config.DoclingEnabled = val
	case "docling_service_url":
		config.DoclingServiceURL = cv.Value
	case "docling_timeout":
		val, _ := strconv.Atoi(cv.Value)
		config.DoclingTimeout = time.Duration(val) * time.Second
	case "docling_fallback":
		val, _ := strconv.ParseBool(cv.Value)
		config.DoclingFallback = val
	}

	return nil
}

// GetOllamaModels queries Ollama for available models
func (c *DBConfigService) GetOllamaModels(ctx context.Context) ([]string, error) {
	// Get Ollama host from config
	config, err := c.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Try to fetch models from Ollama API
	url := config.OllamaHost + "/api/tags"

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return c.getFallbackModels(), nil // Return fallback on error
	}

	resp, err := client.Do(req)
	if err != nil {
		return c.getFallbackModels(), nil // Return fallback on error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.getFallbackModels(), nil // Return fallback on error
	}

	// Parse Ollama response
	var ollamaResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return c.getFallbackModels(), nil // Return fallback on error
	}

	// Extract model names
	models := make([]string, 0, len(ollamaResp.Models))
	for _, model := range ollamaResp.Models {
		models = append(models, model.Name)
	}

	// If no models found, return fallback
	if len(models) == 0 {
		return c.getFallbackModels(), nil
	}

	return models, nil
}

// getFallbackModels returns common embedding models as fallback
func (c *DBConfigService) getFallbackModels() []string {
	return []string{
		"nomic-embed-text",
		"mxbai-embed-large",
		"all-minilm",
		"sentence-transformers/all-MiniLM-L6-v2",
		"bge-large-en-v1.5",
	}
}
