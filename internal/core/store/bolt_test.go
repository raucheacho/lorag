package store

import (
	"os"
	"testing"
)

func TestBoltStore(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "lorag-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := NewBoltStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Test document operations
	t.Run("DocumentCRUD", func(t *testing.T) {
		// Create document
		doc := NewDocument("/path/to/file.txt", "file.txt", ".txt", 1024)
		doc.Checksum = "abc123"

		// Save
		if err := store.SaveDocument(doc); err != nil {
			t.Errorf("failed to save document: %v", err)
		}

		// Get by ID
		retrieved, err := store.GetDocument(doc.ID)
		if err != nil {
			t.Errorf("failed to get document: %v", err)
		}
		if retrieved.ID != doc.ID {
			t.Errorf("document ID mismatch: got %s, want %s", retrieved.ID, doc.ID)
		}

		// Get by path
		byPath, err := store.GetDocumentByPath(doc.Path)
		if err != nil {
			t.Errorf("failed to get document by path: %v", err)
		}
		if byPath.ID != doc.ID {
			t.Errorf("document by path ID mismatch: got %s, want %s", byPath.ID, doc.ID)
		}

		// List
		docs, err := store.ListDocuments(0, 10)
		if err != nil {
			t.Errorf("failed to list documents: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("expected 1 document, got %d", len(docs))
		}

		// Count
		count, err := store.CountDocuments()
		if err != nil {
			t.Errorf("failed to count documents: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}

		// Delete
		if err := store.DeleteDocument(doc.ID); err != nil {
			t.Errorf("failed to delete document: %v", err)
		}

		// Verify deleted
		_, err = store.GetDocument(doc.ID)
		if err == nil {
			t.Error("expected error for deleted document")
		}
	})

	// Test chunk operations
	t.Run("ChunkCRUD", func(t *testing.T) {
		doc := NewDocument("/path/to/file.txt", "file.txt", ".txt", 1024)
		store.SaveDocument(doc)

		// Create chunk
		chunk := NewChunk(doc.ID, "This is test content", 0, 0, 20)

		// Save
		if err := store.SaveChunk(chunk); err != nil {
			t.Errorf("failed to save chunk: %v", err)
		}

		// Get
		retrieved, err := store.GetChunk(chunk.ID)
		if err != nil {
			t.Errorf("failed to get chunk: %v", err)
		}
		if retrieved.ID != chunk.ID {
			t.Errorf("chunk ID mismatch: got %s, want %s", retrieved.ID, chunk.ID)
		}

		// Get by document
		chunks, err := store.GetChunksByDocument(doc.ID)
		if err != nil {
			t.Errorf("failed to get chunks by document: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}

		// Count
		count, err := store.CountChunks()
		if err != nil {
			t.Errorf("failed to count chunks: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}

		// Delete by document
		if err := store.DeleteChunksByDocument(doc.ID); err != nil {
			t.Errorf("failed to delete chunks by document: %v", err)
		}

		// Verify deleted
		_, err = store.GetChunk(chunk.ID)
		if err == nil {
			t.Error("expected error for deleted chunk")
		}
	})
}
