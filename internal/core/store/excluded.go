package store

import (
	"encoding/json"
	"sort"
	"time"

	goBolt "go.etcd.io/bbolt"
)

// excludedBucket holds the files the user removed from the store: a watched
// folder or a directory ingest skips them, so a document ingested by mistake
// stays out. Indexing one of them by its own path brings it back.
const excludedBucket = "excluded_paths" // path -> ExcludedPath (JSON)

// ExcludedPath is a file kept out of the index.
type ExcludedPath struct {
	Path       string    `json:"path"`
	ExcludedAt time.Time `json:"excluded_at"`
}

// ExcludePaths keeps these files out of later directory ingests and watches.
func (s *BoltStore) ExcludePaths(paths ...string) error {
	now := time.Now().UTC()
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(excludedBucket))
		for _, p := range paths {
			data, _ := json.Marshal(ExcludedPath{Path: p, ExcludedAt: now})
			if err := b.Put([]byte(p), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// IncludePaths lets these files be indexed again.
func (s *BoltStore) IncludePaths(paths ...string) error {
	return s.db.Update(func(tx *goBolt.Tx) error {
		b := tx.Bucket([]byte(excludedBucket))
		for _, p := range paths {
			if err := b.Delete([]byte(p)); err != nil {
				return err
			}
		}
		return nil
	})
}

// IsExcluded reports whether a file was removed by the user.
func (s *BoltStore) IsExcluded(path string) bool {
	found := false
	_ = s.db.View(func(tx *goBolt.Tx) error {
		found = tx.Bucket([]byte(excludedBucket)).Get([]byte(path)) != nil
		return nil
	})
	return found
}

// ListExcluded returns the excluded files, most recent first.
func (s *BoltStore) ListExcluded() ([]ExcludedPath, error) {
	out := []ExcludedPath{}
	err := s.db.View(func(tx *goBolt.Tx) error {
		return tx.Bucket([]byte(excludedBucket)).ForEach(func(k, v []byte) error {
			var e ExcludedPath
			if json.Unmarshal(v, &e) != nil {
				e = ExcludedPath{Path: string(k)}
			}
			out = append(out, e)
			return nil
		})
	})
	sort.Slice(out, func(a, b int) bool {
		if !out[a].ExcludedAt.Equal(out[b].ExcludedAt) {
			return out[a].ExcludedAt.After(out[b].ExcludedAt)
		}
		return out[a].Path < out[b].Path
	})
	return out, err
}
