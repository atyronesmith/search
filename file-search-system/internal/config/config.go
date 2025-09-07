package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DatabaseURL     string
	DatabasePool    int
	DatabaseTimeout time.Duration

	// Ollama
	OllamaHost      string
	OllamaModel     string
	EmbeddingDim    int
	OllamaTimeout   time.Duration

	// API Server
	APIHost    string
	APIPort    int
	APIWorkers int

	// Indexing
	IndexBatchSize      int
	IndexMaxFileSizeMB  int
	IndexChunkSize      int
	IndexChunkOverlap   int
	IndexWorkers        int

	// File Monitoring
	WatchPaths        []string
	WatchInterval     time.Duration
	WatchIgnorePatterns []string

	// Resource Management
	CPUThreshold      float64
	MemoryThreshold   float64
	RateLimitFiles    int
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
	DoclingEnabled     bool
	DoclingServiceURL  string
	DoclingTimeout     time.Duration
	DoclingFallback    bool

	// Service
	ServiceName      string
	ServiceAutoStart bool
	StartDelay       time.Duration
}

func Load(path string) (*Config, error) {
	// Load .env file if it exists
	if path != "" {
		if err := godotenv.Load(path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	cfg := &Config{
		// Database defaults
		DatabaseURL:     getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/file_search?sslmode=disable"),
		DatabasePool:    getEnvInt("DATABASE_POOL_SIZE", 10),
		DatabaseTimeout: getEnvDuration("DATABASE_TIMEOUT", "30s"),

		// Ollama defaults
		OllamaHost:    getEnv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		EmbeddingDim:  getEnvInt("OLLAMA_EMBEDDING_DIM", 768),
		OllamaTimeout: getEnvDuration("OLLAMA_TIMEOUT", "30s"),

		// API defaults
		APIHost:    getEnv("API_HOST", "127.0.0.1"),
		APIPort:    getEnvInt("API_PORT", 8080),
		APIWorkers: getEnvInt("API_WORKERS", 4),

		// Indexing defaults
		IndexBatchSize:     getEnvInt("INDEX_BATCH_SIZE", 32),
		IndexMaxFileSizeMB: getEnvInt("INDEX_MAX_FILE_SIZE_MB", 100),
		IndexChunkSize:     getEnvInt("INDEX_CHUNK_SIZE", 512),
		IndexChunkOverlap:  getEnvInt("INDEX_CHUNK_OVERLAP", 64),
		IndexWorkers:       getEnvInt("INDEX_WORKERS", 4),

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