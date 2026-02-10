package vector

import (
	"container/heap"
	"encoding/gob"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

// HNSWIndex implements Hierarchical Navigable Small World algorithm
type HNSWIndex struct {
	mu          sync.RWMutex
	dim         int
	maxElements int
	M           int
	Mmax        int
	Mmax0       int
	efConstruct int
	efSearch    int
	
	// Multi-layer structure
	layer0      *Layer
	upperLayers []*Layer
	maxLayer    int
	enterPoint  *Node
	
	elementCount int
	distanceFunc DistanceFunc
}

// Layer represents one layer in the HNSW graph
type Layer struct {
	nodes map[string]*Node
	edges map[string][]*Edge
}

// Node represents a point in the vector space
type Node struct {
	ID        string
	Vector    []float32
	Layer     int
	MaxLayer  int
}

// Edge represents a connection between nodes
type Edge struct {
	To     string
	Weight float32
}

// DistanceFunc calculates distance between vectors
type DistanceFunc func(a, b []float32) float32

// SearchResult represents a search result
type SearchResult struct {
	ID       string
	Distance float32
}

// NewHNSWIndex creates a new HNSW index
func NewHNSWIndex(dim, M, efConstruct, maxElements int) *HNSWIndex {
	Mmax := M
	Mmax0 := M * 2
	
	return &HNSWIndex{
		dim:         dim,
		maxElements: maxElements,
		M:           M,
		Mmax:        Mmax,
		Mmax0:       Mmax0,
		efConstruct: efConstruct,
		efSearch:    efConstruct,
		layer0: &Layer{
			nodes: make(map[string]*Node),
			edges: make(map[string][]*Edge),
		},
		upperLayers: make([]*Layer, 0),
		maxLayer:    0,
		distanceFunc: CosineDistance,
	}
	
}

// SetDistanceFunc sets the distance function
func (h *HNSWIndex) SetDistanceFunc(fn DistanceFunc) {
	h.distanceFunc = fn
}

// Add adds a vector to the index
func (h *HNSWIndex) Add(id string, vector []float32) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if len(vector) != h.dim {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", h.dim, len(vector))
	}
	
	if h.elementCount >= h.maxElements {
		return fmt.Errorf("index is full: max elements %d", h.maxElements)
	}
	
	// Calculate max layer for this node using exponential decay
	level := h.randomLevel()
	
	node := &Node{
		ID:       id,
		Vector:   make([]float32, len(vector)),
		Layer:    0,
		MaxLayer: level,
	}
	copy(node.Vector, vector)
	
	// Add to layer 0
	h.layer0.nodes[id] = node
	
	// If this is the first element, set as entry point
	if h.elementCount == 0 {
		h.enterPoint = node
		h.maxLayer = level
		h.elementCount++
		return nil
	}
	
	// Search for nearest neighbors
	currObj := h.enterPoint
	currDist := h.distanceFunc(vector, currObj.Vector)
	
	// Search from top layer down to layer 0
	for layer := h.maxLayer; layer > 0; layer-- {
		changed := true
		for changed {
			changed = false
			if edges, ok := h.getLayerEdges(layer)[currObj.ID]; ok {
				for _, edge := range edges {
					if neighbor, ok := h.getLayerNodes(layer)[edge.To]; ok {
						d := h.distanceFunc(vector, neighbor.Vector)
						if d < currDist {
							currDist = d
							currObj = neighbor
							changed = true
						}
					}
				}
			}
		}
	}
	
	// Connect to neighbors at each level
	for layer := min(level, h.maxLayer); layer >= 0; layer-- {
		neighbors := h.searchLayer(vector, currObj, layer, h.M)
		h.connectNeighbors(node, neighbors, layer)
	}
	
	// Update entry point if needed
	if level > h.maxLayer {
		h.maxLayer = level
		h.enterPoint = node
	}
	
	h.elementCount++
	return nil
}

// Search finds k nearest neighbors
func (h *HNSWIndex) Search(query []float32, k int) ([]SearchResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if len(query) != h.dim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", h.dim, len(query))
	}
	
	if h.elementCount == 0 {
		return []SearchResult{}, nil
	}
	
	// Search from top layer
	currObj := h.enterPoint
	currDist := h.distanceFunc(query, currObj.Vector)
	
	for layer := h.maxLayer; layer > 0; layer-- {
		changed := true
		for changed {
			changed = false
			if edges, ok := h.getLayerEdges(layer)[currObj.ID]; ok {
				for _, edge := range edges {
					if neighbor, ok := h.getLayerNodes(layer)[edge.To]; ok {
						d := h.distanceFunc(query, neighbor.Vector)
						if d < currDist {
							currDist = d
							currObj = neighbor
							changed = true
						}
					}
				}
			}
		}
	}
	
	// Search at layer 0 with efSearch
	candidates := h.searchLayer(query, currObj, 0, max(k, h.efSearch))
	
	// Return top k
	if len(candidates) > k {
		candidates = candidates[:k]
	}
	
	return candidates, nil
}

// Delete removes a vector from the index
func (h *HNSWIndex) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	node, ok := h.layer0.nodes[id]
	if !ok {
		return fmt.Errorf("node not found: %s", id)
	}
	
	// Remove from all layers
	for layer := 0; layer <= node.MaxLayer; layer++ {
		l := h.getLayer(layer)
		delete(l.nodes, id)
		delete(l.edges, id)
		
		// Remove edges pointing to this node
		for fromID, edges := range l.edges {
			var newEdges []*Edge
			for _, edge := range edges {
				if edge.To != id {
					newEdges = append(newEdges, edge)
				}
			}
			l.edges[fromID] = newEdges
		}
	}
	
	h.elementCount--
	return nil
}

// searchLayer searches for nearest neighbors in a specific layer
func (h *HNSWIndex) searchLayer(query []float32, entry *Node, layer, ef int) []SearchResult {
	visited := make(map[string]bool)
	candidates := &CandidateQueue{}
	results := &CandidateQueue{}
	
	entryDist := h.distanceFunc(query, entry.Vector)
	heap.Push(candidates, &Candidate{ID: entry.ID, Distance: entryDist})
	heap.Push(results, &Candidate{ID: entry.ID, Distance: entryDist})
	visited[entry.ID] = true
	
	for candidates.Len() > 0 {
		curr := heap.Pop(candidates).(*Candidate)
		worstResult := heap.Pop(results).(*Candidate)
		heap.Push(results, worstResult)
		
		if curr.Distance > worstResult.Distance {
			break
		}
		
		l := h.getLayer(layer)
		if edges, ok := l.edges[curr.ID]; ok {
			for _, edge := range edges {
				if visited[edge.To] {
					continue
				}
				visited[edge.To] = true
				
				if neighbor, ok := l.nodes[edge.To]; ok {
					d := h.distanceFunc(query, neighbor.Vector)
					
					if results.Len() < ef {
						heap.Push(candidates, &Candidate{ID: edge.To, Distance: d})
						heap.Push(results, &Candidate{ID: edge.To, Distance: d})
					} else if d < worstResult.Distance {
						heap.Push(candidates, &Candidate{ID: edge.To, Distance: d})
						heap.Pop(results)
						heap.Push(results, &Candidate{ID: edge.To, Distance: d})
					}
				}
			}
		}
	}
	
	// Convert results
	var res []SearchResult
	for results.Len() > 0 {
		c := heap.Pop(results).(*Candidate)
		res = append([]SearchResult{{ID: c.ID, Distance: c.Distance}}, res...)
	}
	
	return res
}

// connectNeighbors connects a node to its neighbors
func (h *HNSWIndex) connectNeighbors(node *Node, neighbors []SearchResult, layer int) {
	l := h.getLayer(layer)
	
	// Add forward edges
	for _, n := range neighbors {
		if n.ID == node.ID {
			continue
		}
		l.edges[node.ID] = append(l.edges[node.ID], &Edge{
			To:     n.ID,
			Weight: n.Distance,
		})
		
		// Add backward edge
		l.edges[n.ID] = append(l.edges[n.ID], &Edge{
			To:     node.ID,
			Weight: n.Distance,
		})
		
		// Trim edges if necessary
		if len(l.edges[n.ID]) > h.Mmax {
			l.edges[n.ID] = h.selectNeighbors(l.nodes[n.ID], l.edges[n.ID], h.Mmax)
		}
	}
}

// selectNeighbors selects the best neighbors to keep
func (h *HNSWIndex) selectNeighbors(node *Node, edges []*Edge, M int) []*Edge {
	if len(edges) <= M {
		return edges
	}
	
	// Sort by distance and keep closest M
	// Simple selection - can be improved with heuristic
	candidates := make([]*Edge, len(edges))
	copy(candidates, edges)
	
	// Simple bubble sort for small M
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Weight < candidates[i].Weight {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	
	return candidates[:M]
}

// getLayer returns the layer at the given level
func (h *HNSWIndex) getLayer(level int) *Layer {
	if level == 0 {
		return h.layer0
	}
	
	// Extend layers if needed
	for len(h.upperLayers) < level {
		h.upperLayers = append(h.upperLayers, &Layer{
			nodes: make(map[string]*Node),
			edges: make(map[string][]*Edge),
		})
	}
	
	return h.upperLayers[level-1]
}

// getLayerNodes returns nodes for a layer
func (h *HNSWIndex) getLayerNodes(layer int) map[string]*Node {
	return h.getLayer(layer).nodes
}

// getLayerEdges returns edges for a layer
func (h *HNSWIndex) getLayerEdges(layer int) map[string][]*Edge {
	return h.getLayer(layer).edges
}

// randomLevel generates a random level using exponential decay
func (h *HNSWIndex) randomLevel() int {
	// Simple implementation - use mL = 1/ln(M)
	ml := 1.0 / math.Log(float64(h.M))
	
	level := 0
	for level < h.maxLayer+1 {
		if randFloat() > ml {
			break
		}
		level++
	}
	
	return level
}

// Stats returns index statistics
func (h *HNSWIndex) Stats() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return map[string]int{
		"elements":   h.elementCount,
		"max_layer":  h.maxLayer,
		"dimension":  h.dim,
		"layer0_nodes": len(h.layer0.nodes),
	}
}

// SerializableLayer for persistence
type SerializableLayer struct {
	Nodes map[string]*Node
	Edges map[string][]*Edge
}

// SerializableHNSW for persistence
type SerializableHNSW struct {
	Dim           int
	MaxElements   int
	M             int
	Mmax          int
	Mmax0         int
	EfConstruct   int
	EfSearch      int
	Layer0        SerializableLayer
	UpperLayers   []SerializableLayer
	MaxLayer      int
	EnterPointID  string
	ElementCount  int
}

// Save persists the index to disk
func (h *HNSWIndex) Save(path string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data := SerializableHNSW{
		Dim:          h.dim,
		MaxElements:  h.maxElements,
		M:            h.M,
		Mmax:         h.Mmax,
		Mmax0:        h.Mmax0,
		EfConstruct:  h.efConstruct,
		EfSearch:     h.efSearch,
		Layer0: SerializableLayer{
			Nodes: h.layer0.nodes,
			Edges: h.layer0.edges,
		},
		UpperLayers:  make([]SerializableLayer, len(h.upperLayers)),
		MaxLayer:     h.maxLayer,
		ElementCount: h.elementCount,
	}

	for i, layer := range h.upperLayers {
		data.UpperLayers[i] = SerializableLayer{
			Nodes: layer.nodes,
			Edges: layer.edges,
		}
	}

	if h.enterPoint != nil {
		data.EnterPointID = h.enterPoint.ID
	}

	return saveToGob(path, data)
}

// Load restores the index from disk
func (h *HNSWIndex) Load(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var data SerializableHNSW
	if err := loadFromGob(path, &data); err != nil {
		return err
	}

	h.dim = data.Dim
	h.maxElements = data.MaxElements
	h.M = data.M
	h.Mmax = data.Mmax
	h.Mmax0 = data.Mmax0
	h.efConstruct = data.EfConstruct
	h.efSearch = data.EfSearch
	h.maxLayer = data.MaxLayer
	h.elementCount = data.ElementCount

	h.layer0 = &Layer{
		nodes: data.Layer0.Nodes,
		edges: data.Layer0.Edges,
	}

	h.upperLayers = make([]*Layer, len(data.UpperLayers))
	for i, sl := range data.UpperLayers {
		h.upperLayers[i] = &Layer{
			nodes: sl.Nodes,
			edges: sl.Edges,
		}
	}

	// Restore enter point reference
	if data.EnterPointID != "" && h.layer0 != nil {
		h.enterPoint = h.layer0.nodes[data.EnterPointID]
	}

	return nil
}

// CosineDistance calculates cosine distance between two vectors
func CosineDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 1.0
	}
	
	var dot, normA, normB float32
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 1.0
	}
	
	cosine := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1 - cosine
}

// EuclideanDistance calculates euclidean distance
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return math.MaxFloat32
	}
	
	var sum float32
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	
	return float32(math.Sqrt(float64(sum)))
}

// DotProductDistance calculates negative dot product (for similarity)
func DotProductDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dot float32
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
	}
	
	return -dot // Negative because we want smaller distance = more similar
}

// Helper types for priority queue
type Candidate struct {
	ID       string
	Distance float32
}

type CandidateQueue []*Candidate

func (cq CandidateQueue) Len() int           { return len(cq) }
func (cq CandidateQueue) Less(i, j int) bool { return cq[i].Distance < cq[j].Distance }
func (cq CandidateQueue) Swap(i, j int)      { cq[i], cq[j] = cq[j], cq[i] }

func (cq *CandidateQueue) Push(x interface{}) {
	*cq = append(*cq, x.(*Candidate))
}

func (cq *CandidateQueue) Pop() interface{} {
	old := *cq
	n := len(old)
	item := old[n-1]
	*cq = old[:n-1]
	return item
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randFloat() float64 {
	// Returns random float between 0 and 1
	return rand.Float64()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// saveToGob saves data to a file using gob encoding
func saveToGob(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}
	return nil
}

// loadFromGob loads data from a file using gob decoding
func loadFromGob(path string, data interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(data); err != nil {
		return fmt.Errorf("failed to decode data: %w", err)
	}
	return nil
}
