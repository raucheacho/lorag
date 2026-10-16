package store

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goBolt "go.etcd.io/bbolt"
)

// BoltStore is a BoltDB-backed store
type BoltStore struct {
	db   *goBolt.DB
	path string
}

const (
	documentsBucket = "documents"
	chunksBucket    = "chunks"
	settingsBucket  = "settings"

	// Secondary indexes.
	chunksByDocBucket = "chunks_by_doc" // docID \x00 chunkID -> nil
	docsByPathBucket  = "docs_by_path"  // path -> docID
)

// NewBoltStore creates a new BoltDB store
func NewBoltStore(dataDir string) (*BoltStore, error) {
	path := filepath.Join(dataDir, "lorag.db")

	db, err := goBolt.Open(path, 0600, &goBolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open %s (is another lorag running?): %w", path, err)
	}

	store := &BoltStore{
		db:   db,
		path: path,
	}

	if err := store.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Path returns the database file path.
func (s *BoltStore) Path() string { return s.path }

// initBuckets creates all required buckets
func (s *BoltStore) initBuckets() error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		buckets := []string{documentsBucket, chunksBucket, settingsBucket, chunksByDocBucket, docsByPathBucket}
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}
		return nil
	})
}

// migrate builds the secondary indexes for databases created before they
// existed, and purges soft-deleted documents left by older versions.
func (s *BoltStore) migrate() error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		docs := tx.Bucket([]byte(documentsBucket))
		chunks := tx.Bucket([]byte(chunksBucket))
		byDoc := tx.Bucket([]byte(chunksByDocBucket))
		byPath := tx.Bucket([]byte(docsByPathBucket))

		if byPath.Stats().KeyN == 0 && docs.Stats().KeyN > 0 {
			var dead [][]byte
			err := docs.ForEach(func(k, v []byte) error {
				d, err := UnmarshalDocument(v)
				if err != nil {
					return nil
				}
				if d.IsDeleted {
					dead = append(dead, append([]byte(nil), k...))
					return nil
				}
				return byPath.Put([]byte(d.Path), []byte(d.ID))
			})
			if err != nil {
				return err
			}
			for _, k := range dead {
				if err := docs.Delete(k); err != nil {
					return err
				}
			}
		}

		if byDoc.Stats().KeyN == 0 && chunks.Stats().KeyN > 0 {
			return chunks.ForEach(func(k, v []byte) error {
				c, err := UnmarshalChunk(v)
				if err != nil {
					return nil
				}
				return byDoc.Put(chunkKey(c.DocumentID, c.ID), nil)
			})
		}
		return nil
	})
}

func chunkKey(docID, chunkID string) []byte {
	return []byte(docID + "\x00" + chunkID)
}

// Close closes the database
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// SaveDocument saves a document. The extracted text is not persisted: it
// lives in the chunks.
func (s *BoltStore) SaveDocument(doc *Document) error {
	stored := *doc
	stored.Content = ""
	data, err := stored.Marshal()
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *goBolt.Tx) error {
		if err := tx.Bucket([]byte(documentsBucket)).Put([]byte(doc.ID), data); err != nil {
			return err
		}
		return tx.Bucket([]byte(docsByPathBucket)).Put([]byte(doc.Path), []byte(doc.ID))
	})
}

func getDocument(tx *goBolt.Tx, id string) (*Document, error) {
	data := tx.Bucket([]byte(documentsBucket)).Get([]byte(id))
	if data == nil {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	doc, err := UnmarshalDocument(data)
	if err != nil {
		return nil, err
	}
	if doc.IsDeleted {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return doc, nil
}

// GetDocument retrieves a document by ID
func (s *BoltStore) GetDocument(id string) (*Document, error) {
	var doc *Document
	err := s.db.View(func(tx *goBolt.Tx) error {
		var err error
		doc, err = getDocument(tx, id)
		return err
	})
	return doc, err
}

// GetDocumentByPath retrieves a document by its file path
func (s *BoltStore) GetDocumentByPath(path string) (*Document, error) {
	var doc *Document
	err := s.db.View(func(tx *goBolt.Tx) error {
		id := tx.Bucket([]byte(docsByPathBucket)).Get([]byte(path))
		if id == nil {
			return fmt.Errorf("document not found: %s", path)
		}
		var err error
		doc, err = getDocument(tx, string(id))
		return err
	})
	return doc, err
}

// DeleteDocument removes a document, its chunks and its index entries.
func (s *BoltStore) DeleteDocument(id string) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		docs := tx.Bucket([]byte(documentsBucket))
		data := docs.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("document not found: %s", id)
		}
		if doc, err := UnmarshalDocument(data); err == nil {
			byPath := tx.Bucket([]byte(docsByPathBucket))
			if string(byPath.Get([]byte(doc.Path))) == id {
				if err := byPath.Delete([]byte(doc.Path)); err != nil {
					return err
				}
			}
		}
		if err := deleteChunks(tx, id); err != nil {
			return err
		}
		return docs.Delete([]byte(id))
	})
}

// ListDocuments lists documents in storage order.
func (s *BoltStore) ListDocuments(offset, limit int) ([]*Document, error) {
	docs, _, err := s.QueryDocuments(DocumentQuery{Offset: offset, Limit: limit})
	return docs, err
}

// DocumentQuery filters and paginates documents.
type DocumentQuery struct {
	Offset int
	Limit  int
	// Search matches filename and path, case-insensitive.
	Search string
	// Extension matches exactly, e.g. ".md".
	Extension string
	// Sort is one of "updated" (default, newest first), "name", "size", "chunks".
	Sort string
}

// QueryDocuments returns one page of matching documents and the total count.
func (s *BoltStore) QueryDocuments(q DocumentQuery) ([]*Document, int, error) {
	var all []*Document
	needle := strings.ToLower(q.Search)
	err := s.db.View(func(tx *goBolt.Tx) error {
		return tx.Bucket([]byte(documentsBucket)).ForEach(func(_, v []byte) error {
			doc, err := UnmarshalDocument(v)
			if err != nil || doc.IsDeleted {
				return nil
			}
			if q.Extension != "" && doc.Extension != q.Extension {
				return nil
			}
			if needle != "" && !strings.Contains(strings.ToLower(doc.Path), needle) {
				return nil
			}
			all = append(all, doc)
			return nil
		})
	})
	if err != nil {
		return nil, 0, err
	}

	switch q.Sort {
	case "name":
		sort.Slice(all, func(i, j int) bool { return strings.ToLower(all[i].Filename) < strings.ToLower(all[j].Filename) })
	case "size":
		sort.Slice(all, func(i, j int) bool { return all[i].Size > all[j].Size })
	case "chunks":
		sort.Slice(all, func(i, j int) bool { return all[i].ChunkCount > all[j].ChunkCount })
	default:
		sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt.After(all[j].UpdatedAt) })
	}

	total := len(all)
	if q.Offset >= total {
		return []*Document{}, total, nil
	}
	all = all[q.Offset:]
	if q.Limit > 0 && len(all) > q.Limit {
		all = all[:q.Limit]
	}
	return all, total, nil
}

// SaveChunk saves a chunk to the store
func (s *BoltStore) SaveChunk(chunk *Chunk) error {
	data, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *goBolt.Tx) error {
		if err := tx.Bucket([]byte(chunksBucket)).Put([]byte(chunk.ID), data); err != nil {
			return err
		}
		return tx.Bucket([]byte(chunksByDocBucket)).Put(chunkKey(chunk.DocumentID, chunk.ID), nil)
	})
}

// GetChunk retrieves a chunk by ID
func (s *BoltStore) GetChunk(id string) (*Chunk, error) {
	var chunk *Chunk
	err := s.db.View(func(tx *goBolt.Tx) error {
		data := tx.Bucket([]byte(chunksBucket)).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("chunk not found: %s", id)
		}
		var err error
		chunk, err = UnmarshalChunk(data)
		return err
	})
	return chunk, err
}

// GetChunksByDocument retrieves all chunks for a document, ordered by position.
func (s *BoltStore) GetChunksByDocument(docID string) ([]*Chunk, error) {
	var chunks []*Chunk
	prefix := []byte(docID + "\x00")
	err := s.db.View(func(tx *goBolt.Tx) error {
		data := tx.Bucket([]byte(chunksBucket))
		c := tx.Bucket([]byte(chunksByDocBucket)).Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			v := data.Get(k[len(prefix):])
			if v == nil {
				continue
			}
			chunk, err := UnmarshalChunk(v)
			if err != nil {
				continue
			}
			chunks = append(chunks, chunk)
		}
		return nil
	})
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Position < chunks[j].Position })
	return chunks, err
}

func deleteChunks(tx *goBolt.Tx, docID string) error {
	prefix := []byte(docID + "\x00")
	data := tx.Bucket([]byte(chunksBucket))
	idx := tx.Bucket([]byte(chunksByDocBucket))
	var keys [][]byte
	c := idx.Cursor()
	for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		keys = append(keys, append([]byte(nil), k...))
	}
	for _, k := range keys {
		if err := data.Delete(k[len(prefix):]); err != nil {
			return err
		}
		if err := idx.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// DeleteChunksByDocument removes all chunks for a document
func (s *BoltStore) DeleteChunksByDocument(docID string) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		return deleteChunks(tx, docID)
	})
}

// ForEachChunk calls fn for every stored chunk, in storage order.
func (s *BoltStore) ForEachChunk(fn func(*Chunk) error) error {
	return s.db.View(func(tx *goBolt.Tx) error {
		return tx.Bucket([]byte(chunksBucket)).ForEach(func(_, v []byte) error {
			c, err := UnmarshalChunk(v)
			if err != nil {
				return nil
			}
			return fn(c)
		})
	})
}

// CountDocuments returns the number of documents
func (s *BoltStore) CountDocuments() (int, error) {
	var count int
	err := s.db.View(func(tx *goBolt.Tx) error {
		count = tx.Bucket([]byte(docsByPathBucket)).Stats().KeyN
		return nil
	})
	return count, err
}

// CountChunks returns the total number of chunks
func (s *BoltStore) CountChunks() (int, error) {
	var count int
	err := s.db.View(func(tx *goBolt.Tx) error {
		count = tx.Bucket([]byte(chunksBucket)).Stats().KeyN
		return nil
	})
	return count, err
}

// Size returns the size of the database file in bytes.
func (s *BoltStore) Size() int64 {
	var n int64
	s.db.View(func(tx *goBolt.Tx) error {
		n = tx.Size()
		return nil
	})
	return n
}
