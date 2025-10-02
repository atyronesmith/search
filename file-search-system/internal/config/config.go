package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	// Database
	DatabaseURL     string
	DatabasePool    int
	DatabaseTimeout time.Duration

	// Ollama
	OllamaHost    string
	OllamaModel   string
	LLMModel      string
	EmbeddingDim  int
	OllamaTimeout time.Duration

	// API Server
	APIHost         string
	APIPort         int
	APIWorkers      int
	APIDevKey       string   // Development API key
	APIProdKey      string   // Production API key
	AllowedOrigins  []string // CORS allowed origins
	RequireWSAuth   bool     // Require authentication for WebSocket
	WSAuthToken     string   // WebSocket authentication token

	// Indexing
	IndexBatchSize     int
	IndexMaxFileSizeMB int
	IndexChunkSize     int
	IndexChunkOverlap  int
	IndexWorkers       int

	// File Monitoring
	WatchPaths          []string
	WatchInterval       time.Duration
	WatchIgnorePatterns []string

	// Resource Management
	CPUThreshold        float64
	MemoryThreshold     float64
	RateLimitFiles      int
	RateLimitEmbeddings int

	// Search
	SearchVectorWeight   float64
	SearchBM25Weight     float64
	SearchMetadataWeight float64
	SearchCacheTTL       time.Duration
	SearchDefaultLimit   int

	// Logging
	LogLevel string
	LogFile  string

	// Docling Service
	DoclingEnabled    bool
	DoclingServiceURL string
	DoclingTimeout    time.Duration
	DoclingFallback   bool

	// Service
	ServiceName      string
	ServiceAutoStart bool
	StartDelay       time.Duration

	// Additional fields for config file support
	ConfigFile string // Path to config file being used
	DevMode    bool   // Development mode
	HotReload  bool   // Hot reload enabled
	Debug      bool   // Debug mode
}

// Load loads configuration from config files and environment variables
func Load(configPath string) (*Config, error) {
	// If a specific path was provided, try to use it
	if configPath != "" && configPath != "../search.cfg" {
		return LoadFromFiles(configPath)
	}

	// Otherwise, search for config file in multiple locations
	searchPaths := []string{
		"search.cfg",           // Current directory
		"../search.cfg",        // Parent directory (running from file-search-system)
		"../../search.cfg",     // Two levels up (running from cmd/server)
		"../../../search.cfg",  // Three levels up (for nested development)
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return LoadFromFiles(path)
		}
	}

	// Fall back to environment variables only
	return LoadFromEnv()
}

// LoadFromFiles loads configuration from search.cfg and secrets.cfg
func LoadFromFiles(configPath string) (*Config, error) {
	// Create parser and load main config
	parser := NewParser()
	if err := parser.LoadFile(configPath); err != nil {
		return nil, fmt.Errorf("failed to load config file: %v", err)
	}

	// Load secrets file (optional)
	secretsPath := strings.Replace(configPath, "search.cfg", "secrets.cfg", 1)
	if _, err := os.Stat(secretsPath); err == nil {
		secretsParser := NewParser()
		if err := secretsParser.LoadFile(secretsPath); err == nil {
			parser.Merge(secretsParser)
		}
	}

	// Build database URL from components
	dbPassword := parser.GetString("database", "password", "postgres")
	dbHost := parser.GetString("database", "host", "localhost")
	dbPort := parser.GetInt("database", "port", 5432)
	dbName := parser.GetString("database", "name", "file_search")
	dbUser := parser.GetString("database", "user", "postgres")
	dbSSLMode := parser.GetString("database", "ssl_mode", "disable")
	databaseURL := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)

	cfg := &Config{
		// Database
		DatabaseURL:     getEnvOrConfig(parser, "DATABASE_URL", "", "", databaseURL),
		DatabasePool:    getEnvIntOrConfig(parser, "DATABASE_POOL_SIZE", "database", "max_connections", 10),
		DatabaseTimeout: getEnvDurationOrConfig(parser, "DATABASE_TIMEOUT", "database", "timeout", 30*time.Second),

		// Ollama
		OllamaHost:    getEnvOrConfig(parser, "OLLAMA_HOST", "ollama", "host", "http://localhost:11434"),
		OllamaModel:   getEnvOrConfig(parser, "OLLAMA_MODEL", "ollama", "embedding_model", "nomic-embed-text"),
		LLMModel:      getEnvOrConfig(parser, "LLM_MODEL", "ollama", "llm_model", "phi3:mini"),
		EmbeddingDim:  getEnvIntOrConfig(parser, "OLLAMA_EMBEDDING_DIM", "ollama", "embedding_dim", 768),
		OllamaTimeout: getEnvDurationOrConfig(parser, "OLLAMA_TIMEOUT", "ollama", "timeout", 30*time.Second),

		// API
		APIHost:        getEnvOrConfig(parser, "API_HOST", "server", "host", "127.0.0.1"),
		APIPort:        getEnvIntOrConfig(parser, "API_PORT", "server", "port", 8080),
		APIWorkers:     getEnvIntOrConfig(parser, "API_WORKERS", "monitoring", "max_workers", 4),
		APIDevKey:      getEnvOrConfig(parser, "API_DEV_KEY", "api_keys", "dev_key", ""),
		APIProdKey:     getEnvOrConfig(parser, "API_PROD_KEY", "api_keys", "prod_key", ""),
		AllowedOrigins: getEnvStringSliceOrConfig(parser, "ALLOWED_ORIGINS", "security", "allowed_origins", []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}),
		RequireWSAuth:  getEnvBoolOrConfig(parser, "REQUIRE_WS_AUTH", "security", "require_ws_auth", true),
		WSAuthToken:    getEnvOrConfig(parser, "WS_AUTH_TOKEN", "api_keys", "ws_token", ""),

		// Indexing
		IndexBatchSize:     getEnvIntOrConfig(parser, "INDEX_BATCH_SIZE", "indexing", "batch_size", 32),
		IndexMaxFileSizeMB: getEnvIntOrConfig(parser, "INDEX_MAX_FILE_SIZE_MB", "indexing", "max_file_size_mb", 100),
		IndexChunkSize:     getEnvIntOrConfig(parser, "INDEX_CHUNK_SIZE", "indexing", "chunk_size", 512),
		IndexChunkOverlap:  getEnvIntOrConfig(parser, "INDEX_CHUNK_OVERLAP", "indexing", "chunk_overlap", 64),
		IndexWorkers:       getEnvIntOrConfig(parser, "INDEX_WORKERS", "indexing", "workers", 4),

		// File Monitoring
		WatchPaths:          getEnvStringSliceOrConfig(parser, "WATCH_PATHS", "indexing", "watch_paths", []string{"~/Documents", "~/Downloads"}),
		WatchInterval:       getEnvDurationOrConfig(parser, "WATCH_INTERVAL", "monitoring", "scan_interval", 5*time.Second),
		WatchIgnorePatterns: getEnvStringSliceOrConfig(parser, "WATCH_IGNORE_PATTERNS", "indexing", "ignore_patterns", []string{".*", "~*", "*.tmp", "__pycache__", "node_modules", ".git"}),

		// Resource Management
		CPUThreshold:        getEnvFloatOrConfig(parser, "CPU_THRESHOLD_PERCENT", "performance", "cpu_threshold", 70.0),
		MemoryThreshold:     getEnvFloatOrConfig(parser, "MEMORY_THRESHOLD_PERCENT", "performance", "memory_threshold", 80.0),
		RateLimitFiles:      getEnvIntOrConfig(parser, "RATE_LIMIT_FILES_PER_MINUTE", "performance", "files_per_minute", 60),
		RateLimitEmbeddings: getEnvIntOrConfig(parser, "RATE_LIMIT_EMBEDDINGS_PER_MINUTE", "performance", "embeddings_per_minute", 120),

		// Search
		SearchVectorWeight:   getEnvFloatOrConfig(parser, "SEARCH_VECTOR_WEIGHT", "search", "vector_weight", 0.6),
		SearchBM25Weight:     getEnvFloatOrConfig(parser, "SEARCH_BM25_WEIGHT", "search", "bm25_weight", 0.3),
		SearchMetadataWeight: getEnvFloatOrConfig(parser, "SEARCH_METADATA_WEIGHT", "search", "metadata_weight", 0.1),
		SearchCacheTTL:       getEnvDurationOrConfig(parser, "SEARCH_CACHE_TTL", "search", "cache_ttl", 15*time.Minute),
		SearchDefaultLimit:   getEnvIntOrConfig(parser, "SEARCH_DEFAULT_LIMIT", "search", "default_limit", 20),

		// Logging
		LogLevel: getEnvOrConfig(parser, "LOG_LEVEL", "logging", "level", "INFO"),
		LogFile:  getEnvOrConfig(parser, "LOG_FILE", "logging", "file", ""),

		// Docling
		DoclingEnabled:    getEnvBoolOrConfig(parser, "DOCLING_ENABLED", "docling", "enabled", false),
		DoclingServiceURL: getEnvOrConfig(parser, "DOCLING_SERVICE_URL", "docling", "service_url", "http://localhost:5000"),
		DoclingTimeout:    getEnvDurationOrConfig(parser, "DOCLING_TIMEOUT", "docling", "timeout", 300*time.Second),
		DoclingFallback:   getEnvBoolOrConfig(parser, "DOCLING_FALLBACK", "docling", "fallback", true),

		// Service
		ServiceName:      getEnvOrConfig(parser, "SERVICE_NAME", "service", "name", "FileSearchService"),
		ServiceAutoStart: getEnvBoolOrConfig(parser, "SERVICE_AUTO_START", "service", "auto_start", true),
		StartDelay:       getEnvDurationOrConfig(parser, "SERVICE_START_DELAY", "service", "start_delay", 10*time.Second),

		// Additional fields
		ConfigFile: configPath,
		DevMode:    getEnvBoolOrConfig(parser, "DEV_MODE", "development", "dev_mode", false),
		HotReload:  getEnvBoolOrConfig(parser, "HOT_RELOAD", "development", "hot_reload", false),
		Debug:      getEnvBoolOrConfig(parser, "DEBUG", "development", "debug", false),
	}

	// Expand home directory in watch paths
	for i, path := range cfg.WatchPaths {
		cfg.WatchPaths[i] = expandPath(path)
	}

	return cfg, nil
}

// LoadFromEnv loads configuration from environment variables only (fallback)
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		// Database defaults (use empty default to force configuration)
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		DatabasePool:    getEnvInt("DATABASE_POOL_SIZE", 10),
		DatabaseTimeout: getEnvDuration("DATABASE_TIMEOUT", "30s"),

		// Ollama defaults
		OllamaHost:    getEnv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		LLMModel:      getEnv("LLM_MODEL", "phi3:mini"),
		EmbeddingDim:  getEnvInt("OLLAMA_EMBEDDING_DIM", 768),
		OllamaTimeout: getEnvDuration("OLLAMA_TIMEOUT", "30s"),

		// API defaults
		APIHost:        getEnv("API_HOST", "127.0.0.1"),
		APIPort:        getEnvInt("API_PORT", 8080),
		APIWorkers:     getEnvInt("API_WORKERS", 4),
		APIDevKey:      getEnv("API_DEV_KEY", ""),
		APIProdKey:     getEnv("API_PROD_KEY", ""),
		AllowedOrigins: getEnvStringSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}),
		RequireWSAuth:  getEnvBool("REQUIRE_WS_AUTH", true),
		WSAuthToken:    getEnv("WS_AUTH_TOKEN", ""),

		// Indexing defaults
		IndexBatchSize:     getEnvInt("INDEX_BATCH_SIZE", 32),
		IndexMaxFileSizeMB: getEnvInt("INDEX_MAX_FILE_SIZE_MB", 100),
		IndexChunkSize:     getEnvInt("INDEX_CHUNK_SIZE", 512),
		IndexChunkOverlap:  getEnvInt("INDEX_CHUNK_OVERLAP", 64),
		IndexWorkers:       getEnvInt("INDEX_WORKERS", 1),

		// File Monitoring defaults
		WatchPaths:          getEnvStringSlice("WATCH_PATHS", []string{"~/Documents", "~/Desktop", "~/Downloads"}),
		WatchInterval:       getEnvDuration("WATCH_INTERVAL", "5s"),
		WatchIgnorePatterns: getEnvStringSlice("WATCH_IGNORE_PATTERNS", []string{".*", "~*", "*.tmp", "__pycache__", "node_modules", ".git"}),

		// Resource Management defaults
		CPUThreshold:        getEnvFloat("CPU_THRESHOLD_PERCENT", 70.0),
		MemoryThreshold:     getEnvFloat("MEMORY_THRESHOLD_PERCENT", 80.0),
		RateLimitFiles:      getEnvInt("RATE_LIMIT_FILES_PER_MINUTE", 60),
		RateLimitEmbeddings: getEnvInt("RATE_LIMIT_EMBEDDINGS_PER_MINUTE", 120),

		// Search defaults
		SearchVectorWeight:   getEnvFloat("SEARCH_VECTOR_WEIGHT", 0.6),
		SearchBM25Weight:     getEnvFloat("SEARCH_BM25_WEIGHT", 0.3),
		SearchMetadataWeight: getEnvFloat("SEARCH_METADATA_WEIGHT", 0.1),
		SearchCacheTTL:       getEnvDuration("SEARCH_CACHE_TTL", "15m"),
		SearchDefaultLimit:   getEnvInt("SEARCH_DEFAULT_LIMIT", 20),

		// Logging defaults
		LogLevel: getEnv("LOG_LEVEL", "INFO"),
		LogFile:  getEnv("LOG_FILE", ""),

		// Docling service defaults
		DoclingEnabled:    getEnvBool("DOCLING_ENABLED", true),
		DoclingServiceURL: getEnv("DOCLING_SERVICE_URL", "http://localhost:8082"),
		DoclingTimeout:    getEnvDuration("DOCLING_TIMEOUT", "300s"),
		DoclingFallback:   getEnvBool("DOCLING_FALLBACK", true),

		// Service defaults
		ServiceName:      getEnv("SERVICE_NAME", "FileSearchService"),
		ServiceAutoStart: getEnvBool("SERVICE_AUTO_START", true),
		StartDelay:       getEnvDuration("SERVICE_START_DELAY", "10s"),
	}

	// If no watch paths are set, default to ~/Documents and ~/Downloads
	if len(cfg.WatchPaths) == 0 {
		home, _ := os.UserHomeDir()
		cfg.WatchPaths = []string{
			home + "/Documents",
			home + "/Downloads",
		}
	}
	// Expand home directory in paths
	for i, path := range cfg.WatchPaths {
		cfg.WatchPaths[i] = expandPath(path)
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	// Try parsing as seconds if duration parsing fails
	if i, err := strconv.Atoi(value); err == nil {
		return time.Duration(i) * time.Second
	}
	// Return default duration
	d, _ := time.ParseDuration(defaultValue)
	return d
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return strings.Replace(path, "~", home, 1)
	}
	return path
}

// Helper functions to get values from environment or config file
func getEnvOrConfig(parser *Parser, envKey, section, key, defaultValue string) string {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetString(section, key, defaultValue)
	}
	return defaultValue
}

func getEnvIntOrConfig(parser *Parser, envKey, section, key string, defaultValue int) int {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetInt(section, key, defaultValue)
	}
	return defaultValue
}

func getEnvFloatOrConfig(parser *Parser, envKey, section, key string, defaultValue float64) float64 {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetFloat(section, key, defaultValue)
	}
	return defaultValue
}

func getEnvBoolOrConfig(parser *Parser, envKey, section, key string, defaultValue bool) bool {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetBool(section, key, defaultValue)
	}
	return defaultValue
}

func getEnvDurationOrConfig(parser *Parser, envKey, section, key string, defaultValue time.Duration) time.Duration {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		// Try parsing as seconds
		if i, err := strconv.Atoi(value); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetDuration(section, key, defaultValue)
	}
	return defaultValue
}

func getEnvStringSliceOrConfig(parser *Parser, envKey, section, key string, defaultValue []string) []string {
	// Environment variable takes precedence
	if value := os.Getenv(envKey); value != "" {
		return strings.Split(value, ",")
	}
	// Then check config file
	if parser != nil && section != "" && key != "" {
		return parser.GetStringSlice(section, key, defaultValue)
	}
	return defaultValue
}
