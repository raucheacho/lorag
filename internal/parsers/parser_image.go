package parsers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageParser extracts text from images using Ollama Vision models
type ImageParser struct {
	ollamaURL string
	model     string
	timeout   time.Duration
}

// NewImageParser creates a new ImageParser with default config
func NewImageParser() *ImageParser {
	return NewImageParserWithConfig("http://localhost:11434", "llava:7b")
}

// NewImageParserWithConfig creates a new ImageParser with custom config
func NewImageParserWithConfig(ollamaURL, model string) *ImageParser {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llava:7b"
	}
	return &ImageParser{
		ollamaURL: ollamaURL,
		model:     model,
		timeout:   60 * time.Second,
	}
}

// ParseFile extracts text from an image using Ollama Vision
func (p *ImageParser) ParseFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat image: %w", err)
	}

	baseDesc := p.formatImageMetadata(path, info)

	// Check Ollama is available
	if !p.isOllamaAvailable() {
		return "", fmt.Errorf("OLLAMA_NOT_RUNNING: Ollama n'est pas accessible à %s. Veuillez démarrer Ollama avec 'ollama serve'", p.ollamaURL)
	}

	// Check vision model is installed
	if !p.isModelInstalled() {
		return "", fmt.Errorf("VISION_MODEL_NOT_INSTALLED: Le modèle vision '%s' n'est pas installé. Veuillez l'installer avec: ollama pull %s", p.model, p.model)
	}

	// Read image
	imageData, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// Extract with vision model
	text, err := p.extractWithVisionModel(imageData)
	if err != nil {
		return "", fmt.Errorf("VISION_EXTRACTION_FAILED: Échec de l'extraction avec %s: %w", p.model, err)
	}

	return baseDesc + "\n=== Vision AI (" + p.model + ") ===\n" + text, nil
}

// isOllamaAvailable checks if Ollama server is running
func (p *ImageParser) isOllamaAvailable() bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(p.ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// isModelInstalled checks if the vision model is installed
func (p *ImageParser) isModelInstalled() bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(p.ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	modelName := strings.Split(p.model, ":")[0]
	for _, model := range result.Models {
		if strings.HasPrefix(model.Name, p.model) || strings.HasPrefix(model.Name, modelName) {
			return true
		}
	}
	return false
}

// extractWithVisionModel uses Ollama vision model to describe image content
func (p *ImageParser) extractWithVisionModel(imageData []byte) (string, error) {
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	requestBody, _ := json.Marshal(map[string]interface{}{
		"model": p.model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": "Décris cette image en détail. Extrais tout le texte visible et décris le contenu visuel.",
				"images": []string{base64Image},
			},
		},
		"stream": false,
	})

	client := http.Client{Timeout: p.timeout}
	resp, err := client.Post(p.ollamaURL+"/api/chat", "application/json", strings.NewReader(string(requestBody)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return strings.TrimSpace(result.Message.Content), nil
}

// formatImageMetadata creates base description with image metadata
func (p *ImageParser) formatImageMetadata(path string, info os.FileInfo) string {
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".jpg", ".jpeg":
		format = "JPEG"
	case ".png":
		format = "PNG"
	case ".gif":
		format = "GIF"
	case ".webp":
		format = "WebP"
	case ".bmp":
		format = "BMP"
	case ".tiff", ".tif":
		format = "TIFF"
	default:
		format = "Image"
	}

	return fmt.Sprintf("=== %s Image ===\nFile: %s\nSize: %d bytes", format, filepath.Base(path), info.Size())
}

// Supports returns supported image extensions
func (p *ImageParser) Supports(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return true
	}
	return false
}
