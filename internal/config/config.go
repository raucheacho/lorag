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
	DataDir    string `env:"LORAG_DATA_DIR"`

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
	return cfg, nil
}

// Home is the per-user directory of Lorag: ~/.lorag. It holds the data
// directory and the downloaded Whisper models.
func Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lorag"
	}
	return filepath.Join(home, ".lorag")
}

// DefaultDataDir is the data directory used when neither LORAG_DATA_DIR nor
// --data-dir is given.
func DefaultDataDir() string {
	return filepath.Join(Home(), "data")
}

// ResolveDataDir picks the data directory, makes it absolute and creates it.
// Earlier versions defaulted to ./data, relative to where the server was
// started: such a directory is kept, with a notice, as long as the new
// default holds no database, so an upgrade never starts from an empty store.
func (c *Config) ResolveDataDir() (notice string, err error) {
	if c.DataDir == "" {
		c.DataDir = DefaultDataDir()
		if exists(filepath.Join("data", "lorag.db")) && !exists(filepath.Join(c.DataDir, "lorag.db")) {
			c.DataDir = "data"
			notice = fmt.Sprintf("using ./data from an earlier version; move it to %s or pass --data-dir", DefaultDataDir())
		}
	}
	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return "", err
	}
	c.DataDir = abs
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	return notice, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
