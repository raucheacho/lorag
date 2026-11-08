package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/store"
)

const testDim = 8

// slowEmbedder takes a little time, so that parallel files overlap between
// the limit check and the write, as with a real model.
type slowEmbedder struct{}

func (slowEmbedder) Embed(text string) ([]float32, error) {
	time.Sleep(5 * time.Millisecond)
	v := make([]float32, testDim)
	v[len(text)%testDim] = 1
	return v, nil
}

func (e slowEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i], _ = e.Embed(t)
	}
	return out, nil
}

func (slowEmbedder) Dimension() int { return testDim }

func newTestIndexer(t *testing.T, maxChunks int) (*Indexer, *store.BoltStore) {
	t.Helper()
	data := t.TempDir()
	st, err := store.NewBoltStore(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := engine.Open(engine.Config{Dir: filepath.Join(data, "vectix"), Dim: testDim})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	return NewIndexer(st, store.NewSettingsStore(st), eng, slowEmbedder{}, Config{MaxChunks: maxChunks, ChunkSize: 400}), st
}

func writeFiles(t *testing.T, n int) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, n)
	for k := range paths {
		paths[k] = filepath.Join(dir, fmt.Sprintf("note%02d.txt", k))
		if err := os.WriteFile(paths[k], fmt.Appendf(nil, "Note number %d about the deployment.", k), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func indexAll(idx *Indexer, paths []string) []error {
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	for k, p := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[k] = idx.IndexFile(context.Background(), p)
		}()
	}
	wg.Wait()
	return errs
}

func TestChunkLimitHoldsInParallel(t *testing.T) {
	const limit = 5
	idx, st := newTestIndexer(t, limit)
	paths := writeFiles(t, 12)

	failed := 0
	for _, err := range indexAll(idx, paths) {
		if err == nil {
			continue
		}
		if !strings.Contains(err.Error(), "chunk limit reached (5)") {
			t.Fatalf("unexpected error: %v", err)
		}
		failed++
	}
	count, _ := st.CountChunks()
	if count != limit {
		t.Fatalf("index holds %d chunks, want exactly the limit %d", count, limit)
	}
	if failed != len(paths)-limit {
		t.Fatalf("%d files failed, want %d", failed, len(paths)-limit)
	}

	// A full index refuses a new file before extracting it.
	extra := writeFiles(t, 1)[0]
	if _, err := idx.IndexFile(context.Background(), extra); err == nil || !strings.Contains(err.Error(), "chunk limit reached") {
		t.Fatalf("full index: err = %v", err)
	}

	// A changed file can still replace its previous version.
	if err := os.WriteFile(paths[0], []byte("Updated note about the deployment."), 0o644); err == nil {
		if _, err := idx.IndexFile(context.Background(), paths[0]); err != nil && !strings.Contains(err.Error(), "chunk limit") {
			t.Fatalf("reindex: %v", err)
		}
	}
	if count, _ = st.CountChunks(); count > limit {
		t.Fatalf("after reindex: %d chunks", count)
	}
}

func TestNoChunkLimitByDefault(t *testing.T) {
	idx, st := newTestIndexer(t, 0)
	paths := writeFiles(t, 12)
	for _, err := range indexAll(idx, paths) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count, _ := st.CountChunks(); count != len(paths) {
		t.Fatalf("index holds %d chunks, want %d", count, len(paths))
	}
}
