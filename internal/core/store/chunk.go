package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Chunk represents a text chunk from a document
type Chunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding,omitempty"`
	Position   int       `json:"position"`
	StartByte  int       `json:"start_byte"`
	EndByte    int       `json:"end_byte"`
	Tokens     int       `json:"tokens"`
	CreatedAt  time.Time `json:"created_at"`
}

// NewChunk creates a new chunk
func NewChunk(docID, content string, position, startByte, endByte int) *Chunk {
	return &Chunk{
		ID:         uuid.New().String(),
		DocumentID: docID,
		Content:    content,
		Position:   position,
		StartByte:  startByte,
		EndByte:    endByte,
		CreatedAt:  time.Now().UTC(),
	}
}

// SetEmbedding sets the embedding vector
func (c *Chunk) SetEmbedding(embedding []float32) {
	c.Embedding = make([]float32, len(embedding))
	copy(c.Embedding, embedding)
}

// Marshal serializes the chunk to JSON
func (c *Chunk) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalChunk deserializes JSON to chunk
func UnmarshalChunk(data []byte) (*Chunk, error) {
	var c Chunk
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
	}
	return &c, nil
}
