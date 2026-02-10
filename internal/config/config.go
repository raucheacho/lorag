package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v10"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	ServerHost string `env:"LORAG_HOST" envDefault:"localhost"`
	ServerPort int    `env:"LORAG_PORT" envDefault:"8080"`

	// Data settings
	DataDir string `env:"LORAG_DATA_DIR" envDefault:"./data"`

	// Embedding settings
	EmbeddingProvider string  `env:"LORAG_EMBEDDING_PROVIDER" envDefault:"ollama"`
	EmbeddingModel    string  `env:"LORAG_EMBEDDING_MODEL" envDefault:"nomic-embed-text"`
	EmbeddingURL      string  `env:"LORAG_EMBEDDING_URL" envDefault:"http://localhost:11434"`
	EmbeddingDim      int     `env:"LORAG_EMBEDDING_DIM" envDefault:"768"`

	// Vector settings
	VectorIndexType string  `env:"LORAG_VECTOR_INDEX" envDefault:"hnsw"`
	VectorM         int     `env:"LORAG_VECTOR_M" envDefault:"16"`
	VectorEF        int     `env:"LORAG_VECTOR_EF" envDefault:"200"`

	// Indexing settings
	MaxChunks       int     `env:"LORAG_MAX_CHUNKS" envDefault:"50000"`
	ChunkSize       int     `env:"LORAG_CHUNK_SIZE" envDefault:"512"`
	ChunkOverlap    int     `env:"LORAG_CHUNK_OVERLAP" envDefault:"50"`
	IndexWorkers    int     `env:"LORAG_INDEX_WORKERS" envDefault:"3"`      // Concurrent indexing workers
	MaxQueueSize    int     `env:"LORAG_MAX_QUEUE_SIZE" envDefault:"100"` // Max queue size

	// OCR settings
	OCREnabled      bool    `env:"LORAG_OCR_ENABLED" envDefault:"false"`

	// Security settings
	EncryptionEnabled bool `env:"LORAG_ENCRYPTION" envDefault:"false"`
	EncryptionKey     string `env:"LORAG_ENCRYPTION_KEY" envDefault:""`

	// License
	LicenseKey      string  `env:"LORAG_LICENSE_KEY" envDefault:""`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return cfg, nil
}

// DatabasePath returns the path to the BoltDB database
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "lorag.db")
}

// VectorIndexPath returns the path to the vector index
func (c *Config) VectorIndexPath() string {
	return filepath.Join(c.DataDir, "vector.idx")
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
