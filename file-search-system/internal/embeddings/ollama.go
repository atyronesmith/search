package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// OllamaClient handles communication with Ollama API
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
	log        *logrus.Logger
}

// OllamaConfig holds configuration for Ollama client
type OllamaConfig struct {
	Host    string        `json:"host"`
	Model   string        `json:"model"`
	Timeout time.Duration `json:"timeout"`
}

// EmbeddingRequest represents a request to Ollama embedding API
type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbeddingResponse represents a response from Ollama embedding API
type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// GenerateRequest represents a request to Ollama generate API
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateResponse represents a response from Ollama generate API
type GenerateResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

// ModelInfo represents information about an Ollama model
type ModelInfo struct {
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	Digest       string            `json:"digest"`
	ModifiedAt   time.Time         `json:"modified_at"`
	Details      ModelDetails      `json:"details"`
}

// ModelDetails represents details about an Ollama model
type ModelDetails struct {
	Format            string `json:"format"`
	Family            string `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string `json:"parameter_size"`
	QuantizationLevel string `json:"quantization_level"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(config *OllamaConfig, log *logrus.Logger) *OllamaClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	return &OllamaClient{
		baseURL: config.Host,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}
}

// Embed generates embeddings for the given text
func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error) {
	// Check for empty or whitespace-only text
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		// Return empty embedding vector instead of error for empty content
		// This allows processing to continue for files with no meaningful content
		return make([]float64, 0), nil
	}

	req := EmbeddingRequest{
		Model:  c.model,
		Prompt: trimmedText,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	c.log.WithFields(logrus.Fields{
		"text_length": len(text),
		"model":       c.model,
	}).Debug("Generating embedding")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if embeddingResp.Error != "" {
		return nil, fmt.Errorf("embedding API error: %s", embeddingResp.Error)
	}

	if len(embeddingResp.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return embeddingResp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts in batch
func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return make([][]float64, 0), nil
	}

	embeddings := make([][]float64, len(texts))
	errors := make([]error, len(texts))

	// Process embeddings concurrently with rate limiting
	semaphore := make(chan struct{}, 5) // Limit concurrent requests
	done := make(chan int, len(texts))

	for i, text := range texts {
		go func(index int, content string) {
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			embedding, err := c.Embed(ctx, content)
			embeddings[index] = embedding
			errors[index] = err
			done <- index
		}(i, text)
	}

	// Wait for all to complete
	for i := 0; i < len(texts); i++ {
		<-done
	}

	// Check for errors
	var firstError error
	successCount := 0
	for i, err := range errors {
		if err != nil {
			c.log.WithError(err).WithField("index", i).Error("Failed to generate embedding")
			if firstError == nil {
				firstError = err
			}
		} else {
			successCount++
		}
	}

	c.log.WithFields(logrus.Fields{
		"total":     len(texts),
		"success":   successCount,
		"failures":  len(texts) - successCount,
	}).Info("Batch embedding completed")

	// Return partial results if some succeeded
	if successCount > 0 {
		return embeddings, nil
	}

	return nil, firstError
}

// IsHealthy checks if Ollama service is healthy
func (c *OllamaClient) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ListModels returns available models
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response struct {
		Models []ModelInfo `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Models, nil
}

// PullModel downloads a model from Ollama registry
func (c *OllamaClient) PullModel(ctx context.Context, modelName string) error {
	req := struct {
		Name string `json:"name"`
	}{
		Name: modelName,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/pull", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetModelInfo returns information about a specific model
func (c *OllamaClient) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.Name == modelName {
			return &model, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", modelName)
}

// TestEmbedding tests the embedding functionality with a simple text
func (c *OllamaClient) TestEmbedding(ctx context.Context) error {
	testText := "This is a test sentence for embedding."
	
	c.log.WithField("model", c.model).Info("Testing embedding functionality")
	
	embedding, err := c.Embed(ctx, testText)
	if err != nil {
		return fmt.Errorf("embedding test failed: %w", err)
	}

	if len(embedding) == 0 {
		return fmt.Errorf("embedding test returned empty vector")
	}

	c.log.WithFields(logrus.Fields{
		"model":           c.model,
		"embedding_size":  len(embedding),
		"first_values":    embedding[:min(5, len(embedding))],
	}).Info("Embedding test successful")

	return nil
}

// GetEmbeddingDimension returns the dimension of embeddings for the current model
func (c *OllamaClient) GetEmbeddingDimension(ctx context.Context) (int, error) {
	// Test with a short text to get dimension
	embedding, err := c.Embed(ctx, "test")
	if err != nil {
		return 0, err
	}
	return len(embedding), nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EmbeddingService provides higher-level embedding operations
type EmbeddingService struct {
	client    *OllamaClient
	batchSize int
	log       *logrus.Logger
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(client *OllamaClient, batchSize int, log *logrus.Logger) *EmbeddingService {
	if batchSize <= 0 {
		batchSize = 32 // Default batch size
	}
	
	return &EmbeddingService{
		client:    client,
		batchSize: batchSize,
		log:       log,
	}
}

// ProcessTexts processes multiple texts in batches
func (s *EmbeddingService) ProcessTexts(ctx context.Context, texts []string, callback func(index int, embedding []float64, err error)) error {
	if len(texts) == 0 {
		return nil
	}

	s.log.WithFields(logrus.Fields{
		"total_texts": len(texts),
		"batch_size":  s.batchSize,
	}).Info("Starting batch processing")

	for i := 0; i < len(texts); i += s.batchSize {
		end := i + s.batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := s.client.EmbedBatch(ctx, batch)
		
		if err != nil {
			// Handle partial failures
			for j := range batch {
				callback(i+j, nil, err)
			}
			continue
		}

		// Process successful embeddings
		for j, embedding := range embeddings {
			callback(i+j, embedding, nil)
		}
	}

	s.log.Info("Batch processing completed")
	return nil
}