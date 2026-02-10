package vector

import (
	"fmt"
)

// Index is the interface for vector indexes
type Index interface {
	Add(id string, vector []float32) error
	Search(query []float32, k int) ([]SearchResult, error)
	Delete(id string) error
	Stats() map[string]int
	Save(path string) error
	Load(path string) error
}

// IndexType represents the type of vector index
type IndexType string

const (
	IndexTypeHNSW  IndexType = "hnsw"
	IndexTypeFlat  IndexType = "flat"
)

// Config holds vector index configuration
type Config struct {
	Type        IndexType
	Dimension   int
	M           int
	EfConstruct int
	MaxElements int
	Distance    string // "cosine", "euclidean", "dot"
}

// NewIndex creates a new vector index
func NewIndex(cfg Config) (Index, error) {
	switch cfg.Type {
	case IndexTypeHNSW:
		idx := NewHNSWIndex(cfg.Dimension, cfg.M, cfg.EfConstruct, cfg.MaxElements)
		
		// Set distance function
		switch cfg.Distance {
		case "euclidean":
			idx.SetDistanceFunc(EuclideanDistance)
		case "dot":
			idx.SetDistanceFunc(DotProductDistance)
		default:
			idx.SetDistanceFunc(CosineDistance)
		}
		
		return idx, nil
		
	case IndexTypeFlat:
		idx := NewFlatIndex(cfg.Dimension)
		
		switch cfg.Distance {
		case "euclidean":
			idx.SetDistanceFunc(EuclideanDistance)
		case "dot":
			idx.SetDistanceFunc(DotProductDistance)
		default:
			idx.SetDistanceFunc(CosineDistance)
		}
		
		return idx, nil
		
	default:
		return nil, fmt.Errorf("unknown index type: %s", cfg.Type)
	}
}

// FlatIndex is a simple brute-force index
type FlatIndex struct {
	vectors      map[string][]float32
	dim          int
	distanceFunc DistanceFunc
}

// NewFlatIndex creates a new flat (brute-force) index
func NewFlatIndex(dim int) *FlatIndex {
	return &FlatIndex{
		vectors:      make(map[string][]float32),
		dim:          dim,
		distanceFunc: CosineDistance,
	}
}

// SetDistanceFunc sets the distance function
func (f *FlatIndex) SetDistanceFunc(fn DistanceFunc) {
	f.distanceFunc = fn
}

// Add adds a vector
func (f *FlatIndex) Add(id string, vector []float32) error {
	if len(vector) != f.dim {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", f.dim, len(vector))
	}
	
	vec := make([]float32, len(vector))
	copy(vec, vector)
	f.vectors[id] = vec
	
	return nil
}

// Search finds k nearest neighbors (brute force)
func (f *FlatIndex) Search(query []float32, k int) ([]SearchResult, error) {
	if len(query) != f.dim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", f.dim, len(query))
	}
	
	if len(f.vectors) == 0 {
		return []SearchResult{}, nil
	}
	
	// Calculate all distances
	results := make([]SearchResult, 0, len(f.vectors))
	for id, vec := range f.vectors {
		dist := f.distanceFunc(query, vec)
		results = append(results, SearchResult{ID: id, Distance: dist})
	}
	
	// Simple selection sort for top k
	for i := 0; i < len(results)-1 && i < k; i++ {
		minIdx := i
		for j := i + 1; j < len(results); j++ {
			if results[j].Distance < results[minIdx].Distance {
				minIdx = j
			}
		}
		results[i], results[minIdx] = results[minIdx], results[i]
	}
	
	if len(results) > k {
		results = results[:k]
	}
	
	return results, nil
}

// Delete removes a vector
func (f *FlatIndex) Delete(id string) error {
	delete(f.vectors, id)
	return nil
}

// Stats returns index statistics
func (f *FlatIndex) Stats() map[string]int {
	return map[string]int{
		"elements":  len(f.vectors),
		"dimension": f.dim,
	}
}

// Save persists the index to disk
func (f *FlatIndex) Save(path string) error {
	return saveToGob(path, f.vectors)
}

// Load restores the index from disk
func (f *FlatIndex) Load(path string) error {
	var data map[string][]float32
	if err := loadFromGob(path, &data); err != nil {
		return err
	}
	f.vectors = data
	return nil
}
