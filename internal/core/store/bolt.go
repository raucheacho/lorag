package store

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	goBolt "go.etcd.io/bbolt"
)

// BoltStore is a BoltDB-backed store
type BoltStore struct {
	db     *goBolt.DB
	path   string
}

const (
	// Buckets
	documentsBucket = "documents"
	chunksBucket    = "chunks"
	metadataBucket  = "metadata"
	watchesBucket   = "watches"
	logsBucket      = "logs"
	settingsBucket  = "settings"
	
	// Keys
	statsKey = "stats"
)

// NewBoltStore creates a new BoltDB store
func NewBoltStore(dataDir string) (*BoltStore, error) {
	path := filepath.Join(dataDir, "lorag.db")
	
	db, err := goBolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	store := &BoltStore{
		db:   db,
		path: path,
	}

	// Initialize buckets
	if err := store.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initBuckets creates all required buckets
func (s *BoltStore) initBuckets() error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		buckets := []string{documentsBucket, chunksBucket, metadataBucket, watchesBucket, logsBucket, settingsBucket}
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}
		return nil
	})
}

// Close closes the database
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// SaveDocument saves a document to the store
func (s *BoltStore) SaveDocument(doc *Document) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		
		data, err := doc.Marshal()
		if err != nil {
			return err
		}
		
		return b.Put([]byte(doc.ID), data)
	})
}

// GetDocument retrieves a document by ID
func (s *BoltStore) GetDocument(id string) (*Document, error) {
	var doc *Document
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("document not found: %s", id)
		}
		
		var err error
		doc, err = UnmarshalDocument(data)
		return err
	})
	
	return doc, err
}

// GetDocumentByPath retrieves a document by its file path
func (s *BoltStore) GetDocumentByPath(path string) (*Document, error) {
	var doc *Document
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		c := b.Cursor()
		
		for k, v := c.First(); k != nil; k, v = c.Next() {
			d, err := UnmarshalDocument(v)
			if err != nil {
				continue
			}
			if d.Path == path && !d.IsDeleted {
				doc = d
				return nil
			}
		}
		return fmt.Errorf("document not found: %s", path)
	})
	
	return doc, err
}

// DeleteDocument marks a document as deleted
func (s *BoltStore) DeleteDocument(id string) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("document not found: %s", id)
		}
		
		doc, err := UnmarshalDocument(data)
		if err != nil {
			return err
		}
		
		doc.MarkDeleted()
		
		newData, err := doc.Marshal()
		if err != nil {
			return err
		}
		
		return b.Put([]byte(id), newData)
	})
}

// ListDocuments lists all non-deleted documents
func (s *BoltStore) ListDocuments(offset, limit int) ([]*Document, error) {
	var docs []*Document
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		c := b.Cursor()
		
		count := 0
		skipped := 0
		
		for k, v := c.First(); k != nil; k, v = c.Next() {
			doc, err := UnmarshalDocument(v)
			if err != nil {
				continue
			}
			
			if doc.IsDeleted {
				continue
			}
			
			if skipped < offset {
				skipped++
				continue
			}
			
			docs = append(docs, doc)
			count++
			
			if limit > 0 && count >= limit {
				break
			}
		}
		return nil
	})
	
	return docs, err
}

// SaveChunk saves a chunk to the store
func (s *BoltStore) SaveChunk(chunk *Chunk) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(chunksBucket))
		
		data, err := chunk.Marshal()
		if err != nil {
			return err
		}
		
		return b.Put([]byte(chunk.ID), data)
	})
}

// GetChunk retrieves a chunk by ID
func (s *BoltStore) GetChunk(id string) (*Chunk, error) {
	var chunk *Chunk
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(chunksBucket))
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("chunk not found: %s", id)
		}
		
		var err error
		chunk, err = UnmarshalChunk(data)
		return err
	})
	
	return chunk, err
}

// GetChunksByDocument retrieves all chunks for a document
func (s *BoltStore) GetChunksByDocument(docID string) ([]*Chunk, error) {
	var chunks []*Chunk
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(chunksBucket))
		c := b.Cursor()
		
		for k, v := c.First(); k != nil; k, v = c.Next() {
			chunk, err := UnmarshalChunk(v)
			if err != nil {
				continue
			}
			if chunk.DocumentID == docID {
				chunks = append(chunks, chunk)
			}
		}
		return nil
	})
	
	return chunks, err
}

// DeleteChunksByDocument removes all chunks for a document
func (s *BoltStore) DeleteChunksByDocument(docID string) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(chunksBucket))
		c := b.Cursor()
		
		for k, v := c.First(); k != nil; k, v = c.Next() {
			chunk, err := UnmarshalChunk(v)
			if err != nil {
				continue
			}
			if chunk.DocumentID == docID {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// CountDocuments returns the total number of non-deleted documents
func (s *BoltStore) CountDocuments() (int, error) {
	var count int
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(documentsBucket))
		c := b.Cursor()
		
		for k, v := c.First(); k != nil; k, v = c.Next() {
			doc, err := UnmarshalDocument(v)
			if err != nil {
				continue
			}
			if !doc.IsDeleted {
				count++
			}
		}
		return nil
	})
	
	return count, err
}

// CountChunks returns the total number of chunks
func (s *BoltStore) CountChunks() (int, error) {
	var count int
	
	err := s.db.View(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(chunksBucket))
		count = b.Stats().KeyN
		return nil
	})
	
	return count, err
}

// itob converts int to bytes
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
