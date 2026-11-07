package store

import (
	"encoding/json"
	"fmt"

	goBolt "go.etcd.io/bbolt"
)

// SettingsStore manages persistent settings
type SettingsStore struct {
	store *BoltStore
}

// NewSettingsStore creates a new settings store
func NewSettingsStore(store *BoltStore) *SettingsStore {
	return &SettingsStore{store: store}
}

// Get retrieves a setting value
func (s *SettingsStore) Get(key string) (string, error) {
	var value string
	err := s.store.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(settingsBucket))
		if b == nil {
			return fmt.Errorf("settings bucket not found")
		}
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("setting not found: %s", key)
		}
		value = string(v)
		return nil
	})
	return value, err
}

// GetJSON retrieves a setting as JSON
func (s *SettingsStore) GetJSON(key string, out interface{}) error {
	data, err := s.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), out)
}

// Set saves a setting value
func (s *SettingsStore) Set(key string, value string) error {
	return s.store.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(settingsBucket))
		if b == nil {
			var err error
			b, err = tx.CreateBucketIfNotExists([]byte(settingsBucket))
			if err != nil {
				return err
			}
		}
		return b.Put([]byte(key), []byte(value))
	})
}

// SetJSON saves a value as JSON
func (s *SettingsStore) SetJSON(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.Set(key, string(data))
}

// Delete removes a setting
func (s *SettingsStore) Delete(key string) error {
	return s.store.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(settingsBucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// GetAll retrieves all settings as a map
func (s *SettingsStore) GetAll() (map[string]string, error) {
	settings := make(map[string]string)
	err := s.store.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(settingsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			settings[string(k)] = string(v)
			return nil
		})
	})
	return settings, err
}

// AppSettings represents the application settings
type AppSettings struct {
	// Server
	Host string `json:"host"`
	Port int    `json:"port"`

	// Embeddings
	EmbeddingProvider string `json:"embedding_provider"`
	EmbeddingModel    string `json:"embedding_model"`
	EmbeddingURL      string `json:"embedding_url"`
	EmbeddingDim      int    `json:"embedding_dim"`

	// Vector index (VectixDB HNSW)
	VectorMetric    string `json:"vector_metric"` // cosine, l2, dot
	VectorQuantized bool   `json:"vector_quantized"`
	VectorM         int    `json:"vector_m"`
	VectorEf        int    `json:"vector_ef"`
	VectorEfSearch  int    `json:"vector_ef_search"`

	// Keyword index (VectixDB BM25)
	BM25K1 float32 `json:"bm25_k1"`
	BM25B  float32 `json:"bm25_b"`

	// Indexing
	MaxChunks    int `json:"max_chunks"`
	ChunkSize    int `json:"chunk_size"`
	ChunkOverlap int `json:"chunk_overlap"`
	IndexWorkers int `json:"index_workers"`

	// MCP output template
	DefaultTemplate string `json:"default_template"`

	// Memory recall thresholds (0: calibrated defaults for the embedding model)
	MemoryMinSimilarity     float64 `json:"memory_min_similarity"`
	MemoryMinSimilarityText float64 `json:"memory_min_similarity_text"`

	// OCR & Vision
	OcrEnabled           bool   `json:"ocr_enabled"`
	OcrModel             string `json:"ocr_model"`
	TranscriptionEnabled bool   `json:"transcription_enabled"`
	WhisperModel         string `json:"whisper_model"` // tiny, base, small
}

// DefaultSettings returns default settings
func DefaultSettings() *AppSettings {
	return &AppSettings{
		Host:                 "localhost",
		Port:                 8080,
		EmbeddingProvider:    "ollama",
		EmbeddingModel:       "bge-m3", // multilingual (French, Arabic…); see docs/MEMORY.md
		EmbeddingURL:         "http://localhost:11434",
		EmbeddingDim:         1024,
		VectorMetric:         "cosine",
		VectorM:              16,
		VectorEf:             200,
		VectorEfSearch:       64,
		BM25K1:               1.2,
		BM25B:                0.75,
		MaxChunks:            50000,
		ChunkSize:            512,
		ChunkOverlap:         50,
		IndexWorkers:         3,
		DefaultTemplate:      "default",
		OcrModel:             "llava:7b",
		TranscriptionEnabled: true, // fails with a clear message when ffmpeg or whisper-cli is missing
		WhisperModel:         "tiny",
	}
}

// GetAppSettings retrieves all app settings
func (s *SettingsStore) GetAppSettings() (*AppSettings, error) {
	settings := DefaultSettings()
	
	data, err := s.Get("app_settings")
	if err != nil {
		return settings, nil // Return defaults if not found
	}
	
	if err := json.Unmarshal([]byte(data), settings); err != nil {
		return settings, err
	}
	
	return settings, nil
}

// SaveAppSettings saves all app settings
func (s *SettingsStore) SaveAppSettings(settings *AppSettings) error {
	return s.SetJSON("app_settings", settings)
}

// WatchSettings represents watched folders
type WatchSettings struct {
	Watches []WatchEntry `json:"watches"`
}

// WatchEntry represents a watched folder
type WatchEntry struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns,omitempty"`
}

// GetWatches retrieves watched folders
func (s *SettingsStore) GetWatches() ([]WatchEntry, error) {
	var watches []WatchEntry
	if err := s.GetJSON("watches", &watches); err != nil {
		return []WatchEntry{}, nil
	}
	return watches, nil
}

// SaveWatches saves watched folders
func (s *SettingsStore) SaveWatches(watches []WatchEntry) error {
	return s.SetJSON("watches", watches)
}
