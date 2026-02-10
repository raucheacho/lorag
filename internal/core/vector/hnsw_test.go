package vector

import (
	"fmt"
	"math"
	"testing"
)

func TestHNSWIndex(t *testing.T) {
	// Create index
	idx := NewHNSWIndex(128, 16, 200, 1000)

	// Add vectors
	vectors := make(map[string][]float32)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("vec_%d", i)
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = float32(i) * 0.1 + float32(j)*0.01
		}
		vectors[id] = vector

		if err := idx.Add(id, vector); err != nil {
			t.Fatalf("failed to add vector %s: %v", id, err)
		}
	}

	// Search
	query := vectors["vec_5"]
	results, err := idx.Search(query, 5)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected results, got none")
	}

	// First result should be the query itself
	if results[0].ID != "vec_5" {
		t.Errorf("expected first result to be vec_5, got %s", results[0].ID)
	}

	// Delete
	if err := idx.Delete("vec_5"); err != nil {
		t.Errorf("failed to delete vector: %v", err)
	}

	// Search again
	results, err = idx.Search(query, 5)
	if err != nil {
		t.Fatalf("failed to search after delete: %v", err)
	}

	// vec_5 should not be in results
	for _, r := range results {
		if r.ID == "vec_5" {
			t.Error("deleted vector should not appear in results")
		}
	}

	// Check stats
	stats := idx.Stats()
	if stats["elements"] != 9 {
		t.Errorf("expected 9 elements after delete, got %d", stats["elements"])
	}
}

func TestCosineDistance(t *testing.T) {
	// Same vectors should have 0 distance
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	dist := CosineDistance(a, b)
	if math.Abs(float64(dist)) > 0.0001 {
		t.Errorf("expected 0 distance for same vectors, got %f", dist)
	}

	// Orthogonal vectors should have distance 1
	c := []float32{0, 1, 0}
	dist = CosineDistance(a, c)
	if math.Abs(float64(dist-1)) > 0.0001 {
		t.Errorf("expected distance 1 for orthogonal vectors, got %f", dist)
	}

	// Opposite vectors should have distance 2
	d := []float32{-1, 0, 0}
	dist = CosineDistance(a, d)
	if math.Abs(float64(dist-2)) > 0.0001 {
		t.Errorf("expected distance 2 for opposite vectors, got %f", dist)
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := []float32{0, 0}
	b := []float32{3, 4}
	dist := EuclideanDistance(a, b)
	if math.Abs(float64(dist-5)) > 0.0001 {
		t.Errorf("expected distance 5, got %f", dist)
	}
}

func TestFlatIndex(t *testing.T) {
	idx := NewFlatIndex(3)

	// Add vectors
	idx.Add("a", []float32{1, 0, 0})
	idx.Add("b", []float32{0, 1, 0})
	idx.Add("c", []float32{0, 0, 1})

	// Search
	results, err := idx.Search([]float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if results[0].ID != "a" {
		t.Errorf("expected first result to be 'a', got %s", results[0].ID)
	}
}
