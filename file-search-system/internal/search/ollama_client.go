package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient provides interface to Ollama API
type OllamaClient struct {
	baseURL string
	client  *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateRequest represents a request to Ollama's generate API
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateResponse represents Ollama's generate API response
type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate sends a prompt to Ollama and returns the response
func (c *OllamaClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	req := GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Ignore close error for HTTP response body
			_ = err
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return genResp.Response, nil
}

// Health checks if Ollama is available
func (c *OllamaClient) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Ignore close error for HTTP response body
			_ = err
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama not healthy (status %d)", resp.StatusCode)
	}

	return nil
}

// ListModels returns available models
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Ignore close error for HTTP response body
			_ = err
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var models []string
	for _, model := range response.Models {
		models = append(models, model.Name)
	}

	return models, nil
}

// PullModel downloads a model if it doesn't exist
func (c *OllamaClient) PullModel(ctx context.Context, modelName string) error {
	req := struct {
		Name string `json:"name"`
	}{
		Name: modelName,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal pull request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/pull", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to pull model: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Ignore close error for HTTP response body
			_ = err
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull model (status %d): %s", resp.StatusCode, string(body))
	}

	// The pull endpoint streams responses, we need to read until done
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var pullResp struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}

		if err := decoder.Decode(&pullResp); err != nil {
			return fmt.Errorf("failed to decode pull response: %w", err)
		}

		if pullResp.Error != "" {
			return fmt.Errorf("model pull error: %s", pullResp.Error)
		}
	}

	return nil
}

// EnsureModel checks if a model exists and pulls it if not
func (c *OllamaClient) EnsureModel(ctx context.Context, modelName string) error {
	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	// Check if model already exists
	for _, model := range models {
		if model == modelName || model == modelName+":latest" {
			return nil // Model exists
		}
	}

	// Model doesn't exist, pull it
	return c.PullModel(ctx, modelName)
}
