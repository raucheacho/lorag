package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v10"
)

// Config holds the process configuration read from the environment.
// Search and indexing parameters live in the persisted settings (Studio →
// Settings); the LORAG_EMBEDDING_* variables only override them when set.
type Config struct {
	ServerHost string `env:"LORAG_HOST" envDefault:"localhost"`
	ServerPort int    `env:"LORAG_PORT" envDefault:"8080"`
	DataDir    string `env:"LORAG_DATA_DIR" envDefault:"./data"`

	EmbeddingProvider string `env:"LORAG_EMBEDDING_PROVIDER"`
	EmbeddingModel    string `env:"LORAG_EMBEDDING_MODEL"`
	EmbeddingURL      string `env:"LORAG_EMBEDDING_URL"`
	EmbeddingDim      int    `env:"LORAG_EMBEDDING_DIM"`
	EmbeddingAPIKey   string `env:"LORAG_EMBEDDING_API_KEY"`

	MaxChunks    int `env:"LORAG_MAX_CHUNKS" envDefault:"50000"`
	MaxQueueSize int `env:"LORAG_MAX_QUEUE_SIZE" envDefault:"100"`

	LicenseKey string `env:"LORAG_LICENSE_KEY"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	return cfg, nil
}

// IndexDir returns the VectixDB database holding the search index
func (c *Config) IndexDir() string {
	return filepath.Join(c.DataDir, "vectix")
}

// GetAddress returns the full server address
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
}

// IsFreeVersion returns true if running free version
func (c *Config) IsFreeVersion() bool {
	return c.LicenseKey == ""
}

// MaxChunksAllowed returns max chunks based on license
func (c *Config) MaxChunksAllowed() int {
	if c.IsFreeVersion() {
		return 5000
	}
	return c.MaxChunks
}
