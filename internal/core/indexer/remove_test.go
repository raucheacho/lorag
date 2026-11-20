package indexer

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRemoveAndExcludeDocuments(t *testing.T) {
	ctx := context.Background()
	idx, st := newTestIndexer(t, 0)
	paths := writeFiles(t, 3)
	dir := filepath.Dir(paths[0])
	if _, err := idx.IndexDirectory(ctx, dir, false); err != nil {
		t.Fatal(err)
	}
	id := func(p string) string {
		d, err := st.GetDocumentByPath(p)
		if err != nil {
			t.Fatalf("%s not indexed: %v", p, err)
		}
		return d.ID
	}

	// Delete: out of Lorag, but a directory ingest brings it back.
	removed, failed := idx.RemoveDocuments(ctx, []string{id(paths[0])}, false)
	if len(removed) != 1 || len(failed) != 0 || idx.Excluded(paths[0]) {
		t.Fatalf("delete: removed %v failed %v excluded %v", removed, failed, idx.Excluded(paths[0]))
	}
	if _, err := st.GetDocumentByPath(paths[0]); err == nil {
		t.Fatal("deleted document still in the store")
	}

	// Exclude: out of Lorag and skipped by directory ingests.
	removed, failed = idx.RemoveDocuments(ctx, []string{id(paths[1]), "missing-id"}, true)
	if len(removed) != 1 || failed["missing-id"] == "" || !idx.Excluded(paths[1]) {
		t.Fatalf("exclude: removed %v failed %v", removed, failed)
	}
	// A relative or unclean form of the path is the same file.
	if !idx.Excluded(filepath.Join(dir, ".", filepath.Base(paths[1]))) {
		t.Fatal("exclusion depends on the path's spelling")
	}

	if _, err := idx.IndexDirectory(ctx, dir, false); err != nil {
		t.Fatal(err)
	}
	id(paths[0]) // deleted only: back
	if _, err := st.GetDocumentByPath(paths[1]); err == nil {
		t.Fatal("excluded file indexed again by a directory ingest")
	}
	if n, _ := st.CountDocuments(); n != 2 {
		t.Fatalf("%d documents, want 2", n)
	}

	// Include: indexed again by the next directory ingest.
	if err := idx.Include(paths[1]); err != nil {
		t.Fatal(err)
	}
	if list, _ := st.ListExcluded(); len(list) != 0 {
		t.Fatalf("still excluded: %v", list)
	}
	if _, err := idx.IndexDirectory(ctx, dir, false); err != nil {
		t.Fatal(err)
	}
	id(paths[1])
}
