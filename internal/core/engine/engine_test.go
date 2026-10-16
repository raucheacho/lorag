package engine

import (
	"fmt"
	"sync"
	"testing"

	vtx "github.com/raucheacho/vectixdb/bindings/go"
)

func vec(seed, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32((seed*31+i*7)%13) - 6
	}
	return v
}

func chunk(i, dim int) Chunk {
	return Chunk{
		ID:         fmt.Sprintf("c%d", i),
		DocumentID: fmt.Sprintf("d%d", i%3),
		Content:    fmt.Sprintf("chunk number %d about topic%d", i, i%5),
		Path:       fmt.Sprintf("/notes/%d.md", i%3),
		Filename:   fmt.Sprintf("%d.md", i%3),
		Extension:  ".md",
		Position:   i,
		Vector:     vec(i, dim),
	}
}

func ids(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.ID
	}
	return out
}

func TestPutSearchReopen(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, Dim: 8, M: 8, EfConstruction: 50}
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := e.Put(chunk(i, 8)); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Remove("c3"); err != nil {
		t.Fatal(err)
	}
	if err := e.Put(Chunk{ID: "bad", Vector: []float32{1}}); err == nil {
		t.Fatal("expected dimension error")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	e, err = Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if n := e.Len(); n != 49 {
		t.Fatalf("Len after reopen = %d, want 49", n)
	}
	hits, err := e.Search(Request{Vector: vec(7, 8), K: 1})
	if err != nil || len(hits) != 1 || hits[0].ID != "c7" {
		t.Fatalf("vector search: %v %+v", err, hits)
	}
	if h := hits[0]; h.Path != "/notes/1.md" || h.Position != 7 || h.Content == "" || h.VectorRank != 1 {
		t.Fatalf("fields not returned: %+v", h)
	}
	kw, _ := e.Search(Request{Text: "topic3", K: 20})
	for _, h := range kw {
		if h.ID == "c3" {
			t.Fatal("removed chunk returned")
		}
		if h.TextRank == 0 || h.VectorRank != 0 {
			t.Fatalf("text-only ranks: %+v", h)
		}
	}
	if len(kw) != 9 {
		t.Fatalf("keyword hits = %d, want 9", len(kw))
	}
	if s := e.Stats(); s.Chunks != 49 || s.DiskBytes == 0 {
		t.Fatalf("unexpected stats %+v", s)
	}
}

func TestHybridFilter(t *testing.T) {
	e, err := Open(Config{Dir: t.TempDir(), Dim: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var all []Chunk
	for i := 0; i < 30; i++ {
		all = append(all, chunk(i, 8))
	}
	if err := e.Put(all...); err != nil {
		t.Fatal(err)
	}
	hits, err := e.Search(Request{
		Vector: vec(4, 8), Text: "topic4", K: 5,
		Filter: vtx.Eq(FieldDocumentID, vtx.Keyword("d1")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 5 {
		t.Fatalf("hits = %v", ids(hits))
	}
	for _, h := range hits {
		if h.DocumentID != "d1" {
			t.Fatalf("filter ignored: %+v", h)
		}
	}
	if hits[0].ID != "c4" || hits[0].VectorRank != 1 || hits[0].TextRank == 0 {
		t.Fatalf("hybrid top: %+v", hits[0])
	}
}

func TestReplaceIsAtomic(t *testing.T) {
	e, _ := Open(Config{Dir: t.TempDir(), Dim: 4})
	defer e.Close()
	e.Put(Chunk{ID: "a1", Content: "old", Vector: vec(1, 4)}, Chunk{ID: "a2", Content: "old", Vector: vec(2, 4)})
	if err := e.Replace([]string{"a1", "a2"}, []Chunk{{ID: "b1", Content: "new", Vector: vec(3, 4)}}); err != nil {
		t.Fatal(err)
	}
	if e.Len() != 1 {
		t.Fatalf("Len = %d", e.Len())
	}
	if c, _ := e.Get("b1"); c == nil || c.Content != "new" {
		t.Fatalf("Get = %+v", c)
	}
}

func TestConfigChangeStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	e, _ := Open(Config{Dir: dir, Dim: 4})
	e.Put(Chunk{ID: "a", Content: "alpha", Vector: []float32{1, 0, 0, 0}})
	e.Close()

	e, err := Open(Config{Dir: dir, Dim: 6})
	if err != nil {
		t.Fatal(err)
	}
	if e.Len() != 0 {
		t.Fatal("index with stale dimension was kept")
	}
	if s := e.Stats(); s.Collection != "chunks_2" {
		t.Fatalf("collection %s", s.Collection)
	}
	e.Close()

	// Same configuration again: the generation is kept.
	e, _ = Open(Config{Dir: dir, Dim: 6})
	defer e.Close()
	if s := e.Stats(); s.Collection != "chunks_2" {
		t.Fatalf("collection %s after reopen", s.Collection)
	}
}

func TestRebuild(t *testing.T) {
	dir := t.TempDir()
	e, _ := Open(Config{Dir: dir, Dim: 4})
	e.Put(Chunk{ID: "old", Content: "stale", Vector: []float32{1, 0, 0, 0}})
	err := e.Rebuild(func(yield func(Chunk) error) error {
		for i := 0; i < 10; i++ {
			if err := yield(Chunk{ID: fmt.Sprintf("n%d", i), Content: "fresh", Vector: vec(i, 4)}); err != nil {
				return err
			}
			if i == 5 {
				// A write during the rebuild reaches the new generation.
				if err := e.Put(Chunk{ID: "live", Content: "written meanwhile", Vector: vec(99, 4)}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Len() != 11 {
		t.Fatalf("Len = %d, want 11", e.Len())
	}
	if hits, _ := e.Search(Request{Text: "stale"}); len(hits) != 0 {
		t.Fatal("old entries survived rebuild")
	}
	e.Close()

	e, _ = Open(Config{Dir: dir, Dim: 4})
	defer e.Close()
	if e.Len() != 11 {
		t.Fatalf("Len after reopen = %d, want 11", e.Len())
	}
	if s := e.Stats(); s.Collection != "chunks_2" {
		t.Fatalf("collection %s", s.Collection)
	}
}

func TestRebuildFailureKeepsIndex(t *testing.T) {
	e, _ := Open(Config{Dir: t.TempDir(), Dim: 4})
	defer e.Close()
	e.Put(Chunk{ID: "keep", Content: "kept", Vector: vec(1, 4)})
	err := e.Rebuild(func(yield func(Chunk) error) error {
		yield(Chunk{ID: "x", Vector: vec(2, 4)})
		return fmt.Errorf("source failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if c, _ := e.Get("keep"); c == nil || e.Len() != 1 {
		t.Fatal("index lost after a failed rebuild")
	}
	if e.Put(Chunk{ID: "after", Vector: vec(3, 4)}) != nil || e.Len() != 2 {
		t.Fatal("writes broken after a failed rebuild")
	}
}

func TestConcurrentSearchDuringRebuild(t *testing.T) {
	e, _ := Open(Config{Dir: t.TempDir(), Dim: 8})
	defer e.Close()
	for i := 0; i < 100; i++ {
		e.Put(chunk(i, 8))
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := e.Search(Request{Vector: vec(3, 8), Text: "topic1", K: 5}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for r := 0; r < 3; r++ {
		if err := e.Rebuild(func(yield func(Chunk) error) error {
			for i := 0; i < 100; i++ {
				if err := yield(chunk(i, 8)); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
	if e.Len() != 100 {
		t.Fatalf("Len = %d", e.Len())
	}
}
