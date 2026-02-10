package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Document represents a file/document in the system
type Document struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Filename    string            `json:"filename"`
	Extension   string            `json:"extension"`
	Size        int64             `json:"size"`
	MimeType    string            `json:"mime_type"`
	Checksum    string            `json:"checksum"`
	Content     string            `json:"content,omitempty"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	IndexedAt   *time.Time        `json:"indexed_at,omitempty"`
	ChunkCount  int               `json:"chunk_count"`
	Tags        []string          `json:"tags"`
	IsDeleted   bool              `json:"is_deleted"`
}

// NewDocument creates a new document
func NewDocument(path, filename, ext string, size int64) *Document {
	now := time.Now().UTC()
	return &Document{
		ID:        uuid.New().String(),
		Path:      path,
		Filename:  filename,
		Extension: ext,
		Size:      size,
		Metadata:  make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      []string{},
	}
}

// Marshal serializes the document to JSON
func (d *Document) Marshal() ([]byte, error) {
	return json.Marshal(d)
}

// UnmarshalDocument deserializes JSON to document
func UnmarshalDocument(data []byte) (*Document, error) {
	var d Document
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}
	return &d, nil
}

// MarkIndexed marks the document as indexed
func (d *Document) MarkIndexed(chunkCount int) {
	now := time.Now().UTC()
	d.IndexedAt = &now
	d.ChunkCount = chunkCount
	d.UpdatedAt = now
}

// MarkDeleted marks the document as deleted
func (d *Document) MarkDeleted() {
	d.IsDeleted = true
	d.UpdatedAt = time.Now().UTC()
}

// AddTag adds a tag to the document
func (d *Document) AddTag(tag string) {
	for _, t := range d.Tags {
		if t == tag {
			return
		}
	}
	d.Tags = append(d.Tags, tag)
}

// SetMetadata sets a metadata key-value pair
func (d *Document) SetMetadata(key, value string) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]string)
	}
	d.Metadata[key] = value
}
