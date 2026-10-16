package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/embeddings"
	"github.com/lorag/lorag/internal/parsers"
)

// Indexer handles document indexing
type Indexer struct {
	store         *store.BoltStore
	settingsStore *store.SettingsStore
	index         *engine.Engine
	embedder      embeddings.Provider
	parserReg     *parsers.Registry
	chunker       *Chunker
	maxChunks     int
	queue         *Queue // Async queue for parallel indexing

	eventsMu sync.Mutex
	events   []Event // most recent last
}

// Event records the outcome of indexing one file.
type Event struct {
	Time       time.Time `json:"time"`
	Path       string    `json:"path"`
	DocumentID string    `json:"document_id,omitempty"`
	Status     string    `json:"status"` // indexed, unchanged, failed
	Chunks     int       `json:"chunks"`
	DurationMs float64   `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}

const maxEvents = 200

func (i *Indexer) record(ev Event) {
	i.eventsMu.Lock()
	defer i.eventsMu.Unlock()
	i.events = append(i.events, ev)
	if len(i.events) > maxEvents {
		i.events = i.events[len(i.events)-maxEvents:]
	}
}

// RecentEvents returns up to limit indexing events, newest first.
func (i *Indexer) RecentEvents(limit int) []Event {
	i.eventsMu.Lock()
	defer i.eventsMu.Unlock()
	out := make([]Event, 0, min(limit, len(i.events)))
	for j := len(i.events) - 1; j >= 0 && len(out) < limit; j-- {
		out = append(out, i.events[j])
	}
	return out
}

// Config holds indexer configuration
type Config struct {
	MaxChunks    int
	ChunkSize    int
	ChunkOverlap int
	Workers      int // Number of concurrent workers
	MaxQueue     int // Max queue size
}

// NewIndexer creates a new indexer
func NewIndexer(
	st *store.BoltStore,
	settings *store.SettingsStore,
	index *engine.Engine,
	embedder embeddings.Provider,
	cfg Config,
) *Indexer {
	// Load parser config from settings
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".lorag")
	
	parserCfg := parsers.ParserConfig{
		OcrEnabled:           false,
		OcrModel:             "llava:7b",
		OllamaURL:            "http://localhost:11434",
		TranscriptionEnabled: false,
		WhisperModel:         parsers.WhisperTiny,
		DataDir:              dataDir,
	}
	
	if settings != nil {
		if appSettings, err := settings.GetAppSettings(); err == nil {
			parserCfg.OcrEnabled = appSettings.OcrEnabled
			parserCfg.OcrModel = appSettings.OcrModel
			parserCfg.OllamaURL = appSettings.EmbeddingURL
			parserCfg.TranscriptionEnabled = appSettings.TranscriptionEnabled
			// Parse whisper model from string
			switch appSettings.WhisperModel {
			case "base":
				parserCfg.WhisperModel = parsers.WhisperBase
			case "small":
				parserCfg.WhisperModel = parsers.WhisperSmall
			default:
				parserCfg.WhisperModel = parsers.WhisperTiny
			}
		}
	}

	idx := &Indexer{
		store:         st,
		settingsStore: settings,
		index:         index,
		embedder:      embedder,
		parserReg:     parsers.NewRegistryWithConfig(parserCfg),
		chunker:       NewChunker(cfg.ChunkSize, cfg.ChunkOverlap),
		maxChunks:     cfg.MaxChunks,
	}

	// Initialize queue if workers configured
	if cfg.Workers > 0 {
		idx.queue = NewQueue(idx, cfg.Workers, cfg.MaxQueue)
	}

	return idx
}

// SubmitTask submits a file for async indexing
func (i *Indexer) SubmitTask(ctx context.Context, path string) (*Task, error) {
	if i.queue == nil {
		return nil, fmt.Errorf("async indexing not enabled")
	}
	return i.queue.Submit(ctx, path, TaskTypeFile)
}

// SubmitDirectoryTask submits a directory for async indexing
func (i *Indexer) SubmitDirectoryTask(ctx context.Context, path string) (*Task, error) {
	if i.queue == nil {
		return nil, fmt.Errorf("async indexing not enabled")
	}
	return i.queue.Submit(ctx, path, TaskTypeDirectory)
}

// GetQueueStats returns queue statistics
func (i *Indexer) GetQueueStats() map[string]interface{} {
	if i.queue == nil {
		return map[string]interface{}{"enabled": false}
	}
	stats := i.queue.GetStats()
	stats["enabled"] = true
	return stats
}

// GetQueue returns the queue (for accessing running/completed tasks)
func (i *Indexer) GetQueue() *Queue {
	return i.queue
}

// Stop gracefully stops the indexer and its queue
func (i *Indexer) Stop() {
	if i.queue != nil {
		i.queue.Stop()
	}
}

// IndexFile indexes a single file, skipping it when its checksum is unchanged.
func (i *Indexer) IndexFile(ctx context.Context, path string) (*store.Document, error) {
	start := time.Now()
	doc, fresh, err := i.indexFile(ctx, path)
	ev := Event{Time: start.UTC(), Path: path, DurationMs: float64(time.Since(start).Microseconds()) / 1000}
	switch {
	case err != nil:
		ev.Status, ev.Error = "failed", err.Error()
	case fresh:
		ev.Status, ev.DocumentID, ev.Chunks = "indexed", doc.ID, doc.ChunkCount
	default:
		ev.Status, ev.DocumentID, ev.Chunks = "unchanged", doc.ID, doc.ChunkCount
	}
	i.record(ev)
	return doc, err
}

func (i *Indexer) indexFile(ctx context.Context, path string) (*store.Document, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false, fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("path is a directory: %s", path)
	}

	checksum, err := i.calculateChecksum(path)
	if err != nil {
		return nil, false, err
	}
	existing, err := i.store.GetDocumentByPath(path)
	if err != nil {
		existing = nil
	}
	if existing != nil && existing.Checksum == checksum && existing.IndexedAt != nil {
		return existing, false, nil
	}

	segments, err := i.parserReg.Extract(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse file: %w", err)
	}
	pieces := i.chunker.ChunkSegments(segments)
	if len(pieces) == 0 {
		return nil, false, fmt.Errorf("no text found in %s", filepath.Base(path))
	}

	currentCount, _ := i.store.CountChunks()
	if existing != nil {
		currentCount -= existing.ChunkCount
	}
	if currentCount+len(pieces) > i.maxChunks {
		return nil, false, fmt.Errorf("max chunks limit reached (%d)", i.maxChunks)
	}

	// Embed everything before touching the store, so that a failure (e.g. the
	// embedding service being down) leaves the previous version intact.
	ext := strings.ToLower(filepath.Ext(path))
	doc := store.NewDocument(path, filepath.Base(path), ext, info.Size())
	doc.Checksum = checksum
	doc.Kind = string(parsers.KindOf(ext))
	chunks := make([]*store.Chunk, 0, len(pieces))
	for idx, p := range pieces {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		chunk := store.NewChunk(doc.ID, p.Text, idx, p.StartByte, p.EndByte)
		chunk.Tokens = i.chunker.CountTokens(p.Text)
		chunk.Page, chunk.LineStart, chunk.LineEnd = p.Page, p.LineStart, p.LineEnd
		chunk.StartMs, chunk.EndMs, chunk.Section = p.StartMs, p.EndMs, p.Section
		// The section (heading path, function) gives the chunk its context.
		embedText := p.Text
		if p.Section != "" && p.Section != "screen" {
			embedText = p.Section + "\n\n" + p.Text
		}
		embedding, err := i.embedder.Embed(embedText)
		if err != nil {
			return nil, false, fmt.Errorf("failed to embed chunk %d: %w", idx, err)
		}
		if len(embedding) != i.index.Dim() {
			return nil, false, fmt.Errorf("embedding model returned %d dimensions, index is configured for %d (check embedding_dim)", len(embedding), i.index.Dim())
		}
		chunk.SetEmbedding(embedding)
		chunks = append(chunks, chunk)
	}

	var stale []string
	if existing != nil {
		old, err := i.store.GetChunksByDocument(existing.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read previous version: %w", err)
		}
		for _, c := range old {
			stale = append(stale, c.ID)
		}
		if err := i.store.DeleteDocument(existing.ID); err != nil {
			return nil, false, fmt.Errorf("failed to delete previous version: %w", err)
		}
	}
	doc.MarkIndexed(len(chunks))
	if err := i.store.SaveDocument(doc); err != nil {
		return nil, false, fmt.Errorf("failed to save document: %w", err)
	}
	entries := make([]engine.Chunk, len(chunks))
	for n, chunk := range chunks {
		if err := i.store.SaveChunk(chunk); err != nil {
			i.DeleteDocument(ctx, doc.ID)
			i.index.Remove(stale...)
			return nil, false, fmt.Errorf("failed to save chunk: %w", err)
		}
		entries[n] = indexEntry(doc, chunk)
	}
	// Old chunks leave the index in the same write as the new ones arrive.
	if err := i.index.Replace(stale, entries); err != nil {
		i.DeleteDocument(ctx, doc.ID)
		i.index.Remove(stale...)
		return nil, false, fmt.Errorf("failed to index chunks: %w", err)
	}
	return doc, true, nil
}

// indexEntry is what the search index keeps of a chunk.
func indexEntry(doc *store.Document, c *store.Chunk) engine.Chunk {
	kind := doc.Kind
	if kind == "" {
		kind = string(parsers.KindOf(doc.Extension))
	}
	return engine.Chunk{
		ID:         c.ID,
		DocumentID: c.DocumentID,
		Content:    c.Content,
		Path:       doc.Path,
		Filename:   doc.Filename,
		Extension:  doc.Extension,
		Position:   c.Position,
		StartByte:  c.StartByte,
		EndByte:    c.EndByte,
		Kind:       kind,
		Page:       c.Page,
		LineStart:  c.LineStart,
		LineEnd:    c.LineEnd,
		StartMs:    c.StartMs,
		EndMs:      c.EndMs,
		Section:    c.Section,
		Vector:     c.Embedding,
	}
}

// IndexDirectory indexes all files in a directory
// If the indexer has a queue configured, files are processed in parallel
func (i *Indexer) IndexDirectory(ctx context.Context, dir string, recursive bool) (*IndexResult, error) {
	result := &IndexResult{
		StartTime: time.Now(),
	}

	// Collect all files to index
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		// Skip directories
		if info.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file type is supported
		if !i.isSupportedFile(path) {
			result.Skipped++
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return result, err
	}

	// If queue is available, use parallel processing
	if i.queue != nil && len(files) > 0 {
		return i.indexDirectoryParallel(ctx, files, result)
	}

	// Sequential processing (fallback)
	for _, path := range files {
		doc, err := i.IndexFile(ctx, path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			result.Failed++
		} else {
			if doc.IndexedAt != nil && doc.IndexedAt.After(result.StartTime) {
				result.Indexed++
			} else {
				result.Skipped++
			}
			result.TotalChunks += doc.ChunkCount
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// indexDirectoryParallel processes files in parallel using the queue
func (i *Indexer) indexDirectoryParallel(ctx context.Context, files []string, result *IndexResult) (*IndexResult, error) {
	// Submit all files to the queue
	tasks := make([]*Task, len(files))
	for idx, path := range files {
		task, err := i.queue.Submit(ctx, path, TaskTypeFile)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to submit: %v", path, err))
			result.Failed++
			continue
		}
		tasks[idx] = task
	}

	// Wait for all tasks to complete
	for idx, task := range tasks {
		if task == nil {
			continue
		}

		select {
		case taskResult := <-task.Result:
			if taskResult.Error != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", files[idx], taskResult.Error))
				result.Failed++
			} else if taskResult.Document != nil {
				if taskResult.Document.IndexedAt != nil && taskResult.Document.IndexedAt.After(result.StartTime) {
					result.Indexed++
				} else {
					result.Skipped++
				}
				result.TotalChunks += taskResult.ChunksCount
			}
		case <-ctx.Done():
			result.Errors = append(result.Errors, fmt.Sprintf("%s: context cancelled", files[idx]))
			result.Failed++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// DeleteDocument removes a document and its chunks from the store and the indexes.
func (i *Indexer) DeleteDocument(ctx context.Context, docID string) error {
	chunks, err := i.store.GetChunksByDocument(docID)
	if err != nil {
		return err
	}
	ids := make([]string, len(chunks))
	for n, chunk := range chunks {
		ids[n] = chunk.ID
	}
	if err := i.index.Remove(ids...); err != nil {
		return err
	}
	return i.store.DeleteDocument(docID)
}

// ReindexDocument reindexes a document even if its file is unchanged.
func (i *Indexer) ReindexDocument(ctx context.Context, docID string) (*store.Document, error) {
	doc, err := i.store.GetDocument(docID)
	if err != nil {
		return nil, err
	}
	if err := i.DeleteDocument(ctx, docID); err != nil {
		return nil, err
	}
	return i.IndexFile(ctx, doc.Path)
}

// Rebuild repopulates both indexes from the chunks in the store. Chunks whose
// stored embedding does not match the index dimension are re-embedded.
func (i *Indexer) Rebuild(ctx context.Context) (int, error) {
	n := 0
	// Documents are loaded first: a Bolt read transaction must not be
	// opened inside the one that iterates the chunks.
	all, err := i.store.ListDocuments(0, math.MaxInt)
	if err != nil {
		return 0, err
	}
	docs := make(map[string]*store.Document, len(all))
	for _, d := range all {
		docs[d.ID] = d
	}
	err = i.index.Rebuild(func(yield func(engine.Chunk) error) error {
		return i.store.ForEachChunk(func(c *store.Chunk) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			doc, ok := docs[c.DocumentID]
			if !ok {
				return nil // orphan chunk: its document is gone
			}
			vec := c.Embedding
			if len(vec) != i.index.Dim() {
				var err error
				if vec, err = i.embedder.Embed(c.Content); err != nil {
					return fmt.Errorf("re-embed chunk %s: %w", c.ID, err)
				}
			}
			n++
			en := indexEntry(doc, c)
			en.Vector = vec
			return yield(en)
		})
	})
	return n, err
}

// GetStats returns indexer statistics
func (i *Indexer) GetStats() map[string]interface{} {
	docCount, _ := i.store.CountDocuments()
	chunkCount, _ := i.store.CountChunks()
	return map[string]interface{}{
		"documents": docCount,
		"chunks":    chunkCount,
		"index":     i.index.Stats(),
		"maxChunks": i.maxChunks,
	}
}


// calculateChecksum calculates file checksum
func (i *Indexer) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// isSupportedFile checks if a file type is supported
func (i *Indexer) isSupportedFile(path string) bool {
	return parsers.KindOf(filepath.Ext(path)) != ""
}

// IndexResult holds the result of indexing
type IndexResult struct {
	Indexed     int
	Skipped     int
	Failed      int
	TotalChunks int
	Errors      []string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

// String returns a string representation of the result
func (r *IndexResult) String() string {
	return fmt.Sprintf(
		"Indexed: %d, Skipped: %d, Failed: %d, Total chunks: %d, Duration: %v",
		r.Indexed, r.Skipped, r.Failed, r.TotalChunks, r.Duration,
	)
}
