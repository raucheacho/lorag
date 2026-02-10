package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Provider is the interface for embedding providers
type Provider interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
	Dimension() int
}

// ProviderType represents the type of embedding provider
type ProviderType string

const (
	ProviderOllama   ProviderType = "ollama"
	ProviderLocal    ProviderType = "local"
	ProviderOpenAI   ProviderType = "openai"
	ProviderCustom   ProviderType = "custom"
)

// Config holds embedding provider configuration
type Config struct {
	Type      ProviderType
	Model     string
	URL       string
	APIKey    string
	Dimension int
	Timeout   time.Duration
}

// NewProvider creates a new embedding provider
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Type {
	case ProviderOllama:
		return NewOllamaProvider(cfg), nil
	case ProviderLocal:
		return NewLocalProvider(cfg)
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

// OllamaProvider implements Ollama embedding API
type OllamaProvider struct {
	config Config
	client *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(cfg Config) *OllamaProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	
	return &OllamaProvider{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// ollamaRequest represents the request to Ollama
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaResponse represents the response from Ollama
type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates embedding for a single text
func (o *OllamaProvider) Embed(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embeddings", o.config.URL)
	
	reqBody := ollamaRequest{
		Model:  o.config.Model,
		Prompt: text,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := o.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	
	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (o *OllamaProvider) EmbedBatch(texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	
	for i, text := range texts {
		embedding, err := o.Embed(text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		results[i] = embedding
	}
	
	return results, nil
}

// Dimension returns the embedding dimension
func (o *OllamaProvider) Dimension() int {
	return o.config.Dimension
}

// OpenAIProvider implements OpenAI embedding API
type OpenAIProvider struct {
	config Config
	client *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg Config) *OpenAIProvider {
	if cfg.URL == "" {
		cfg.URL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	
	return &OpenAIProvider{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// openAIRequest represents the request to OpenAI
type openAIRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// openAIResponse represents the response from OpenAI
type openAIResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed generates embedding for a single text
func (o *OpenAIProvider) Embed(text string) ([]float32, error) {
	embeddings, err := o.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (o *OpenAIProvider) EmbedBatch(texts []string) ([][]float32, error) {
	url := fmt.Sprintf("%s/embeddings", o.config.URL)
	
	reqBody := openAIRequest{
		Model: o.config.Model,
		Input: texts,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.config.APIKey)
	
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call openai: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}
	
	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Sort by index
	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}
	
	return embeddings, nil
}

// Dimension returns the embedding dimension
func (o *OpenAIProvider) Dimension() int {
	return o.config.Dimension
}

// LocalProvider implements local ONNX/GGML embedding inference
type LocalProvider struct {
	config Config
}

// NewLocalProvider creates a new local provider
func NewLocalProvider(cfg Config) (*LocalProvider, error) {
	// TODO: Load ONNX model or GGML model
	return &LocalProvider{config: cfg}, nil
}

// Embed generates embedding locally
func (l *LocalProvider) Embed(text string) ([]float32, error) {
	// TODO: Implement local inference
	return nil, fmt.Errorf("local provider not yet implemented")
}

// EmbedBatch generates embeddings locally
func (l *LocalProvider) EmbedBatch(texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	
	for i, text := range texts {
		embedding, err := l.Embed(text)
		if err != nil {
			return nil, err
		}
		results[i] = embedding
	}
	
	return results, nil
}

// Dimension returns the embedding dimension
func (l *LocalProvider) Dimension() int {
	return l.config.Dimension
}
