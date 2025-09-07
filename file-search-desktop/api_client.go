package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// APIClient handles communication with the backend server
type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Increased timeout for LLM processing
		},
	}
}

// SearchAPIRequest represents the API search request
type SearchAPIRequest struct {
	Query   string                 `json:"query"`
	Filters map[string]interface{} `json:"filters,omitempty"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
}

// EnhancedQuery represents LLM-enhanced query details
type EnhancedQuery struct {
	Original         string                   `json:"original"`
	Enhanced         string                   `json:"enhanced"`
	SearchTerms      []string                 `json:"search_terms"`
	ContentFilters   []interface{}            `json:"content_filters"`
	MetadataFilters  []interface{}            `json:"metadata_filters"`
	Intent           string                   `json:"intent"`
	RequiresCount    bool                     `json:"requires_count"`
}

// SearchAPIResponse represents the API search response
type SearchAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		QueryID string `json:"query_id"`
		Results struct {
			Query         string         `json:"query"`
			EnhancedQuery *EnhancedQuery `json:"enhanced_query,omitempty"`
			Results       []SearchAPIResultItem    `json:"results"`
			TotalCount    int                      `json:"total_count"`
			SearchTime    int64                    `json:"search_time"`
			Cached        bool                     `json:"cached"`
			UsedLLM       bool                     `json:"used_llm"`
		} `json:"results"`
	} `json:"data"`
}

// SearchAPIResultItem represents a single search result from the API
type SearchAPIResultItem struct {
	FileID       int64                  `json:"file_id"`
	ChunkID      int64                  `json:"chunk_id"`
	FilePath     string                 `json:"file_path"`
	Filename     string                 `json:"filename"`
	FileType     string                 `json:"file_type"`
	Content      string                 `json:"content"`
	Score        float64                `json:"score"`
	VectorScore  float64                `json:"vector_score"`
	TextScore    float64                `json:"text_score"`
	MetadataScore float64               `json:"metadata_score"`
	Highlights   []string               `json:"highlights"`
	StartLine    int                    `json:"start_line"`
	CharStart    int                    `json:"char_start"`
	CharEnd      int                    `json:"char_end"`
	// Metadata field removed to avoid map[string]interface{} binding issues
}

// Search performs a search via the API
func (c *APIClient) Search(request SearchRequest) ([]SearchResult, error) {
	log.Printf("APIClient.Search called with request: %+v", request)
	// Use request limit/offset or defaults
	limit := request.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	
	apiReq := SearchAPIRequest{
		Query:   request.Query,
		Filters: make(map[string]interface{}), // Empty filters
		Limit:   limit,
		Offset:  request.Offset,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/search",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp SearchAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("search API returned error response")
	}

	// Convert API result items to our SearchResult format
	log.Printf("Converting %d API results to SearchResult format", len(apiResp.Data.Results.Results))
	results := make([]SearchResult, len(apiResp.Data.Results.Results))
	for i, apiItem := range apiResp.Data.Results.Results {
		log.Printf("Converting result %d: %+v", i, apiItem)
		
		// Add defensive handling for potentially problematic fields
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC during result %d conversion: %v", i, r)
			}
		}()
		
		// Sanitize content to avoid binding issues
		sanitizedContent := strings.ReplaceAll(apiItem.Content, "<script>", "&lt;script&gt;")
		sanitizedContent = strings.ReplaceAll(sanitizedContent, "</script>", "&lt;/script&gt;")
		if len(sanitizedContent) > 500 {
			sanitizedContent = sanitizedContent[:500] + "..."
		}
		
		sanitizedHighlights := make([]string, len(apiItem.Highlights))
		for j, highlight := range apiItem.Highlights {
			sanitizedHighlights[j] = strings.ReplaceAll(highlight, "<script>", "&lt;script&gt;")
			sanitizedHighlights[j] = strings.ReplaceAll(sanitizedHighlights[j], "</script>", "&lt;/script&gt;")
			if len(sanitizedHighlights[j]) > 200 {
				sanitizedHighlights[j] = sanitizedHighlights[j][:200] + "..."
			}
		}
		
		results[i] = SearchResult{
			ID:           fmt.Sprintf("%d", apiItem.ChunkID),
			Path:         apiItem.FilePath,
			Name:         apiItem.Filename,
			Type:         apiItem.FileType,
			Size:         0, // Size not provided in chunk response, could be added later
			ModifiedAt:   time.Now().Format(time.RFC3339), // ModifiedAt not in chunk response, could be added later
			Score:        apiItem.Score,
			Highlights:   sanitizedHighlights,
			Snippet:      sanitizedContent,
			TotalResults: apiResp.Data.Results.TotalCount,
		}
		log.Printf("Successfully converted result %d", i)
	}
	log.Printf("Finished converting all results, preparing to return")

	return results, nil
}

// SearchWithDetails performs a search and returns enhanced query information
func (c *APIClient) SearchWithDetails(request SearchRequest) (SearchResponseWithDetails, error) {
	log.Printf("APIClient.SearchWithDetails called with request: %+v", request)
	
	// Use request limit/offset or defaults
	limit := request.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	
	apiReq := SearchAPIRequest{
		Query:   request.Query,
		Filters: make(map[string]interface{}), // Empty filters
		Limit:   limit,
		Offset:  request.Offset,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return SearchResponseWithDetails{}, fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/search",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return SearchResponseWithDetails{}, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return SearchResponseWithDetails{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body first for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return SearchResponseWithDetails{}, fmt.Errorf("failed to read response body: %v", err)
	}
	
	log.Printf("DEBUG: Raw API response: %s", string(bodyBytes))
	
	var apiResp SearchAPIResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return SearchResponseWithDetails{}, fmt.Errorf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		return SearchResponseWithDetails{}, fmt.Errorf("search API returned error response")
	}

	// Convert API result items to our SearchResult format
	log.Printf("Converting %d API results to SearchResult format", len(apiResp.Data.Results.Results))
	results := make([]SearchResult, len(apiResp.Data.Results.Results))
	for i, apiItem := range apiResp.Data.Results.Results {
		log.Printf("Converting result %d: %+v", i, apiItem)
		
		// Sanitize content to prevent XSS
		sanitizedContent := strings.ReplaceAll(apiItem.Content, "<script>", "&lt;script&gt;")
		sanitizedContent = strings.ReplaceAll(sanitizedContent, "</script>", "&lt;/script&gt;")
		if len(sanitizedContent) > 500 {
			sanitizedContent = sanitizedContent[:500] + "..."
		}
		
		// Sanitize highlights
		sanitizedHighlights := make([]string, len(apiItem.Highlights))
		for j, highlight := range apiItem.Highlights {
			sanitizedHighlights[j] = strings.ReplaceAll(highlight, "<script>", "&lt;script&gt;")
			sanitizedHighlights[j] = strings.ReplaceAll(sanitizedHighlights[j], "</script>", "&lt;/script&gt;")
			if len(sanitizedHighlights[j]) > 200 {
				sanitizedHighlights[j] = sanitizedHighlights[j][:200] + "..."
			}
		}
		
		results[i] = SearchResult{
			ID:           fmt.Sprintf("%d", apiItem.ChunkID),
			Path:         apiItem.FilePath,
			Name:         apiItem.Filename,
			Type:         apiItem.FileType,
			Size:         0, // Size not provided in chunk response
			ModifiedAt:   time.Now().Format(time.RFC3339), // ModifiedAt not in chunk response
			Score:        apiItem.Score,
			Highlights:   sanitizedHighlights,
			Snippet:      sanitizedContent,
			TotalResults: apiResp.Data.Results.TotalCount,
		}
	}

	// Create detailed response with enhanced query information
	response := SearchResponseWithDetails{
		Results:       results,
		EnhancedQuery: apiResp.Data.Results.EnhancedQuery,
		UsedLLM:       apiResp.Data.Results.UsedLLM,
		SearchTime:    apiResp.Data.Results.SearchTime,
		TotalCount:    apiResp.Data.Results.TotalCount,
	}

	log.Printf("DEBUG: Enhanced query from API: %+v", apiResp.Data.Results.EnhancedQuery)
	if apiResp.Data.Results.EnhancedQuery != nil {
		log.Printf("DEBUG: Search terms from API: %+v", apiResp.Data.Results.EnhancedQuery.SearchTerms)
	}
	log.Printf("Final SearchWithDetails response created")
	return response, nil
}

// StartIndexing starts the indexing process via the API
func (c *APIClient) StartIndexing(path string) error {
	// Convert single path to paths array and set recursive=true
	paths := []string{}
	if path != "" {
		paths = []string{path}
	}
	// If no path provided, send empty array to let backend use configured WATCH_PATHS
	
	body, _ := json.Marshal(map[string]interface{}{
		"paths":     paths,
		"recursive": true,
	})
	
	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/indexing/start",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// StopIndexing stops the indexing process via the API
func (c *APIClient) StopIndexing() error {
	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/indexing/stop",
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PauseIndexing pauses the indexing process via the API
func (c *APIClient) PauseIndexing() error {
	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/indexing/pause",
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ResumeIndexing resumes the indexing process via the API
func (c *APIClient) ResumeIndexing() error {
	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/indexing/resume",
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetIndexingStatus gets the indexing status via the API
func (c *APIClient) GetIndexingStatus() (IndexingStatus, error) {
	// Get detailed progress from system status
	systemStatus, err := c.GetSystemStatus()
	if err != nil {
		return IndexingStatus{State: "unknown"}, nil
	}

	// Get indexing state from indexing status endpoint
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/indexing/status")
	if err != nil {
		return IndexingStatus{}, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	var indexingState string = "unknown"
	if resp.StatusCode == http.StatusOK {
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err == nil {
			if getBool(response, "success") {
				data := getMap(response, "data")
				indexingState = getIndexingState(data)
			}
		}
	}

	// Get the data from the API response directly
	resp2, err := c.httpClient.Get(c.baseURL + "/api/v1/status")
	if err != nil {
		return IndexingStatus{State: indexingState}, nil
	}
	defer resp2.Body.Close()
	
	var indexedFiles, pendingFiles, failedFiles int
	var totalFiles int
	
	if resp2.StatusCode == http.StatusOK {
		var response map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&response); err == nil {
			if getBool(response, "success") {
				data := getMap(response, "data")
				indexedFiles = int(getFloat(data, "indexed_files"))
				pendingFiles = int(getFloat(data, "pending_files"))
				failedFiles = int(getFloat(data, "failed_files"))
				totalFiles = int(getFloat(data, "total_files"))
			}
		}
	}
	
	// If total is 0, calculate from indexed + pending
	if totalFiles == 0 {
		totalFiles = indexedFiles + pendingFiles
	}
	
	// Convert the response to our IndexingStatus struct  
	status := IndexingStatus{
		State:          indexingState,
		FilesProcessed: indexedFiles,
		TotalFiles:     totalFiles,
		PendingFiles:   pendingFiles,
		CurrentFile:    "",  // TODO: Get from current processing file
		Errors:         failedFiles,
		ElapsedTime:    systemStatus.Uptime,
	}

	return status, nil
}

// GetSystemStatus gets the system status via the API
func (c *APIClient) GetSystemStatus() (SystemStatus, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/status")
	if err != nil {
		// Return a default status if the API is not available
		return SystemStatus{
			Status: "disconnected",
			Database: map[string]interface{}{
				"connected": false,
			},
			Embeddings: map[string]interface{}{
				"available": false,
			},
			Indexing: map[string]interface{}{
				"active": false,
				"state":  "unknown",
			},
			Resources: map[string]interface{}{
				"cpu":    0,
				"memory": 0,
				"disk":   0,
			},
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SystemStatus{
			Status: "error",
		}, nil
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return SystemStatus{}, fmt.Errorf("failed to decode response: %v", err)
	}

	// Check if the response has the expected structure
	if !getBool(response, "success") {
		return SystemStatus{
			Status: "error",
		}, nil
	}

	// Get the data field from the response
	data := getMap(response, "data")
	if len(data) == 0 {
		return SystemStatus{
			Status: "error",
		}, nil
	}

	// Debug: log the raw data received
	log.Printf("Raw API data: total_files=%f, indexed_files=%f, pending_files=%f, failed_files=%f",
		getFloat(data, "total_files"), getFloat(data, "indexed_files"), getFloat(data, "pending_files"), getFloat(data, "failed_files"))
	
	// Get resource usage
	resourceUsage := getMap(data, "resource_usage")

	// Convert the response to our SystemStatus struct
	systemStatus := SystemStatus{
		Status:       "healthy", // Backend is responding with success
		Uptime:       int64(getFloat(data, "uptime") / 1000000000), // Convert nanoseconds to seconds
		TotalFiles:   int64(getFloat(data, "total_files")),
		IndexedFiles: int64(getFloat(data, "indexed_files")),
		PendingFiles: int64(getFloat(data, "pending_files")),
		FailedFiles:  int64(getFloat(data, "failed_files")),
		Database: map[string]interface{}{
			"connected": true, // If we got a response, database is connected
			"size":      int64(getFloat(data, "database_size")),
			"size_info": getMap(data, "database_size_info"),
		},
		Embeddings: map[string]interface{}{
			"available": true, // If we got a successful status response, embeddings are available
		},
		Indexing: map[string]interface{}{
			"active":       getBool(data, "indexing_active"),
			"paused":       getBool(data, "indexing_paused"),
			"total_files":  int64(getFloat(data, "total_files")),
			"indexed_files": int64(getFloat(data, "indexed_files")),
			"pending_files": int64(getFloat(data, "pending_files")),
			"failed_files":  int64(getFloat(data, "failed_files")),
		},
		Resources: map[string]interface{}{
			"cpu":    getFloat(resourceUsage, "cpu_percent"),
			"memory": getFloat(resourceUsage, "memory_percent"),
			"disk":   getFloat(resourceUsage, "disk_used_gb") / getFloat(resourceUsage, "disk_total_gb") * 100,
		},
	}

	// Debug: log what we're returning
	log.Printf("APIClient.GetSystemStatus returning: Status=%s, TotalFiles=%d, IndexedFiles=%d, PendingFiles=%d, FailedFiles=%d", 
		systemStatus.Status, systemStatus.TotalFiles, systemStatus.IndexedFiles, systemStatus.PendingFiles, systemStatus.FailedFiles)
	
	return systemStatus, nil
}

// GetConfig gets the configuration via the API
func (c *APIClient) GetConfig() (string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/config")
	if err != nil {
		// Return default config if API is not available - using database config field names
		return `{
			"success": true,
			"data": {
				"database_url": "postgresql://localhost/filesearch",
				"ollama_host": "http://localhost:11434",
				"embedding_model": "nomic-embed-text",
				"watch_paths": ["~/Documents", "~/Downloads"],
				"ignore_patterns": [".*", "~*", "*.tmp", "__pycache__", "node_modules", ".git", "*.log"]
			}
		}`, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	return string(body), nil
}

// UpdateConfig updates the configuration via the API
func (c *APIClient) UpdateConfig(configJSON string) error {
	req, err := http.NewRequest(
		"PUT",
		c.baseURL+"/api/v1/config",
		bytes.NewBuffer([]byte(configJSON)),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetFiles gets the list of files via the API
func (c *APIClient) GetFiles(limit, offset int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/files?limit=%d&offset=%d", c.baseURL, limit, offset)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		// Return empty list if API is not available
		return []map[string]interface{}{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []map[string]interface{}{}, nil
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []map[string]interface{}{}, fmt.Errorf("failed to decode response: %v", err)
	}

	// Check if the response is successful
	if success, ok := response["success"].(bool); !ok || !success {
		return []map[string]interface{}{}, nil
	}

	// Extract files from data field
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	files, ok := data["files"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	// Convert to []map[string]interface{}
	result := make([]map[string]interface{}, len(files))
	for i, file := range files {
		if fileMap, ok := file.(map[string]interface{}); ok {
			result[i] = fileMap
		}
	}

	return result, nil
}

// ResetDatabase resets the database via the API
func (c *APIClient) ResetDatabase() error {
	log.Println("APIClient.ResetDatabase: Starting database reset request")
	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/database/reset",
		"application/json",
		nil,
	)
	if err != nil {
		log.Printf("APIClient.ResetDatabase: API request failed: %v", err)
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("APIClient.ResetDatabase: API returned status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Println("APIClient.ResetDatabase: Database reset completed successfully")
	return nil
}

// Helper functions to safely extract values from map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return map[string]interface{}{}
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getIndexingState(data map[string]interface{}) string {
	if getBool(data, "active") {
		if getBool(data, "paused") {
			return "paused"
		}
		if getBool(data, "scanning") {
			return "scanning"
		}
		return "running"
	}
	return "idle"
}

// CallAPI makes a generic HTTP request to the backend API
func (c *APIClient) CallAPI(method, endpoint, body string) (string, error) {
	url := c.baseURL + endpoint
	log.Printf("Making %s request to: %s", method, url)

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	return string(responseBody), nil
}