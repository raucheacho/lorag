package query

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/core/vector"
	"github.com/lorag/lorag/internal/embeddings"
)

// Engine handles search queries
type Engine struct {
	store     *store.BoltStore
	vectorIdx vector.Index
	embedder  embeddings.Provider
}

// Result represents a search result
type Result struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	Source     *Source `json:"source"`
}

// Source contains source document info
type Source struct {
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Position  int    `json:"position"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}

// Query represents a search query
type Query struct {
	Text       string   `json:"text"`
	TopK       int      `json:"top_k"`
	Filters    []Filter `json:"filters,omitempty"`
	MinScore   float32  `json:"min_score,omitempty"`
}

// Filter represents a search filter
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// NewEngine creates a new query engine
func NewEngine(
	st *store.BoltStore,
	vectorIdx vector.Index,
	embedder embeddings.Provider,
) *Engine {
	return &Engine{
		store:     st,
		vectorIdx: vectorIdx,
		embedder:  embedder,
	}
}

// Search performs a semantic search
func (e *Engine) Search(ctx context.Context, q Query) ([]Result, error) {
	if q.TopK <= 0 {
		q.TopK = 10
	}
	
	// Generate query embedding
	queryEmbedding, err := e.embedder.Embed(q.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	
	// Search vector index
	vectorResults, err := e.vectorIdx.Search(queryEmbedding, q.TopK*2) // Get more for filtering
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	
	// Convert to results
	var results []Result
	for _, vr := range vectorResults {
		// Convert distance to score (0-1, higher is better)
		score := 1.0 - vr.Distance
		
		if q.MinScore > 0 && score < q.MinScore {
			continue
		}
		
		// Get chunk
		chunk, err := e.store.GetChunk(vr.ID)
		if err != nil {
			continue // Skip missing chunks
		}
		
		// Get document
		doc, err := e.store.GetDocument(chunk.DocumentID)
		if err != nil {
			continue
		}
		
		// Apply filters
		if !e.matchesFilters(doc, q.Filters) {
			continue
		}
		
		result := Result{
			ChunkID:    chunk.ID,
			DocumentID: chunk.DocumentID,
			Content:    chunk.Content,
			Score:      score,
			Source: &Source{
				Path:      doc.Path,
				Filename:  doc.Filename,
				Position:  chunk.Position,
				StartByte: chunk.StartByte,
				EndByte:   chunk.EndByte,
			},
		}
		
		results = append(results, result)
		
		if len(results) >= q.TopK {
			break
		}
	}
	
	return results, nil
}

// SearchWithContext performs a search and returns context around matches
func (e *Engine) SearchWithContext(ctx context.Context, q Query, contextChunks int) ([]Result, error) {
	results, err := e.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	
	if contextChunks <= 0 {
		return results, nil
	}
	
	// Add context chunks
	var enrichedResults []Result
	for _, result := range results {
		// Get adjacent chunks
		siblings, err := e.getContextChunks(result.DocumentID, result.Source.Position, contextChunks)
		if err != nil {
			continue
		}
		
		// Combine content
		var parts []string
		for _, s := range siblings {
			parts = append(parts, s.Content)
		}
		
		enriched := result
		enriched.Content = strings.Join(parts, "\n\n---\n\n")
		enrichedResults = append(enrichedResults, enriched)
	}
	
	return enrichedResults, nil
}

// GetDocument retrieves a document by ID
func (e *Engine) GetDocument(docID string) (*store.Document, error) {
	return e.store.GetDocument(docID)
}

// GetDocumentChunks retrieves all chunks for a document
func (e *Engine) GetDocumentChunks(docID string) ([]*store.Chunk, error) {
	return e.store.GetChunksByDocument(docID)
}

// getContextChunks gets chunks around a position
func (e *Engine) getContextChunks(docID string, position, context int) ([]*store.Chunk, error) {
	chunks, err := e.store.GetChunksByDocument(docID)
	if err != nil {
		return nil, err
	}
	
	// Sort by position
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Position < chunks[j].Position
	})
	
	// Find the chunk and get context
	var result []*store.Chunk
	for i, chunk := range chunks {
		if chunk.Position == position {
			// Get before
			start := i - context
			if start < 0 {
				start = 0
			}
			
			// Get after
			end := i + context + 1
			if end > len(chunks) {
				end = len(chunks)
			}
			
			result = chunks[start:end]
			break
		}
	}
	
	return result, nil
}

// matchesFilters checks if a document matches all filters
func (e *Engine) matchesFilters(doc *store.Document, filters []Filter) bool {
	if len(filters) == 0 {
		return true
	}
	
	for _, f := range filters {
		value := ""
		switch f.Field {
		case "path":
			value = doc.Path
		case "filename":
			value = doc.Filename
		case "extension":
			value = doc.Extension
		default:
			if v, ok := doc.Metadata[f.Field]; ok {
				value = v
			}
		}
		
		match := false
		switch f.Operator {
		case "eq", "=":
			match = value == f.Value
		case "contains":
			match = strings.Contains(value, f.Value)
		case "startswith":
			match = strings.HasPrefix(value, f.Value)
		case "endswith":
			match = strings.HasSuffix(value, f.Value)
		}
		
		if !match {
			return false
		}
	}
	
	return true
}

// GeneratePrompt generates a prompt with search results
func (e *Engine) GeneratePrompt(query string, results []Result, template string) string {
	if template == "" {
		template = "Based on the following context:\n\n{{context}}\n\nAnswer the question: {{query}}"
	}
	
	// Build context
	var contextParts []string
	for i, r := range results {
		contextParts = append(contextParts, fmt.Sprintf(
			"[%d] Source: %s (Score: %.3f)\n%s",
			i+1, r.Source.Filename, r.Score, r.Content,
		))
	}
	context := strings.Join(contextParts, "\n\n")
	
	// Replace placeholders
	prompt := strings.ReplaceAll(template, "{{context}}", context)
	prompt = strings.ReplaceAll(prompt, "{{query}}", query)
	
	return prompt
}
