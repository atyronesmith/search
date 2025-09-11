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
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Ignore close error for HTTP response body
			_ = err
		}
	}()

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

