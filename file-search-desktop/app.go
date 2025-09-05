package main

import (
	"context"
	"log"
	"os"
	"time"
)

// App struct
type App struct {
	ctx       context.Context
	apiClient *APIClient
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Size         int64     `json:"size"`
	ModifiedAt   string    `json:"modifiedAt"`
	Score        float64   `json:"score"`
	Highlights   []string  `json:"highlights"`
	Snippet      string    `json:"snippet"`
	TotalResults int       `json:"totalResults"`
}

// IndexingStatus represents indexing status
type IndexingStatus struct {
	State           string `json:"state"`
	FilesProcessed  int    `json:"filesProcessed"`
	TotalFiles      int    `json:"totalFiles"`
	PendingFiles    int    `json:"pendingFiles"`
	CurrentFile     string `json:"currentFile"`
	Errors          int    `json:"errors"`
	ElapsedTime     int64  `json:"elapsedTime"`
}

// SystemStatus represents system status
type SystemStatus struct {
	Status         string                 `json:"status"`
	Uptime         int64                  `json:"uptime"`
	TotalFiles     int64                  `json:"total_files"`
	IndexedFiles   int64                  `json:"indexed_files"`
	PendingFiles   int64                  `json:"pending_files"`
	FailedFiles    int64                  `json:"failed_files"`
	Database       map[string]interface{} `json:"database"`
	Embeddings     map[string]interface{} `json:"embeddings"`
	Indexing       map[string]interface{} `json:"indexing"`
	Resources      map[string]interface{} `json:"resources"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	// Get backend URL from environment or use default
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	return &App{
		apiClient: NewAPIClient(backendURL),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("File Search Desktop app initialized")
	log.Printf("Connecting to backend at: %s", a.apiClient.baseURL)
	
	// Test backend connection
	status, err := a.apiClient.GetSystemStatus()
	if err != nil {
		log.Printf("Warning: Backend connection failed: %v", err)
		log.Println("The app will work with limited functionality until the backend is available")
	} else {
		log.Printf("Backend connected successfully. Status: %s", status.Status)
	}
}

// Search performs a search operation via the backend API
func (a *App) Search(request SearchRequest) ([]SearchResult, error) {
	log.Printf("Search request received: %+v", request)
	
	// Try to search via API
	log.Printf("Calling API client search...")
	results, err := a.apiClient.Search(request)
	if err != nil {
		log.Printf("API search failed: %v, using demo data", err)
		
		// Fallback to demo data if API is not available
		return []SearchResult{
			{
				ID:         "demo-1",
				Path:       "/demo/no-backend-connected.txt",
				Name:       "no-backend-connected.txt",
				Type:       "text",
				Size:       1024,
				ModifiedAt: time.Now().Format(time.RFC3339),
				Score:      0.95,
				Highlights: []string{"Backend server is not running"},
				Snippet:    "Please start the backend server on port 8080 to enable real search",
			},
			{
				ID:         "demo-2",
				Path:       "/demo/start-backend.md",
				Name:       "start-backend.md",
				Type:       "markdown",
				Size:       2048,
				ModifiedAt: time.Now().Format(time.RFC3339),
				Score:      0.87,
				Highlights: []string{"cd file-search-system && go run cmd/server/main.go"},
				Snippet:    "Run the backend server from the file-search-system directory",
			},
		}, nil
	}

	log.Printf("API search successful, returning %d results", len(results))
	return results, nil
}

// StartIndexing starts the indexing process via the API
func (a *App) StartIndexing(path string) error {
	log.Printf("Starting indexing for path: %s", path)
	
	err := a.apiClient.StartIndexing(path)
	if err != nil {
		log.Printf("Failed to start indexing: %v", err)
		return err
	}
	
	return nil
}

// StopIndexing stops the indexing process via the API
func (a *App) StopIndexing() error {
	log.Println("Stopping indexing")
	
	err := a.apiClient.StopIndexing()
	if err != nil {
		log.Printf("Failed to stop indexing: %v", err)
		return err
	}
	
	return nil
}

// PauseIndexing pauses the indexing process via the API
func (a *App) PauseIndexing() error {
	log.Println("Pausing indexing")
	
	err := a.apiClient.PauseIndexing()
	if err != nil {
		log.Printf("Failed to pause indexing: %v", err)
		return err
	}
	
	return nil
}

// ResumeIndexing resumes the indexing process via the API
func (a *App) ResumeIndexing() error {
	log.Println("Resuming indexing")
	
	err := a.apiClient.ResumeIndexing()
	if err != nil {
		log.Printf("Failed to resume indexing: %v", err)
		return err
	}
	
	return nil
}

// GetIndexingStatus returns the current indexing status from the API
func (a *App) GetIndexingStatus() (IndexingStatus, error) {
	status, err := a.apiClient.GetIndexingStatus()
	if err != nil {
		log.Printf("Failed to get indexing status: %v", err)
		// Return a default status
		return IndexingStatus{
			State:          "unknown",
			FilesProcessed: 0,
			TotalFiles:     0,
			CurrentFile:    "",
			Errors:         0,
			ElapsedTime:    0,
		}, nil
	}
	
	return status, nil
}

// GetSystemStatus returns the current system status from the API
func (a *App) GetSystemStatus() (SystemStatus, error) {
	status, err := a.apiClient.GetSystemStatus()
	if err != nil {
		log.Printf("Failed to get system status: %v", err)
	}
	
	log.Printf("GetSystemStatus returning: Status=%s, TotalFiles=%d, IndexedFiles=%d, PendingFiles=%d, FailedFiles=%d", 
		status.Status, status.TotalFiles, status.IndexedFiles, status.PendingFiles, status.FailedFiles)
	
	return status, nil
}

// GetConfig returns the current configuration from the API
func (a *App) GetConfig() (string, error) {
	config, err := a.apiClient.GetConfig()
	if err != nil {
		log.Printf("Failed to get config: %v", err)
	}
	
	return config, nil
}

// UpdateConfig updates the configuration via the API
func (a *App) UpdateConfig(configJSON string) error {
	log.Printf("Updating config")
	
	err := a.apiClient.UpdateConfig(configJSON)
	if err != nil {
		log.Printf("Failed to update config: %v", err)
		return err
	}
	
	return nil
}

// GetFiles returns a list of indexed files from the API
func (a *App) GetFiles(limit, offset int) ([]map[string]interface{}, error) {
	files, err := a.apiClient.GetFiles(limit, offset)
	if err != nil {
		log.Printf("Failed to get files: %v", err)
		// Return empty list if API fails
		return []map[string]interface{}{}, nil
	}
	
	return files, nil
}

// ResetDatabase resets the database via the API
func (a *App) ResetDatabase() error {
	log.Println("Resetting database")
	
	err := a.apiClient.ResetDatabase()
	if err != nil {
		log.Printf("Failed to reset database: %v", err)
		return err
	}
	
	log.Println("Database reset successfully")
	return nil
}